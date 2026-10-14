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
	"time"

	utls "github.com/refraction-networking/utls"
	"golang.org/x/crypto/hkdf"
)

type RealityConfig struct {
	ServerHost  string
	ServerPort  int
	PublicKey   []byte
	ShortID     []byte
	SNI         string // например, dl.google.com
	Fingerprint string // "chrome"
}

// Dial выполняет TCP-соединение и TLS 1.3 рукопожатие по протоколу Reality с маскировкой под Chrome.
func Dial(ctx context.Context, cfg *RealityConfig) (net.Conn, error) {
	d := &net.Dialer{Timeout: 10 * time.Second}
	addr := net.JoinHostPort(cfg.ServerHost, fmt.Sprintf("%d", cfg.ServerPort))
	rawConn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("tcp dial: %w", err)
	}

	tlsConfig := &utls.Config{
		ServerName:             cfg.SNI,
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

	if err := uConn.HandshakeContext(ctx); err != nil {
		rawConn.Close()
		return nil, fmt.Errorf("utls reality handshake: %w", err)
	}

	return uConn, nil
}
