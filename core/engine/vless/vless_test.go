package vless

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"testing"
	"time"

	"golang.org/x/crypto/hkdf"
)

func TestRequestEncodeWithVision(t *testing.T) {
	var testUUID [16]byte
	copy(testUUID[:], []byte("1234567890123456"))

	req := &Request{
		UUID:   testUUID,
		Cmd:    CmdTCP,
		Target: "dl.google.com:443",
		Flow:   AddonFlowVision,
	}

	encoded, err := req.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// 1. Проверяем версию
	if encoded[0] != 0 {
		t.Errorf("expected version 0, got %d", encoded[0])
	}

	// 2. Проверяем UUID (байты 1..16)
	if !bytes.Equal(encoded[1:17], testUUID[:]) {
		t.Errorf("UUID mismatch in header")
	}

	// 3. Проверяем addons: [длина M][тип=1][длина flow][значение]
	addonLen := int(encoded[17])
	expectedFlow := AddonFlowVision
	expectedAddonLen := 2 + len(expectedFlow)
	if addonLen != expectedAddonLen {
		t.Fatalf("expected addon length %d, got %d", expectedAddonLen, addonLen)
	}

	addonBytes := encoded[18 : 18+addonLen]
	if addonBytes[0] != 0x0a {
		t.Errorf("expected protobuf tag 0x0a (field 1 wire 2), got %d", addonBytes[0])
	}
	if int(addonBytes[1]) != len(expectedFlow) {
		t.Errorf("expected flow string length %d, got %d", len(expectedFlow), addonBytes[1])
	}
	if string(addonBytes[2:]) != expectedFlow {
		t.Errorf("expected flow %s, got %s", expectedFlow, string(addonBytes[2:]))
	}

	// 4. Проверяем команду после addon
	cmdIdx := 18 + addonLen
	if encoded[cmdIdx] != CmdTCP {
		t.Errorf("expected CmdTCP (%d), got %d", CmdTCP, encoded[cmdIdx])
	}

	// 5. Проверяем порт (443 = 0x01BB)
	port := binary.BigEndian.Uint16(encoded[cmdIdx+1 : cmdIdx+3])
	if port != 443 {
		t.Errorf("expected port 443, got %d", port)
	}

	// 6. Проверяем тип адреса и домен
	addrType := encoded[cmdIdx+3]
	if addrType != AddrTypeDomain {
		t.Errorf("expected AddrTypeDomain (%d), got %d", AddrTypeDomain, addrType)
	}
	domainLen := int(encoded[cmdIdx+4])
	domain := string(encoded[cmdIdx+5 : cmdIdx+5+domainLen])
	if domain != "dl.google.com" {
		t.Errorf("expected target domain 'dl.google.com', got '%s'", domain)
	}
}

func TestVLESSHeaderIPv4(t *testing.T) {
	var testUUID [16]byte
	req := &Request{
		UUID:   testUUID,
		Cmd:    CmdUDP,
		Target: "1.1.1.1:53",
		Flow:   "",
	}

	encoded, err := req.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Без addons длина addons = 0
	if encoded[17] != 0 {
		t.Errorf("expected empty addon length 0, got %d", encoded[17])
	}

	// CmdUDP = 2
	if encoded[18] != CmdUDP {
		t.Errorf("expected CmdUDP, got %d", encoded[18])
	}

	// Port 53
	port := binary.BigEndian.Uint16(encoded[19:21])
	if port != 53 {
		t.Errorf("expected port 53, got %d", port)
	}

	// AddrTypeIPv4
	if encoded[21] != AddrTypeIPv4 {
		t.Errorf("expected AddrTypeIPv4, got %d", encoded[21])
	}

	// IPv4 = 1.1.1.1
	if !bytes.Equal(encoded[22:26], []byte{1, 1, 1, 1}) {
		t.Errorf("expected IP [1 1 1 1], got %v", encoded[22:26])
	}
}

