package main

import (
	"bytes"
	"encoding/binary"
	"net"
	"strings"
	"testing"
)

// makeTestClientHello генерирует валидный ClientHello с заданным SNI для тестов парсинга и фрагментации.
func makeTestClientHello(sni string) []byte {
	var buf bytes.Buffer

	// TLS Record Header (5 bytes)
	buf.WriteByte(0x16)         // Handshake
	buf.Write([]byte{0x03, 0x01}) // TLS 1.0 (Record Version)
	buf.Write([]byte{0x00, 0x00}) // Placeholder for Record Length

	// Handshake Header (4 bytes)
	handshakeStart := buf.Len()
	buf.WriteByte(0x01)                 // ClientHello
	buf.Write([]byte{0x00, 0x00, 0x00}) // Placeholder for Handshake Length

	// Handshake Body
	buf.Write([]byte{0x03, 0x03}) // Client Version (TLS 1.2)
	buf.Write(make([]byte, 32))   // Random (32 bytes)
	buf.WriteByte(0x00)           // Session ID Length (0)

	// Cipher Suites
	buf.Write([]byte{0x00, 0x02, 0x13, 0x01}) // 1 suite: TLS_AES_128_GCM_SHA256

	// Compression Methods
	buf.Write([]byte{0x01, 0x00}) // 1 method: null

	// Extensions Length placeholder
	extStart := buf.Len()
	buf.Write([]byte{0x00, 0x00})

	// Server Name Extension (SNI)
	buf.Write([]byte{0x00, 0x00}) // Extension Type: server_name (0x0000)
	sniExtLenPlaceholder := buf.Len()
	buf.Write([]byte{0x00, 0x00}) // Extension Length placeholder

	sniListStart := buf.Len()
	// Server Name List Length
	sniBytes := []byte(sni)
	serverNameListLen := uint16(3 + len(sniBytes))
	var lenBuf [2]byte
	binary.BigEndian.PutUint16(lenBuf[:], serverNameListLen)
	buf.Write(lenBuf[:])

	// Server Name Type (0 = host_name)
	buf.WriteByte(0x00)

	// Host Name Length
	binary.BigEndian.PutUint16(lenBuf[:], uint16(len(sniBytes)))
	buf.Write(lenBuf[:])
	buf.Write(sniBytes)

	// Update SNI Extension Length
	sniExtLen := uint16(buf.Len() - sniListStart)
	allBytes := buf.Bytes()
	binary.BigEndian.PutUint16(allBytes[sniExtLenPlaceholder:sniExtLenPlaceholder+2], sniExtLen)

	// Update Extensions Length
	totalExtLen := uint16(buf.Len() - extStart - 2)
	binary.BigEndian.PutUint16(allBytes[extStart:extStart+2], totalExtLen)

	// Update Handshake Length
	handshakeLen := uint32(buf.Len() - handshakeStart - 4)
	allBytes[handshakeStart+1] = byte(handshakeLen >> 16)
	allBytes[handshakeStart+2] = byte(handshakeLen >> 8)
	allBytes[handshakeStart+3] = byte(handshakeLen)

	// Update Record Length
	recordLen := uint16(buf.Len() - 5)
	binary.BigEndian.PutUint16(allBytes[3:5], recordLen)

	return allBytes
}

func TestExtractSNI(t *testing.T) {
	testCases := []string{
		"googlevideo.com",
		"rr1---sn-ax87enel.googlevideo.com",
		"discord.com",
		"gateway.discord.gg",
		"rutracker.org",
	}

	for _, expectedSNI := range testCases {
		data := makeTestClientHello(expectedSNI)
		gotSNI, offset := ExtractSNI(data)

		if gotSNI != expectedSNI {
			t.Errorf("ExtractSNI: ожидался %q, получено %q", expectedSNI, gotSNI)
		}
		if offset <= 0 || offset >= len(data) {
			t.Errorf("ExtractSNI: некорректное смещение %d для длины %d", offset, len(data))
		}
		// Проверяем, что по указанному смещению действительно лежит имя хоста
		extracted := string(data[offset : offset+len(expectedSNI)])
		if extracted != expectedSNI {
			t.Errorf("Срез по offset %d дает %q, ожидалось %q", offset, extracted, expectedSNI)
		}
	}
}

