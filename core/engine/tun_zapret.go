package engine

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"sirin/engine/zapret"
)

const (
	// Диапазон эфемерных портов для внутренних активных TCP-соединений через VPN-фоллбэк (dialUpstreamTCP),
	// чтобы netToTun безошибочно отличал их от обычных сокетов ОС.
	upstreamProxyPortMin = 61000
	upstreamProxyPortMax = 64999

	tunTCPMaxSeg     = 1200
	tunTCPWindowSize = 65535
)

// recomputeIPv4Checksums пересчитывает контрольные суммы заголовка IPv4 и вложенного TCP/UDP сегмента.
func recomputeIPv4Checksums(pkt []byte) {
	if len(pkt) < 20 || pkt[0]>>4 != 4 {
		return
	}
	ihl := int(pkt[0]&0x0f) * 4
	if ihl < 20 || len(pkt) < ihl {
		return
	}
	totalLen := int(binary.BigEndian.Uint16(pkt[2:4]))
	if totalLen > len(pkt) || totalLen < ihl {
		totalLen = len(pkt)
	}

	// 1. IPv4 Header Checksum
	pkt[10] = 0
	pkt[11] = 0
	var ipSum uint32
	for i := 0; i+1 < ihl; i += 2 {
		ipSum += uint32(binary.BigEndian.Uint16(pkt[i : i+2]))
	}
	for (ipSum >> 16) > 0 {
		ipSum = (ipSum & 0xffff) + (ipSum >> 16)
	}
	binary.BigEndian.PutUint16(pkt[10:12], ^uint16(ipSum))

	proto := pkt[9]
	l4 := pkt[ihl:totalLen]
	l4Len := len(l4)

	if proto == 6 && l4Len >= 20 { // TCP
		l4[16] = 0
		l4[17] = 0
		sum := pseudoHeaderSum(pkt[12:16], pkt[16:20], 6, l4Len)
		sum += checksumBytes(l4)
		for (sum >> 16) > 0 {
			sum = (sum & 0xffff) + (sum >> 16)
		}
		binary.BigEndian.PutUint16(l4[16:18], ^uint16(sum))
	} else if proto == 17 && l4Len >= 8 { // UDP
		l4[6] = 0
		l4[7] = 0
		sum := pseudoHeaderSum(pkt[12:16], pkt[16:20], 17, l4Len)
		sum += checksumBytes(l4)
		for (sum >> 16) > 0 {
			sum = (sum & 0xffff) + (sum >> 16)
		}
		cs := ^uint16(sum)
		if cs == 0 {
			cs = 0xffff
		}
		binary.BigEndian.PutUint16(l4[6:8], cs)
	}
}

func pseudoHeaderSum(srcIP, dstIP []byte, proto byte, length int) uint32 {
	var sum uint32
	sum += uint32(binary.BigEndian.Uint16(srcIP[0:2]))
	sum += uint32(binary.BigEndian.Uint16(srcIP[2:4]))
	sum += uint32(binary.BigEndian.Uint16(dstIP[0:2]))
	sum += uint32(binary.BigEndian.Uint16(dstIP[2:4]))
	sum += uint32(proto)
	sum += uint32(length)
	return sum
}

func checksumBytes(b []byte) uint32 {
	var sum uint32
	n := len(b)
	for i := 0; i+1 < n; i += 2 {
		sum += uint32(binary.BigEndian.Uint16(b[i : i+2]))
	}
	if n%2 == 1 {
		sum += uint32(b[n-1]) << 8
	}
	return sum
}

// buildIPv6TCPReset строит IPv6 TCP RST в ответ на входящий IPv6 TCP пакет.
// Используется для мгновенного отклонения IPv6-соединений: движок не поддерживает IPv6 на VPS,
// поэтому вместо тихого дропа (→ таймаут Happy Eyeballs 2–5 сек) шлём RST,
// и приложения моментально переходят на IPv4.
func buildIPv6TCPReset(origPkt []byte) []byte {
	// IPv6 header = 40 bytes, TCP header min = 20 bytes
	if len(origPkt) < 60 || origPkt[0]>>4 != 6 || origPkt[6] != 6 {
		return nil
	}
	srcIP := origPkt[8:24]
	dstIP := origPkt[24:40]
	l4 := origPkt[40:]
	if len(l4) < 20 {
		return nil
	}
	srcPort := l4[0:2]
	dstPort := l4[2:4]
	seqNum := binary.BigEndian.Uint32(l4[4:8])
	ackNum := binary.BigEndian.Uint32(l4[8:12])
	flags := l4[13]

	// RST ответ: меняем src/dst местами
	const totalLen = 40 + 20
	pkt := make([]byte, totalLen)

	// IPv6 header
	pkt[0] = 0x60 // version=6, traffic class=0
	// payload length = 20 (TCP header)
	binary.BigEndian.PutUint16(pkt[4:6], 20)
	pkt[6] = 6  // next header = TCP
	pkt[7] = 64 // hop limit
	copy(pkt[8:24], dstIP)
	copy(pkt[24:40], srcIP)

	// TCP header
	tcp := pkt[40:]
	copy(tcp[0:2], dstPort)
	copy(tcp[2:4], srcPort)

	// seq/ack для RST
	var rstSeq, rstAck uint32
	if flags&0x10 != 0 { // ACK set → seq = peer's ack
		rstSeq = ackNum
	} else {
		rstSeq = 0
		rstAck = seqNum + 1
		tcp[13] = 0x14 // RST|ACK
	}
	binary.BigEndian.PutUint32(tcp[4:8], rstSeq)
	binary.BigEndian.PutUint32(tcp[8:12], rstAck)
	tcp[12] = 5 << 4 // data offset = 5 (20 bytes)
	if tcp[13] == 0 {
		tcp[13] = 0x04 // RST
	}

	// TCP checksum (pseudo-header IPv6)
	var cs uint32
	for i := 0; i < 16; i += 2 {
		cs += uint32(binary.BigEndian.Uint16(pkt[8+i : 10+i]))
		cs += uint32(binary.BigEndian.Uint16(pkt[24+i : 26+i]))
	}
	cs += 6  // next header = TCP
	cs += 20 // TCP length
	for i := 0; i < 20; i += 2 {
		cs += uint32(binary.BigEndian.Uint16(tcp[i : i+2]))
	}
	for cs>>16 > 0 {
		cs = (cs & 0xffff) + (cs >> 16)
	}
	binary.BigEndian.PutUint16(tcp[16:18], ^uint16(cs))

	return pkt
}

