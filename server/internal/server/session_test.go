package server

import (
	"bytes"
	"net"
	"testing"
	"time"

	"sirin/server/internal/config"
	"sirin/server/internal/noise"
	"sirin/server/internal/obfs"
)

// clientSimulator эмулирует поведение мобильного клиента Sirin.
type clientSimulator struct {
	uuid      string
	wrapKey   []byte
	serverPub []byte

	obfsTx *obfs.State
	obfsRx *obfs.State

	ephPriv []byte
	ephPub  []byte
	h       [32]byte
	ck      [32]byte

	send *noise.CipherState
	recv *noise.CipherState
}

func newClientSimulator(uuid string, serverPub []byte) (*clientSimulator, error) {
	wrapKey, err := obfs.DeriveWrapKey(uuid)
	if err != nil {
		return nil, err
	}
	tx, _ := obfs.NewState(wrapKey)
	rx, _ := obfs.NewState(wrapKey)

	c := &clientSimulator{
		uuid:      uuid,
		wrapKey:   wrapKey,
		serverPub: serverPub,
		obfsTx:    tx,
		obfsRx:    rx,
	}

	copy(c.h[:], noise.ProtocolName)
	c.ck = c.h
	c.h = noise.MixHash(c.h, nil)
	c.h = noise.MixHash(c.h, serverPub)

	priv, pub, err := noise.GenerateKeyPair()
	if err != nil {
		return nil, err
	}
	c.ephPriv = priv
	c.ephPub = pub

	return c, nil
}

func (c *clientSimulator) makeMsg1() ([]byte, error) {
	c.h = noise.MixHash(c.h, c.ephPub)
	es, err := noise.ECDH(c.ephPriv, c.serverPub)
	if err != nil {
		return nil, err
	}
	nextCk, tempK := noise.MixKey(c.ck, es)
	c.ck = nextCk

	cs, err := noise.NewCipherState(tempK[:])
	if err != nil {
		return nil, err
	}

	ct, err := cs.Encrypt(c.h[:], []byte{noise.ProtocolVersion})
	if err != nil {
		return nil, err
	}
	c.h = noise.MixHash(c.h, ct)

	noiseMsg1 := append(append([]byte{}, c.ephPub...), ct...) // 49 байт
	return c.obfsTx.Wrap(noiseMsg1)                           // в RTP конверте
}

func (c *clientSimulator) handleMsg2(wire []byte) error {
	plain, err := c.obfsRx.Unwrap(wire)
	if err != nil {
		return err
	}
	rePub := plain[:32]
	ct := plain[32:]

	c.h = noise.MixHash(c.h, rePub)
	ee, err := noise.ECDH(c.ephPriv, rePub)
	if err != nil {
		return err
	}
	nextCk, tempK := noise.MixKey(c.ck, ee)
	c.ck = nextCk

	cs, err := noise.NewCipherState(tempK[:])
	if err != nil {
		return err
	}

	_, err = cs.Decrypt(c.h[:], ct)
	if err != nil {
		return err
	}
	c.h = noise.MixHash(c.h, ct)

	k1, k2 := noise.HKDF2(c.ck[:], nil)
	c.send, _ = noise.NewCipherState(k1[:])
	c.recv, _ = noise.NewCipherState(k2[:])
	return nil
}

func (c *clientSimulator) makeAuth() ([]byte, error) {
	authPlain := []byte("AUTH|" + c.uuid + "|test-device-id|echo")
	enc, err := c.send.Encrypt(nil, authPlain)
	if err != nil {
		return nil, err
	}
	return c.obfsTx.Wrap(enc)
}

func (c *clientSimulator) handleOK(wire []byte) (string, error) {
	plain, err := c.obfsRx.Unwrap(wire)
	if err != nil {
		return "", err
	}
	dec, err := c.recv.Decrypt(nil, plain)
	if err != nil {
		return "", err
	}
	return string(dec), nil
}

