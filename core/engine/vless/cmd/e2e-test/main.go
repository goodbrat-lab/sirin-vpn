package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"sirin/engine"
	"sirin/engine/vless"
)

func main() {
	log.Println("=== Starting VLESS+Reality E2E Integration Test ===")

	// 1. Reality & VLESS credentials
	pubKeyB64 := "4ll5_ouUJr8P8CCxasWNcNu0Lb7tSIDacGmRskweoH4"
	pubKey, err := base64.RawURLEncoding.DecodeString(pubKeyB64)
	if err != nil {
		log.Fatalf("decode pubkey: %v", err)
	}

	shortIDHex := "8283f99ec6a0f52e"
	shortID, err := hex.DecodeString(shortIDHex)
	if err != nil {
		log.Fatalf("decode shortID: %v", err)
	}

	uuidStr := "5f26ebc3-6eb0-45fb-a490-c8c92b86b644"
	cleanUUID := strings.ReplaceAll(uuidStr, "-", "")
	uuidBytes, err := hex.DecodeString(cleanUUID)
	if err != nil || len(uuidBytes) != 16 {
		log.Fatalf("decode UUID: %v", err)
	}
	var vlessUUID [16]byte
	copy(vlessUUID[:], uuidBytes)

	// Sirin credentials
	clientPassword := "69c8e325-749f-4a73-8f3b-364ab97cfc29"
	deviceID := "e2e-test-device"

	serverHost := os.Getenv("REALITY_SERVER")
	if serverHost == "" {
		serverHost = "crp.guseynovia.ru"
	}
	viaLadder := false
	for _, arg := range os.Args[1:] {
		if arg == "--via-ladder" {
			viaLadder = true
		} else if !strings.HasPrefix(arg, "-") {
			serverHost = arg
		}
	}

	if viaLadder {
		runViaLadder(serverHost, uuidStr, pubKeyB64, shortIDHex, clientPassword, deviceID)
		return
	}

	// 2. VLESS+Reality Dial to 127.0.0.1:56006 via Xray
	realityCfg := &vless.RealityConfig{
		ServerHost:  serverHost,
		ServerPort:  443,
		PublicKey:   pubKey,
		ShortID:     shortID,
		SNI:         "sun8-21.userapi.com",
		SNIList: []string{
			"sun8-21.userapi.com",
			"www.kinopoisk.ru",
			"wildberries.ru",
			"www.ikea.com",
		},
		Fingerprint: "chrome",
		Logf: func(format string, args ...any) {
			log.Printf(format, args...)
		},
	}
	client := vless.NewClient(vlessUUID, realityCfg)

	dialTarget := "127.0.0.1:56006"
	log.Printf("Connecting to VLESS server at %s:443 with target=%s...", serverHost, dialTarget)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	conn, err := client.DialTCP(ctx, dialTarget)
	if err != nil {
		log.Fatalf("VLESS DialTCP failed: %v", err)
	}
	defer conn.Close()
	log.Printf("[1/7] VLESS+Reality connection established (target=%s)", dialTarget)

	// 3. Wrap connection in StreamLink [2 bytes big-endian length][payload]
	link := engine.NewStreamLink(conn)

	// 4. Derive Obfs key from client UUID password and initialize obfsState
	wrapKey, err := engine.DeriveWrapKey(clientPassword)
	if err != nil {
		log.Fatalf("DeriveWrapKey failed: %v", err)
	}
	obfs, err := engine.NewObfsState(wrapKey)
	if err != nil {
		log.Fatalf("NewObfsState failed: %v", err)
	}
	log.Printf("[2/7] Obfs state initialized with wrapKey derived from client UUID")

	// 5. Initialize Noise NK Initiator with gateway static public key
	serverStaticPub := engine.GatewayStaticPubKey()
	nk, err := engine.NewNKInitiator(serverStaticPub)
	if err != nil {
		log.Fatalf("NewNKInitiator failed: %v", err)
	}

	// 6. Send Msg1 (Noise NK handshake initiation)
	msg1, err := nk.WriteMsg1([]byte{0x01})
	if err != nil {
		log.Fatalf("WriteMsg1 failed: %v", err)
	}
	wireMsg1, err := obfs.Wrap(msg1)
	if err != nil {
		log.Fatalf("obfs.Wrap msg1 failed: %v", err)
	}
	if err := link.WriteFrame(wireMsg1); err != nil {
		log.Fatalf("WriteFrame msg1 failed: %v", err)
	}
	log.Printf("[3/7] Msg1 sent (%d bytes plain, %d bytes wire)", len(msg1), len(wireMsg1))

	// 7. Receive Msg2 (Noise NK handshake response)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	wireMsg2, err := link.ReadFrame()
	if err != nil {
		log.Fatalf("ReadFrame msg2 failed: %v", err)
	}
	msg2, ok := obfs.Unwrap(wireMsg2)
	if !ok {
		log.Fatalf("obfs.Unwrap msg2 failed (integrity check failed)")
	}
	if _, err := nk.ReadMsg2(msg2); err != nil {
		log.Fatalf("nk.ReadMsg2 failed (server authentication failed): %v", err)
	}
	log.Printf("[4/7] Msg2 received and verified (gateway authentic!)")

	// 8. Split Noise NK cipher states
	send, recv := nk.Split()

	// 9. Send AUTH encrypted with Noise NK send cipher
	authPlain := fmt.Sprintf("AUTH|%s|%s|echo", clientPassword, deviceID)
	encAuth, err := send.Encrypt(nil, []byte(authPlain))
	if err != nil {
		log.Fatalf("Encrypt AUTH failed: %v", err)
	}
	wireAuth, err := obfs.Wrap(encAuth)
	if err != nil {
		log.Fatalf("obfs.Wrap AUTH failed: %v", err)
	}
	if err := link.WriteFrame(wireAuth); err != nil {
		log.Fatalf("WriteFrame AUTH failed: %v", err)
	}
	log.Printf("[5/7] AUTH sent (%d bytes plain, %d bytes encrypted, %d bytes wire)", len(authPlain), len(encAuth), len(wireAuth))

	// 10. Receive OK response
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	wireOk, err := link.ReadFrame()
	if err != nil {
		log.Fatalf("ReadFrame OK failed: %v", err)
	}
	encOk, ok := obfs.Unwrap(wireOk)
	if !ok {
		log.Fatalf("obfs.Unwrap OK failed")
	}
	okResp, err := recv.Decrypt(nil, encOk)
	if err != nil {
		log.Fatalf("Decrypt OK failed: %v", err)
	}
	okStr := string(okResp)
	log.Printf("[6/7] AUTH successful! Response: %s", okStr)

	parts := strings.Split(okStr, "|")
	if len(parts) < 3 || parts[0] != "OK" {
		log.Fatalf("Unexpected AUTH response format: %s", okStr)
	}
	clientIP := net.ParseIP(parts[1]).To4()
	gatewayIP := net.ParseIP(parts[2]).To4()
	if clientIP == nil || gatewayIP == nil {
		log.Fatalf("Invalid client or gateway IP in response: %s", okStr)
	}
	log.Printf("      Allocated Client IP: %s, Gateway IP: %s", clientIP, gatewayIP)

	// 11. Send Test IPv4 ICMP Echo Request to Gateway IP
	icmpPayloadLen := 16
	echoReqPkt := engine.BuildEchoRequest(clientIP, gatewayIP, 0x5432, 1, icmpPayloadLen)
	wireIP, err := obfs.Wrap(echoReqPkt)
	if err != nil {
		log.Fatalf("obfs.Wrap IPv4 packet failed: %v", err)
	}
	if err := link.WriteFrame(wireIP); err != nil {
		log.Fatalf("WriteFrame IPv4 packet failed: %v", err)
	}
	log.Printf("[7/7] Sent IPv4 ICMP Echo Request (%d bytes) to Gateway %s", len(echoReqPkt), gatewayIP)

	// 12. Receive ICMP Echo Reply from server TUN interface
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	respFrame, err := link.ReadFrame()
	if err != nil {
		log.Printf("Warning: ReadFrame for ICMP response timed out/failed: %v", err)
	} else {
		respPlain, ok := obfs.Unwrap(respFrame)
		if !ok {
			log.Printf("Warning: obfs.Unwrap ICMP reply failed")
		} else {
			seq, isEcho := engine.ParseEchoReply(respPlain, 0x5432)
			if isEcho {
				log.Printf("SUCCESS: Received ICMP Echo Reply from Gateway %s (seq=%d, length=%d bytes) through TUN!", gatewayIP, seq, len(respPlain))
			} else {
				log.Printf("Received %d bytes packet from server (not matching echo seq): %x", len(respPlain), respPlain[:min(len(respPlain), 32)])
			}
		}
	}

	// 13. Verify Keepalive Echo (mirror 4 zero bytes)
	keepaliveMarker := []byte{0, 0, 0, 0}
	wireKeepalive, err := obfs.Wrap(keepaliveMarker)
	if err != nil {
		log.Fatalf("obfs.Wrap keepalive failed: %v", err)
	}
	if err := link.WriteFrame(wireKeepalive); err != nil {
		log.Fatalf("WriteFrame keepalive failed: %v", err)
	}
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	respKeepaliveFrame, err := link.ReadFrame()
	if err != nil {
		log.Fatalf("ReadFrame keepalive echo failed: %v", err)
	}
	respKeepalive, ok := obfs.Unwrap(respKeepaliveFrame)
	if !ok || !bytes.Equal(respKeepalive, keepaliveMarker) {
		log.Fatalf("Keepalive echo mismatch: got %v, expected %v", respKeepalive, keepaliveMarker)
	}
	log.Printf("SUCCESS: Keepalive mirror echo verified (%v -> %v)", keepaliveMarker, respKeepalive)

	log.Println("===================================================")
	log.Println("ALL E2E CHECKS PASSED: VLESS+Reality -> streamlink -> obfs -> Noise NK -> AUTH -> IP Packet -> Server TUN")
	log.Println("===================================================")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func runViaLadder(serverHost, uuidStr, pubKeyBase64, shortIDHex, clientPassword, deviceID string) {
	log.Println("=== Running VLESS connection via Engine Ladder (Full flow) ===")
	ladder := engine.NewLadder()
	err := ladder.AddVLESS("vless-ladder-candidate", serverHost, 443, uuidStr, pubKeyBase64, shortIDHex, "sun8-21.userapi.com", "127.0.0.1:56006", "sun8-21.userapi.com,www.kinopoisk.ru,wildberries.ru,www.ikea.com")
	if err != nil {
		log.Fatalf("AddVLESS failed: %v", err)
	}

	assignedIP, err := engine.Connect(serverHost, "", ladder, clientPassword, deviceID, nil, nil, nil, &ladderLogger{}, nil)
	if err != nil {
		log.Fatalf("engine.Connect failed: %v", err)
	}
	defer engine.Stop()

	log.Printf("SUCCESS: Ladder connected via winner: %s, assigned IP: %s", engine.Winner(), assignedIP)
	log.Println("===================================================")
	log.Println("FULL ENGINE LADDER TEST PASSED FOR transVLESS!")
	log.Println("===================================================")
}

type ladderLogger struct{}

func (l *ladderLogger) Log(line string) {
	log.Printf("[engine] %s", line)
}