func TestRealitySessionID(t *testing.T) {
	// Имитация клиент-серверной проверки Reality Session ID
	curve := ecdh.X25519()

	// 1. Серверные ключи
	serverPriv, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("server keygen: %v", err)
	}
	serverPub := serverPriv.PublicKey()

	// 2. Клиентский эфемерный ключ
	clientPriv, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("client keygen: %v", err)
	}
	clientPub := clientPriv.PublicKey()

	// 3. Вычисление общего секрета с обеих сторон
	clientShared, err := clientPriv.ECDH(serverPub)
	if err != nil {
		t.Fatalf("client ECDH: %v", err)
	}
	serverShared, err := serverPriv.ECDH(clientPub)
	if err != nil {
		t.Fatalf("server ECDH: %v", err)
	}
	if !bytes.Equal(clientShared, serverShared) {
		t.Fatalf("ECDH shared secrets do not match")
	}

	// 4. Имитация TLS ClientHello.Random (32 байта)
	helloRandom := make([]byte, 32)
	rand.Read(helloRandom)

	// Клиент формирует authKey через HKDF
	clientHK := hkdf.New(sha256.New, clientShared, helloRandom[:20], []byte("REALITY"))
	clientAuthKey := make([]byte, 32)
	clientHK.Read(clientAuthKey)

	// Клиент шифрует Session ID
	block, _ := aes.NewCipher(clientAuthKey)
	aead, _ := cipher.NewGCM(block)

	shortID := []byte{0x82, 0x83, 0xf9, 0x9e, 0xc6, 0xa0, 0xf5, 0x2e}
	plainText := make([]byte, 16)
	binary.BigEndian.PutUint32(plainText[4:8], uint32(time.Now().Unix()))
	copy(plainText[8:16], shortID)

	sessionID := aead.Seal(nil, helloRandom[20:], plainText, nil)
	if len(sessionID) != 32 {
		t.Fatalf("expected 32 bytes sessionID, got %d", len(sessionID))
	}

	// 5. Сервер расшифровывает Session ID
	serverHK := hkdf.New(sha256.New, serverShared, helloRandom[:20], []byte("REALITY"))
	serverAuthKey := make([]byte, 32)
	serverHK.Read(serverAuthKey)

	sBlock, _ := aes.NewCipher(serverAuthKey)
	sAEAD, _ := cipher.NewGCM(sBlock)

	decrypted, err := sAEAD.Open(nil, helloRandom[20:], sessionID, nil)
	if err != nil {
		t.Fatalf("server AEAD.Open failed: %v", err)
	}

	if !bytes.Equal(decrypted[8:16], shortID) {
		t.Errorf("ShortID mismatch: expected %x, got %x", shortID, decrypted[8:16])
	}
}

func TestParseUUID(t *testing.T) {
	validStr := "5f26ebc3-6eb0-45fb-a490-c8c92b86b644"
	expectedBytes := [16]byte{0x5f, 0x26, 0xeb, 0xc3, 0x6e, 0xb0, 0x45, 0xfb, 0xa4, 0x90, 0xc8, 0xc9, 0x2b, 0x86, 0xb6, 0x44}

	uuid, err := ParseUUID(validStr)
	if err != nil {
		t.Fatalf("ParseUUID failed on valid string: %v", err)
	}
	if uuid != expectedBytes {
		t.Fatalf("ParseUUID mismatch: expected %x, got %x", expectedBytes, uuid)
	}

	// Без дефисов тоже должен парситься (32 hex)
	noDashes := "5f26ebc36eb045fba490c8c92b86b644"
	uuid2, err := ParseUUID(noDashes)
	if err != nil {
		t.Fatalf("ParseUUID failed on no-dashes string: %v", err)
	}
	if uuid2 != expectedBytes {
		t.Fatalf("ParseUUID mismatch: expected %x, got %x", expectedBytes, uuid2)
	}

	// Невалидная длина
	if _, err := ParseUUID("5f26ebc3"); err == nil {
		t.Errorf("expected error on short UUID string")
	}

	// Невалидный hex
	if _, err := ParseUUID("zz26ebc3-6eb0-45fb-a490-c8c92b86b644"); err == nil {
		t.Errorf("expected error on invalid hex characters")
	}
}
