// Package zapret реализует нативный движок обхода DPI по технологиям Zapret 2:
//   - Потоковая и пакетная десинхронизация TLS ClientHello и HTTP Host
//   - Разбиение TLS Record на 2 и 3 валидные записи прямо внутри поля SNI (tlsrec + multisplit)
//   - Инъекция срочного байта TCP Out-of-Band (MSG_OOB) на границе разреза SNI (tpws --oob)
//   - Инъекция фейкового TLS ClientHello с испорченной контрольной суммой (badsum) и малым TTL (nfqws2 fake)
//   - Пакетный UDP/QUIC Fake Burst с пониженным IP_TTL=3 для обхода эвристики ТСПУ на UDP
//   - Автоматический подбор и запоминание рабочей стратегии по доменам (аналог zapret-auto.lua)
//   - Иерархический кэш бесшовного переключения на VPN (exact SNI / IP / *.basedomain) при жёсткой блокировке по IP.
package zapret

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

// StrategyID определяет стратегию десинхронизации DPI.
type StrategyID int

const (
	// StrategyTLSRecSplit — разбиение TLS Record на 2 записи посередине SNI + 2-байтовый TCP сплит.
	StrategyTLSRecSplit StrategyID = iota
	// StrategyMultiSplit — разбиение TLS ClientHello на 3 валидные TLS-записи (до SNI, 1-я половина SNI, 2-я половина SNI) + микросегментация.
	StrategyMultiSplit
	// StrategyOOBSplit — разбиение TLS Record посередине SNI с инъекцией срочного байта TCP Urgent (MSG_OOB).
	StrategyOOBSplit
	// StrategyFakeLowTTLSplit — инъекция фейкового ClientHello (badsum + TTL=4 на Linux root) + MSG_OOB + 3-кратный TLS Record split.
	StrategyFakeLowTTLSplit

	strategyCount = 4
)

func (s StrategyID) String() string {
	switch s {
	case StrategyTLSRecSplit:
		return "tlsrec_sni_split"
	case StrategyMultiSplit:
		return "tlsrec_multisplit"
	case StrategyOOBSplit:
		return "oob_tlsrec_split"
	case StrategyFakeLowTTLSplit:
		return "fake_lowttl_multisplit"
	default:
		return "unknown"
	}
}

const (
	DefaultWanProbeTimeout = 550 * time.Millisecond
	ExactHostBlockedTTL    = 60 * time.Minute
	WildcardBlockedTTL     = 20 * time.Minute
	WildcardFailThreshold  = 2
	WinnerStrategyTTL      = 6 * time.Hour
)

// DefaultBaseDomains — список базовых доменных зон сервисов для автообучения стратегий и wildcard-фоллбэка.
var DefaultBaseDomains = []string{
	// YouTube / Google Video
	"googlevideo.com",
	"youtube.com",
	"ytimg.com",
	"ggpht.com",
	"youtu.be",
	"youtubei.googleapis.com",
	"wide-youtube.l.google.com",
	// Instagram / Meta
	"instagram.com",
	"cdninstagram.com",
	"fbcdn.net",
	"facebook.com",
	"ig.me",
	// Discord
	"discord.com",
	"discordapp.com",
	"discord.gg",
	"discordapp.net",
	"discord.media",
	// X / Twitter
	"x.com",
	"twitter.com",
	"twimg.com",
	"t.co",
	// Популярные ресурсы с DPI-фильтрацией
	"rutracker.org",
	"rutracker.net",
	"ntc.party",
	"ttvnw.net",
}

// DefaultTargetDomains возвращает полный набор целевых доменов для dnsmasq/маршрутизации.
func DefaultTargetDomains(youtube, instagram, discord bool, custom []string) []string {
	var domains []string
	if youtube {
		domains = append(domains,
			"googlevideo.com",
			"youtube.com",
			"www.youtube.com",
			"m.youtube.com",
			"ytimg.com",
			"ggpht.com",
			"youtu.be",
			"youtubei.googleapis.com",
		)
	}
	if instagram {
		domains = append(domains,
			"instagram.com",
			"www.instagram.com",
			"i.instagram.com",
			"cdninstagram.com",
			"scontent.cdninstagram.com",
			"graph.instagram.com",
			"fbcdn.net",
			"facebook.com",
			"www.facebook.com",
			"connect.facebook.net",
			"ig.me",
			"x.com",
			"twitter.com",
			"twimg.com",
			"abs.twimg.com",
			"pbs.twimg.com",
			"video.twimg.com",
			"t.co",
		)
	}
	if discord {
		domains = append(domains,
			"discord.com",
			"www.discord.com",
			"discordapp.com",
			"discord.gg",
			"discordapp.net",
			"gateway.discord.gg",
			"cdn.discordapp.com",
			"status.discord.com",
			"discord.media",
			"media.discordapp.net",
		)
	}
	for _, d := range custom {
		d = strings.TrimSpace(d)
		if d != "" {
			domains = append(domains, d)
		}
	}
	return domains
}

