package main

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"time"

	"meridian/engine/vless"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: reality-test <target-host:port>")
		os.Exit(1)
	}
	target := os.Args[1]

	// Public Key
	pubKeyB64 := "4ll5_ouUJr8P8CCxasWNcNu0Lb7tSIDacGmRskweoH4"
	pubKey, err := base64.RawURLEncoding.DecodeString(pubKeyB64)
	if err != nil {
		log.Fatalf("decode pubkey: %v", err)
	}
	log.Printf("pubkey len: %d", len(pubKey))

	// ShortID
	shortID, _ := hex.DecodeString("8283f99ec6a0f52e")
	log.Printf("shortID len: %d", len(shortID))

	// UUID
	uuidStr := "5f26ebc3-6eb0-45fb-a490-c8c92b86b644"
	var uuid [16]byte
	// parse UUID (формат: 8-4-4-4-12 hex)
	hexStr := ""
	for _, c := range uuidStr {
		if c != '-' {
			hexStr += string(c)
		}
	}
	uuidBytes, _ := hex.DecodeString(hexStr)
	copy(uuid[:], uuidBytes)

	serverHost := os.Getenv("REALITY_SERVER")
	if serverHost == "" {
		serverHost = "crp.guseynovia.ru"
	}

	cfg := &vless.RealityConfig{
		ServerHost:  serverHost,
		ServerPort:  443,
		PublicKey:   pubKey,
		ShortID:     shortID,
		SNI:         "dl.google.com",
		Fingerprint: "chrome",
	}

	client := vless.NewClient(uuid, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	log.Printf("connecting to %s via Reality...", target)
	conn, err := client.DialTCP(ctx, target)
	if err != nil {
		log.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	log.Printf("VLESS+Reality connected to %s", target)

	// HTTP/1.1 запрос
	host, _, err := net.SplitHostPort(target)
	if err != nil {
		host = target
	}
	req := "GET / HTTP/1.1\r\n" +
		"Host: " + host + "\r\n" +
		"User-Agent: Mozilla/5.0 (Test)\r\n" +
		"Connection: close\r\n" +
		"\r\n"

	if _, err := conn.Write([]byte(req)); err != nil {
		log.Fatalf("write: %v", err)
	}
	log.Printf("sent HTTP request, reading response...")

	// Читаем ответ (первые 2 КБ)
	buf := make([]byte, 2048)
	n, err := io.ReadAtLeast(conn, buf, 100)
	if err != nil && err != io.ErrUnexpectedEOF {
		log.Fatalf("read: %v (got %d bytes)", err, n)
	}

	fmt.Println("=== Response (first 500 bytes) ===")
	limit := 500
	if n < limit {
		limit = n
	}
	fmt.Println(string(buf[:limit]))
	fmt.Println("=== END ===")

	if n > 0 {
		fmt.Println("SUCCESS: Reality+VLESS works!")
	} else {
		fmt.Println("FAIL: no response")
	}

	_ = net.IPv4zero // prevent unused import
}