// buildDNSEmptyAAAAResponse строит DNS-ответ NOERROR с 0 записями для AAAA-запроса.
// Используется для подавления IPv6 адресов CDN-доменов на стороне TUN:
// без этого ExoPlayer/Cronet получает AAAA ответы и пытается подключиться по IPv6,
// который движок не поддерживает → 2–5 секундный таймаут Happy Eyeballs на каждое соединение.
func buildDNSEmptyAAAAResponse(query []byte) []byte {
	if len(query) < 12 {
		return nil
	}
	resp := make([]byte, len(query))
	copy(resp, query)
	// QR=1 (response), OPCODE=0, AA=0, TC=0, RD=1, RA=1, RCODE=0 (NOERROR)
	resp[2] = (query[2] & 0x01) | 0x81 // QR=1, RD=copy, RA=1
	resp[3] = 0x80                      // RA=1, RCODE=0
	// ANCOUNT=0, NSCOUNT=0, ARCOUNT=0 (оставляем нулями)
	binary.BigEndian.PutUint16(resp[6:8], 0)
	binary.BigEndian.PutUint16(resp[8:10], 0)
	binary.BigEndian.PutUint16(resp[10:12], 0)
	return resp
}

// dnsQueryType извлекает тип запроса (QTYPE) из DNS-пакета (первый вопрос).
func dnsQueryType(query []byte) uint16 {
	if len(query) < 12 {
		return 0
	}
	pos := 12
	for pos < len(query) {
		l := int(query[pos])
		if l == 0 {
			pos++
			break
		}
		if l&0xC0 == 0xC0 {
			pos += 2
			break
		}
		pos += 1 + l
	}
	if pos+4 > len(query) {
		return 0
	}
	return binary.BigEndian.Uint16(query[pos : pos+2])
}

// dnsQueryName извлекает первое имя запроса из DNS-пакета.
func dnsQueryName(query []byte) string {
	if len(query) < 12 {
		return ""
	}
	var name []byte
	pos := 12
	for pos < len(query) {
		l := int(query[pos])
		if l == 0 {
			break
		}
		if l&0xC0 == 0xC0 {
			break
		}
		if len(name) > 0 {
			name = append(name, '.')
		}
		end := pos + 1 + l
		if end > len(query) {
			break
		}
		name = append(name, query[pos+1:end]...)
		pos = end
	}
	return string(name)
}

// dnsParseARecords извлекает все A-записи из DNS-ответа.
func dnsParseARecords(resp []byte) []net.IP {
	if len(resp) < 12 {
		return nil
	}
	ancount := int(binary.BigEndian.Uint16(resp[6:8]))
	if ancount == 0 {
		return nil
	}
	// Пропускаем секцию вопросов
	pos := 12
	qdcount := int(binary.BigEndian.Uint16(resp[4:6]))
	for i := 0; i < qdcount; i++ {
		for pos < len(resp) {
			l := int(resp[pos])
			if l == 0 {
				pos++
				break
			}
			if l&0xC0 == 0xC0 {
				pos += 2
				break
			}
			pos += 1 + l
		}
		pos += 4 // QTYPE + QCLASS
	}

	var ips []net.IP
	for i := 0; i < ancount && pos < len(resp); i++ {
		// Имя (может быть pointer или labels)
		for pos < len(resp) {
			l := int(resp[pos])
			if l == 0 {
				pos++
				break
			}
			if l&0xC0 == 0xC0 {
				pos += 2
				break
			}
			pos += 1 + l
		}
		if pos+10 > len(resp) {
			break
		}
		rtype := binary.BigEndian.Uint16(resp[pos : pos+2])
		rdlen := int(binary.BigEndian.Uint16(resp[pos+8 : pos+10]))
		pos += 10
		if pos+rdlen > len(resp) {
			break
		}
		if rtype == 1 && rdlen == 4 { // A record
			ip := make(net.IP, 4)
			copy(ip, resp[pos:pos+4])
			ips = append(ips, ip)
		}
		pos += rdlen
	}
	return ips
}

// buildICMPPortUnreachable конструирует IPv4 ICMP Type 3 Code 3 (Port Unreachable) в ответ на UDP-пакет (например, QUIC :443).
// Мгновенно переключает браузеры/приложения с QUIC на TCP :443 без ожидания таймаута.
func buildICMPPortUnreachable(origPkt []byte) []byte {
	if len(origPkt) < 28 || origPkt[0]>>4 != 4 {
		return nil
	}
	ihl := int(origPkt[0]&0x0f) * 4
	if len(origPkt) < ihl+8 {
		return nil
	}
	quotedLen := ihl + 8
	totalLen := 20 + 8 + quotedLen
	pkt := make([]byte, totalLen)

	// IPv4 header
	pkt[0] = 0x45
	binary.BigEndian.PutUint16(pkt[2:4], uint16(totalLen))
	binary.BigEndian.PutUint16(pkt[4:6], 0)
	pkt[6] = 0x40
	pkt[8] = 64
	pkt[9] = 1 // ICMP
	copy(pkt[12:16], origPkt[16:20]) // src = orig dst
	copy(pkt[16:20], origPkt[12:16]) // dst = orig src

	var ipSum uint32
	for i := 0; i < 20; i += 2 {
		ipSum += uint32(binary.BigEndian.Uint16(pkt[i : i+2]))
	}
	for (ipSum >> 16) > 0 {
		ipSum = (ipSum & 0xffff) + (ipSum >> 16)
	}
	binary.BigEndian.PutUint16(pkt[10:12], ^uint16(ipSum))

	// ICMP header (Type 3 = Destination Unreachable, Code 3 = Port Unreachable)
	icmp := pkt[20:]
	icmp[0] = 3
	icmp[1] = 3
	copy(icmp[8:], origPkt[:quotedLen])

	cs := checksumBytes(icmp)
	for (cs >> 16) > 0 {
		cs = (cs & 0xffff) + (cs >> 16)
	}
	binary.BigEndian.PutUint16(icmp[2:4], ^uint16(cs))
	return pkt
}

