package obfs

import (
	"bytes"
	"testing"
)

func TestDeriveWrapKeyConsistency(t *testing.T) {
	password := "test-uuid-client-12345"
	key1, err := DeriveWrapKey(password)
	if err != nil {
		t.Fatalf("DeriveWrapKey: %v", err)
	}
	if len(key1) != 32 {
		t.Fatalf("длина ключа = %d, ожидалось 32", len(key1))
	}

	key2, err := DeriveWrapKey(password)
	if err != nil {
		t.Fatalf("DeriveWrapKey: %v", err)
	}

	if !bytes.Equal(key1, key2) {
		t.Fatal("DeriveWrapKey недетерминирован для одного и того же пароля")
	}

	diffKey, _ := DeriveWrapKey("different-password")
	if bytes.Equal(key1, diffKey) {
		t.Fatal("DeriveWrapKey дал одинаковый ключ для разных паролей")
	}
}

func TestWrapUnwrapCycle(t *testing.T) {
	password := "test-client-secret-uuid"
	key, err := DeriveWrapKey(password)
	if err != nil {
		t.Fatalf("DeriveWrapKey: %v", err)
	}

	// Отправитель (например, клиент)
	sender, err := NewState(key)
	if err != nil {
		t.Fatalf("NewState sender: %v", err)
	}

	// Получатель (например, сервер)
	receiver, err := NewState(key)
	if err != nil {
		t.Fatalf("NewState receiver: %v", err)
	}

	payloads := [][]byte{
		[]byte("AUTH|9b1deb4d-3b7d-4bad-9bdd-2b0d7b3dcb6d|device-1|echo"),
		[]byte("OK|10.77.77.2|10.77.77.1|ffffff00|echo"),
		[]byte{0x01}, // Noise msg1 payload
		make([]byte, 1400), // Полноразмерный IP-пакет (MTU 1400)
		[]byte("echo-keepalive"), // < 20 байт
	}

	for i, payload := range payloads {
		// Упаковываем
		wire, err := sender.Wrap(payload)
		if err != nil {
			t.Fatalf("тест %d: Wrap: %v", i, err)
		}

		// Проверяем минимальный размер и RTP заголовок
		if len(wire) < len(payload)+HeaderLen+TagLen+1 {
			t.Fatalf("тест %d: размер wire слишком мал: %d", i, len(wire))
		}
		if wire[0]>>6 != 2 {
			t.Fatalf("тест %d: версия RTP != 2", i)
		}
		if wire[1]&0x7F != PayloadType {
			t.Fatalf("тест %d: PayloadType != %d", i, PayloadType)
		}

		// Распаковываем
		unwrapped, err := receiver.Unwrap(wire)
		if err != nil {
			t.Fatalf("тест %d: Unwrap: %v", i, err)
		}

		if !bytes.Equal(unwrapped, payload) {
			t.Fatalf("тест %d: полезная нагрузка искажена", i)
		}
	}
}

func TestUnwrapTamperedWire(t *testing.T) {
	key, _ := DeriveWrapKey("some-password")
	sender, _ := NewState(key)
	receiver, _ := NewState(key)

	wire, _ := sender.Wrap([]byte("secret message"))

	// 1. Искажение байта в RTP-заголовке (проверка AAD)
	tamperedHeader := append([]byte{}, wire...)
	tamperedHeader[3] ^= 0x01
	if _, err := receiver.Unwrap(tamperedHeader); err == nil {
		t.Fatal("ожидалась ошибка аутентификации при искажении заголовка RTP, получен nil")
	}

	// 2. Искажение байта в теле шифротекста
	tamperedBody := append([]byte{}, wire...)
	tamperedBody[HeaderLen+2] ^= 0x01
	if _, err := receiver.Unwrap(tamperedBody); err == nil {
		t.Fatal("ожидалась ошибка аутентификации при искажении тела пакета, получен nil")
	}

	// 3. Пакет с неверным ключом
	otherKey, _ := DeriveWrapKey("wrong-password")
	wrongReceiver, _ := NewState(otherKey)
	if _, err := wrongReceiver.Unwrap(wire); err == nil {
		t.Fatal("ожидалась ошибка при расшифровке чужим ключом, получен nil")
	}
}