// GeoBlockedDomains — список зарубежных сервисов, блокирующих доступ с российских IP-адресов на уровне GeoIP
// (после успешного TLS-рукопожатия возвращают HTTP 403/451). Для них прямой обход ТСПУ через WAN не подходит —
// требуется немедленное пробуждение ленивого VPN-туннеля до зарубежного VPS.
var GeoBlockedDomains = []string{
	// OpenAI / ChatGPT / Sora
	"openai.com",
	"chatgpt.com",
	"oaistatic.com",
	"oaiusercontent.com",
	"sora.com",
	// Anthropic / Claude
	"anthropic.com",
	"claude.ai",
	// Google Gemini / AI Studio / NotebookLM
	"gemini.google.com",
	"aistudio.google.com",
	"makersuite.google.com",
	"notebooklm.google.com",
	"labs.google",
	// xAI Grok / Microsoft Copilot
	"grok.com",
	"x.ai",
	"copilot.microsoft.com",
	// Spotify
	"spotify.com",
	"scdn.co",
	"spotifycdn.com",
	// Autodesk / CAD / Design / Dev tools
	"autodesk.com",
	"autodesk.net",
	"canva.com",
	"notion.so",
	"notion.site",
	"jetbrains.com",
	// PlayStation Store / PSN / Sony Account
	"playstation.com",
	"playstation.net",
	"playstation.org",
	"sonyentertainmentnetwork.com",
	"sony.com",
	"sie.com",
	"arkoselabs.com",
	"funcaptcha.com",
	// Проверка внешнего IP туннеля
	"api.ipify.org",
}

// IsGeoBlockedSNI возвращает true, если домен блокирует российские IP-адреса со своей стороны (GeoIP)
// и должен маршрутизироваться через зарубежный VPS-туннель, а не через прямой WAN с Zapret 2.
func IsGeoBlockedSNI(sni string) bool {
	sni = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(sni), "."))
	if sni == "" {
		return false
	}
	for _, d := range GeoBlockedDomains {
		if sni == d || strings.HasSuffix(sni, "."+d) {
			return true
		}
	}
	return false
}

// ForceVPNDomains — домены потоковых сервисов и CDN, для которых ТСПУ на мобильных сетях РФ
// пропускает TLS Handshake, но жёстко душит пропускную способность потока (~112 Б/с).
// Zapret 2 в этом случае ошибочно считает обход успешным (ServerHello получен), но видео
// бесконечно буферизируется. Такие домены должны сразу идти через L3 fast-path VPS-туннеля.
var ForceVPNDomains = []string{
	"googlevideo.com",
	"youtube.com",
	"ytimg.com",
	"ggpht.com",
	"youtu.be",
	"youtubei.googleapis.com",
	"yt3.ggpht.com",
	"wide-youtube.l.google.com",
}

// IsForceVPNDomain возвращает true, если домен подвержен скоростному троттлингу ТСПУ
// (YouTube / GoogleVideo CDN) и должен всегда маршрутизироваться через VPN fast-path.
func IsForceVPNDomain(sni string) bool {
	sni = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(sni), "."))
	if sni == "" {
		return false
	}
	for _, d := range ForceVPNDomains {
		if sni == d || strings.HasSuffix(sni, "."+d) {
			return true
		}
	}
	return false
}

// MatchBaseDomain возвращает базовый домен из DefaultBaseDomains, если sni принадлежит ему.
func MatchBaseDomain(sni string) string {
	sni = strings.ToLower(strings.TrimSpace(sni))
	if sni == "" {
		return ""
	}
	for _, base := range DefaultBaseDomains {
		if sni == base || strings.HasSuffix(sni, "."+base) {
			return base
		}
	}
	parts := strings.Split(sni, ".")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "." + parts[len(parts)-1]
	}
	return sni
}

// ExtractSNI парсит TLS ClientHello и извлекает Server Name Indication (SNI) и его смещение в пакете.
func ExtractSNI(data []byte) (string, int) {
	if len(data) < 43 {
		return "", -1
	}
	if data[0] != 0x16 || data[5] != 0x01 {
		return "", -1
	}
	pos := 43
	if pos >= len(data) {
		return "", -1
	}
	sessionIDLen := int(data[pos])
	pos += 1 + sessionIDLen
	if pos+2 > len(data) {
		return "", -1
	}
	cipherLen := int(binary.BigEndian.Uint16(data[pos : pos+2]))
	pos += 2 + cipherLen
	if pos+1 > len(data) {
		return "", -1
	}
	compLen := int(data[pos])
	pos += 1 + compLen
	if pos+2 > len(data) {
		return "", -1
	}
	extTotalLen := int(binary.BigEndian.Uint16(data[pos : pos+2]))
	pos += 2
	end := pos + extTotalLen
	if end > len(data) {
		end = len(data)
	}

	for pos+4 <= end {
		extType := binary.BigEndian.Uint16(data[pos : pos+2])
		extLen := int(binary.BigEndian.Uint16(data[pos+2 : pos+4]))
		pos += 4

		if extType == 0x0000 {
			if pos+2 > end {
				return "", -1
			}
			listLen := int(binary.BigEndian.Uint16(data[pos : pos+2]))
			subPos := pos + 2
			subEnd := subPos + listLen
			if subEnd > end {
				subEnd = end
			}
			for subPos+3 <= subEnd {
				nameType := data[subPos]
				nameLen := int(binary.BigEndian.Uint16(data[subPos+1 : subPos+3]))
				subPos += 3
				if nameType == 0 {
					if subPos+nameLen <= subEnd {
						return string(data[subPos : subPos+nameLen]), subPos
					}
				}
				subPos += nameLen
			}
		}
		pos += extLen
	}
	return "", -1
}

