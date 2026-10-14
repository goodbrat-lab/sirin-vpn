package server

import (
	"net"
	"strings"
	"testing"
	"time"

	"sirin/server/internal/config"
	"sirin/server/internal/noise"
)

func TestTCPHandshakeAndAuth(t *testing.T) {
	serverPriv, serverPub, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	testUUID := "9b1deb4d-3b7d-4bad-9bdd-2b0d7b3dcb6d"
	cfg := &config.Config{
		ListenAddr:    "127.0.0.1:0",
		TCPListenAddr: "127.0.0.1:0",
		AllowedUUIDs:  []string{testUUID},
		TunIP:         "10.77.77.1",
		SessionTTL:    30,
	}

	srv, err := New(cfg, serverPriv)
	if err != nil {
		t.Fatalf("New server: %v", err)
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start: %v", err)
	}
	defer srv.Stop()

	tcpAddr := srv.TCPAddr()
	if tcpAddr == "" {
		t.Fatalf("TCPAddr is empty")
	}

	// 1. Подключаемся клиентом по TCP
	conn, err := net.DialTimeout("tcp", tcpAddr, 2*time.Second)
	if err != nil {
		t.Fatalf("net.Dial TCP: %v", err)
	}
	defer conn.Close()

	clientLink := NewStreamLink(conn)
	sim, err := newClientSimulator(testUUID, serverPub)
	if err != nil {
		t.Fatalf("newClientSimulator: %v", err)
	}

	// 2. Клиент отправляет msg1
	wireMsg1, err := sim.makeMsg1()
	if err != nil {
		t.Fatalf("makeMsg1: %v", err)
	}
	if err := clientLink.WriteFrame(wireMsg1); err != nil {
		t.Fatalf("WriteFrame msg1: %v", err)
	}

	// 3. Клиент читает msg2
	wireMsg2, err := clientLink.ReadFrame()
	if err != nil {
		t.Fatalf("ReadFrame msg2: %v", err)
	}
	if err := sim.handleMsg2(wireMsg2); err != nil {
		t.Fatalf("handleMsg2: %v", err)
	}

	// 4. Клиент отправляет AUTH
	wireAuth, err := sim.makeAuth()
	if err != nil {
		t.Fatalf("makeAuth: %v", err)
	}
	if err := clientLink.WriteFrame(wireAuth); err != nil {
		t.Fatalf("WriteFrame AUTH: %v", err)
	}

	// 5. Клиент читает ответ OK
	wireOK, err := clientLink.ReadFrame()
	if err != nil {
		t.Fatalf("ReadFrame OK: %v", err)
	}
	okStr, err := sim.handleOK(wireOK)
	if err != nil {
		t.Fatalf("handleOK: %v", err)
	}
	if !strings.HasPrefix(okStr, "OK|10.77.77.") {
		t.Fatalf("Некорректный ответ сервера: %s", okStr)
	}

	// 6. Клиент отправляет тестовый IPv4 пакет
	// IPv4 пакет: 20 байт, src=10.77.77.2, dst=1.1.1.1
	ipv4Pkt := make([]byte, 20)
	ipv4Pkt[0] = 0x45 // version 4, IHL 5
	copy(ipv4Pkt[12:16], []byte{10, 77, 77, 2})
	copy(ipv4Pkt[16:20], []byte{1, 1, 1, 1})

	wireIP, err := sim.obfsTx.Wrap(ipv4Pkt)
	if err != nil {
		t.Fatalf("Wrap IPv4: %v", err)
	}
	if err := clientLink.WriteFrame(wireIP); err != nil {
		t.Fatalf("WriteFrame IPv4: %v", err)
	}

	// 7. Проверяем keepalive эхо
	keepalive := []byte("PING")
	wirePing, err := sim.obfsTx.Wrap(keepalive)
	if err != nil {
		t.Fatalf("Wrap PING: %v", err)
	}
	if err := clientLink.WriteFrame(wirePing); err != nil {
		t.Fatalf("WriteFrame PING: %v", err)
	}

	wirePong, err := clientLink.ReadFrame()
	if err != nil {
		t.Fatalf("ReadFrame PONG: %v", err)
	}
	plainPong, err := sim.obfsRx.Unwrap(wirePong)
	if err != nil {
		t.Fatalf("Unwrap PONG: %v", err)
	}
	if string(plainPong) != "PING" {
		t.Fatalf("expected PING echo, got %s", string(plainPong))
	}
}

func TestTCPUnknownUUIDDropped(t *testing.T) {
	serverPriv, serverPub, _ := noise.GenerateKeyPair()

	cfg := &config.Config{
		ListenAddr:    "127.0.0.1:0",
		TCPListenAddr: "127.0.0.1:0",
		AllowedUUIDs:  []string{"valid-uuid"},
	}

	srv, _ := New(cfg, serverPriv)
	_ = srv.Start()
	defer srv.Stop()

	conn, err := net.Dial("tcp", srv.TCPAddr())
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	clientLink := NewStreamLink(conn)
	sim, _ := newClientSimulator("intruder-uuid-unknown", serverPub)

	wireMsg1, _ := sim.makeMsg1()
	_ = clientLink.WriteFrame(wireMsg1)

	// Сервер должен закрыть соединение (EOF при чтении)
	_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, err = clientLink.ReadFrame()
	if err == nil {
		t.Fatalf("expected EOF or error from server for unknown UUID, got nil")
	}
}

func TestStreamLinkMaxFrame(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	sLink := NewStreamLink(server)
	cLink := NewStreamLink(client)

	// Попытка отправить кадр больше tcpFrameMax (2048 байт)
	tooBig := make([]byte, tcpFrameMax+1)
	err := sLink.WriteFrame(tooBig)
	if err == nil {
		t.Fatalf("expected error for oversized frame")
	}

	// Отправка нормального кадра
	normal := []byte("hello streamlink")
	go func() {
		_ = sLink.WriteFrame(normal)
	}()

	recvd, err := cLink.ReadFrame()
	if err != nil {
		t.Fatalf("ReadFrame: %v", err)
	}
	if string(recvd) != "hello streamlink" {
		t.Fatalf("expected %s, got %s", normal, recvd)
	}
}