func TestFullHandshakeFlow(t *testing.T) {
	staticPriv, staticPub, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	testUUID := "9b1deb4d-3b7d-4bad-9bdd-2b0d7b3dcb6d"
	cfg := &config.Config{
		ListenAddr:   "127.0.0.1:0",
		TunIP:        "10.77.77.1",
		ClientSubnet: "10.77.77.0/24",
		AllowedUUIDs: []string{testUUID},
		SessionTTL:   120,
	}

	srv, err := New(cfg, staticPriv)
	if err != nil {
		t.Fatalf("New server: %v", err)
	}
	if err := srv.Start(); err != nil {
		t.Fatalf("Start server: %v", err)
	}
	defer srv.Stop()

	serverUDPAddr := srv.conn.LocalAddr().(*net.UDPAddr)

	clientConn, err := net.DialUDP("udp", nil, serverUDPAddr)
	if err != nil {
		t.Fatalf("DialUDP: %v", err)
	}
	defer clientConn.Close()

	client, err := newClientSimulator(testUUID, staticPub)
	if err != nil {
		t.Fatalf("newClientSimulator: %v", err)
	}

	// 1. Отправляем Msg1
	msg1Wire, err := client.makeMsg1()
	if err != nil {
		t.Fatalf("makeMsg1: %v", err)
	}
	if _, err := clientConn.Write(msg1Wire); err != nil {
		t.Fatalf("write msg1: %v", err)
	}

	// 2. Ждём Msg2
	buf := make([]byte, 2048)
	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := clientConn.Read(buf)
	if err != nil {
		t.Fatalf("read msg2: %v", err)
	}

	if err := client.handleMsg2(buf[:n]); err != nil {
		t.Fatalf("handleMsg2: %v", err)
	}

	// 3. Отправляем AUTH
	authWire, err := client.makeAuth()
	if err != nil {
		t.Fatalf("makeAuth: %v", err)
	}
	if _, err := clientConn.Write(authWire); err != nil {
		t.Fatalf("write auth: %v", err)
	}

	// 4. Ждём OK
	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err = clientConn.Read(buf)
	if err != nil {
		t.Fatalf("read OK: %v", err)
	}

	okStr, err := client.handleOK(buf[:n])
	if err != nil {
		t.Fatalf("handleOK: %v", err)
	}

	if !bytes.HasPrefix([]byte(okStr), []byte("OK|10.77.77.2|10.77.77.1")) {
		t.Fatalf("неожиданный ответ OK: %s", okStr)
	}
}

func TestUnknownUUIDDropped(t *testing.T) {
	staticPriv, staticPub, _ := noise.GenerateKeyPair()
	cfg := &config.Config{
		ListenAddr:   "127.0.0.1:0",
		TunIP:        "10.77.77.1",
		ClientSubnet: "10.77.77.0/24",
		AllowedUUIDs: []string{"valid-uuid-1", "valid-uuid-2"},
		SessionTTL:   120,
	}

	srv, _ := New(cfg, staticPriv)
	_ = srv.Start()
	defer srv.Stop()

	clientConn, _ := net.DialUDP("udp", nil, srv.conn.LocalAddr().(*net.UDPAddr))
	defer clientConn.Close()

	// Клиент с неизвестным UUID
	client, _ := newClientSimulator("unregistered-unknown-uuid", staticPub)
	msg1Wire, _ := client.makeMsg1()
	_, _ = clientConn.Write(msg1Wire)

	buf := make([]byte, 2048)
	_ = clientConn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	_, err := clientConn.Read(buf)
	if err == nil {
		t.Fatal("ожидалось молчание сервера на неизвестный UUID, получен ответ")
	}
}

