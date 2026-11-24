package vless

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/http2"
)

// XHTTPConfig содержит параметры для соединения VLESS через XHTTP (stream-one).
type XHTTPConfig struct {
	Reality *RealityConfig
	Path    string
	Mode    string
	CDNHost string // хост назначения за CDN (для :authority и Host), при этом SNI в TLS остаётся фронт-доменом
}

// buildXHTTPRequest формирует HTTP/2 POST запрос для XHTTP соединения с учётом CDN фронтинга.
func buildXHTTPRequest(ctx context.Context, cfg *XHTTPConfig, pr io.Reader) (*http.Request, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	sni := ""
	if cfg.Reality != nil {
		sni = cfg.Reality.SNI
		if sni == "" && len(cfg.Reality.SNIList) > 0 {
			sni = cfg.Reality.SNIList[0]
		}
	}
	if sni == "" {
		sni = DefaultRealitySNIs[0]
	}

	normPath := cfg.Path
	if normPath == "" {
		normPath = "/s1r1nxhttp2026k"
	}
	if !strings.HasPrefix(normPath, "/") {
		normPath = "/" + normPath
	}
	if !strings.HasSuffix(normPath, "/") {
		normPath = normPath + "/"
	}

	// Для CDN фронтинга: TLS ClientHello SNI остаётся доменом CDN (cfg.Reality.SNI),
	// а HTTP/2 :authority и Host заголовок указывают на фактический бэкенд (cfg.CDNHost).
	hostHeader := sni
	if cfg.CDNHost != "" {
		hostHeader = cfg.CDNHost
	}

	reqURL := fmt.Sprintf("https://%s%s", hostHeader, normPath)
	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, pr)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Host = hostHeader

	ua, chPlatform, chMobile := xhttpBrowserHeadersForOS(runtime.GOOS)
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Content-Type", "application/grpc")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Priority", "u=1, i")
	req.Header.Set("Referer", fmt.Sprintf("https://%s%s?x_padding=XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX", hostHeader, normPath))
	req.Header.Set("Sec-CH-UA", `"Chromium";v="152", "Not?A_Brand";v="24", "Google Chrome";v="152"`)
	req.Header.Set("Sec-CH-UA-Mobile", chMobile)
	req.Header.Set("Sec-CH-UA-Platform", chPlatform)
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("X-Accel-Buffering", "no")

	return req, nil
}

// xhttpBrowserHeadersForOS подбирает консистентную тройку (User-Agent, Sec-CH-UA-Platform, Sec-CH-UA-Mobile)
// под реальную ОС клиента, чтобы исключить несоответствие с параметрами TCP/IP-стека (TTL, TCP Timestamp) при CDN-фронтинге.
func xhttpBrowserHeadersForOS(goos string) (userAgent, secPlatform, secMobile string) {
	const chromeVer = "152." + "0.0.0"
	switch goos {
	case "android":
		return "Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/" + chromeVer + " Mobile Safari/537.36", `"Android"`, "?1"
	case "darwin":
		return "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/" + chromeVer + " Safari/537.36", `"macOS"`, "?0"
	case "ios":
		return "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/" + chromeVer + " Mobile/15E148 Safari/604.1", `"iOS"`, "?1"
	case "linux":
		return "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/" + chromeVer + " Safari/537.36", `"Linux"`, "?0"
	default:
		return "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/" + chromeVer + " Safari/537.36", `"Windows"`, "?0"
	}
}

// DialXHTTP выполняет TLS Reality рукопожатие, инициирует HTTP/2 POST стрим
// и передаёт заголовок VLESS, возвращая net.Conn для прозрачной передачи данных.
func DialXHTTP(ctx context.Context, uuid [16]byte, target string, cfg *XHTTPConfig) (net.Conn, error) {
	conn, err := Dial(ctx, cfg.Reality)
	if err != nil {
		return nil, fmt.Errorf("reality dial: %w", err)
	}

	t := &http2.Transport{
		AllowHTTP: false,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	cc, err := t.NewClientConn(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("http2 client conn: %w", err)
	}

	vlessReq := &Request{
		UUID:   uuid,
		Cmd:    CmdTCP,
		Target: target,
		Flow:   "",
	}
	hdr, err := vlessReq.Encode()
	if err != nil {
		cc.Close()
		conn.Close()
		return nil, fmt.Errorf("encode vless header: %w", err)
	}

	pr, pw := io.Pipe()

	req, err := buildXHTTPRequest(ctx, cfg, pr)
	if err != nil {
		_ = pr.Close()
		_ = pw.Close()
		_ = cc.Close()
		_ = conn.Close()
		return nil, err
	}
	req.Header.Set("X-Accel-Buffering", "no")

	go func() {
		_, _ = pw.Write(hdr)
	}()

	resp, err := cc.RoundTrip(req)
	if err != nil {
		_ = pr.Close()
		_ = pw.Close()
		_ = cc.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("xhttp roundtrip: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		_ = pr.Close()
		_ = pw.Close()
		_ = cc.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("xhttp unexpected status: %s", resp.Status)
	}

	return &XHTTPConn{
		uConn:    conn,
		h2Conn:   cc,
		pr:       pr,
		pw:       pw,
		respBody: resp.Body,
	}, nil
}

// XHTTPConn реализует интерфейс net.Conn поверх HTTP/2 потока XHTTP.
type XHTTPConn struct {
	uConn        net.Conn
	h2Conn       *http2.ClientConn
	pr           *io.PipeReader
	pw           *io.PipeWriter
	respBody     io.ReadCloser
	mu           sync.Mutex
	responseRead bool
	closed       bool
}

func (c *XHTTPConn) Read(b []byte) (int, error) {
	c.mu.Lock()
	if !c.responseRead {
		var respHeader [2]byte
		if _, err := io.ReadFull(c.respBody, respHeader[:]); err != nil {
			c.mu.Unlock()
			return 0, fmt.Errorf("read vless response header: %w", err)
		}
		addonLen := int(respHeader[1])
		if addonLen > 0 {
			addonBuf := make([]byte, addonLen)
			if _, err := io.ReadFull(c.respBody, addonBuf); err != nil {
				c.mu.Unlock()
				return 0, fmt.Errorf("read vless response addon: %w", err)
			}
		}
		c.responseRead = true
	}
	c.mu.Unlock()

	return c.respBody.Read(b)
}

func (c *XHTTPConn) Write(b []byte) (int, error) {
	return c.pw.Write(b)
}

func (c *XHTTPConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	if c.pr != nil {
		_ = c.pr.Close()
	}
	if c.pw != nil {
		_ = c.pw.Close()
	}
	if c.respBody != nil {
		_ = c.respBody.Close()
	}
	if c.h2Conn != nil {
		_ = c.h2Conn.Close()
	}
	if c.uConn != nil {
		return c.uConn.Close()
	}
	return nil
}

func (c *XHTTPConn) LocalAddr() net.Addr {
	return c.uConn.LocalAddr()
}

func (c *XHTTPConn) RemoteAddr() net.Addr {
	return c.uConn.RemoteAddr()
}

func (c *XHTTPConn) SetDeadline(t time.Time) error {
	return c.uConn.SetDeadline(t)
}

func (c *XHTTPConn) SetReadDeadline(t time.Time) error {
	return c.uConn.SetReadDeadline(t)
}

func (c *XHTTPConn) SetWriteDeadline(t time.Time) error {
	return c.uConn.SetWriteDeadline(t)
}
