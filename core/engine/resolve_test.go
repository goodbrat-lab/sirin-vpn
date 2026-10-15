package engine

import (
	"encoding/binary"
	"testing"
)

func TestBuildDNSQueryShape(t *testing.T) {
	q := buildDNSQuery("edge.example.org")

	if len(q) < 12 {
		t.Fatalf("запрос короче заголовка: %d байт", len(q))
	}
	if flags := binary.BigEndian.Uint16(q[2:4]); flags != 0x0100 {
		t.Errorf("флаги = %#04x, хочу RD=1 (0x0100)", flags)
	}
	if qd := binary.BigEndian.Uint16(q[4:6]); qd != 1 {
		t.Errorf("QDCOUNT = %d, хочу 1", qd)
	}
	if an := binary.BigEndian.Uint16(q[6:8]); an != 0 {
		t.Errorf("ANCOUNT = %d, хочу 0 в запросе", an)
	}

	// Имя закодировано метками: 4"edge" 7"example" 3"org" 0.
	off := 12
	wantLabels := []string{"edge", "example", "org"}
	for _, label := range wantLabels {
		if int(q[off]) != len(label) {
			t.Fatalf("метка на смещении %d: длина %d, хочу %d (%q)", off, q[off], len(label), label)
		}
		got := string(q[off+1 : off+1+len(label)])
		if got != label {
			t.Fatalf("метка на смещении %d: %q, хочу %q", off, got, label)
		}
		off += 1 + len(label)
	}
	if q[off] != 0 {
		t.Errorf("имя не завершено нулевым байтом на смещении %d", off)
	}
	off++
	if qtype := binary.BigEndian.Uint16(q[off : off+2]); qtype != 1 {
		t.Errorf("QTYPE = %d, хочу 1 (A)", qtype)
	}
	if qclass := binary.BigEndian.Uint16(q[off+2 : off+4]); qclass != 1 {
		t.Errorf("QCLASS = %d, хочу 1 (IN)", qclass)
	}
}

// buildTestResponse строит валидный DNS-ответ (RFC 1035) на запрос host:
// вопрос как в реальном запросе, один ответ типа rtype с телом rdata, имя
// ответа — указатель сжатия на вопрос (0xC00C), как отвечают все живые
// резолверы.
func buildTestResponse(t *testing.T, host string, rtype uint16, rdata []byte) []byte {
	t.Helper()
	q := buildDNSQuery(host)

	resp := make([]byte, len(q))
	copy(resp, q)
	binary.BigEndian.PutUint16(resp[6:8], 1) // ANCOUNT = 1

	resp = append(resp, 0xC0, 0x0C) // указатель на имя вопроса (смещение 12)
	var typeBuf, classBuf [2]byte
	binary.BigEndian.PutUint16(typeBuf[:], rtype)
	binary.BigEndian.PutUint16(classBuf[:], 1) // CLASS IN
	resp = append(resp, typeBuf[:]...)
	resp = append(resp, classBuf[:]...)
	resp = append(resp, 0, 0, 0, 60) // TTL, значение неважно тесту
	var rdlen [2]byte
	binary.BigEndian.PutUint16(rdlen[:], uint16(len(rdata)))
	resp = append(resp, rdlen[:]...)
	resp = append(resp, rdata...)
	return resp
}

func TestParseDNSResponseFindsARecord(t *testing.T) {
	resp := buildTestResponse(t, "edge.example.org", 1, []byte{192, 0, 2, 1})

	ip, err := parseDNSResponse(resp)
	if err != nil {
		t.Fatalf("parseDNSResponse: %v", err)
	}
	if ip != "192.0.2.1" {
		t.Errorf("ip = %q, хочу 192.0.2.1", ip)
	}
}

func TestParseDNSResponseSkipsNonARecord(t *testing.T) {
	// AAAA (TYPE 28) с телом в 16 байт — parseDNSResponse обязан
	// пропустить её по RDLENGTH и не найти A-запись следом, раз её нет.
	resp := buildTestResponse(t, "edge.example.org", 28, make([]byte, 16))

	_, err := parseDNSResponse(resp)
	if err == nil {
		t.Fatal("ожидал ошибку: в ответе нет A-записи")
	}
}

func TestParseDNSResponseTruncated(t *testing.T) {
	_, err := parseDNSResponse([]byte{1, 2, 3})
	if err == nil {
		t.Fatal("ожидал ошибку на обрезанном ответе")
	}
}

func TestIsPrivateOrSpecial(t *testing.T) {
	cases := map[string]bool{
		"192.0.2.1":     false,
		"198.51.100.1":  false,
		"1.1.1.1":       false,
		"10.0.0.1":      true,
		"172.16.5.5":    true,
		"192.168.1.1":   true,
		"127.0.0.1":     true,
		"169.254.1.1":   true,
		"0.0.0.0":       true,
		"не-ip-адрес":   true,
	}
	for addr, wantPrivate := range cases {
		if got := isPrivateOrSpecial(addr); got != wantPrivate {
			t.Errorf("isPrivateOrSpecial(%q) = %v, хочу %v", addr, got, wantPrivate)
		}
	}
}

func TestResolveGatewaySkipsResolutionForLiteralIP(t *testing.T) {
	addr, via, err := resolveGateway("198.51.100.1", "", nil, nil)
	if err != nil {
		t.Fatalf("resolveGateway на IP-литерале вернул ошибку: %v", err)
	}
	if addr != "198.51.100.1" || via != "" {
		t.Errorf("addr=%q via=%q, хочу addr без изменений и пустой via", addr, via)
	}
}