func TestExtractSNI_InvalidData(t *testing.T) {
	// Пустые данные или мусор
	if sni, _ := ExtractSNI(nil); sni != "" {
		t.Errorf("Ожидался пустой SNI для nil")
	}
	if sni, _ := ExtractSNI([]byte{0x16, 0x03, 0x01}); sni != "" {
		t.Errorf("Ожидался пустой SNI для слишком короткого пакета")
	}
	// Не-TLS трафик (например, HTTP GET)
	httpData := []byte("GET / HTTP/1.1\r\nHost: example.com\r\n\r\n")
	if sni, _ := ExtractSNI(httpData); sni != "" {
		t.Errorf("Ожидался пустой SNI для HTTP запроса")
	}
}

func TestAntiDPIConfig_GetTargetDomains(t *testing.T) {
	cfg := DefaultAntiDPIConfig()
	cfg.CustomDomains = []string{"custom.blocked.site"}

	domains := cfg.GetTargetDomains()

	foundYouTube := false
	foundDiscord := false
	foundCustom := false

	for _, d := range domains {
		if strings.Contains(d, "googlevideo.com") {
			foundYouTube = true
		}
		if strings.Contains(d, "discord.com") {
			foundDiscord = true
		}
		if d == "custom.blocked.site" {
			foundCustom = true
		}
	}

	if !foundYouTube {
		t.Errorf("YouTube домены должны присутствовать при youtube=true")
	}
	if !foundDiscord {
		t.Errorf("Discord домены должны присутствовать при discord=true")
	}
	if !foundCustom {
		t.Errorf("Пользовательские домены должны присутствовать в списке")
	}
}

// mockConn имитирует сетевое соединение и записывает переданные фрагменты
type mockConn struct {
	net.Conn
	chunks [][]byte
}

func (m *mockConn) Write(b []byte) (int, error) {
	chunk := make([]byte, len(b))
	copy(chunk, b)
	m.chunks = append(m.chunks, chunk)
	return len(b), nil
}

func (m *mockConn) SetNoDelay(bool) error { return nil }

func TestFragmentAndSend(t *testing.T) {
	rawClientHello := makeTestClientHello("rr1---sn-ax87enel.googlevideo.com")
	sni, offset := ExtractSNI(rawClientHello)

	mock := &mockConn{}
	err := FragmentAndSend(mock, rawClientHello, 2, offset)
	if err != nil {
		t.Fatalf("FragmentAndSend завершился с ошибкой: %v", err)
	}

	if len(mock.chunks) != 2 {
		t.Fatalf("Ожидалось ровно 2 фрагмента, получено %d", len(mock.chunks))
	}

	// Первый фрагмент должен быть длиной 2 байта
	if len(mock.chunks[0]) != 2 {
		t.Errorf("Ожидался split_pos=2 в первом фрагменте, получено: %d", len(mock.chunks[0]))
	}

	// Сумма фрагментов должна в точности восстанавливать исходный ClientHello
	reconstructed := append(mock.chunks[0], mock.chunks[1]...)
	if !bytes.Equal(reconstructed, rawClientHello) {
		t.Errorf("Восстановленные данные не совпадают с исходными")
	}

	_ = sni
}

func TestSplitTLSRecordAndSend(t *testing.T) {
	sniHost := "www.youtube.com"
	rawClientHello := makeTestClientHello(sniHost)
	sni, offset := ExtractSNI(rawClientHello)

	mock := &mockConn{}
	err := SplitTLSRecordAndSend(mock, rawClientHello, 2, sni, offset)
	if err != nil {
		t.Fatalf("SplitTLSRecordAndSend завершился с ошибкой: %v", err)
	}

	if len(mock.chunks) < 2 {
		t.Fatalf("Ожидалось минимум 2 TCP-сегмента, получено %d", len(mock.chunks))
	}

	// Ни один отдельный сегмент не должен содержать полное имя SNI
	for i, ch := range mock.chunks {
		if bytes.Contains(ch, []byte(sniHost)) {
			t.Errorf("Сегмент %d содержит неразбитый SNI %q", i, sniHost)
		}
	}
}
