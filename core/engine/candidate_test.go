package engine

import (
	"bytes"
	"testing"
)

func TestLadderAddVLESS(t *testing.T) {
	ladder := NewLadder()

	var uuid [16]byte
	copy(uuid[:], []byte("1234567890123456"))
	pubKey := []byte("0123456789abcdef0123456789abcdef")
	shortID := []byte("8283f99e")
	sni := "dl.google.com"
	target := "127.0.0.1:56006"

	ladder.AddVLESS("vless-test", "crp.guseynovia.ru", 443, uuid, pubKey, shortID, sni, target)

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
	if c.vlessUUID != uuid {
		t.Errorf("expected uuid %v, got %v", uuid, c.vlessUUID)
	}
	if !bytes.Equal(c.vlessPublicKey, pubKey) {
		t.Errorf("expected pubKey %v, got %v", pubKey, c.vlessPublicKey)
	}
	if !bytes.Equal(c.vlessShortID, shortID) {
		t.Errorf("expected shortID %v, got %v", shortID, c.vlessShortID)
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