type tcpFlowKey struct {
	clientIP   [4]byte
	clientPort uint16
	targetIP   [4]byte
	targetPort uint16
}

type udpFlowKey struct {
	clientIP   [4]byte
	clientPort uint16
	targetIP   [4]byte
	targetPort uint16
}

type tunUDPFlow struct {
	key        udpFlowKey
	pc         net.PacketConn
	lastActive atomic.Int64
	pktsSent   atomic.Uint32
	pktsRecv   atomic.Uint32
	firstSent  time.Time
}

// tunZapret перехватывает трафик из TUN на всех платформах (Android, Windows, Linux),
// применяет техники десинхронизации Zapret 2 напрямую через защищённый WAN-сокет,
// а при блокировке домена/IP бесшовно будит ленивый VPN-туннель до VPS и переключает поток в него.
type tunZapret struct {
	s          *session
	tunWriteMu sync.Mutex

	tcpMu    sync.Mutex
	tcpConns map[tcpFlowKey]*tunTCPConn

	upMu      sync.Mutex
	upConns   map[uint16]*tunTCPConn
	nextUpPrt uint16

	udpMu    sync.Mutex
	udpFlows map[udpFlowKey]*tunUDPFlow

	dohOnce    sync.Once
	dohClients []*tunDoHClient
}

type tunDoHClient struct {
	url    string
	client *http.Client
}

func newTunZapret(s *session) *tunZapret {
	tz := &tunZapret{
		s:         s,
		tcpConns:  make(map[tcpFlowKey]*tunTCPConn),
		upConns:   make(map[uint16]*tunTCPConn),
		nextUpPrt: upstreamProxyPortMin,
		udpFlows:  make(map[udpFlowKey]*tunUDPFlow),
	}
	if zapret.GlobalEngine.Logf == nil {
		zapret.GlobalEngine.Logf = s.logf
	}
	return tz
}

func (tz *tunZapret) writeTun(pkt []byte) error {
	tz.tunWriteMu.Lock()
	defer tz.tunWriteMu.Unlock()
	if tz.s.tun == nil {
		return errors.New("tun closed")
	}
	_, err := tz.s.tun.Write(pkt)
	return err
}

func (tz *tunZapret) isBypassedDest(dstIP [4]byte) bool {
	// Внутренняя подсеть туннеля 10.77.0.0/16 или loopback
	if (dstIP[0] == 10 && dstIP[1] == 77) || dstIP[0] == 127 {
		return true
	}
	// Частные локальные сети (RFC 1918)
	if dstIP[0] == 10 || (dstIP[0] == 192 && dstIP[1] == 168) || (dstIP[0] == 172 && dstIP[1] >= 16 && dstIP[1] <= 31) {
		return true
	}
	return false
}

