//go:build !android

package engine

import (
	"crypto/ecdh"
	"encoding/binary"
	"fmt"
	"io"
	"net"
)

// Exported helpers for E2E tests and integration

// GatewayStaticPubKey возвращает публичный ключ шлюза для Noise NK.
func GatewayStaticPubKey() *ecdh.PublicKey {
	return gatewayStaticPublicKey
}

// DeriveWrapKey выводит ключ обфускации из пароля (UUID) клиента.
func DeriveWrapKey(password string) ([]byte, error) {
	return deriveWrapKey(password)
}

// ObfsState экспортирует обфускацию для E2E тестов.
type ObfsState struct {
	st *obfsState
}

// NewObfsState создает состояние обфускации.
func NewObfsState(key []byte) (*ObfsState, error) {
	st, err := newObfsState(key)
	if err != nil {
		return nil, err
	}
	return &ObfsState{st: st}, nil
}

// Wrap упаковывает полезную нагрузку в RTP-пакет обфускации.
func (o *ObfsState) Wrap(payload []byte) ([]byte, error) {
	return o.st.wrap(payload)
}

// Unwrap распаковывает RTP-пакет обфускации.
func (o *ObfsState) Unwrap(wire []byte) ([]byte, bool) {
	dst := make([]byte, len(wire))
	n, ok := o.st.unwrap(wire, dst)
	if !ok {
		return nil, false
	}
	res := make([]byte, n)
	copy(res, dst[:n])
	return res, true
}

// NKInitiator экспортирует Noise NK инициатор.
type NKInitiator struct {
	ni *nkInitiator
}

// NewNKInitiator создает клиентский инициатор Noise NK.
func NewNKInitiator(rs *ecdh.PublicKey) (*NKInitiator, error) {
	ni, err := newNKInitiator(rs)
	if err != nil {
		return nil, err
	}
	return &NKInitiator{ni: ni}, nil
}

func (n *NKInitiator) WriteMsg1(payload []byte) ([]byte, error) {
	return n.ni.writeMsg1(payload)
}

func (n *NKInitiator) ReadMsg2(msg []byte) ([]byte, error) {
	return n.ni.readMsg2(msg)
}

type CipherState struct {
	cs *cipherState
}

func (c *CipherState) Encrypt(ad, plaintext []byte) ([]byte, error) {
	return c.cs.encrypt(ad, plaintext)
}

func (c *CipherState) Decrypt(ad, ciphertext []byte) ([]byte, error) {
	return c.cs.decrypt(ad, ciphertext)
}

func (n *NKInitiator) Split() (*CipherState, *CipherState) {
	send, recv := n.ni.split()
	return &CipherState{cs: &send}, &CipherState{cs: &recv}
}

// StreamLink представляет кадрирование потока [2B len big-endian][body].
type StreamLink struct {
	conn net.Conn
}

func NewStreamLink(conn net.Conn) *StreamLink {
	return &StreamLink{conn: conn}
}

func (s *StreamLink) ReadFrame() ([]byte, error) {
	var lenBuf [2]byte
	if _, err := io.ReadFull(s.conn, lenBuf[:]); err != nil {
		return nil, err
	}
	n := int(binary.BigEndian.Uint16(lenBuf[:]))
	if n == 0 || n > tcpFrameMax {
		return nil, fmt.Errorf("bad frame length: %d", n)
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(s.conn, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func (s *StreamLink) WriteFrame(data []byte) error {
	frame := make([]byte, 2+len(data))
	binary.BigEndian.PutUint16(frame[:2], uint16(len(data)))
	copy(frame[2:], data)
	_, err := s.conn.Write(frame)
	return err
}

func (s *StreamLink) Conn() net.Conn {
	return s.conn
}

// BuildEchoRequest собирает тестовый IPv4 ICMP Echo Request пакет.
func BuildEchoRequest(src, dst net.IP, id, seq uint16, payload int) []byte {
	return buildEchoRequest(src, dst, id, seq, payload)
}

// ParseEchoReply разбирает ICMP Echo Reply пакет.
func ParseEchoReply(pkt []byte, id uint16) (uint16, bool) {
	return parseEchoReply(pkt, id)
}
