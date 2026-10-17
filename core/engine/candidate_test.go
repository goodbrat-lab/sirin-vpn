package engine

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"testing"

	"sirin/engine/vless"
)

func TestLadderAddVLESS(t *testing.T) {
	ladder := NewLadder()

	uuidStr := "5f26ebc3-6eb0-45fb-a490-c8c92b86b644"
	pubKeyBase64 := "4ll5_ouUJr8P8CCxasWNcNu0Lb7tSIDacGmRskweoH4"
	shortIDHex := "8283f99ec6a0f52e"
	sni := "dl.google.com"
	target := "127.0.0.1:56006"

	err := ladder.AddVLESS("vless-test", "crp.guseynovia.ru", 443, uuidStr, pubKeyBase64, shortIDHex, sni, target, "sun8-21.userapi.com,www.kinopoisk.ru")
	if err != nil {
		t.Fatalf("AddVLESS failed: %v", err)
	}
	if len(ladder.items[0].sniList) != 2 {
		t.Fatalf("expected 2 snis in sniList, got %d", len(ladder.items[0].sniList))
	}

	if ladder.Len() != 1 {
		t.Fatalf("expected 1 candidate, got %d", ladder.Len())
	}

	c := ladder.items[0]
	if c.name != "vless-test" {
		t.Errorf("expected name vless-test, got %s", c.name)
	}
	if c.host != "crp.guseynovia.ru" {
		t.Errorf("expected host crp.guseynovia.ru, got %s", c.host)
	}
	if c.port != 443 {
		t.Errorf("expected port 443, got %d", c.port)
	}
	if c.transport != transVLESS {
		t.Errorf("expected transport transVLESS, got %v", c.transport)
	}
	if c.sni != sni {
		t.Errorf("expected sni %s, got %s", sni, c.sni)
	}
	expectedUUID, _ := vless.ParseUUID(uuidStr)
	expectedPubKey, _ := base64.RawURLEncoding.DecodeString(pubKeyBase64)
	expectedShortID, _ := hex.DecodeString(shortIDHex)

	if c.vlessUUID != expectedUUID {
		t.Errorf("expected uuid %v, got %v", expectedUUID, c.vlessUUID)
	}
	if !bytes.Equal(c.vlessPublicKey, expectedPubKey) {
		t.Errorf("expected pubKey %v, got %v", expectedPubKey, c.vlessPublicKey)
	}
	if !bytes.Equal(c.vlessShortID, expectedShortID) {
		t.Errorf("expected shortID %v, got %v", expectedShortID, c.vlessShortID)
	}
	if c.vlessTarget != target {
		t.Errorf("expected target %s, got %s", target, c.vlessTarget)
	}
	if c.mtu != rawTunMTU {
		t.Errorf("expected mtu %d, got %d", rawTunMTU, c.mtu)
	}
	if streamKindText(transVLESS) != "VLESS" {
		t.Errorf("expected streamKindText VLESS, got %s", streamKindText(transVLESS))
	}
}