func indexHostHeader(data []byte) int {
	for i := 0; i+7 <= len(data); i++ {
		if data[i] == '\r' && data[i+1] == '\n' &&
			(data[i+2]|0x20) == 'h' &&
			(data[i+3]|0x20) == 'o' &&
			(data[i+4]|0x20) == 's' &&
			(data[i+5]|0x20) == 't' &&
			data[i+6] == ':' {
			return i
		}
	}
	return -1
}

// ExtractHTTPHost извлекает заголовок Host из открытого HTTP-запроса и возвращает хост и смещение значения.
func ExtractHTTPHost(data []byte) (string, int) {
	if len(data) < 16 {
		return "", -1
	}
	// Проверяем типичные HTTP методы
	if !bytes.HasPrefix(data, []byte("GET ")) &&
		!bytes.HasPrefix(data, []byte("POST ")) &&
		!bytes.HasPrefix(data, []byte("HEAD ")) &&
		!bytes.HasPrefix(data, []byte("PUT ")) &&
		!bytes.HasPrefix(data, []byte("OPTIONS ")) {
		return "", -1
	}
	idx := indexHostHeader(data)
	if idx == -1 {
		return "", -1
	}
	valStart := idx + 7
	for valStart < len(data) && (data[valStart] == ' ' || data[valStart] == '\t') {
		valStart++
	}
	valEnd := valStart
	for valEnd < len(data) && data[valEnd] != '\r' && data[valEnd] != '\n' {
		valEnd++
	}
	host := strings.TrimSpace(string(data[valStart:valEnd]))
	if colon := strings.IndexByte(host, ':'); colon != -1 {
		host = host[:colon]
	}
	return host, valStart
}

// BuildFakeClientHello конструирует валидный компактный TLS ClientHello с безопасным SNI (например, www.vk.com)
// для пакетной инъекции фейка (nfqws2 fake).
func BuildFakeClientHello(fakeSNI string) []byte {
	if fakeSNI == "" {
		fakeSNI = "www.vk.com"
	}
	sniBytes := []byte(fakeSNI)
	sniExtLen := 2 + 1 + 2 + len(sniBytes)
	extsLen := 4 + sniExtLen
	bodyLen := 2 + 32 + 1 + 2 + 2 + 1 + 1 + 2 + extsLen
	recLen := 4 + bodyLen

	buf := make([]byte, 5+recLen)
	buf[0] = 0x16 // Handshake
	buf[1] = 0x03
	buf[2] = 0x01 // TLS 1.0 record layer
	binary.BigEndian.PutUint16(buf[3:5], uint16(recLen))

	buf[5] = 0x01 // ClientHello
	buf[6] = 0x00
	binary.BigEndian.PutUint16(buf[7:9], uint16(bodyLen))
	buf[9] = 0x03
	buf[10] = 0x03 // TLS 1.2
	// 11..43: 32 байта Random (оставляем ненулевым паттерном)
	for i := 11; i < 43; i++ {
		buf[i] = byte(i * 7)
	}
	pos := 43
	buf[pos] = 0x00 // SessionID len = 0
	pos++
	binary.BigEndian.PutUint16(buf[pos:pos+2], 2) // CipherSuites len = 2
	binary.BigEndian.PutUint16(buf[pos+2:pos+4], 0x1301)
	pos += 4
	buf[pos] = 0x01 // Compression methods len = 1
	buf[pos+1] = 0x00
	pos += 2

	binary.BigEndian.PutUint16(buf[pos:pos+2], uint16(extsLen))
	pos += 2
	// Extension: server_name (0x0000)
	binary.BigEndian.PutUint16(buf[pos:pos+2], 0x0000)
	binary.BigEndian.PutUint16(buf[pos+2:pos+4], uint16(sniExtLen))
	pos += 4
	binary.BigEndian.PutUint16(buf[pos:pos+2], uint16(sniExtLen-2))
	buf[pos+2] = 0x00 // host_name
	binary.BigEndian.PutUint16(buf[pos+3:pos+5], uint16(len(sniBytes)))
	copy(buf[pos+5:], sniBytes)

	return buf
}

// buildTLSRecord оборачивает полезную нагрузку в заголовок TLS-записи с версией из origHeader[1:3].
func buildTLSRecord(verMajor, verMinor byte, payload []byte) []byte {
	rec := make([]byte, 5+len(payload))
	rec[0] = 0x16
	rec[1] = verMajor
	rec[2] = verMinor
	binary.BigEndian.PutUint16(rec[3:5], uint16(len(payload)))
	copy(rec[5:], payload)
	return rec
}

// SplitTLSRecordAndSend реализует Стратегию 0 (tlsrec_sni_split):
// разбивает один TLS ClientHello Record на две валидные TLS-записи прямо посередине поля SNI
// и отправляет их отдельными TCP-сегментами (TCP_NODELAY).
func SplitTLSRecordAndSend(dst net.Conn, data []byte, splitPos int, sni string, sniOffset int) error {
	if tcpConn, ok := dst.(*net.TCPConn); ok {
		_ = tcpConn.SetNoDelay(true)
	}

	if len(data) >= 5 && data[0] == 0x16 {
		recLen := int(binary.BigEndian.Uint16(data[3:5]))
		if 5+recLen <= len(data) {
			cutInData := 0
			if sniOffset > 5 && len(sni) > 2 && sniOffset+len(sni)/2 < 5+recLen {
				cutInData = sniOffset + len(sni)/2
			} else if splitPos > 0 && 5+splitPos < 5+recLen {
				cutInData = 5 + splitPos
			}
			if cutInData > 5 && cutInData < 5+recLen {
				part1 := data[5:cutInData]
				part2 := data[cutInData : 5+recLen]
				tail := data[5+recLen:]

				rec1 := buildTLSRecord(data[1], data[2], part1)
				rec2 := buildTLSRecord(data[1], data[2], part2)
				if len(tail) > 0 {
					rec2 = append(rec2, tail...)
				}

				if len(rec1) > 2 {
					if _, err := dst.Write(rec1[:2]); err != nil {
						return err
					}
					time.Sleep(1 * time.Millisecond)
					if _, err := dst.Write(rec1[2:]); err != nil {
						return err
					}
				} else {
					if _, err := dst.Write(rec1); err != nil {
						return err
					}
				}
				time.Sleep(2 * time.Millisecond)
				_, err := dst.Write(rec2)
				return err
			}
		}
	}

	return FragmentAndSend(dst, data, splitPos, sniOffset)
}

