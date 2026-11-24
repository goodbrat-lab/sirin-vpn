package engine

import (
	"bytes"
	"encoding/binary"
	"testing"
)

const (
	testTCPFlagSYN = 0x02
	testTCPFlagACK = 0x10
	testTCPFlagPSH = 0x08
)

func buildTestTCPPacket(srcIP [4]byte, srcPort uint16, dstIP [4]byte, dstPort uint16, seq, ack uint32, flags byte, payload []byte) []byte {
	totalLen := 20 + 20 + len(payload)
	pkt := make([]byte, totalLen)
	pkt[0] = 0x45
	binary.BigEndian.PutUint16(pkt[2:4], uint16(totalLen))
	pkt[6] = 0x40
	pkt[8] = 64
	pkt[9] = 6
	copy(pkt[12:16], srcIP[:])
	copy(pkt[16:20], dstIP[:])

	tcp := pkt[20:]
	binary.BigEndian.PutUint16(tcp[0:2], srcPort)
	binary.BigEndian.PutUint16(tcp[2:4], dstPort)
	binary.BigEndian.PutUint32(tcp[4:8], seq)
	binary.BigEndian.PutUint32(tcp[8:12], ack)
	tcp[12] = 5 << 4
	tcp[13] = flags
	binary.BigEndian.PutUint16(tcp[14:16], 65535)
	copy(tcp[20:], payload)
	recomputeIPv4Checksums(pkt)
	return pkt
}

