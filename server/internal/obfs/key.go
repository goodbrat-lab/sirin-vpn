package obfs

import (
	"crypto/sha256"
	"io"

	"golang.org/x/crypto/hkdf"
)

const (
	wrapSalt = "WDTT-WRAP-v1"
	wrapInfo = "rtp-obfs/chacha20poly1305"
	WrapKeyLen = 32
)

// DeriveWrapKey выводит 32-байтный ключ ChaCha20-Poly1305
// из пароля (UUID клиента) с использованием HKDF-SHA256 (RFC 5869).
func DeriveWrapKey(password string) ([]byte, error) {
	r := hkdf.New(
		sha256.New,
		[]byte(password),
		[]byte(wrapSalt),
		[]byte(wrapInfo),
	)
	key := make([]byte, WrapKeyLen)
	if _, err := io.ReadFull(r, key); err != nil {
		return nil, err
	}
	return key, nil
}