// MultiSplitTLSRecordAndSend реализует Стратегию 1 (tlsrec_multisplit):
// разбивает TLS ClientHello на 3 независимые валидные TLS-записи (до начала SNI,
// первая половина SNI, вторая половина SNI + хвост) и отправляет каждую в режиме микросегментации.
func MultiSplitTLSRecordAndSend(dst net.Conn, data []byte, sni string, sniOffset int) error {
	if tcpConn, ok := dst.(*net.TCPConn); ok {
		_ = tcpConn.SetNoDelay(true)
	}

	if len(data) >= 5 && data[0] == 0x16 {
		recLen := int(binary.BigEndian.Uint16(data[3:5]))
		if 5+recLen <= len(data) && sniOffset > 6 && len(sni) >= 4 && sniOffset+len(sni) <= 5+recLen {
			cut1 := sniOffset + 1
			cut2 := sniOffset + len(sni)/2
			if cut2 <= cut1 {
				cut2 = cut1 + 1
			}
			if cut2 < 5+recLen {
				p1 := data[5:cut1]
				p2 := data[cut1:cut2]
				p3 := data[cut2 : 5+recLen]
				tail := data[5+recLen:]

				rec1 := buildTLSRecord(data[1], data[2], p1)
				rec2 := buildTLSRecord(data[1], data[2], p2)
				rec3 := buildTLSRecord(data[1], data[2], p3)
				if len(tail) > 0 {
					rec3 = append(rec3, tail...)
				}

				// 1-й сегмент: первые 2 байта заголовка
				if _, err := dst.Write(rec1[:2]); err != nil {
					return err
				}
				time.Sleep(1 * time.Millisecond)
				if _, err := dst.Write(rec1[2:]); err != nil {
					return err
				}
				time.Sleep(1 * time.Millisecond)
				if _, err := dst.Write(rec2); err != nil {
					return err
				}
				time.Sleep(1 * time.Millisecond)
				_, err := dst.Write(rec3)
				return err
			}
		}
	}
	return SplitTLSRecordAndSend(dst, data, 2, sni, sniOffset)
}

// OOBSplitTLSRecordAndSend реализует Стратегию 2 (oob_tlsrec_split — техника Zapret tpws --oob / ByeDPI):
// разбивает TLS Record посередине SNI и отправляет первую запись вместе с 1 срочным байтом
// TCP Out-of-Band (MSG_OOB). Анализатор ТСПУ склеивает поток вместе с OOB-байтом (получая битый SNI),
// а целевой TLS-сервер игнорирует внеполосный байт.
func OOBSplitTLSRecordAndSend(dst net.Conn, data []byte, sni string, sniOffset int) error {
	if tcpConn, ok := dst.(*net.TCPConn); ok {
		_ = tcpConn.SetNoDelay(true)
	}

	if len(data) >= 5 && data[0] == 0x16 {
		recLen := int(binary.BigEndian.Uint16(data[3:5]))
		if 5+recLen <= len(data) {
			cutInData := 5 + recLen/2
			if sniOffset > 5 && len(sni) > 2 && sniOffset+len(sni)/2 < 5+recLen {
				cutInData = sniOffset + len(sni)/2
			}
			if cutInData > 5 && cutInData < 5+recLen {
				part1 := data[5:cutInData]
				part2 := data[cutInData : 5+recLen]
				tail := data[5+recLen:]

				rec1 := buildTLSRecord(data[1], data[2], part1)
				rec2 := buildTLSRecord(data[1], data[2], part2)
				if len(tail) > 0 {
					rec2 = append(rec2, tail...)
				}

				if err := sendOOB(dst, rec1, '!'); err != nil {
					// Если платформа не поддержала MSG_OOB, откатываемся на MultiSplit
					return MultiSplitTLSRecordAndSend(dst, data, sni, sniOffset)
				}
				time.Sleep(2 * time.Millisecond)
				_, err := dst.Write(rec2)
				return err
			}
		}
	}
	return SplitTLSRecordAndSend(dst, data, 2, sni, sniOffset)
}