// handleOutbound обрабатывает пакет, прочитанный из TUN в tunToNet.
// Возвращает true, если пакет обработан движком Zapret 2 локально (не нужно слать в VPS).
// Возвращает false, если пакет должен идти через VPN-туннель на VPS.
func (tz *tunZapret) handleOutbound(pkt []byte) bool {
	// IPv6: движок не поддерживает IPv6 на VPS (нет маршрутизации/NAT6 на сервере).
	// Вместо тихого дропа (→ 2–5 сек таймаут Happy Eyeballs) немедленно шлём TCP RST
	// или ICMPv6 Destination Unreachable, чтобы приложения мгновенно перешли на IPv4.
	if len(pkt) >= 40 && pkt[0]>>4 == 6 {
		if pkt[6] == 6 && len(pkt) >= 60 { // IPv6+TCP
			if rst := buildIPv6TCPReset(pkt); rst != nil {
				_ = tz.writeTun(rst)
			}
		}
		// Для UDP/ICMP6 — просто дропаем (нет смысла слать в VPS без v6 NAT)
		return true
	}

	if len(pkt) < 20 || pkt[0]>>4 != 4 {
		return false
	}
	ihl := int(pkt[0]&0x0f) * 4
	totalLen := int(binary.BigEndian.Uint16(pkt[2:4]))
	if ihl < 20 || totalLen < ihl || totalLen > len(pkt) {
		return false
	}
	// Фрагментированные IP-пакеты пропускаем в обычный путь
	if binary.BigEndian.Uint16(pkt[6:8])&0x3fff != 0 {
		return false
	}

	var srcIP, dstIP [4]byte
	copy(srcIP[:], pkt[12:16])
	copy(dstIP[:], pkt[16:20])

	if tz.isBypassedDest(dstIP) {
		return false
	}

	proto := pkt[9]
	l4 := pkt[ihl:totalLen]

	// 1. Обработка TCP
	if proto == 6 {
		if len(l4) < 20 {
			return false
		}
		srcPort := binary.BigEndian.Uint16(l4[0:2])
		dstPort := binary.BigEndian.Uint16(l4[2:4])
		key := tcpFlowKey{
			clientIP:   srcIP,
			clientPort: srcPort,
			targetIP:   dstIP,
			targetPort: dstPort,
		}

		// Если соединение УЖЕ открыто в tunZapret — отдаём пакет ему
		tz.tcpMu.Lock()
		existing, ok := tz.tcpConns[key]
		tz.tcpMu.Unlock()
		if ok {
			existing.onPeerPacket(l4)
			return true
		}

		// Если IP уже помечен в кэше VPN-фоллбэка — НЕ перехватываем новые соединения,
		// пусть идут напрямую через L3 VPN-туннель до VPS!
		if zapret.GlobalEngine.IsIPFallbackActive(net.IP(dstIP[:])) {
			return false
		}

		// Перехватываем новые исходящие TCP-соединения на порты 80 и 443 (и любые другие порты, пока VPS в спящем режиме)
		flags := l4[13]
		isSYN := (flags&0x02 != 0) && (flags&0x10 == 0)
		if !isSYN {
			// Если VPS уже поднят — пробрасываем пакет в него; иначе отвечаем RST, чтобы ОС переоткрыла соединение
			if tz.s.hasActiveUpstream() {
				return false
			}
			return true
		}

		if dstPort != 80 && dstPort != 443 && tz.s.hasActiveUpstream() {
			return false
		}

		seq := binary.BigEndian.Uint32(l4[4:8])
		conn := newPassiveTunTCPConn(tz, key, seq)
		tz.tcpMu.Lock()
		tz.tcpConns[key] = conn
		tz.tcpMu.Unlock()

		conn.sendSynAck()
		go tz.serveInterceptedTCP(conn)
		return true
	}

	// 2. Обработка UDP
	if proto == 17 {
		if len(l4) < 8 {
			return false
		}
		srcPort := binary.BigEndian.Uint16(l4[0:2])
		dstPort := binary.BigEndian.Uint16(l4[2:4])

		// Если целевой IP уже в VPN-фоллбэке — отправляем через VPS
		if zapret.GlobalEngine.IsIPFallbackActive(net.IP(dstIP[:])) {
			return false
		}

		// QUIC (UDP :443): отправляем мгновенный ICMP Port Unreachable, чтобы браузер/YouTube/Instagram
		// без задержки перешёл на TCP :443, где работает разбиение TLS SNI и автоподбор стратегий Zapret 2.
		if dstPort == 443 {
			if icmp := buildICMPPortUnreachable(pkt[:totalLen]); icmp != nil {
				_ = tz.writeTun(icmp)
			}
			return true
		}

		// DNS (UDP :53): если VPS-туннель уже поднят — отправляем через него (return false).
		// Если VPS спит или недоступен (lazyMode) — проксируем через DoH (1.1.1.1 / 8.8.8.8),
		// чтобы провайдер/ТСПУ не подменял ответы и DNS работал даже без доступа к VPS.
		if dstPort == 53 {
			if tz.s != nil && tz.s.hasActiveUpstream() {
				return false
			}
			dnsReq := append([]byte(nil), l4[8:]...)
			if len(dnsReq) > 0 {
				go tz.handleOutboundDNS(srcIP, srcPort, dstIP, dstPort, dnsReq)
				return true
			}
			return false
		}

		// Остальной UDP (NTP :123, STUN / Discord Voice и т.д.) при спящем VPS
		// отправляем напрямую через защищённый WAN UDP-сокет с техникой Zapret 2 UDP Fake Burst!
		payload := l4[8:]
		if tz.handleOutboundUDP(srcIP, srcPort, dstIP, dstPort, payload) {
			return true
		}
		return false
	}

	return false
}

func (tz *tunZapret) handleOutboundUDP(srcIP [4]byte, srcPort uint16, dstIP [4]byte, dstPort uint16, payload []byte) bool {
	key := udpFlowKey{
		clientIP:   srcIP,
		clientPort: srcPort,
		targetIP:   dstIP,
		targetPort: dstPort,
	}

	tz.udpMu.Lock()
	flow, ok := tz.udpFlows[key]
	if !ok {
		pc, err := tz.s.listenProtected(tz.s.ctx, tz.s.prot, "", "zapret-udp")
		if err != nil {
			tz.udpMu.Unlock()
			return false
		}
		flow = &tunUDPFlow{
			key:       key,
			pc:        pc,
			firstSent: time.Now(),
		}
		flow.lastActive.Store(time.Now().UnixNano())
		tz.udpFlows[key] = flow
		tz.udpMu.Unlock()
		go tz.readLoopUDP(flow)
	} else {
		tz.udpMu.Unlock()
		flow.lastActive.Store(time.Now().UnixNano())
	}

	dstAddr := &net.UDPAddr{IP: net.IP(dstIP[:]), Port: int(dstPort)}
	sent := flow.pktsSent.Add(1)

	// На первом пакете не-DNS потока отправляем Zapret 2 UDP Fake Burst (IP_TTL=3) для обхода DPI (Discord Voice / STUN)
	if sent == 1 && dstPort != 53 && dstPort != 123 {
		_, _ = zapret.SendUDPWithFakeBurst(flow.pc, dstAddr, payload)
	} else {
		_, _ = flow.pc.WriteTo(payload, dstAddr)
	}

	// Если по не-DNS UDP потоку ушло >= 4 пакетов за > 1.5с, а ответов 0 — переключаем IP на VPN
	if dstPort != 53 && dstPort != 123 && sent >= 4 && flow.pktsRecv.Load() == 0 && time.Since(flow.firstSent) > 1500*time.Millisecond {
		ipStr := net.IP(dstIP[:]).String()
		zapret.GlobalEngine.RecordFailure("", ipStr, "")
		go func() {
			_ = tz.s.ensureUpstream()
		}()
		return false
	}
	return true
}

