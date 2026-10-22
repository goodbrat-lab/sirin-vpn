package vless

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"
)

type dummyConn struct {
	net.Conn
	closed bool
}

func (d *dummyConn) Close() error {
	d.closed = true
	return nil
}

func (d *dummyConn) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}
}

func (d *dummyConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8443}
}

func (d *dummyConn) SetDeadline(t time.Time) error      { return nil }
func (d *dummyConn) SetReadDeadline(t time.Time) error  { return nil }
func (d *dummyConn) SetWriteDeadline(t time.Time) error { return nil }

func TestXHTTPConnReadHeaderStripping(t *testing.T) {
	// Simulated server response: [version=0][addon_len=0] + payload "hello xhttp"
	respData := append([]byte{0x00, 0x00}, []byte("hello xhttp")...)
	respBody := io.NopCloser(bytes.NewReader(respData))

	pr, pw := io.Pipe()
	defer pr.Close()
	defer pw.Close()

	dConn := &dummyConn{}
	conn := &XHTTPConn{
		uConn:    dConn,
		pw:       pw,
		respBody: respBody,
	}

	buf := make([]byte, 64)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if string(buf[:n]) != "hello xhttp" {
		t.Fatalf("expected 'hello xhttp', got %q", string(buf[:n]))
	}

	if !conn.responseRead {
		t.Fatalf("expected responseRead to be true")
	}

	_ = conn.Close()
	if !dConn.closed {
		t.Fatalf("expected dummyConn to be closed")
	}
}
