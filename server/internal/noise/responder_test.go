package noise

import (
	"bytes"
	"errors"
	"fmt"
	"testing"
)

// testInitiator реализует независимую версию инициатора Noise NK для тестирования.
type testInitiator struct {
	h       [32]byte
	ck      [32]byte
	rsPub   []byte
	ephPriv []byte
	ephPub  []byte

	send *CipherState // k1 (клиент -> сервер)
	recv *CipherState // k2 (сервер -> клиент)
}

func newTestInitiator(rsPub []byte) (*testInitiator, error) {
	if len(rsPub) != 32 {
		return nil, errors.New("rsPub должен быть 32 байта")
	}

	ti := &testInitiator{
		rsPub: append([]byte{}, rsPub...),
	}

	// Инициализация h и ck
	copy(ti.h[:], ProtocolName)
	ti.ck = ti.h

	// MixHash(prologue=nil) по Noise §5.2
	ti.h = MixHash(ti.h, nil)

	// Предсообщение: "<- s"
	ti.h = MixHash(ti.h, ti.rsPub)

	// Генерация эфемерного ключа инициатора (e)
	priv, pub, err := GenerateKeyPair()
	if err != nil {
		return nil, err
	}
	ti.ephPriv = priv
	ti.ephPub = pub

	return ti, nil
}

func (ti *testInitiator) writeMsg1(payload []byte) ([]byte, error) {
	// 1. mixHash(ephPub)
	ti.h = MixHash(ti.h, ti.ephPub)

	// 2. es = ECDH(ephPriv, rsPub)
	es, err := ECDH(ti.ephPriv, ti.rsPub)
	if err != nil {
		return nil, fmt.Errorf("ECDH es: %w", err)
	}

	// 3. mixKey(es)
	nextCk, tempK := MixKey(ti.ck, es)
	ti.ck = nextCk

	// 4. encrypt(payload) с ad = ti.h
	cs, err := NewCipherState(tempK[:])
	if err != nil {
		return nil, err
	}
	ct, err := cs.Encrypt(ti.h[:], payload)
	if err != nil {
		return nil, err
	}

	// 5. mixHash(ct)
	ti.h = MixHash(ti.h, ct)

	return append(append([]byte{}, ti.ephPub...), ct...), nil
}

func (ti *testInitiator) readMsg2(msg []byte) ([]byte, error) {
	if len(msg) < 32+16 {
		return nil, errors.New("msg2 короче 48 байт")
	}
	rePub := msg[:32]
	ct := msg[32:]

	// 1. mixHash(rePub)
	ti.h = MixHash(ti.h, rePub)

	// 2. ee = ECDH(ephPriv, rePub)
	ee, err := ECDH(ti.ephPriv, rePub)
	if err != nil {
		return nil, fmt.Errorf("ECDH ee: %w", err)
	}

	// 3. mixKey(ee)
	nextCk, tempK := MixKey(ti.ck, ee)
	ti.ck = nextCk

	// 4. decrypt(ct) с ad = ti.h
	cs, err := NewCipherState(tempK[:])
	if err != nil {
		return nil, err
	}
	pt, err := cs.Decrypt(ti.h[:], ct)
	if err != nil {
		return nil, fmt.Errorf("decrypt msg2: %w", err)
	}

	// 5. mixHash(ct)
	ti.h = MixHash(ti.h, ct)

	return pt, nil
}

func (ti *testInitiator) split() error {
	k1, k2 := HKDF2(ti.ck[:], nil)
	sendCS, err := NewCipherState(k1[:])
	if err != nil {
		return err
	}
	recvCS, err := NewCipherState(k2[:])
	if err != nil {
		return err
	}
	ti.send = sendCS
	ti.recv = recvCS
	return nil
}