func buildTestUDPPacket(srcIP [4]byte, srcPort uint16, dstIP [4]byte, dstPort uint16, payload []byte) []byte {
	totalLen := 20 + 8 + len(payload)
	pkt := make([]byte, totalLen)
	pkt[0] = 0x45
	binary.BigEndian.PutUint16(pkt[2:4], uint16(totalLen))
	pkt[6] = 0x40
	pkt[8] = 64
	pkt[9] = 17
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

func TestRecomputeIPv4ChecksumsTCPAndUDP(t *testing.T) {
	// 1. Тест пересчёта контрольной суммы TCP-пакета после 1:1 NAT (10.77.77.2 -> 10.77.0.15)
	tcpPkt := buildTestTCPPacket(
		[4]byte{10, 77, 77, 2}, 54321,
		[4]byte{142, 250, 74, 14}, 443,
		1000, 2000, testTCPFlagACK|testTCPFlagPSH,
		[]byte("hello zapret2"),
	)
	// Меняем IP источника (как делает translateOutboundNAT)
	copy(tcpPkt[12:16], []byte{10, 77, 0, 15})
	recomputeIPv4Checksums(tcpPkt)

	// Проверяем, что сумма IPv4-заголовка даёт 0xffff (дополнение = 0)
	var ipSum uint32
	for i := 0; i < 20; i += 2 {
		ipSum += uint32(binary.BigEndian.Uint16(tcpPkt[i : i+2]))
	}
	for (ipSum >> 16) > 0 {
		ipSum = (ipSum & 0xffff) + (ipSum >> 16)
	}
	if uint16(ipSum) != 0xffff {
		t.Fatalf("неверная контрольная сумма IPv4 заголовка: 0x%04x", ipSum)
	}

	// Проверяем, что сумма TCP-сегмента вместе с псевдозаголовком даёт 0xffff
	l4 := tcpPkt[20:]
	tcpSum := pseudoHeaderSum(tcpPkt[12:16], tcpPkt[16:20], 6, len(l4)) + checksumBytes(l4)
	for (tcpSum >> 16) > 0 {
		tcpSum = (tcpSum & 0xffff) + (tcpSum >> 16)
	}
	if uint16(tcpSum) != 0xffff {
		t.Fatalf("неверная контрольная сумма TCP после NAT: 0x%04x", tcpSum)
	}

	// 2. Тест UDP-пакета
	udpPkt := buildTestUDPPacket(
		[4]byte{10, 77, 77, 2}, 51111,
		[4]byte{1, 1, 1, 1}, 53,
		[]byte{0x12, 0x34, 0x01, 0x00, 0x00, 0x01},
	)
	copy(udpPkt[12:16], []byte{10, 77, 0, 15})
	recomputeIPv4Checksums(udpPkt)
	uL4 := udpPkt[20:]
	udpSum := pseudoHeaderSum(udpPkt[12:16], udpPkt[16:20], 17, len(uL4)) + checksumBytes(uL4)
	for (udpSum >> 16) > 0 {
		udpSum = (udpSum & 0xffff) + (udpSum >> 16)
	}
	if uint16(udpSum) != 0xffff {
		t.Fatalf("неверная контрольная сумма UDP после NAT: 0x%04x", udpSum)
	}
}

func TestBuildICMPPortUnreachableForQUIC(t *testing.T) {
	quicPkt := buildTestUDPPacket(
		[4]byte{10, 77, 77, 2}, 55443,
		[4]byte{142, 250, 74, 14}, 443,
		[]byte("QUIC Initial Payload"),
	)
	icmpPkt := buildICMPPortUnreachable(quicPkt)
	if len(icmpPkt) < 28+28 {
		t.Fatalf("слишком короткий ICMP пакет: %d", len(icmpPkt))
	}
	if icmpPkt[9] != 1 {
		t.Fatalf("протокол должен быть ICMP (1), получено %d", icmpPkt[9])
	}
	// Источник ICMP должен быть IP целевого сервера (142.250.74.14), а получатель — 10.77.77.2
	if !bytes.Equal(icmpPkt[12:16], []byte{142, 250, 74, 14}) {
		t.Fatalf("неверный src IP в ICMP: %v", icmpPkt[12:16])
	}
	if !bytes.Equal(icmpPkt[16:20], []byte{10, 77, 77, 2}) {
		t.Fatalf("неверный dst IP в ICMP: %v", icmpPkt[16:20])
	}
	icmp := icmpPkt[20:]
	if icmp[0] != 3 || icmp[1] != 3 {
		t.Fatalf("ожидался ICMP Type 3 Code 3, получено Type=%d Code=%d", icmp[0], icmp[1])
	}
	cs := checksumBytes(icmp)
	for (cs >> 16) > 0 {
		cs = (cs & 0xffff) + (cs >> 16)
	}
	if uint16(cs) != 0xffff {
		t.Fatalf("неверная контрольная сумма ICMP: 0x%04x", cs)
	}
}

func TestBuildIPv6TCPResetAndICMPv6Unreachable(t *testing.T) {
	// 1. IPv6 TCP SYN -> должен сформироваться валидный IPv6 TCP RST|ACK
	syn := make([]byte, 60)
	syn[0] = 0x60
	binary.BigEndian.PutUint16(syn[4:6], 20)
	syn[6] = 6 // TCP
	syn[7] = 64
	// src = fd00::1, dst = 2a00:1450:4010::1
	syn[8], syn[9], syn[23] = 0xfd, 0x00, 0x01
	syn[24], syn[25], syn[39] = 0x2a, 0x00, 0x01
	binary.BigEndian.PutUint16(syn[40:42], 51234) // srcPort
	binary.BigEndian.PutUint16(syn[42:44], 443)   // dstPort
	binary.BigEndian.PutUint32(syn[44:48], 1000)  // seq
	syn[52] = 5 << 4
	syn[53] = 0x02 // SYN

	rst := buildIPv6TCPReset(syn)
	if len(rst) != 60 {
		t.Fatalf("expected 60-byte IPv6 TCP RST, got %d", len(rst))
	}
	if rst[0]>>4 != 6 || rst[6] != 6 {
		t.Fatalf("invalid IPv6 header in RST")
	}
	if !bytes.Equal(rst[8:24], syn[24:40]) || !bytes.Equal(rst[24:40], syn[8:24]) {
		t.Fatalf("IPv6 src/dst not swapped in RST")
	}
	if rst[53] != 0x14 { // RST|ACK
		t.Fatalf("expected RST|ACK (0x14), got 0x%02x", rst[53])
	}
	if binary.BigEndian.Uint32(rst[48:52]) != 1001 {
		t.Fatalf("expected ack=1001, got %d", binary.BigEndian.Uint32(rst[48:52]))
	}
	// Проверка, что на входящий RST не генерируется ответ (исключение петли)
	if buildIPv6TCPReset(rst) != nil {
		t.Fatalf("must not generate RST in response to RST")
	}

	// 2. IPv6 UDP (QUIC :443) -> валидный ICMPv6 Type 1 Code 4
	udp6 := make([]byte, 56)
	udp6[0] = 0x60
	binary.BigEndian.PutUint16(udp6[4:6], 16)
	udp6[6] = 17 // UDP
	udp6[7] = 64
	udp6[8], udp6[9], udp6[23] = 0xfd, 0x00, 0x01
	udp6[24], udp6[25], udp6[39] = 0x2a, 0x00, 0x01
	binary.BigEndian.PutUint16(udp6[40:42], 55555)
	binary.BigEndian.PutUint16(udp6[42:44], 443)
	binary.BigEndian.PutUint16(udp6[44:46], 16)

	icmp6 := buildICMPv6PortUnreachable(udp6)
	if icmp6 == nil || icmp6[6] != 58 || icmp6[40] != 1 || icmp6[41] != 4 {
		t.Fatalf("expected ICMPv6 Type 1 Code 4, got %v", icmp6)
	}
}

func TestDNSFilterAAAAAndParseARecords(t *testing.T) {
	// Запрос AAAA для rr1.googlevideo.com с EDNS0 OPT-записью (ARCOUNT=1)
	var query []byte
	header := make([]byte, 12)
	binary.BigEndian.PutUint16(header[0:2], 0xabcd) // ID
	header[2] = 0x01                                // RD=1
	binary.BigEndian.PutUint16(header[4:6], 1)      // QDCOUNT=1
	binary.BigEndian.PutUint16(header[10:12], 1)    // ARCOUNT=1 (EDNS0)
	query = append(query, header...)
	// QNAME: rr1.googlevideo.com
	query = append(query, 3, 'r', 'r', '1', 11, 'g', 'o', 'o', 'g', 'l', 'e', 'v', 'i', 'd', 'e', 'o', 3, 'c', 'o', 'm', 0)
	qEnd := len(query) + 4
	query = append(query, 0, 28, 0, 1) // QTYPE=AAAA (28), QCLASS=IN (1)
	// EDNS0 OPT additional RR (11 bytes)
	query = append(query, 0, 0, 41, 4, 208, 0, 0, 0, 0, 0, 0)

	if qt := dnsQueryType(query); qt != 28 {
		t.Fatalf("expected QTYPE 28, got %d", qt)
	}
	if qn := dnsQueryName(query); qn != "rr1.googlevideo.com" {
		t.Fatalf("expected QNAME rr1.googlevideo.com, got %q", qn)
	}
	resp := buildDNSEmptyAAAAResponse(query)
	if len(resp) != qEnd {
		t.Fatalf("expected EDNS0 stripped to length %d, got %d", qEnd, len(resp))
	}
	if resp[2]&0x80 == 0 || resp[3]&0x0f != 0 {
		t.Fatalf("expected QR=1 and RCODE=0, got flags 0x%02x 0x%02x", resp[2], resp[3])
	}
	if binary.BigEndian.Uint16(resp[6:8]) != 0 || binary.BigEndian.Uint16(resp[10:12]) != 0 {
		t.Fatalf("expected ANCOUNT=0 and ARCOUNT=0")
	}

	// Синтетический A-ответ для rr1.googlevideo.com -> 173.194.76.12
	var aResp []byte
	aHdr := make([]byte, 12)
	binary.BigEndian.PutUint16(aHdr[0:2], 0x1234)
	aHdr[2] = 0x81
	aHdr[3] = 0x80
	binary.BigEndian.PutUint16(aHdr[4:6], 1) // QDCOUNT=1
	binary.BigEndian.PutUint16(aHdr[6:8], 1) // ANCOUNT=1
	aResp = append(aResp, aHdr...)
	aResp = append(aResp, 3, 'r', 'r', '1', 11, 'g', 'o', 'o', 'g', 'l', 'e', 'v', 'i', 'd', 'e', 'o', 3, 'c', 'o', 'm', 0)
	aResp = append(aResp, 0, 1, 0, 1) // QTYPE=A, QCLASS=IN
	// Answer: pointer 0xC00C, TYPE=A (1), CLASS=IN (1), TTL=60, RDLEN=4, IP=173.194.76.12
	aResp = append(aResp, 0xc0, 0x0c, 0, 1, 0, 1, 0, 0, 0, 60, 0, 4, 173, 194, 76, 12)

	ips := dnsParseARecords(aResp)
	if len(ips) != 1 || ips[0].String() != "173.194.76.12" {
		t.Fatalf("expected [173.194.76.12], got %v", ips)
	}
}

func TestIsSTUNBindingRequestAndOutboundRouting(t *testing.T) {
	// 1. RFC 5389 STUN Binding Request (20 байт, magic cookie 0x2112A442)
	stun5389 := make([]byte, 20)
	binary.BigEndian.PutUint16(stun5389[0:2], 0x0001)
	binary.BigEndian.PutUint16(stun5389[2:4], 0)
	binary.BigEndian.PutUint32(stun5389[4:8], 0x2112A442)
	copy(stun5389[8:20], []byte("123456789012"))

	if !isSTUNBindingRequest(stun5389, 19302) {
		t.Fatalf("expected RFC 5389 STUN Binding Request to be detected")
	}
	if !isSTUNBindingRequest(stun5389, 50000) {
		t.Fatalf("expected RFC 5389 STUN with magic cookie on custom port to be detected")
	}

	// 2. Классический RFC 3489 STUN без magic cookie на порту 3478
	stun3489 := make([]byte, 20)
	binary.BigEndian.PutUint16(stun3489[0:2], 0x0001)
	binary.BigEndian.PutUint16(stun3489[2:4], 0)
	copy(stun3489[4:20], []byte("legacy_tran_id16"))
	if !isSTUNBindingRequest(stun3489, 3478) {
		t.Fatalf("expected RFC 3489 STUN on port 3478 to be detected")
	}
	if isSTUNBindingRequest(stun3489, 27015) {
		t.Fatalf("must not treat non-5389 packet on game port 27015 as STUN")
	}

	// 3. Discord IP Discovery (74 байта: тип 0x0001, длина 70 — не кратна 4, без magic cookie)
	discordDisc := make([]byte, 74)
	binary.BigEndian.PutUint16(discordDisc[0:2], 0x0001)
	binary.BigEndian.PutUint16(discordDisc[2:4], 70)
	if isSTUNBindingRequest(discordDisc, 50005) {
		t.Fatalf("Discord IP Discovery must not be misclassified as STUN")
	}

	// 4. При активном upstream (VPS поднят и есть слот) STUN должен уходить в VPN (handleOutbound == false)
	sess := &session{}
	sess.upstreamReady.Store(true)
	slots := []*rung{{}}
	sess.slots.Store(&slots)
	tz := newTunZapret(sess)
	stunPkt := buildTestUDPPacket(
		[4]byte{10, 77, 77, 2}, 53000,
		[4]byte{74, 125, 250, 129}, 19302, // stun.l.google.com
		stun5389,
	)
	if tz.handleOutbound(stunPkt) {
		t.Fatalf("expected STUN packet to bypass WAN and route via active VPN (handleOutbound=false)")
	}
}

func TestActiveTunTCPSynOptionsAndTTL(t *testing.T) {
	if got := activeInitialTTL("windows"); got != 128 {
		t.Fatalf("expected TTL 128 for windows, got %d", got)
	}
	if got := activeInitialTTL("linux"); got != 64 {
		t.Fatalf("expected TTL 64 for linux, got %d", got)
	}
	if got := activeInitialTTL("android"); got != 64 {
		t.Fatalf("expected TTL 64 for android, got %d", got)
	}

	tz := newTunZapret(&session{})
	key := tcpFlowKey{
		clientIP:   [4]byte{104, 18, 32, 7}, // remote server
		clientPort: 443,
		targetIP:   [4]byte{10, 77, 77, 2}, // local tun IP
		targetPort: 61005,
	}
	activeConn := newActiveTunTCPConn(tz, key, 61005)
	synPkt := activeConn.buildPacket(0x02, activeSynTCPOptions[:], nil, 123456, 0)

	if len(synPkt) != 52 {
		t.Fatalf("expected 52-byte SYN packet (20 IP + 20 TCP + 12 opts), got %d", len(synPkt))
	}
	tcp := synPkt[20:]
	dataOff := int(tcp[12]>>4) * 4
	if dataOff != 32 {
		t.Fatalf("expected TCP header length 32 bytes, got %d", dataOff)
	}
	if !bytes.Equal(tcp[20:32], activeSynTCPOptions[:]) {
		t.Fatalf("unexpected TCP SYN options: %x", tcp[20:32])
	}
	// Проверяем валидность контрольной суммы TCP вместе с 12 байтами опций
	tcpSum := pseudoHeaderSum(synPkt[12:16], synPkt[16:20], 6, len(tcp)) + checksumBytes(tcp)
	for (tcpSum >> 16) > 0 {
		tcpSum = (tcpSum & 0xffff) + (tcpSum >> 16)
	}
	if uint16(tcpSum) != 0xffff {
		t.Fatalf("invalid TCP checksum on SYN with options: 0x%04x", tcpSum)
	}
}