func (tz *tunZapret) readLoopUDP(flow *tunUDPFlow) {
	defer func() {
		_ = flow.pc.Close()
		tz.udpMu.Lock()
		delete(tz.udpFlows, flow.key)
		tz.udpMu.Unlock()
	}()

	buf := make([]byte, 2048)
	for {
		_ = flow.pc.SetReadDeadline(time.Now().Add(45 * time.Second))
		n, _, err := flow.pc.ReadFrom(buf)
		if err != nil {
			return
		}
		if n == 0 {
			continue
		}
		flow.pktsRecv.Add(1)
		flow.lastActive.Store(time.Now().UnixNano())

		pkt := buildUDPIPv4Packet(flow.key.targetIP, flow.key.targetPort, flow.key.clientIP, flow.key.clientPort, buf[:n])
		if pkt == nil {
			continue
		}
		_ = tz.writeTun(pkt)
	}
}

func buildUDPIPv4Packet(srcIP [4]byte, srcPort uint16, dstIP [4]byte, dstPort uint16, payload []byte) []byte {
	totalLen := 20 + 8 + len(payload)
	if totalLen > tunMTU+80 {
		return nil
	}
	pkt := make([]byte, totalLen)
	pkt[0] = 0x45
	binary.BigEndian.PutUint16(pkt[2:4], uint16(totalLen))
	pkt[6] = 0x40
	pkt[8] = 64
	pkt[9] = 17 // UDP
	copy(pkt[12:16], srcIP[:])
	copy(pkt[16:20], dstIP[:])

	udp := pkt[20:]
	binary.BigEndian.PutUint16(udp[0:2], srcPort)
	binary.BigEndian.PutUint16(udp[2:4], dstPort)
	binary.BigEndian.PutUint16(udp[4:6], uint16(8+len(payload)))
	copy(udp[8:], payload)

	recomputeIPv4Checksums(pkt)
	return pkt
}

func (tz *tunZapret) initDoHClients() {
	tz.dohOnce.Do(func() {
		providers := []dohProvider{
			{"1.1.1.1", "cloudflare-dns.com"},
			{"8.8.8.8", "dns.google"},
		}
		var prot Protector
		var logf func(string, ...interface{})
		if tz.s != nil {
			prot = tz.s.prot
			logf = tz.s.logf
		}
		if logf == nil {
			logf = func(string, ...interface{}) {}
		}
		for _, p := range providers {
			tr := &http.Transport{
				DialContext: (&net.Dialer{
					Timeout:   2 * time.Second,
					KeepAlive: 30 * time.Second,
					Control:   protectDialControl(prot, logf, "zapret-doh"),
				}).DialContext,
				TLSClientConfig:     &tls.Config{ServerName: p.sni},
				ForceAttemptHTTP2:   true,
				MaxIdleConns:        4,
				IdleConnTimeout:     60 * time.Second,
				TLSHandshakeTimeout: 2 * time.Second,
			}
			tz.dohClients = append(tz.dohClients, &tunDoHClient{
				url:    "https://" + p.ip + "/dns-query",
				client: &http.Client{Timeout: 3 * time.Second, Transport: tr},
			})
		}
	})
}

func (tz *tunZapret) handleOutboundDNS(srcIP [4]byte, srcPort uint16, dstIP [4]byte, dstPort uint16, query []byte) {
	if tz.s == nil {
		return
	}

	// ФИЛЬТР AAAA: если запрос — Type AAAA (28) для CDN-домена (или вообще для любого домена
	// из DefaultBaseDomains), немедленно возвращаем пустой NOERROR-ответ.
	// Это устраняет 2–5 сек задержки Happy Eyeballs без отключения IPv6 в системе.
	if dnsQueryType(query) == 28 { // AAAA
		if empty := buildDNSEmptyAAAAResponse(query); empty != nil {
			if pkt := buildUDPIPv4Packet(dstIP, dstPort, srcIP, srcPort, empty); pkt != nil {
				_ = tz.writeTun(pkt)
			}
		}
		return
	}

	tz.initDoHClients()

	var respBytes []byte
	for _, dc := range tz.dohClients {
		ctx, cancel := context.WithTimeout(tz.s.ctx, 2500*time.Millisecond)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, dc.url, bytes.NewReader(query))
		if err != nil {
			cancel()
			continue
		}
		req.Header.Set("Content-Type", "application/dns-message")
		req.Header.Set("Accept", "application/dns-message")

		resp, err := dc.client.Do(req)
		if err != nil {
			cancel()
			continue
		}
		body, rerr := io.ReadAll(io.LimitReader(resp.Body, 1400))
		_ = resp.Body.Close()
		cancel()
		if rerr == nil && resp.StatusCode == http.StatusOK && len(body) >= 12 {
			respBytes = body
			break
		}
	}

	// Запасной путь: защищённый UDP :53 на 1.1.1.1, если DoH не ответил.
	if len(respBytes) == 0 {
		pc, err := tz.s.listenProtected(tz.s.ctx, tz.s.prot, "", "zapret-dns-udp")
		if err == nil {
			defer pc.Close()
			_ = pc.SetDeadline(time.Now().Add(1500 * time.Millisecond))
			if _, werr := pc.WriteTo(query, &net.UDPAddr{IP: net.IPv4(1, 1, 1, 1), Port: 53}); werr == nil {
				buf := make([]byte, 1400)
				if n, _, rerr := pc.ReadFrom(buf); rerr == nil && n >= 12 {
					respBytes = buf[:n]
				}
			}
		}
	}

	if len(respBytes) == 0 {
		return
	}

	// DNS IP SNOOPING: если для CDN-домена (googlevideo.com, youtube.com и т.п.)
	// в кэше уже активен VPN-фоллбэк (wildcard) — сразу регистрируем все разрезолвленные A-адреса
	// в кэше как VPN-цели. Это устраняет повторный цикл Zapret 2 для каждого нового CDN IP.
	if qname := dnsQueryName(query); qname != "" {
		if bd := zapret.MatchBaseDomain(qname); bd != "" {
			if ips := dnsParseARecords(respBytes); len(ips) > 0 {
				zapret.GlobalEngine.RecordDNSResolvedIPs(bd, ips)
			}
		}
	}

	if pkt := buildUDPIPv4Packet(dstIP, dstPort, srcIP, srcPort, respBytes); pkt != nil {
		_ = tz.writeTun(pkt)
	}
}

