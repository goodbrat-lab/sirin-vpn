package noise

import (
	"crypto/ecdh"
	"crypto/rand"
	"fmt"
)

// GenerateKeyPair генерирует новую пару ключей X25519 (приватный и публичный).
func GenerateKeyPair() (priv, pub []byte, err error) {
	k, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("GenerateKey: %w", err)
	}
	return k.Bytes(), k.PublicKey().Bytes(), nil
}

// ECDH вычисляет общий секрет по кривой X25519.
func ECDH(privBytes, pubBytes []byte) ([]byte, error) {
	priv, err := ecdh.X25519().NewPrivateKey(privBytes)
	if err != nil {
		return nil, fmt.Errorf("NewPrivateKey: %w", err)
	}
	pub, err := ecdh.X25519().NewPublicKey(pubBytes)
	if err != nil {
		return nil, fmt.Errorf("NewPublicKey: %w", err)
	}
	secret, err := priv.ECDH(pub)
	if err != nil {
		return nil, fmt.Errorf("priv.ECDH: %w", err)
	}
	return secret, nil
}
