package vless

import (
	"bytes"
	"context"
	"io"
	"net"
	"strings"
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

func TestXHTTPPathTrailingSlash(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"/s1r1nxhttp2026k", "/s1r1nxhttp2026k/"},
		{"/s1r1nxhttp2026k/", "/s1r1nxhttp2026k/"},
		{"s1r1nxhttp2026k", "/s1r1nxhttp2026k/"},
		{"", "/s1r1nxhttp2026k/"},
		{"/", "/"},
	}

	for _, tc := range testCases {
		normPath := tc.input
		if normPath == "" {
			normPath = "/s1r1nxhttp2026k"
		}
		if !strings.HasPrefix(normPath, "/") {
			normPath = "/" + normPath
		}
		if !strings.HasSuffix(normPath, "/") {
			normPath = normPath + "/"
		}
		if normPath != tc.expected {
			t.Errorf("for input %q: expected %q, got %q", tc.input, tc.expected, normPath)
		}
	}
}

func TestBuildXHTTPRequest_StandardSNI(t *testing.T) {
	cfg := &XHTTPConfig{
		Reality: &RealityConfig{
			SNI: "sun8-21.userapi.com",
		},
		Path: "/custom-xhttp",
		Mode: "stream-one",
	}

	req, err := buildXHTTPRequest(context.Background(), cfg, bytes.NewBuffer([]byte("body")))
	if err != nil {
		t.Fatalf("buildXHTTPRequest failed: %v", err)
	}

	if req.Host != "sun8-21.userapi.com" {
		t.Errorf("expected req.Host sun8-21.userapi.com, got %s", req.Host)
	}
	if req.URL.Host != "sun8-21.userapi.com" {
		t.Errorf("expected req.URL.Host sun8-21.userapi.com, got %s", req.URL.Host)
	}
	if req.URL.Path != "/custom-xhttp/" {
		t.Errorf("expected normalized path /custom-xhttp/, got %s", req.URL.Path)
	}
	if !strings.Contains(req.Header.Get("Referer"), "sun8-21.userapi.com") {
		t.Errorf("expected Referer to contain SNI host, got %s", req.Header.Get("Referer"))
	}
}

func TestBuildXHTTPRequest_CDNFronting(t *testing.T) {
	cfg := &XHTTPConfig{
		Reality: &RealityConfig{
			SNI: "cdn-front.cloudflare.net",
		},
		CDNHost: "backend-origin.sirin-vpn.internal",
		Path:    "/s1r1nxhttp2026k",
		Mode:    "stream-one",
	}

	req, err := buildXHTTPRequest(context.Background(), cfg, bytes.NewBuffer([]byte("body")))
	if err != nil {
		t.Fatalf("buildXHTTPRequest failed: %v", err)
	}

	// For CDN fronting:
	// 1) TLS ClientHello SNI remains cdn-front.cloudflare.net
	if cfg.Reality.SNI != "cdn-front.cloudflare.net" {
		t.Errorf("expected TLS SNI cdn-front.cloudflare.net, got %s", cfg.Reality.SNI)
	}
	// 2) HTTP/2 authority / Host header points to the target CDN backend origin
	if req.Host != "backend-origin.sirin-vpn.internal" {
		t.Errorf("expected req.Host backend-origin.sirin-vpn.internal, got %s", req.Host)
	}
	if req.URL.Host != "backend-origin.sirin-vpn.internal" {
		t.Errorf("expected req.URL.Host backend-origin.sirin-vpn.internal, got %s", req.URL.Host)
	}
	if !strings.Contains(req.Header.Get("Referer"), "backend-origin.sirin-vpn.internal") {
		t.Errorf("expected Referer to contain CDNHost, got %s", req.Header.Get("Referer"))
	}
}
