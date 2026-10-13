package noise

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"errors"
	"fmt"
)

// CipherState инкапсулирует симметричный шифр AES-256-GCM и счётчик nonce.
type CipherState struct {
	key    [32]byte
	hasKey bool
	n      uint64
}

// NewCipherState инициализирует CipherState с 32-байтным ключом.
func NewCipherState(key []byte) (*CipherState, error) {
	if len(key) != 32 {
		return nil, errors.New("длина ключа должна быть ровно 32 байта")
	}
	cs := &CipherState{hasKey: true, n: 0}
	copy(cs.key[:], key)
	return cs, nil
}

// NonceBytes формирует 12-байтный nonce: 4 нуля + 8 байт big-endian счётчика n.
func (cs *CipherState) NonceBytes() [12]byte {
	var nonce [12]byte
	binary.BigEndian.PutUint64(nonce[4:], cs.n)
	return nonce
}

func (cs *CipherState) aead() (cipher.AEAD, error) {
	block, err := aes.NewCipher(cs.key[:])
	if err != nil {
		return nil, fmt.Errorf("aes.NewCipher: %w", err)
	}
	return cipher.NewGCM(block)
}

// Encrypt шифрует открытый текст с ассоциированными данными ad (AEAD Seal).
// При каждом вызове счётчик nonce инкрементируется: n++.
func (cs *CipherState) Encrypt(ad, plaintext []byte) ([]byte, error) {
	if !cs.hasKey {
		return append([]byte{}, plaintext...), nil
	}
	aead, err := cs.aead()
	if err != nil {
		return nil, err
	}
	nonce := cs.NonceBytes()
	cs.n++
	return aead.Seal(nil, nonce[:], plaintext, ad), nil
}

// Decrypt расшифровывает шифротекст с ассоциированными данными ad (AEAD Open).
// При каждом вызове счётчик nonce инкрементируется: n++.
func (cs *CipherState) Decrypt(ad, ciphertext []byte) ([]byte, error) {
	if !cs.hasKey {
		return append([]byte{}, ciphertext...), nil
	}
	aead, err := cs.aead()
	if err != nil {
		return nil, err
	}
	nonce := cs.NonceBytes()
	cs.n++
	return aead.Open(nil, nonce[:], ciphertext, ad)
}