// FakeLowTTLSplitAndSend реализует Стратегию 3 (fake_lowttl_multisplit — техника Zapret 2 nfqws2):
// при наличии привилегий (Linux/OpenWrt root) инжектирует фейковый пакет ClientHello (SNI=www.vk.com)
// с битой контрольной суммой (badsum) и малым TTL=4, после чего отправляет реальный ClientHello
// через комбинацию OOB + 3-кратного разбиения TLS-записи.
func FakeLowTTLSplitAndSend(dst net.Conn, data []byte, sni string, sniOffset int) error {
	if tcpConn, ok := dst.(*net.TCPConn); ok {
		_ = tcpConn.SetNoDelay(true)
	}
	fakeHello := BuildFakeClientHello("www.vk.com")
	_ = trySendRawFakeTCP(dst, fakeHello, 4)

	// Дополнительно применяем OOB + MultiSplit
	if len(data) >= 5 && data[0] == 0x16 {
		recLen := int(binary.BigEndian.Uint16(data[3:5]))
		if 5+recLen <= len(data) && sniOffset > 6 && len(sni) >= 4 && sniOffset+len(sni) <= 5+recLen {
			cut1 := sniOffset + 1
			cut2 := sniOffset + len(sni)/2
			if cut2 <= cut1 {
				cut2 = cut1 + 1
			}
			if cut2 < 5+recLen {
				rec1 := buildTLSRecord(data[1], data[2], data[5:cut1])
				rec2 := buildTLSRecord(data[1], data[2], data[cut1:cut2])
				rec3 := buildTLSRecord(data[1], data[2], data[cut2:5+recLen])
				if len(data) > 5+recLen {
					rec3 = append(rec3, data[5+recLen:]...)
				}
				if err := sendOOB(dst, rec1, '!'); err != nil {
					if _, werr := dst.Write(rec1); werr != nil {
						return werr
					}
				}
				time.Sleep(1 * time.Millisecond)
				if _, err := dst.Write(rec2); err != nil {
					return err
				}
				time.Sleep(1 * time.Millisecond)
				_, err := dst.Write(rec3)
				return err
			}
		}
	}
	return OOBSplitTLSRecordAndSend(dst, data, sni, sniOffset)
}

// FragmentAndSend разбивает не-TLS (например, HTTP) пакет на 2 TCP-сегмента.
// Для HTTP-запросов также применяет технику hostcase ("hOsT:") и разрезает имя хоста.
func FragmentAndSend(dst net.Conn, data []byte, splitPos int, hostOffset int) error {
	if tcpConn, ok := dst.(*net.TCPConn); ok {
		_ = tcpConn.SetNoDelay(true)
	}
	buf := make([]byte, len(data))
	copy(buf, data)

	// Если это HTTP-запрос — модифицируем регистр заголовка "Host:" -> "hOsT:" (техника hostcase в Zapret)
	if idx := indexHostHeader(buf); idx != -1 {
		buf[idx+2] = 'h'
		buf[idx+3] = 'O'
		buf[idx+4] = 's'
		buf[idx+5] = 'T'
	}

	cut := splitPos
	if hostOffset > 0 && hostOffset+2 < len(buf) {
		cut = hostOffset + 2
	} else if cut <= 0 || cut >= len(buf) {
		cut = len(buf) / 2
	}
	if cut <= 0 {
		cut = 1
	}

	if _, err := dst.Write(buf[:cut]); err != nil {
		return fmt.Errorf("write frag1: %w", err)
	}
	time.Sleep(1 * time.Millisecond)
	if _, err := dst.Write(buf[cut:]); err != nil {
		return fmt.Errorf("write frag2: %w", err)
	}
	return nil
}

// ApplyStrategy отправляет начальный пакет (TLS ClientHello или HTTP запрос) с выбранной стратегией Zapret 2.
func ApplyStrategy(conn net.Conn, strategy StrategyID, data []byte, splitPos int, sni string, sniOffset int) error {
	isTLS := len(data) >= 5 && data[0] == 0x16
	if !isTLS {
		return FragmentAndSend(conn, data, splitPos, sniOffset)
	}
	switch strategy {
	case StrategyTLSRecSplit:
		return SplitTLSRecordAndSend(conn, data, splitPos, sni, sniOffset)
	case StrategyMultiSplit:
		return MultiSplitTLSRecordAndSend(conn, data, sni, sniOffset)
	case StrategyOOBSplit:
		return OOBSplitTLSRecordAndSend(conn, data, sni, sniOffset)
	case StrategyFakeLowTTLSplit:
		return FakeLowTTLSplitAndSend(conn, data, sni, sniOffset)
	default:
		return SplitTLSRecordAndSend(conn, data, splitPos, sni, sniOffset)
	}
}

// SendUDPWithFakeBurst отправляет серию из 2 фейковых UDP-пакетов (Fake QUIC/STUN) с малым IP_TTL=3,
// после чего восстанавливает IP_TTL=64 и отправляет реальный UDP-пакет.
// Позволяет обходить эвристические UDP-блокировки ТСПУ (например, Discord Voice / STUN / QUIC).
func SendUDPWithFakeBurst(pc net.PacketConn, dst net.Addr, payload []byte) (int, error) {
	fakePkt := make([]byte, 64)
	fakePkt[0] = 0xc3 // Похоже на заголовок QUIC Initial (Long Header)
	fakePkt[1] = 0x00
	fakePkt[2] = 0x00
	fakePkt[3] = 0x00
	fakePkt[4] = 0x01

	if err := setPacketConnTTL(pc, 3); err == nil {
		_, _ = pc.WriteTo(fakePkt, dst)
		_, _ = pc.WriteTo(fakePkt, dst)
		_ = setPacketConnTTL(pc, 64)
	}
	return pc.WriteTo(payload, dst)
}

type failCounter struct {
	mu       sync.Mutex
	count    int
	lastFail time.Time
}

func (fc *failCounter) inc() int {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	now := time.Now()
	if now.Sub(fc.lastFail) > 10*time.Minute {
		fc.count = 0
	}
	fc.count++
	fc.lastFail = now
	return fc.count
}