func TestKeepaliveEcho(t *testing.T) {
	staticPriv, staticPub, _ := noise.GenerateKeyPair()
	testUUID := "test-uuid-for-echo"
	cfg := &config.Config{
		ListenAddr:   "127.0.0.1:0",
		TunIP:        "10.77.77.1",
		ClientSubnet: "10.77.77.0/24",
		AllowedUUIDs: []string{testUUID},
		SessionTTL:   120,
	}

	srv, _ := New(cfg, staticPriv)
	_ = srv.Start()
	defer srv.Stop()

	clientConn, _ := net.DialUDP("udp", nil, srv.conn.LocalAddr().(*net.UDPAddr))
	defer clientConn.Close()

	client, _ := newClientSimulator(testUUID, staticPub)

	// Проходим рукопожатие и авторизацию
	msg1, _ := client.makeMsg1()
	_, _ = clientConn.Write(msg1)

	buf := make([]byte, 2048)
	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _ := clientConn.Read(buf)
	_ = client.handleMsg2(buf[:n])

	auth, _ := client.makeAuth()
	_, _ = clientConn.Write(auth)

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _ = clientConn.Read(buf)
	_, _ = client.handleOK(buf[:n])

	// Отправляем метку keepalive (4 нулевых байта)
	keepaliveMarker := []byte{0, 0, 0, 0}
	wireEcho, err := client.obfsTx.Wrap(keepaliveMarker)
	if err != nil {
		t.Fatalf("Wrap keepalive: %v", err)
	}
	if _, err := clientConn.Write(wireEcho); err != nil {
		t.Fatalf("Write keepalive: %v", err)
	}

	// Читаем зеркальное эхо от сервера
	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err = clientConn.Read(buf)
	if err != nil {
		t.Fatalf("Read echo response: %v", err)
	}

	unwrappedEcho, err := client.obfsRx.Unwrap(buf[:n])
	if err != nil {
		t.Fatalf("Unwrap echo: %v", err)
	}

	if !bytes.Equal(unwrappedEcho, keepaliveMarker) {
		t.Fatalf("эхо не совпадает: %v != %v", unwrappedEcho, keepaliveMarker)
	}
}

func TestTunDisabledFallback(t *testing.T) {
	staticPriv, staticPub, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	testUUID := "fallback-test-uuid"
	cfg := &config.Config{
		ListenAddr:   "127.0.0.1:0",
		TunName:      "", // TUN отключен
		TunIP:        "10.77.77.1",
		TunNetmask:   "255.255.255.0",
		ClientSubnet: "10.77.77.0/24",
		AllowedUUIDs: []string{testUUID},
		SessionTTL:   60,
	}

	srv, err := New(cfg, staticPriv)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if srv.TunEnabled() {
		t.Fatalf("ожидалось tunEnabled == false")
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer srv.Stop()

	serverUDPAddr := srv.conn.LocalAddr().(*net.UDPAddr)
	clientConn, err := net.DialUDP("udp", nil, serverUDPAddr)
	if err != nil {
		t.Fatalf("DialUDP: %v", err)
	}
	defer clientConn.Close()

	client, err := newClientSimulator(testUUID, staticPub)
	if err != nil {
		t.Fatalf("newClientSimulator: %v", err)
	}

	// Handshake
	msg1, _ := client.makeMsg1()
	_, _ = clientConn.Write(msg1)

	buf := make([]byte, 2048)
	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _ := clientConn.Read(buf)
	_ = client.handleMsg2(buf[:n])

	auth, _ := client.makeAuth()
	_, _ = clientConn.Write(auth)

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _ = clientConn.Read(buf)
	_, _ = client.handleOK(buf[:n])

	// Отправляем симулированный IPv4 пакет (20 байт)
	fakeIPv4Packet := make([]byte, 20)
	fakeIPv4Packet[0] = 0x45                                     // IPv4, Header length = 5 (20 bytes)
	copy(fakeIPv4Packet[12:16], net.ParseIP("10.77.77.2").To4()) // src
	copy(fakeIPv4Packet[16:20], net.ParseIP("1.1.1.1").To4())    // dst

	wirePkt, err := client.obfsTx.Wrap(fakeIPv4Packet)
	if err != nil {
		t.Fatalf("Wrap fake IPv4: %v", err)
	}
	if _, err := clientConn.Write(wirePkt); err != nil {
		t.Fatalf("Write fake IPv4: %v", err)
	}

	// Даём серверу обработать пакет и залогировать
	time.Sleep(50 * time.Millisecond)
}

