package bot

import (
	qrcode "github.com/skip2/go-qrcode"
)

// GenerateKeyQR создаёт изображение QR-кода размером 300x300 в формате PNG.
func GenerateKeyQR(target string) ([]byte, error) {
	return qrcode.Encode(target, qrcode.Medium, 300)
}