type winnerEntry struct {
	strategy  StrategyID
	expiresAt time.Time
}

// Engine управляет автоподбором стратегий Zapret 2 (zapret-auto) и иерархическим кэшем отката на VPN.
type Engine struct {
	vpnFallbackCache sync.Map // ключ: exactSNI, IP, "/24", "*.baseDomain" -> time.Time
	wildcardFails    sync.Map // ключ: baseDomain -> *failCounter
	winnerStrategies sync.Map // ключ: baseDomain или exactSNI -> winnerEntry
	Logf             func(format string, args ...interface{})
}

// GlobalEngine — общий экземпляр движка Zapret 2 внутри процесса.
var GlobalEngine = NewEngine(nil)

func NewEngine(logf func(string, ...interface{})) *Engine {
	return &Engine{Logf: logf}
}

func (e *Engine) log(format string, args ...interface{}) {
	if e != nil && e.Logf != nil {
		e.Logf(format, args...)
	}
}

func ipSubnet24(ipStr string) string {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return ""
	}
	ip4 := ip.To4()
	if ip4 == nil {
		return ""
	}
	return fmt.Sprintf("%d.%d.%d.0/24", ip4[0], ip4[1], ip4[2])
}

func (e *Engine) checkKeyActive(key string, now time.Time) bool {
	if key == "" {
		return false
	}
	if expVal, ok := e.vpnFallbackCache.Load(key); ok {
		if expTime, ok := expVal.(time.Time); ok {
			if now.Before(expTime) {
				return true
			}
			e.vpnFallbackCache.Delete(key)
		}
	}
	return false
}

// IsFallbackActive проверяет, должен ли данный хост/IP/базовый домен идти сразу через VPN.
func (e *Engine) IsFallbackActive(exactSNI, ipStr, baseDomain string) bool {
	now := time.Now()
	if e.checkKeyActive(exactSNI, now) {
		return true
	}
	if e.checkKeyActive(ipStr, now) {
		return true
	}
	if sub := ipSubnet24(ipStr); sub != "" && e.checkKeyActive(sub, now) {
		return true
	}
	if baseDomain != "" && e.checkKeyActive("*."+baseDomain, now) {
		return true
	}
	return false
}

// IsIPFallbackActive проверяет по IP-адресу назначения, помечен ли этот узел/подсеть для принудительного прохода через VPN.
func (e *Engine) IsIPFallbackActive(ip net.IP) bool {
	if ip == nil {
		return false
	}
	ipStr := ip.String()
	return e.IsFallbackActive("", ipStr, "")
}

// MarkIPFallback принудительно переводит IP-адрес (и при необходимости домен / подсеть)
// в кэш VPN-фоллбэка (например, при обнаружении троттлинга скорости на прямом канале).
func (e *Engine) MarkIPFallback(ipStr, sni string) {
	now := time.Now()
	if ipStr != "" {
		e.vpnFallbackCache.Store(ipStr, now.Add(ExactHostBlockedTTL))
		if sub := ipSubnet24(ipStr); sub != "" {
			e.vpnFallbackCache.Store(sub, now.Add(WildcardBlockedTTL))
		}
	}
	if sni != "" {
		e.vpnFallbackCache.Store(sni, now.Add(ExactHostBlockedTTL))
		if base := MatchBaseDomain(sni); base != "" {
			e.PromoteDomainGroupToVPN(base, now)
		}
	}
}

// RecordFailure фиксирует отказ всех стратегий Zapret 2 для данного SNI/IP и переводит его на VPN.
// При 2+ сбоях активирует wildcard и продвигает всю группу сервисов (например, все домены YouTube)
// в VPN fast-path через PromoteDomainGroupToVPN.
func (e *Engine) RecordFailure(exactSNI, ipStr, baseDomain string) {
	now := time.Now()
	if exactSNI != "" {
		e.vpnFallbackCache.Store(exactSNI, now.Add(ExactHostBlockedTTL))
	}
	if ipStr != "" {
		e.vpnFallbackCache.Store(ipStr, now.Add(ExactHostBlockedTTL))
	}
	if baseDomain != "" {
		val, _ := e.wildcardFails.LoadOrStore(baseDomain, &failCounter{})
		fc := val.(*failCounter)
		fails := fc.inc()
		if fails >= WildcardFailThreshold {
			wildcardKey := "*." + baseDomain
			e.vpnFallbackCache.Store(wildcardKey, now.Add(WildcardBlockedTTL))
			if sub := ipSubnet24(ipStr); sub != "" {
				e.vpnFallbackCache.Store(sub, now.Add(WildcardBlockedTTL))
			}
			e.log("[zapret2] %d сбоев прямого обхода для %s -> активирован бесшовный VPN-фоллбэк %q на %v",
				fails, baseDomain, wildcardKey, WildcardBlockedTTL)
			// Продвигаем всю группу (YouTube → все CDN-домены) в VPN fast-path.
			e.PromoteDomainGroupToVPN(baseDomain, now)
		}
	}
}

