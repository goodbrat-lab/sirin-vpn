package engine

import (
	"bytes"
	"crypto/rand"
	"net"
	"testing"
	"time"
)

type dummyPacketConn struct {
	readCh  chan []byte
	writeCh chan []byte
	closed  bool
}

func newDummyPacketConn() *dummyPacketConn {
	return &dummyPacketConn{
		readCh:  make(chan []byte, 10),
		writeCh: make(chan []byte, 10),
	}
}

func (d *dummyPacketConn) ReadFrom(p []byte) (int, net.Addr, error) {
	data, ok := <-d.readCh
	if !ok {
		return 0, nil, net.ErrClosed
	}
	n := copy(p, data)
	return n, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 56005}, nil
}

func (d *dummyPacketConn) WriteTo(p []byte, addr net.Addr) (int, error) {
	buf := make([]byte, len(p))
	copy(buf, p)
	d.writeCh <- buf
	return len(p), nil
}

func (d *dummyPacketConn) Close() error {
	if !d.closed {
		d.closed = true
		close(d.readCh)
	}
	return nil
}

func (d *dummyPacketConn) LocalAddr() net.Addr                { return &net.UDPAddr{Port: 12345} }
func (d *dummyPacketConn) SetDeadline(t time.Time) error      { return nil }
func (d *dummyPacketConn) SetReadDeadline(t time.Time) error  { return nil }
func (d *dummyPacketConn) SetWriteDeadline(t time.Time) error { return nil }

func TestObfsWrapAndUnwrap(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}

	sender, err := newObfsState(key)
	if err != nil {
		t.Fatalf("newObfsState: %v", err)
	}
	receiver, err := newObfsState(key)
	if err != nil {
		t.Fatalf("newObfsState receiver: %v", err)
	}

	payload := []byte("Hello, Sirin Obfuscation 2026/2027!")
	wire, err := sender.wrap(payload)
	if err != nil {
		t.Fatalf("wrap failed: %v", err)
	}

	dst := make([]byte, 2048)
	n, ok := receiver.unwrap(wire, dst)
	if !ok {
		t.Fatalf("unwrap failed")
	}

	if !bytes.Equal(dst[:n], payload) {
		t.Fatalf("payload mismatch: expected %q, got %q", string(payload), string(dst[:n]))
	}
}

func TestObfsWrapDummyJunkPacket(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}

	sender, err := newObfsState(key)
	if err != nil {
		t.Fatalf("newObfsState: %v", err)
	}
	receiver, err := newObfsState(key)
	if err != nil {
		t.Fatalf("newObfsState receiver: %v", err)
	}

	// 1. Сформировать dummy junk RTP-пакет
	wire, err := sender.wrapDummy(96)
	if err != nil {
		t.Fatalf("wrapDummy failed: %v", err)
	}

	// 2. Распаковка должна вернуть ok=true, но длину 0 (поглощение фиктивного пакета)
	dst := make([]byte, 2048)
	n, ok := receiver.unwrap(wire, dst)
	if !ok {
		t.Fatalf("unwrap dummy failed: expected ok=true")
	}
	if n != 0 {
		t.Fatalf("expected dummy packet to return length 0, got %d", n)
	}
}

func TestUDPPacer_MicrosecondSpacing(t *testing.T) {
	minInterval := 2 * time.Millisecond
	pacer := newUDPPacer(0, minInterval)

	start := time.Now()
	// Send 3 packets back to back
	pacer.Pace(100)
	pacer.Pace(100)
	pacer.Pace(100)
	elapsed := time.Since(start)

	// 3 packets with 2ms spacing between consecutive writes should take at least 3.5ms
	if elapsed < 3*time.Millisecond {
		t.Errorf("expected elapsed time >= 3ms, got %v", elapsed)
	}
}

func TestObfsPacketConn_JunkInjectionAndPacing(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}

	senderState, _ := newObfsState(key)
	receiverState, _ := newObfsState(key)

	dummyConn := newDummyPacketConn()
	targetAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 56005}

	pcSender := newObfsPacketConn(dummyConn, senderState, nil)
	pcReceiver := newObfsPacketConn(dummyConn, receiverState, nil)

	// Включаем шейпинг на клиенте
	pcSender.SetPacing(0, 1*time.Millisecond)

	// 1. Отправляем dummy junk пакет
	if err := pcSender.InjectJunk(targetAddr, 80); err != nil {
		t.Fatalf("InjectJunk failed: %v", err)
	}

	junkWire := <-dummyConn.writeCh
	dummyConn.readCh <- junkWire

	// 2. Отправляем реальный полезный пакет
	realPayload := []byte("REAL_VPN_DATA_STREAM")
	if _, err := pcSender.WriteTo(realPayload, targetAddr); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	realWire := <-dummyConn.writeCh
	dummyConn.readCh <- realWire

	// 3. Читаем из pcReceiver:
	// Первый пакет (junk) должен быть молча поглощён внутри ReadFrom,
	// а возвращён должен быть сразу реальный полезный пакет!
	buf := make([]byte, 2048)
	n, _, err := pcReceiver.ReadFrom(buf)
	if err != nil {
		t.Fatalf("ReadFrom failed: %v", err)
	}

	if !bytes.Equal(buf[:n], realPayload) {
		t.Fatalf("expected real payload %q, got %q", string(realPayload), string(buf[:n]))
	}
}
