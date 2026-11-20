package bot

import (
	"bytes"
	"image/png"
	"testing"
)

func TestGenerateKeyQR(t *testing.T) {
	testURL := "https://crp.guseynovia.ru/key?uuid=12345678-1234-1234-1234-123456789abc"
	pngBytes, err := GenerateKeyQR(testURL)
	if err != nil {
		t.Fatalf("GenerateKeyQR failed: %v", err)
	}
	if len(pngBytes) == 0 {
		t.Fatal("GenerateKeyQR returned empty bytes")
	}

	// Проверяем, что результат действительно является валидным изображением PNG
	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("Failed to decode PNG: %v", err)
	}

	bounds := img.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		t.Fatalf("Invalid image dimensions: %v", bounds)
	}
}
