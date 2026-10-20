package vless

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"syscall"
	"time"

	utls "github.com/refraction-networking/utls"
	"golang.org/x/crypto/hkdf"
)

type RealityConfig struct {
	ServerHost  string
	ServerPort  int
	PublicKey   []byte
	ShortID     []byte
	SNI         string   // например, sun8-21.userapi.com
	SNIList     []string // NEW: список SNI для перебора
	Fingerprint string   // "chrome"
	Control     func(network, address string, c syscall.RawConn) error
	Logf        func(format string, args ...any)
}

// DefaultRealitySNIs contains trusted TLS 1.3 domains (VK, Kinopoisk, WB, IKEA, Microsoft, Apple, Samsung)
// used for Reality masking when neither SNIList nor SNI is specified.
var DefaultRealitySNIs = []string{
	"sun8-21.userapi.com",
	"www.kinopoisk.ru",
	"wildberries.ru",
	"www.ikea.com",
	"www.microsoft.com",
	"dl.delivery.mp.microsoft.com",
	"swcdn.apple.com",
	"www.samsung.com",
}

// Dial выполняет TCP-соединение и TLS 1.3 рукопожатие по протоколу Reality с маскировкой под Chrome.
// Если SNIList не пустой — пробует каждый SNI по очереди.
func Dial(ctx context.Context, cfg *RealityConfig) (net.Conn, error) {
	snis := cfg.SNIList
	if len(snis) == 0 {
		if cfg.SNI != "" {
			snis = []string{cfg.SNI}
		} else {
			snis = DefaultRealitySNIs
		}
	}

	logf := cfg.Logf
	if logf == nil {
		logf = func(string, ...any) {}
	}

	var lastErr error
	for _, sni := range snis {
		logf("[engine] vless: trying SNI %s", sni)
		conn, err := dialOne(ctx, cfg, sni)
		if err == nil {
			logf("[engine] vless: handshake OK")
			return conn, nil
		}
		logf("[engine] vless: handshake failed (%v)", err)
		lastErr = err
	}
	return nil, lastErr
}

func dialOne(ctx context.Context, cfg *RealityConfig, sni string) (net.Conn, error) {
	d := &net.Dialer{
		Timeout: 5 * time.Second,
		Control: cfg.Control,
	}
	addr := net.JoinHostPort(cfg.ServerHost, fmt.Sprintf("%d", cfg.ServerPort))
	rawConn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("tcp dial: %w", err)
	}

	tlsConfig := &utls.Config{
		ServerName:             sni,
		InsecureSkipVerify:     true,
		SessionTicketsDisabled: true,
	}

	uConn := utls.UClient(rawConn, tlsConfig, utls.HelloChrome_Auto)
	if err := uConn.BuildHandshakeState(); err != nil {
		rawConn.Close()
		return nil, fmt.Errorf("build handshake state: %w", err)
	}

	hello := uConn.HandshakeState.Hello
	hello.SessionId = make([]byte, 32)
	copy(hello.Raw[39:], hello.SessionId) // the fixed location of `Session ID`
	hello.SessionId[0] = 26               // Xray major version
	hello.SessionId[1] = 3                // Xray minor version
	hello.SessionId[2] = 27               // Xray patch version
	hello.SessionId[3] = 0                // reserved
	binary.BigEndian.PutUint32(hello.SessionId[4:], uint32(time.Now().Unix()))
	copy(hello.SessionId[8:], cfg.ShortID)

	publicKey, err := ecdh.X25519().NewPublicKey(cfg.PublicKey)
	if err != nil {
		rawConn.Close()
		return nil, fmt.Errorf("invalid server public key: %w", err)
	}

	ecdhe := uConn.HandshakeState.State13.KeyShareKeys.Ecdhe
	if ecdhe == nil {
		ecdhe = uConn.HandshakeState.State13.KeyShareKeys.MlkemEcdhe
	}
	if ecdhe == nil {
		rawConn.Close()
		return nil, fmt.Errorf("fingerprint does not support TLS 1.3")
	}

	sharedKey, err := ecdhe.ECDH(publicKey)
	if err != nil {
		rawConn.Close()
		return nil, fmt.Errorf("ecdh: %w", err)
	}

	hReader := hkdf.New(sha256.New, sharedKey, hello.Random[:20], []byte("REALITY"))
	authKey := make([]byte, 32)
	if _, err := io.ReadFull(hReader, authKey); err != nil {
		rawConn.Close()
		return nil, fmt.Errorf("hkdf: %w", err)
	}

	block, err := aes.NewCipher(authKey)
	if err != nil {
		rawConn.Close()
		return nil, fmt.Errorf("aes cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		rawConn.Close()
		return nil, fmt.Errorf("aes gcm: %w", err)
	}

	aead.Seal(hello.SessionId[:0], hello.Random[20:], hello.SessionId[:16], hello.Raw)
	copy(hello.Raw[39:], hello.SessionId)

	hsCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	if err := uConn.HandshakeContext(hsCtx); err != nil {
		rawConn.Close()
		return nil, fmt.Errorf("utls reality handshake: %w", err)
	}

	return uConn, nil
}