func TestFullNoiseHandshakeAndAuth(t *testing.T) {
	// 1. Сервер генерирует статический ключ
	serverPriv, serverPub, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	responder, err := NewResponder(serverPriv)
	if err != nil {
		t.Fatalf("NewResponder: %v", err)
	}

	// 2. Клиент инициализируется со статическим публичным ключом сервера
	initiator, err := newTestInitiator(serverPub)
	if err != nil {
		t.Fatalf("newTestInitiator: %v", err)
	}

	// 3. Клиент формирует msg1 (49 байт: 32 ключа + 1 байт версии 0x01 + 16 байт тега)
	msg1, err := initiator.writeMsg1([]byte{ProtocolVersion})
	if err != nil {
		t.Fatalf("initiator.writeMsg1: %v", err)
	}
	if len(msg1) != 49 {
		t.Fatalf("длина msg1 = %d, ожидалась 49", len(msg1))
	}

	// 4. Сервер принимает msg1 и проверяет payload
	p1, err := responder.ReadMsg1(msg1)
	if err != nil {
		t.Fatalf("responder.ReadMsg1: %v", err)
	}
	if !bytes.Equal(p1, []byte{ProtocolVersion}) {
		t.Fatalf("payload1 не совпадает: получено %v, ожидалось %v", p1, []byte{ProtocolVersion})
	}

	// 5. Сервер формирует msg2 (48 байт: 32 ключа + 0 байт полезной нагрузки + 16 байт тега)
	msg2, err := responder.WriteMsg2([]byte{})
	if err != nil {
		t.Fatalf("responder.WriteMsg2: %v", err)
	}
	if len(msg2) != 48 {
		t.Fatalf("длина msg2 = %d, ожидалась 48", len(msg2))
	}

	// 6. Клиент принимает msg2
	p2, err := initiator.readMsg2(msg2)
	if err != nil {
		t.Fatalf("initiator.readMsg2: %v", err)
	}
	if len(p2) != 0 {
		t.Fatalf("payload2 не пустой: %v", p2)
	}

	// 7. Обе стороны выполняют Split()
	if err := responder.Split(); err != nil {
		t.Fatalf("responder.Split: %v", err)
	}
	if err := initiator.split(); err != nil {
		t.Fatalf("initiator.split: %v", err)
	}

	// 8. Обмен AUTH (клиент -> сервер)
	authMsg := []byte("AUTH|test-password-1234|device-x|caps")
	authCipher, err := initiator.send.Encrypt(nil, authMsg)
	if err != nil {
		t.Fatalf("initiator.send.Encrypt auth: %v", err)
	}

	decryptedAuth, err := responder.DecryptAuth(authCipher)
	if err != nil {
		t.Fatalf("responder.DecryptAuth: %v", err)
	}
	if !bytes.Equal(decryptedAuth, authMsg) {
		t.Fatalf("decryptedAuth не совпадает: %s != %s", string(decryptedAuth), string(authMsg))
	}

	// 9. Ответ сервера (сервер -> клиент)
	responseMsg := []byte("OK|10.77.77.2|10.77.77.1|ffffff00|echo")
	respCipher, err := responder.EncryptResponse(responseMsg)
	if err != nil {
		t.Fatalf("responder.EncryptResponse: %v", err)
	}

	decryptedResp, err := initiator.recv.Decrypt(nil, respCipher)
	if err != nil {
		t.Fatalf("initiator.recv.Decrypt response: %v", err)
	}
	if !bytes.Equal(decryptedResp, responseMsg) {
		t.Fatalf("decryptedResp не совпадает: %s != %s", string(decryptedResp), string(responseMsg))
	}
}

func TestHandshakeBadVersion(t *testing.T) {
	serverPriv, serverPub, _ := GenerateKeyPair()
	responder, _ := NewResponder(serverPriv)
	initiator, _ := newTestInitiator(serverPub)

	// Отправляем версию 0x02 вместо 0x01
	msg1, err := initiator.writeMsg1([]byte{0x02})
	if err != nil {
		t.Fatalf("writeMsg1: %v", err)
	}

	_, err = responder.ReadMsg1(msg1)
	if err == nil {
		t.Fatal("ожидалась ошибка из-за неверной версии протокола, получен nil")
	}
}

func TestHandshakeWrongStaticKey(t *testing.T) {
	serverPriv, _, _ := GenerateKeyPair()
	responder, _ := NewResponder(serverPriv)

	// Клиент пытается подключиться к другому статическому ключу
	_, fakePub, _ := GenerateKeyPair()
	initiator, _ := newTestInitiator(fakePub)

	msg1, err := initiator.writeMsg1([]byte{ProtocolVersion})
	if err != nil {
		t.Fatalf("writeMsg1: %v", err)
	}

	_, err = responder.ReadMsg1(msg1)
	if err == nil {
		t.Fatal("ожидалась ошибка аутентификации (чужой статический ключ), получен nil")
	}
}

func TestHandshakeTamperedMsg1(t *testing.T) {
	serverPriv, serverPub, _ := GenerateKeyPair()
	responder, _ := NewResponder(serverPriv)
	initiator, _ := newTestInitiator(serverPub)

	msg1, _ := initiator.writeMsg1([]byte{ProtocolVersion})
	// Портим один байт в шифротексте
	msg1[len(msg1)-1] ^= 0xFF

	_, err := responder.ReadMsg1(msg1)
	if err == nil {
		t.Fatal("ожидалась ошибка проверки тега AEAD, получен nil")
	}
}