// relatedServiceDomains возвращает все домены одной группы сервиса для данного baseDomain.
// Используется для группового перевода в VPN при блокировке любого из доменов группы.
func relatedServiceDomains(baseDomain string) []string {
	ytGroup := []string{
		"youtube.com", "googlevideo.com", "ytimg.com", "ggpht.com",
		"youtu.be", "youtubei.googleapis.com", "yt3.ggpht.com", "wide-youtube.l.google.com",
	}
	for _, d := range ytGroup {
		if baseDomain == d {
			return ytGroup
		}
	}
	igGroup := []string{
		"instagram.com", "cdninstagram.com", "fbcdn.net",
		"facebook.com", "twitter.com", "x.com", "twimg.com", "t.co",
	}
	for _, d := range igGroup {
		if baseDomain == d {
			return igGroup
		}
	}
	dcGroup := []string{
		"discord.com", "discordapp.com", "discordapp.net",
		"discord.gg", "discord.media", "discordapp.io", "dis.gd",
	}
	for _, d := range dcGroup {
		if baseDomain == d {
			return dcGroup
		}
	}
	return []string{baseDomain}
}

// PromoteDomainGroupToVPN переводит всю группу доменов заблокированного сервиса
// (например, YouTube: youtube.com + googlevideo.com + ytimg.com + ...) в wildcard VPN-фоллбэк.
// Это устраняет повторные 4-секундные таймауты на каждом новом CDN IP-адресе группы.
func (e *Engine) PromoteDomainGroupToVPN(baseDomain string, now time.Time) {
	group := relatedServiceDomains(baseDomain)
	promoted := false
	for _, d := range group {
		if _, loaded := e.vpnFallbackCache.LoadOrStore("*."+d, now.Add(WildcardBlockedTTL)); !loaded {
			promoted = true
		}
	}
	if promoted {
		e.log("[zapret2] Группа доменов %v переведена в VPN fast-path (wildcard) на %v", group, WildcardBlockedTTL)
	}
}

// RecordDNSResolvedIPs записывает IPv4-адреса из DNS-ответа в кэш VPN-фоллбэка,
// если домен относится к GeoBlocked, ForceVPN (YouTube/GoogleVideo) или уже находится в VPN-кэше.
// Вызывается как для DoH-ответов (спящий VPS), так и для входящих UDP:53 ответов от активного VPS.
// Это обеспечивает мгновенный ядерный L3 fast-path уже с самого первого TCP SYN.
func (e *Engine) RecordDNSResolvedIPs(domain string, ips []net.IP) {
	domain = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(domain), "."))
	if domain == "" || len(ips) == 0 {
		return
	}
	baseDomain := MatchBaseDomain(domain)
	forceVPN := IsForceVPNDomain(domain)
	geoBlocked := IsGeoBlockedSNI(domain)
	if !forceVPN && !geoBlocked && !e.IsFallbackActive(domain, "", baseDomain) {
		return
	}
	now := time.Now()
	if forceVPN && baseDomain != "" {
		e.PromoteDomainGroupToVPN(baseDomain, now)
	}
	for _, ip := range ips {
		if ip4 := ip.To4(); ip4 != nil {
			ipStr := ip4.String()
			e.vpnFallbackCache.Store(ipStr, now.Add(ExactHostBlockedTTL))
			// Для CDN googlevideo.com кэшируем также /24 подсеть, т.к. узлы кластера ротируются.
			if baseDomain == "googlevideo.com" {
				if sub := ipSubnet24(ipStr); sub != "" {
					e.vpnFallbackCache.Store(sub, now.Add(WildcardBlockedTTL))
				}
			}
		}
	}
}

// PreferredStrategy возвращает запомненную рабочую стратегию для домена (или StrategyTLSRecSplit по умолчанию).
func (e *Engine) PreferredStrategy(key string) StrategyID {
	if key == "" {
		return StrategyTLSRecSplit
	}
	if v, ok := e.winnerStrategies.Load(key); ok {
		we := v.(winnerEntry)
		if time.Now().Before(we.expiresAt) {
			return we.strategy
		}
		e.winnerStrategies.Delete(key)
	}
	return StrategyTLSRecSplit
}

// RecordSuccess сохраняет рабочую стратегию Zapret 2 для домена.
func (e *Engine) RecordSuccess(key string, s StrategyID) {
	if key == "" {
		return
	}
	e.winnerStrategies.Store(key, winnerEntry{
		strategy:  s,
		expiresAt: time.Now().Add(WinnerStrategyTTL),
	})
}

// CandidateStrategies возвращает упорядоченный список стратегий для перебора (zapret-auto).
// Начинает с запомненной рабочей стратегии для этого домена, затем пробует более агрессивные стратегии.
func (e *Engine) CandidateStrategies(domainKey string, maxAttempts int) []StrategyID {
	pref := e.PreferredStrategy(domainKey)
	all := []StrategyID{
		pref,
		StrategyOOBSplit,
		StrategyFakeLowTTLSplit,
		StrategyMultiSplit,
		StrategyTLSRecSplit,
	}
	seen := make(map[StrategyID]bool, strategyCount)
	out := make([]StrategyID, 0, strategyCount)
	for _, s := range all {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
			if maxAttempts > 0 && len(out) >= maxAttempts {
				break
			}
		}
	}
	return out
}

// isInvalidServerResponse проверяет, не вернул ли сервер/ТСПУ TLS Alert (0x15) или страницу блокировки РКН.
func isInvalidServerResponse(isTLS bool, reply []byte) bool {
	if len(reply) == 0 {
		return true
	}
	if isTLS {
		// Валидный ответ TLS сервера должен начинаться с Handshake (0x16) или ChangeCipherSpec (0x14).
		// Если пришёл Alert (0x15) или HTTP редирект от ТСПУ ('H' = 0x48) — обход не сработал.
		if reply[0] != 0x16 && reply[0] != 0x14 {
			return true
		}
		return false
	}
	// Для HTTP проверяем 451 Unavailable For Legal Reasons или редирект на заглушку провайдера
	if bytes.HasPrefix(reply, []byte("HTTP/1.1 451")) || bytes.HasPrefix(reply, []byte("HTTP/1.0 451")) {
		return true
	}
	return false
}

