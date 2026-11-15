package zapret

import (
	"bytes"
	"context"
	"encoding/binary"
	"net"
	"testing"
	"time"
)

func TestBuildAndExtractSNI(t *testing.T) {
	ch := BuildFakeClientHello("www.instagram.com")
	sni, offset := ExtractSNI(ch)
	if sni != "www.instagram.com" {
		t.Fatalf("expected sni 'www.instagram.com', got %q", sni)
	}
	if offset <= 5 || offset+len(sni) > len(ch) {
		t.Fatalf("invalid offset %d for len %d", offset, len(ch))
	}
}

func TestMultiSplitTLSRecordsValid(t *testing.T) {
	ch := BuildFakeClientHello("rr1---sn-gvnuxaxjvh-n8me.googlevideo.com")
	sni, offset := ExtractSNI(ch)

	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	var received bytes.Buffer
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 4096)
		for {
			n, err := c2.Read(buf)
			if n > 0 {
				received.Write(buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()

	if err := MultiSplitTLSRecordAndSend(c1, ch, sni, offset); err != nil {
		t.Fatalf("MultiSplitTLSRecordAndSend failed: %v", err)
	}
	_ = c1.Close()
	<-done

	// Проверяем, что получено ровно 3 валидных TLS Record, которые при склейке payload дают исходный ClientHello
	raw := received.Bytes()
	var reassembled []byte
	records := 0
	for pos := 0; pos < len(raw); {
		if pos+5 > len(raw) {
			t.Fatalf("truncated record header at %d", pos)
		}
		if raw[pos] != 0x16 {
			t.Fatalf("expected 0x16 handshake record at %d, got 0x%02x", pos, raw[pos])
		}
		rlen := int(binary.BigEndian.Uint16(raw[pos+3 : pos+5]))
		if pos+5+rlen > len(raw) {
			t.Fatalf("record length %d exceeds buffer at %d", rlen, pos)
		}
		reassembled = append(reassembled, raw[pos+5:pos+5+rlen]...)
		pos += 5 + rlen
		records++
	}
	if records != 3 {
		t.Fatalf("expected 3 TLS records from MultiSplit, got %d", records)
	}
	if !bytes.Equal(reassembled, ch[5:]) {
		t.Fatalf("reassembled ClientHello body does not match original")
	}
}

func TestAutoStrategyPromotionAndFallback(t *testing.T) {
	eng := NewEngine(nil)
	ch := BuildFakeClientHello("www.instagram.com")

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer ln.Close()

	// Сценарий 1: первая стратегия (StrategyTLSRecSplit) сбрасывается (возвращает TLS Alert 0x15),
	// а вторая стратегия (StrategyOOBSplit) возвращает валидный ServerHello (0x16 0x03 0x03 ...).
	calls := 0
	go func() {
		for {
			cServer, err := ln.Accept()
			if err != nil {
				return
			}
			calls++
			attempt := calls
			go func(c net.Conn, att int) {
				defer c.Close()
				buf := make([]byte, 2048)
				_ = c.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
				// Даём клиенту дописать все сегменты
				time.Sleep(15 * time.Millisecond)
				_, _ = c.Read(buf)
				if att == 1 {
					// Имитируем блокировку первой стратегии со стороны ТСПУ (TLS Alert)
					_, _ = c.Write([]byte{0x15, 0x03, 0x03, 0x00, 0x02, 0x02, 0x28})
					return
				}
				// На второй стратегии отдаём валидный ServerHello
				_, _ = c.Write([]byte{0x16, 0x03, 0x03, 0x00, 0x05, 0x02, 0x00, 0x00, 0x01, 0x00})
			}(cServer, attempt)
		}
	}()

	dialFunc := func(ctx context.Context, network, addr string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, "tcp", ln.Addr().String())
	}

	conn, reply, err := eng.DialAndDesync(context.Background(), dialFunc, "157.240.20.174:443", ch, 2, 200*time.Millisecond)
	if err != nil {
		t.Fatalf("expected DialAndDesync to succeed on 2nd strategy, got err: %v", err)
	}
	_ = conn.Close()
	if len(reply) == 0 || reply[0] != 0x16 {
		t.Fatalf("expected valid ServerHello reply")
	}
	if calls != 2 {
		t.Fatalf("expected 2 strategy attempts, got %d", calls)
	}
	if pref := eng.PreferredStrategy("instagram.com"); pref != StrategyOOBSplit {
		t.Fatalf("expected StrategyOOBSplit to be promoted as preferred for instagram.com, got %v", pref)
	}
}