func (tz *tunZapret) serveInterceptedTCP(clientConn *tunTCPConn) {
	defer func() {
		_ = clientConn.Close()
		tz.tcpMu.Lock()
		delete(tz.tcpConns, clientConn.key)
		tz.tcpMu.Unlock()
	}()

	targetIPStr := net.IP(clientConn.key.targetIP[:]).String()
	targetAddr := net.JoinHostPort(targetIPStr, strconv.Itoa(int(clientConn.key.targetPort)))

	// Ждём первый пакет данных от приложения (TLS ClientHello или HTTP запрос)
	initBuf := make([]byte, 16*1024)
	_ = clientConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, err := clientConn.Read(initBuf)
	_ = clientConn.SetReadDeadline(time.Time{})
	if err != nil && n == 0 {
		return
	}
	clientHello := initBuf[:n]

	wanDial := func(ctx context.Context, network, addr string) (net.Conn, error) {
		d := &net.Dialer{
			Timeout:   3 * time.Second,
			KeepAlive: 30 * time.Second,
			Control:   protectControl(tz.s.prot, tz.s.logf, "zapret2-wan"),
		}
		return d.DialContext(ctx, "tcp4", addr)
	}

	// 1. Пробуем прямой обход ТСПУ через WAN с каскадом стратегий Zapret 2
	remoteConn, firstReply, desyncErr := zapret.GlobalEngine.DialAndDesync(
		tz.s.ctx,
		wanDial,
		targetAddr,
		clientHello,
		2,
		zapret.DefaultWanProbeTimeout,
	)
	if desyncErr == nil && remoteConn != nil {
		defer remoteConn.Close()
		if len(firstReply) > 0 {
			if _, werr := clientConn.Write(firstReply); werr != nil {
				return
			}
		}
		bridgeConns(clientConn, remoteConn)
		return
	}

	// 2. Прямой обход Zapret 2 не сработал (IP заблокирован ТСПУ или геоблок сервиса).
	// Бесшовно поднимаем ленивый VPN-туннель до VPS и проводим текущее соединение через него!
	sni, _ := zapret.ExtractSNI(clientHello)
	if sni == "" {
		sni, _ = zapret.ExtractHTTPHost(clientHello)
	}
	tz.s.logf("[zapret2] прямой обход не прошёл для %q (%s: %v) -> бесшовное переключение на VPS VPN",
		sni, targetAddr, desyncErr)

	if err := tz.s.ensureUpstream(); err != nil {
		tz.s.logf("[zapret2] не удалось поднять резервный VPN-туннель до VPS: %v", err)
		clientConn.sendRST()
		return
	}

	upConn, err := tz.dialUpstreamTCP(tz.s.ctx, clientConn.key.targetIP, clientConn.key.targetPort)
	if err != nil {
		// Если активный L3-прокси не успел, отправляем быстрый RST: IP уже в кэше VPN-фоллбэка,
		// и мгновенный повтор со стороны ОС/браузера пойдёт напрямую через L3 VPN-туннель!
		clientConn.sendRST()
		return
	}
	defer upConn.Close()

	if len(clientHello) > 0 {
		if _, err := upConn.Write(clientHello); err != nil {
			clientConn.sendRST()
			return
		}
	}
	bridgeConns(clientConn, upConn)
}

func bridgeConns(c1, c2 net.Conn) {
	done := make(chan struct{}, 2)
	cp := func(dst, src net.Conn) {
		buf := make([]byte, 32*1024)
		_, _ = io.CopyBuffer(dst, src, buf)
		done <- struct{}{}
	}
	go cp(c1, c2)
	go cp(c2, c1)
	<-done
	_ = c1.Close()
	_ = c2.Close()
	<-done
}

// dialUpstreamTCP открывает исходящее TCP-соединение внутри L3 VPN-туннеля (r.link)
// для бесшовного завершения первой попытки подключения, обнаружившей блокировку WAN.
func (tz *tunZapret) dialUpstreamTCP(ctx context.Context, dstIP [4]byte, dstPort uint16) (net.Conn, error) {
	tz.upMu.Lock()
	var localPort uint16
	for i := 0; i < (upstreamProxyPortMax - upstreamProxyPortMin); i++ {
		p := tz.nextUpPrt
		tz.nextUpPrt++
		if tz.nextUpPrt > upstreamProxyPortMax {
			tz.nextUpPrt = upstreamProxyPortMin
		}
		if _, busy := tz.upConns[p]; !busy {
			localPort = p
			break
		}
	}
	if localPort == 0 {
		tz.upMu.Unlock()
		return nil, errors.New("нет свободных портов для VPN-фоллбэка")
	}

	localIP := tz.s.localTunIPBytes()
	key := tcpFlowKey{
		clientIP:   dstIP,
		clientPort: dstPort,
		targetIP:   localIP,
		targetPort: localPort,
	}
	conn := newActiveTunTCPConn(tz, key, localPort)
	tz.upConns[localPort] = conn
	tz.upMu.Unlock()

	conn.sendSyn()

	select {
	case <-conn.establishedCh:
		return conn, nil
	case <-conn.closedCh:
		tz.removeUpstreamConn(localPort)
		return nil, errors.New("соединение через VPN сброшено")
	case <-ctx.Done():
		_ = conn.Close()
		tz.removeUpstreamConn(localPort)
		return nil, ctx.Err()
	case <-time.After(4 * time.Second):
		_ = conn.Close()
		tz.removeUpstreamConn(localPort)
		return nil, errors.New("таймаут подключения через VPN")
	}
}

