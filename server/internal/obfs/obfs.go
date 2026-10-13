package obfs

import (
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"sync"

	"golang.org/x/crypto/chacha20poly1305"
)

const (
	HeaderLen   = 12
	PayloadType = 111 // Opus audio
	PaddingMax  = 24  // Audio padding max
	TagLen      = 16
)

// State хранит состояние обфускации для одного направления RTP-потока.
type State struct {
	mu      sync.Mutex
	aead    cipher.AEAD
	ssrc    uint32
	initSeq uint16
	initTs  uint32
	count   uint64
}

// NewState инициализирует состояние обфускации с заданным 32-байтным ключом.
func NewState(key []byte) (*State, error) {
	if len(key) != 32 {
		return nil, errors.New("длина ключа obfs должна быть 32 байта")
	}
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return nil, err
	}
	var b [10]byte
	if _, err := rand.Read(b[:]); err != nil {
		return nil, err
	}
	return &State{
		aead:    aead,
		ssrc:    binary.BigEndian.Uint32(b[0:4]),
		initSeq: binary.BigEndian.Uint16(b[4:6]),
		initTs:  binary.BigEndian.Uint32(b[6:10]),
	}, nil
}

// Nonce собирает 12-байтный nonce из полей RTP-заголовка.
// Порядок полей: SSRC (4Б), Seq (2Б), 0x00, 0x00 (2Б), Timestamp (4Б).
func Nonce(n *[12]byte, ssrc uint32, seq uint16, ts uint32) {
	binary.BigEndian.PutUint32(n[0:4], ssrc)
	binary.BigEndian.PutUint16(n[4:6], seq)
	n[6], n[7] = 0, 0
	binary.BigEndian.PutUint32(n[8:12], ts)
}

// Wrap упаковывает полезную нагрузку в RTP-пакет с шифрованием ChaCha20-Poly1305 и набивкой.
// Итоговый пакет: [12Б заголовок][шифротекст+тег][набивка][1Б длина набивки].
func (s *State) Wrap(payload []byte) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	c := s.count
	s.count++

	seq := s.initSeq + uint16(c)
	ts := s.initTs + uint32(c)*960 + uint32(c>>16)

	// Длина набивки от 1 до PaddingMax байт
	var padByte [1]byte
	if _, err := rand.Read(padByte[:]); err != nil {
		return nil, err
	}
	padTotal := int(padByte[0])%PaddingMax + 1

	hdr := make([]byte, HeaderLen)
	hdr[0] = 0x80 | 0x20 // V=2, P=1, X=0, CC=0
	hdr[1] = PayloadType & 0x7F
	binary.BigEndian.PutUint16(hdr[2:4], seq)
	binary.BigEndian.PutUint32(hdr[4:8], ts)
	binary.BigEndian.PutUint32(hdr[8:12], s.ssrc)

	var nonce [12]byte
	Nonce(&nonce, s.ssrc, seq, ts)

	out := make([]byte, 0, HeaderLen+len(payload)+TagLen+padTotal)
	out = append(out, hdr...)
	// AAD — сам 12-байтный заголовок RTP
	out = s.aead.Seal(out, nonce[:], payload, hdr)

	pad := make([]byte, padTotal)
	if padTotal > 1 {
		if _, err := rand.Read(pad[:padTotal-1]); err != nil {
			return nil, err
		}
	}
	pad[padTotal-1] = byte(padTotal)
	out = append(out, pad...)
	return out, nil
}

// Unwrap разбирает входящий RTP-пакет и расшифровывает полезную нагрузку.
// Возвращает полезную нагрузку или ошибку, если пакет повреждён или не является валидным RTP.
func (s *State) Unwrap(wire []byte) ([]byte, error) {
	if len(wire) < HeaderLen+1 {
		return nil, errors.New("пакет короче минимальной длины RTP")
	}
	if wire[0]>>6 != 2 { // проверка версии RTP = 2
		return nil, errors.New("неверная версия RTP")
	}

	payloadEnd := len(wire)
	if wire[0]&0x20 != 0 { // бит P — есть набивка
		padLen := int(wire[len(wire)-1])
		if padLen == 0 || padLen > len(wire)-HeaderLen {
			return nil, errors.New("некорректная длина набивки RTP")
		}
		payloadEnd = len(wire) - padLen
	}
	if payloadEnd < HeaderLen+TagLen {
		return nil, errors.New("длина полезной нагрузки меньше тега аутентификации")
	}

	seq := binary.BigEndian.Uint16(wire[2:4])
	ts := binary.BigEndian.Uint32(wire[4:8])
	ssrc := binary.BigEndian.Uint32(wire[8:12])

	var nonce [12]byte
	Nonce(&nonce, ssrc, seq, ts)

	plain, err := s.aead.Open(nil, nonce[:], wire[HeaderLen:payloadEnd], wire[:HeaderLen])
	if err != nil {
		return nil, err
	}
	return plain, nil
}
