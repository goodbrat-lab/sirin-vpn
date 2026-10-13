package noise

import (
	"crypto/ecdh"
	"errors"
	"fmt"
)

const (
	ProtocolName = "Noise_NK_25519_AESGCM_SHA256"
	ProtocolVersion byte = 0x01
)

// Responder реализует серверную часть протокола Noise NK.
type Responder struct {
	staticPriv []byte // Приватный статический ключ X25519 (32 байта)
	staticPub  []byte // Открытый статический ключ X25519 (32 байта)

	// Состояние симметричного протокола Noise:
	h  [32]byte
	ck [32]byte

	// Временные ключи handshake:
	ephPriv []byte // Эфемерный приватный ключ сервера
	ephPub  []byte // Эфемерный открытый ключ сервера
	re      []byte // Эфемерный открытый ключ клиента (из msg1)

	// Сессионные шифры после Split:
	send *CipherState // k2 (сервер -> клиент)
	recv *CipherState // k1 (клиент -> сервер)
}

// NewResponder инициализирует Responder со статическим приватным ключом сервера.
func NewResponder(staticPriv []byte) (*Responder, error) {
	if len(staticPriv) != 32 {
		return nil, errors.New("staticPriv должен быть длиной 32 байта")
	}

	priv, err := ecdh.X25519().NewPrivateKey(staticPriv)
	if err != nil {
		return nil, fmt.Errorf("ecdh.NewPrivateKey: %w", err)
	}
	staticPub := priv.PublicKey().Bytes()

	r := &Responder{
		staticPriv: append([]byte{}, staticPriv...),
		staticPub:  staticPub,
	}

	// Инициализация h: имя протокола длиной 28 байт дополняется 4 нулями до 32 байт
	copy(r.h[:], ProtocolName)
	r.ck = r.h

	// MixHash(prologue=nil) по Noise §5.2 — выполняется всегда,
	// даже если prologue пустой
	r.h = MixHash(r.h, nil)

	// Предсообщение: "<- s" (хеширование открытого статического ключа сервера)
	r.h = MixHash(r.h, r.staticPub)

	return r, nil
}

// ReadMsg1 обрабатывает сообщение 1 от клиента ("-> e, es").
// Ожидает 49 байт: 32 байта эфемерного ключа + 17 байт зашифрованного payload.
func (r *Responder) ReadMsg1(msg []byte) (payload []byte, err error) {
	if len(msg) < 32+16 {
		return nil, errors.New("msg1 короче минимального размера (32+16 байт)")
	}

	// 1. Извлечение эфемерного ключа клиента (re)
	r.re = append([]byte{}, msg[:32]...)
	ciphertext := msg[32:]

	// 2. mixHash(re)
	r.h = MixHash(r.h, r.re)

	// 3. es = ECDH(staticPriv, re)
	es, err := ECDH(r.staticPriv, r.re)
	if err != nil {
		return nil, fmt.Errorf("ECDH es: %w", err)
	}

	// 4. mixKey(es)
	nextCk, tempK := MixKey(r.ck, es)
	r.ck = nextCk

	// 5. Расшифровка payload msg1 с ad = r.h
	cs, err := NewCipherState(tempK[:])
	if err != nil {
		return nil, err
	}

	pt, err := cs.Decrypt(r.h[:], ciphertext)
	if err != nil {
		return nil, fmt.Errorf("расшифровка payload msg1: %w", err)
	}

	// 6. mixHash(ciphertext)
	r.h = MixHash(r.h, ciphertext)

	// Проверка версии протокола
	if len(pt) == 0 || pt[0] != ProtocolVersion {
		return nil, fmt.Errorf("неподдерживаемая версия протокола: %v (ожидалось 0x%02x)", pt, ProtocolVersion)
	}

	return pt, nil
}

// WriteMsg2 формирует сообщение 2 сервер -> клиент ("<- e, ee").
// Возвращает ephPub (32) || ciphertext (len(payload) + 16).
func (r *Responder) WriteMsg2(payload []byte) (msg []byte, err error) {
	if r.re == nil {
		return nil, errors.New("невозможно вызвать WriteMsg2 до успешного ReadMsg1")
	}

	// 1. Генерация серверного эфемерного ключа (e)
	ephPriv, ephPub, err := GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("генерация эфемерного ключа сервера: %w", err)
	}
	r.ephPriv = ephPriv
	r.ephPub = ephPub

	// 2. mixHash(ephPub)
	r.h = MixHash(r.h, r.ephPub)

	// 3. ee = ECDH(ephPriv, re)
	ee, err := ECDH(r.ephPriv, r.re)
	if err != nil {
		return nil, fmt.Errorf("ECDH ee: %w", err)
	}

	// 4. mixKey(ee)
	nextCk, tempK := MixKey(r.ck, ee)
	r.ck = nextCk

	// 5. Шифрование payload msg2 с ad = r.h
	cs, err := NewCipherState(tempK[:])
	if err != nil {
		return nil, err
	}

	ct, err := cs.Encrypt(r.h[:], payload)
	if err != nil {
		return nil, fmt.Errorf("шифрование payload msg2: %w", err)
	}

	// 6. mixHash(ciphertext)
	r.h = MixHash(r.h, ct)

	// Возвращаем ephPub || ct
	return append(append([]byte{}, r.ephPub...), ct...), nil
}

// Split завершает рукопожатие и формирует пару транспортных ключей:
// k1 (recv: клиент -> сервер) и k2 (send: сервер -> клиент).
func (r *Responder) Split() error {
	k1, k2 := HKDF2(r.ck[:], nil)

	recvCS, err := NewCipherState(k1[:])
	if err != nil {
		return err
	}
	sendCS, err := NewCipherState(k2[:])
	if err != nil {
		return err
	}

	r.recv = recvCS
	r.send = sendCS
	return nil
}

// DecryptAuth расшифровывает сообщение AUTH от клиента (ad = nil).
func (r *Responder) DecryptAuth(ciphertext []byte) ([]byte, error) {
	if r.recv == nil {
		return nil, errors.New("Split() ещё не был вызван")
	}
	return r.recv.Decrypt(nil, ciphertext)
}

// EncryptResponse шифрует ответ сервера (например, OK|...) клиенту (ad = nil).
func (r *Responder) EncryptResponse(plaintext []byte) ([]byte, error) {
	if r.send == nil {
		return nil, errors.New("Split() ещё не был вызван")
	}
	return r.send.Encrypt(nil, plaintext)
}

// StaticPublicKey возвращает публичный ключ сервера (32 байта).
func (r *Responder) StaticPublicKey() []byte {
	return append([]byte{}, r.staticPub...)
}