func (tz *tunZapret) removeUpstreamConn(localPort uint16) {
	tz.upMu.Lock()
	delete(tz.upConns, localPort)
	tz.upMu.Unlock()
}

// handleInboundFromUpstream проверяет входящий из VPS-туннеля (netToTun) пакет:
// если он адресован активному соединению dialUpstreamTCP (порт 61000..64999), передаёт его в tunTCPConn.
func (tz *tunZapret) handleInboundFromUpstream(pkt []byte) bool {
	if len(pkt) < 40 || pkt[0]>>4 != 4 || pkt[9] != 6 {
		return false
	}
	ihl := int(pkt[0]&0x0f) * 4
	totalLen := int(binary.BigEndian.Uint16(pkt[2:4]))
	if ihl < 20 || totalLen < ihl+20 || totalLen > len(pkt) {
		return false
	}
	l4 := pkt[ihl:totalLen]
	dstPort := binary.BigEndian.Uint16(l4[2:4])
	if dstPort < upstreamProxyPortMin || dstPort > upstreamProxyPortMax {
		return false
	}
	tz.upMu.Lock()
	conn, ok := tz.upConns[dstPort]
	tz.upMu.Unlock()
	if !ok {
		return false
	}
	conn.onPeerPacket(l4)
	return true
}

func (tz *tunZapret) closeAll() {
	tz.tcpMu.Lock()
	tcpConns := make([]*tunTCPConn, 0, len(tz.tcpConns))
	for _, c := range tz.tcpConns {
		tcpConns = append(tcpConns, c)
	}
	tz.tcpConns = make(map[tcpFlowKey]*tunTCPConn)
	tz.tcpMu.Unlock()
	for _, c := range tcpConns {
		_ = c.Close()
	}

	tz.upMu.Lock()
	upConns := make([]*tunTCPConn, 0, len(tz.upConns))
	for _, c := range tz.upConns {
		upConns = append(upConns, c)
	}
	tz.upConns = make(map[uint16]*tunTCPConn)
	tz.upMu.Unlock()
	for _, c := range upConns {
		_ = c.Close()
	}

	tz.udpMu.Lock()
	udpFlows := make([]*tunUDPFlow, 0, len(tz.udpFlows))
	for _, f := range tz.udpFlows {
		udpFlows = append(udpFlows, f)
	}
	tz.udpFlows = make(map[udpFlowKey]*tunUDPFlow)
	tz.udpMu.Unlock()
	for _, f := range udpFlows {
		_ = f.pc.Close()
	}

	for _, dc := range tz.dohClients {
		if dc != nil && dc.client != nil {
			dc.client.CloseIdleConnections()
		}
	}
}

// tunTCPConn реализует легковесный L4 TCP-эндпоинт поверх сырых IPv4-пакетов (net.Conn).
// Работает как в пассивном режиме (принимая соединение от ОС из TUN), так и в активном
// (открывая соединение внутрь L3 VPN-туннеля при бесшовном фоллбэке).
type tunTCPConn struct {
	tz         *tunZapret
	key        tcpFlowKey
	activeMode bool
	upPort     uint16

	mu            sync.Mutex
	mySeq         uint32
	peerNext      uint32
	outOfOrder    map[uint32][]byte
	rxBuf         bytes.Buffer
	rxNotify      chan struct{}
	established   bool
	establishedCh chan struct{}
	peerEOF       bool
	closed        bool
	closedCh      chan struct{}
	readDeadline  time.Time
}

func newPassiveTunTCPConn(tz *tunZapret, key tcpFlowKey, clientISN uint32) *tunTCPConn {
	return &tunTCPConn{
		tz:            tz,
		key:           key,
		mySeq:         rand.Uint32(),
		peerNext:      clientISN + 1,
		outOfOrder:    make(map[uint32][]byte),
		rxNotify:      make(chan struct{}, 1),
		established:   true,
		establishedCh: make(chan struct{}),
		closedCh:      make(chan struct{}),
	}
}

func newActiveTunTCPConn(tz *tunZapret, key tcpFlowKey, upPort uint16) *tunTCPConn {
	return &tunTCPConn{
		tz:            tz,
		key:           key,
		activeMode:    true,
		upPort:        upPort,
		mySeq:         rand.Uint32(),
		outOfOrder:    make(map[uint32][]byte),
		rxNotify:      make(chan struct{}, 1),
		establishedCh: make(chan struct{}),
		closedCh:      make(chan struct{}),
	}
}

func (c *tunTCPConn) notifyRx() {
	select {
	case c.rxNotify <- struct{}{}:
	default:
	}
}

func (c *tunTCPConn) writePacket(flags byte, payload []byte, seq, ack uint32) error {
	totalLen := 20 + 20 + len(payload)
	pkt := make([]byte, totalLen)

	pkt[0] = 0x45
	binary.BigEndian.PutUint16(pkt[2:4], uint16(totalLen))
	pkt[6] = 0x40
	pkt[8] = 64
	pkt[9] = 6 // TCP
	copy(pkt[12:16], c.key.targetIP[:])
	copy(pkt[16:20], c.key.clientIP[:])

	tcp := pkt[20:]
	binary.BigEndian.PutUint16(tcp[0:2], c.key.targetPort)
	binary.BigEndian.PutUint16(tcp[2:4], c.key.clientPort)
	binary.BigEndian.PutUint32(tcp[4:8], seq)
	binary.BigEndian.PutUint32(tcp[8:12], ack)
	tcp[12] = 5 << 4
	tcp[13] = flags
	binary.BigEndian.PutUint16(tcp[14:16], tunTCPWindowSize)
	if len(payload) > 0 {
		copy(tcp[20:], payload)
	}

	recomputeIPv4Checksums(pkt)
	if c.activeMode {
		return c.tz.s.writeRawToUpstream(pkt)
	}
	return c.tz.writeTun(pkt)
}

