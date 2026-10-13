package noise

import (
	"crypto/hmac"
	"crypto/sha256"
)

// HmacHash вычисляет HMAC-SHA256(key, data).
func HmacHash(key, data []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(data)
	return m.Sum(nil)
}

// HKDF2 выполняет двухвыходный HKDF по спецификации Noise (§4.3).
// temp_key = HMAC-Hash(chaining_key, input_key_material)
// out1 = HMAC-Hash(temp_key, byte(0x01))
// out2 = HMAC-Hash(temp_key, out1 || byte(0x02))
func HKDF2(chainingKey, ikm []byte) (out1, out2 [32]byte) {
	tempKey := HmacHash(chainingKey, ikm)
	o1 := HmacHash(tempKey, []byte{0x01})
	o2 := HmacHash(tempKey, append(append([]byte{}, o1...), 0x02))
	copy(out1[:], o1)
	copy(out2[:], o2)
	return
}

// MixHash обновляет значение h: h = SHA256(h || data).
func MixHash(h [32]byte, data []byte) [32]byte {
	return sha256.Sum256(append(append([]byte{}, h[:]...), data...))
}

// MixKey обновляет chainingKey и возвращает tempKey для шифрования.
func MixKey(ck [32]byte, ikm []byte) (nextCk, tempK [32]byte) {
	return HKDF2(ck[:], ikm)
}