// DialAndDesync пытается установить прямое WAN-соединение и пробить ТСПУ каскадом стратегий Zapret 2.
// Возвращает:
//   - remoteConn: открытое соединение (если прямой обход успешен)
//   - firstReply: первые прочитанные байты ответа сервера (например, ServerHello), которые нужно переслать клиенту
//   - err: ошибка (если все стратегии Zapret 2 не пробили блокировку и нужен бесшовный фоллбэк на VPN)
func (e *Engine) DialAndDesync(
	ctx context.Context,
	dialFunc func(ctx context.Context, network, addr string) (net.Conn, error),
	targetAddr string,
	payload []byte,
	splitPos int,
	probeTimeout time.Duration,
) (net.Conn, []byte, error) {
	hostIP, _, _ := net.SplitHostPort(targetAddr)
	sni, offset := ExtractSNI(payload)
	isTLS := sni != "" || (len(payload) > 5 && payload[0] == 0x16 && payload[5] == 0x01)
	if !isTLS {
		sni, offset = ExtractHTTPHost(payload)
	}
	baseDomain := MatchBaseDomain(sni)
	domainKey := baseDomain
	if domainKey == "" {
		domainKey = hostIP
	}

	if IsGeoBlockedSNI(sni) || IsForceVPNDomain(sni) {
		now := time.Now()
		if hostIP != "" {
			e.vpnFallbackCache.Store(hostIP, now.Add(ExactHostBlockedTTL))
			if baseDomain == "googlevideo.com" {
				if sub := ipSubnet24(hostIP); sub != "" {
					e.vpnFallbackCache.Store(sub, now.Add(WildcardBlockedTTL))
				}
			}
		}
		if sni != "" {
			e.vpnFallbackCache.Store(sni, now.Add(ExactHostBlockedTTL))
		}
		if IsForceVPNDomain(sni) && baseDomain != "" {
			e.PromoteDomainGroupToVPN(baseDomain, now)
		}
		if IsForceVPNDomain(sni) {
			return nil, nil, errors.New("потоковый CDN подвержен троттлингу ТСПУ -> VPN fast-path")
		}
		return nil, nil, errors.New("сервис требует иностранный IP (GeoIP-блок для РФ)")
	}

	if e.IsFallbackActive(sni, hostIP, baseDomain) {
		// Если сработал wildcard по домену, но сам IP ещё не в кэше — записываем IP,
		// чтобы следующее соединение сразу прошло через ядерный L3 fast-path без перехвата.
		if hostIP != "" {
			e.vpnFallbackCache.Store(hostIP, time.Now().Add(ExactHostBlockedTTL))
		}
		return nil, nil, errors.New("активен кэш VPN-фоллбэка для узла")
	}

	if probeTimeout <= 0 {
		probeTimeout = DefaultWanProbeTimeout
	}

	// Пробуем до 2 стратегий из каскада zapret-auto перед переключением на VPN,
	// чтобы при смене правил ТСПУ автоматически находить рабочую стратегию без участия VPS.
	strategies := e.CandidateStrategies(domainKey, 2)
	var lastErr error

	for _, strat := range strategies {
		dialStart := time.Now()
		conn, err := dialFunc(ctx, "tcp", targetAddr)
		if err != nil {
			lastErr = err
			// Если сам TCP SYN не проходит (IP заблокирован на уровне маршрутизации ТСПУ) —
			// нет смысла перебирать остальные L7-стратегии, сразу переключаемся на VPN.
			break
		}
		synRTT := time.Since(dialStart)

		sendErr := ApplyStrategy(conn, strat, payload, splitPos, sni, offset)
		if sendErr != nil {
			_ = conn.Close()
			lastErr = sendErr
			continue
		}

		adaptiveTimeout := probeTimeout
		if minAdaptive := time.Duration(float64(synRTT) * 2.5); minAdaptive > adaptiveTimeout {
			adaptiveTimeout = minAdaptive
		}

		replyBuf := make([]byte, 16*1024)
		_ = conn.SetReadDeadline(time.Now().Add(adaptiveTimeout))
		rn, rerr := conn.Read(replyBuf)
		_ = conn.SetReadDeadline(time.Time{})

		if rn > 0 && !isInvalidServerResponse(isTLS, replyBuf[:rn]) {
			e.RecordSuccess(domainKey, strat)
			if sni != "" {
				e.log("[zapret2] Прямой обход успешен для %q -> %s (стратегия=%s, RTT=%v)",
					sni, targetAddr, strat.String(), synRTT.Round(time.Millisecond))
			}
			return conn, replyBuf[:rn], nil
		}

		_ = conn.Close()
		if rerr != nil {
			lastErr = rerr
		} else if rn == 0 {
			lastErr = io.EOF
		} else {
			lastErr = errors.New("получен TLS Alert или заглушка ТСПУ")
		}
	}

	e.RecordFailure(sni, hostIP, baseDomain)
	if lastErr == nil {
		lastErr = errors.New("прямой обход ТСПУ не удался")
	}
	return nil, nil, lastErr
}