func (c *tunTCPConn) sendSynAck() {
	c.mu.Lock()
	seq := c.mySeq
	c.mySeq++
	ack := c.peerNext
	c.mu.Unlock()
	_ = c.writePacket(0x12, nil, seq, ack) // SYN | ACK
}

func (c *tunTCPConn) sendSyn() {
	c.mu.Lock()
	seq := c.mySeq
	c.mySeq++
	c.mu.Unlock()
	_ = c.writePacket(0x02, nil, seq, 0) // SYN
}

func (c *tunTCPConn) sendRST() {
	c.mu.Lock()
	seq := c.mySeq
	ack := c.peerNext
	c.mu.Unlock()
	_ = c.writePacket(0x14, nil, seq, ack) // RST | ACK
}

func (c *tunTCPConn) onPeerPacket(l4 []byte) {
	if len(l4) < 20 {
		return
	}
	dataOff := int(l4[12]>>4) * 4
	if dataOff < 20 || dataOff > len(l4) {
		return
	}
	seq := binary.BigEndian.Uint32(l4[4:8])
	flags := l4[13]
	payload := l4[dataOff:]

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}

	if flags&0x04 != 0 { // RST
		c.closed = true
		select {
		case <-c.closedCh:
		default:
			close(c.closedCh)
		}
		c.notifyRx()
		c.mu.Unlock()
		return
	}

	// В активном режиме ждём SYN+ACK от сервера
	if c.activeMode && !c.established {
		if flags&0x12 == 0x12 { // SYN | ACK
			c.peerNext = seq + 1
			c.established = true
			mySeq := c.mySeq
			peerNext := c.peerNext
			select {
			case <-c.establishedCh:
			default:
				close(c.establishedCh)
			}
			c.mu.Unlock()
			_ = c.writePacket(0x10, nil, mySeq, peerNext) // ACK
			return
		}
		c.mu.Unlock()
		return
	}

	needAck := false
	if len(payload) > 0 {
		if seq == c.peerNext {
			c.rxBuf.Write(payload)
			c.peerNext += uint32(len(payload))
			for {
				if nextChunk, ok := c.outOfOrder[c.peerNext]; ok {
					delete(c.outOfOrder, c.peerNext)
					c.rxBuf.Write(nextChunk)
					c.peerNext += uint32(len(nextChunk))
				} else {
					break
				}
			}
			needAck = true
			c.notifyRx()
		} else if int32(seq-c.peerNext) > 0 && len(c.outOfOrder) < 32 {
			cp := make([]byte, len(payload))
			copy(cp, payload)
			c.outOfOrder[seq] = cp
			needAck = true
		} else if int32(seq-c.peerNext) < 0 {
			needAck = true
		}
	}

	if flags&0x01 != 0 { // FIN
		if seq+uint32(len(payload)) == c.peerNext {
			c.peerNext++
		}
		c.peerEOF = true
		needAck = true
		c.notifyRx()
	}

	mySeq := c.mySeq
	peerNext := c.peerNext
	c.mu.Unlock()

	if needAck {
		_ = c.writePacket(0x10, nil, mySeq, peerNext) // ACK
	}
}

func (c *tunTCPConn) Read(b []byte) (int, error) {
	for {
		c.mu.Lock()
		if c.rxBuf.Len() > 0 {
			n, err := c.rxBuf.Read(b)
			c.mu.Unlock()
			return n, err
		}
		if c.peerEOF || c.closed {
			c.mu.Unlock()
			return 0, io.EOF
		}
		dl := c.readDeadline
		c.mu.Unlock()

		var timer <-chan time.Time
		if !dl.IsZero() {
			rem := time.Until(dl)
			if rem <= 0 {
				return 0, context.DeadlineExceeded
			}
			t := time.NewTimer(rem)
			defer t.Stop()
			timer = t.C
		}

		select {
		case <-c.rxNotify:
		case <-c.closedCh:
			return 0, io.EOF
		case <-timer:
			return 0, context.DeadlineExceeded
		}
	}
}

func (c *tunTCPConn) Write(b []byte) (int, error) {
	total := 0
	for len(b) > 0 {
		chunk := b
		if len(chunk) > tunTCPMaxSeg {
			chunk = b[:tunTCPMaxSeg]
		}
		b = b[len(chunk):]

		c.mu.Lock()
		if c.closed {
			c.mu.Unlock()
			return total, io.ErrClosedPipe
		}
		seq := c.mySeq
		c.mySeq += uint32(len(chunk))
		ack := c.peerNext
		c.mu.Unlock()

		if err := c.writePacket(0x18, chunk, seq, ack); err != nil { // PSH | ACK
			return total, err
		}
		total += len(chunk)
	}
	return total, nil
}

func (c *tunTCPConn) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	seq := c.mySeq
	c.mySeq++
	ack := c.peerNext
	select {
	case <-c.closedCh:
	default:
		close(c.closedCh)
	}
	c.notifyRx()
	c.mu.Unlock()

	if c.activeMode && c.upPort != 0 {
		c.tz.removeUpstreamConn(c.upPort)
	}
	_ = c.writePacket(0x11, nil, seq, ack) // FIN | ACK
	return nil
}

func (c *tunTCPConn) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.IP(c.key.targetIP[:]), Port: int(c.key.targetPort)}
}

func (c *tunTCPConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.IP(c.key.clientIP[:]), Port: int(c.key.clientPort)}
}

func (c *tunTCPConn) SetDeadline(t time.Time) error {
	c.mu.Lock()
	c.readDeadline = t
	c.mu.Unlock()
	c.notifyRx()
	return nil
}

func (c *tunTCPConn) SetReadDeadline(t time.Time) error {
	return c.SetDeadline(t)
}

func (c *tunTCPConn) SetWriteDeadline(t time.Time) error {
	return nil
}
