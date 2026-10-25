package server

import (
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"sirin/server/internal/config"
	"sirin/server/internal/noise"
	"sirin/server/internal/obfs"
	"sirin/server/internal/tun"
)

type Server struct {
	cfg        *config.Config
	staticPriv []byte
	conn       *net.UDPConn

	tcpListener net.Listener
	tcpAddr     string

	tun        *tun.Device
	tunEnabled bool

	wrapMu         sync.RWMutex      // защита wrapKeys
	wrapKeys       map[string][]byte // UUID -> 32-байтный wrapKey
	clientsPath    string            // путь к clients.json
	clientsModTime time.Time         // mtime последней загрузки

	mu       sync.RWMutex
	sessions map[string]*Session // key = addr.String()
	byTunIP  map[string]*Session // key = tunIP.String()

	clientIPSeq atomic.Uint32
	stopCh      chan struct{}
	wg          sync.WaitGroup
}

func New(cfg *config.Config, staticPriv []byte) (*Server, error) {
	if len(staticPriv) != 32 {
		return nil, errors.New("staticPriv должен содержать 32 байта")
	}

	clientsPath := cfg.ClientsPath
	if clientsPath == "" {
		clientsPath = "/etc/sirin/clients.json"
	}

	uuids, modTime, err := loadAllowedUUIDs(clientsPath, cfg.AllowedUUIDs)
	if err != nil {
		log.Printf("[Sirin Server] WARN: clients.json: %v (fallback на toml)", err)
	}

	wrapKeys := make(map[string][]byte, len(uuids))
	for _, uuid := range uuids {
		key, err := obfs.DeriveWrapKey(uuid)
		if err != nil {
			return nil, fmt.Errorf("derive key %s: %w", uuid, err)
		}
		wrapKeys[uuid] = key
	}
	log.Printf("[Sirin Server] Загружено %d ключей (clients.json + toml fallback)", len(wrapKeys))

	s := &Server{
		cfg:            cfg,
		staticPriv:     append([]byte{}, staticPriv...),
		wrapKeys:       wrapKeys,
		clientsPath:    clientsPath,
		clientsModTime: modTime,
		sessions:       make(map[string]*Session),
		byTunIP:        make(map[string]*Session),
		stopCh:         make(chan struct{}),
	}
	s.clientIPSeq.Store(1) // чтобы первый клиент получил .2 (10.77.77.2)

	if cfg.TunName != "" {
		dev, err := tun.Open(cfg.TunName, 1280)
		if err != nil {
			return nil, fmt.Errorf("открытие TUN %s: %w", cfg.TunName, err)
		}
		if err := dev.Configure(cfg.TunIP, cfg.TunNetmask); err != nil {
			_ = dev.Close()
			return nil, fmt.Errorf("настройка TUN %s: %w", cfg.TunName, err)
		}
		if err := dev.AddRoute(cfg.ClientSubnet); err != nil {
			_ = dev.Close()
			return nil, fmt.Errorf("добавление маршрута %s через TUN: %w", cfg.ClientSubnet, err)
		}
		s.tun = dev
		s.tunEnabled = true
	}

	return s, nil
}

func (s *Server) TunEnabled() bool {
	return s.tunEnabled
}

func (s *Server) TCPAddr() string {
	return s.tcpAddr
}

func (s *Server) Start() error {
	addr, err := net.ResolveUDPAddr("udp", s.cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("ResolveUDPAddr: %w", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("ListenUDP: %w", err)
	}
	s.conn = conn

	log.Printf("[Sirin Server] UDP-сервер слушает на %s", s.cfg.ListenAddr)

	if s.cfg.TCPListenAddr != "" {
		l, err := net.Listen("tcp", s.cfg.TCPListenAddr)
		if err != nil {
			_ = s.conn.Close()
			return fmt.Errorf("ListenTCP: %w", err)
		}
		s.tcpListener = l
		s.tcpAddr = l.Addr().String()
		log.Printf("[Sirin Server] TCP-сервер слушает на %s", s.tcpAddr)

		s.wg.Add(1)
		go s.tcpAcceptLoop()
	}

	s.wg.Add(2)
	go s.readLoop()
	go s.sessionCleanupLoop()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.syncClientsLoop(s.stopCh)
	}()

	if s.tunEnabled {
		s.wg.Add(1)
		go s.tunReadLoop()
	}

	return nil
}

func (s *Server) Stop() {
	select {
	case <-s.stopCh:
		return
	default:
		close(s.stopCh)
	}

	if s.conn != nil {
		_ = s.conn.Close()
	}
	if s.tcpListener != nil {
		_ = s.tcpListener.Close()
	}
	if s.tunEnabled && s.tun != nil {
		_ = s.tun.Close()
	}
	s.wg.Wait()
	log.Printf("[Sirin Server] Сервер остановлен")
}

func (s *Server) readLoop() {
	defer s.wg.Done()
	buf := make([]byte, 2048)

	for {
		n, remoteAddr, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			select {
			case <-s.stopCh:
				return
			default:
				log.Printf("[Sirin Server] Ошибка чтения UDP: %v", err)
				continue
			}
		}

		pkt := make([]byte, n)
		copy(pkt, buf[:n])

		go s.handlePacket(pkt, remoteAddr)
	}
}

func (s *Server) handlePacket(pkt []byte, addr *net.UDPAddr) {
	addrKey := addr.String()

	s.mu.RLock()
	sess, exists := s.sessions[addrKey]
	s.mu.RUnlock()

	if !exists {
		s.handleNewSession(addr, pkt)
	} else {
		s.handleExistingSession(sess, pkt)
	}
}

// handleNewSession перебирает зарегистрированные UUID для распаковки первого пакета.
func (s *Server) handleNewSession(addr *net.UDPAddr, pkt []byte) {
	var matchedUUID string
	var matchedKey []byte
	var msg1Payload []byte

	// 1. Попытка демаскировки ChaCha20-Poly1305 ключами разрешённых пользователей
	s.wrapMu.RLock()
	keysCopy := make(map[string][]byte, len(s.wrapKeys))
	for k, v := range s.wrapKeys {
		keysCopy[k] = v
	}
	s.wrapMu.RUnlock()

	for uuid, key := range keysCopy {
		state, err := obfs.NewState(key)
		if err != nil {
			continue
		}
		plain, err := state.Unwrap(pkt)
		if err != nil {
			continue
		}
		// Длина msg1: 32 (e_pub) + 1 (версия 0x01) + 16 (тег) = 49 байт
		if len(plain) == 49 {
			matchedUUID = uuid
			matchedKey = key
			msg1Payload = plain
			break
		}
	}

	if matchedUUID == "" {
		log.Printf("[Sirin Server] Пакет от %s отброшен: нет подходящего UUID", addr.String())
		return
	}

	log.Printf("[Sirin Server] Обнаружен входящий сеанс от %s (UUID: %s)", addr.String(), matchedUUID)

	// 2. Инициализация Noise Responder
	resp, err := noise.NewResponder(s.staticPriv)
	if err != nil {
		log.Printf("[Sirin Server] Ошибка NewResponder: %v", err)
		return
	}

	// 3. Обработка msg1 клиенту
	p1, err := resp.ReadMsg1(msg1Payload)
	if err != nil {
		log.Printf("[Sirin Server] Сбой проверки msg1: %v", err)
		return
	}
	if len(p1) == 0 || p1[0] != noise.ProtocolVersion {
		log.Printf("[Sirin Server] Неверная версия в msg1: %v", p1)
		return
	}

	// 4. Формирование msg2 (Noise.WriteMsg2)
	msg2, err := resp.WriteMsg2([]byte{})
	if err != nil {
		log.Printf("[Sirin Server] Сбой формирования msg2: %v", err)
		return
	}

	// 5. Инициализация состояний obfs для сессии
	obfsRx, err := obfs.NewState(matchedKey)
	if err != nil {
		return
	}
	obfsTx, err := obfs.NewState(matchedKey)
	if err != nil {
		return
	}

	// 6. Упаковка msg2 в RTP и отправка клиенту
	wireMsg2, err := obfsTx.Wrap(msg2)
	if err != nil {
		log.Printf("[Sirin Server] Сбой упаковки msg2: %v", err)
		return
	}
	if _, err := s.conn.WriteToUDP(wireMsg2, addr); err != nil {
		log.Printf("[Sirin Server] Ошибка отправки msg2: %v", err)
		return
	}

	// 7. Завершение рукопожатия Noise: Split()
	if err := resp.Split(); err != nil {
		log.Printf("[Sirin Server] Ошибка Split: %v", err)
		return
	}

	// 8. Регистрация сессии в состоянии StateHandshakeSent
	sess := &Session{
		addr:       addr,
		clientUUID: matchedUUID,
		wrapKey:    matchedKey,
		obfsRx:     obfsRx,
		obfsTx:     obfsTx,
		noise:      resp,
		state:      StateHandshakeSent,
		lastSeen:   time.Now(),
	}

	s.mu.Lock()
	s.sessions[addr.String()] = sess
	s.mu.Unlock()

	log.Printf("[Sirin Server] Msg2 отправлен клиенту %s, ожидаю AUTH", addr.String())
}

// handleExistingSession обрабатывает пакеты в рамках уже опознанной сессии.
func (s *Server) handleExistingSession(sess *Session, pkt []byte) {
	sess.Touch()

	// 1. Распаковка RTP/ChaCha20-Poly1305
	plain, err := sess.obfsRx.Unwrap(pkt)
	if err != nil {
		return
	}

	st := sess.State()

	switch st {
	case StateHandshakeSent:
		// Ожидается сообщение AUTH, зашифрованное через Noise
		decrypted, err := sess.noise.DecryptAuth(plain)
		if err != nil {
			log.Printf("[Sirin Server] Ошибка DecryptAuth от %s: %v", sess.addr.String(), err)
			return
		}

		authStr := string(decrypted)
		parts := strings.Split(authStr, "|")
		// Ожидаемый формат: AUTH|<uuid>|<device>|<caps>
		if len(parts) < 2 || parts[0] != "AUTH" {
			log.Printf("[Sirin Server] Некорректный синтаксис AUTH от %s: %q", sess.addr.String(), authStr)
			return
		}

		clientUUID := parts[1]
		if clientUUID != sess.clientUUID {
			log.Printf("[Sirin Server] Несовпадение UUID в AUTH (%s != %s)", clientUUID, sess.clientUUID)
			return
		}

		// Выделение туннельного IP (начиная с .2, т.к. шлюз .1)
		seq := s.clientIPSeq.Add(1)
		tunIP := net.IPv4(10, 77, 77, byte(seq)) // 10.77.77.2, .3, ...
		sess.mu.Lock()
		sess.tunIP = tunIP
		sess.state = StateAuthenticated
		sess.mu.Unlock()

		s.mu.Lock()
		s.byTunIP[tunIP.String()] = sess
		s.mu.Unlock()

		// Формирование ответа: OK|<tunIP>|<gatewayIP>|<netmaskHex>|echo
		okResp := fmt.Sprintf("OK|%s|%s|ffffff00|echo", tunIP.String(), s.cfg.TunIP)
		encResp, err := sess.noise.EncryptResponse([]byte(okResp))
		if err != nil {
			log.Printf("[Sirin Server] Ошибка шифрования ответа OK: %v", err)
			return
		}

		wireResp, err := sess.obfsTx.Wrap(encResp)
		if err != nil {
			log.Printf("[Sirin Server] Ошибка упаковки ответа OK: %v", err)
			return
		}

		if _, err := s.conn.WriteToUDP(wireResp, sess.addr); err != nil {
			log.Printf("[Sirin Server] Ошибка отправки OK: %v", err)
			return
		}

		log.Printf("[Sirin Server] Клиент %s успешно авторизован! Выделен IP %s", sess.addr.String(), tunIP.String())

	case StateAuthenticated:
		// Трафик после авторизации: keepalive (< 20 байт) либо сырой IP-пакет
		if len(plain) < 20 {
			// Keepalive метка: отвечаем зеркальным эхо клиенту
			echoWire, err := sess.obfsTx.Wrap(plain)
			if err == nil {
				_, _ = s.conn.WriteToUDP(echoWire, sess.addr)
			}
			return
		}

		// Разбор IP пакета
		version := plain[0] >> 4
		if version == 4 {
			if len(plain) >= 20 {
				if s.tunEnabled {
					if _, err := s.tun.Write(plain); err != nil {
						log.Printf("[Sirin Server] tun write from %s: %v", sess.addr.String(), err)
					}
				} else {
					dstIP := net.IPv4(plain[16], plain[17], plain[18], plain[19]).String()
					log.Printf("[Sirin Server] IPv4 from %s: %d bytes dst=%s (no TUN)",
						sess.addr.String(), len(plain), dstIP)
				}
			}
		} else if version == 6 {
			// IPv6 сбрасывается (как и в клиенте)
			return
		}
	}
}

func (s *Server) tunReadLoop() {
	defer s.wg.Done()
	buf := make([]byte, 2048)

	for {
		select {
		case <-s.stopCh:
			return
		default:
		}

		n, err := s.tun.Read(buf)
		if err != nil {
			select {
			case <-s.stopCh:
				return
			default:
			}
			log.Printf("[Sirin Server] tun read: %v", err)
			continue
		}

		if n < 20 {
			continue // IPv4 минимум 20 байт
		}

		pkt := make([]byte, n)
		copy(pkt, buf[:n])
		go s.handleTunPacket(pkt)
	}
}

func (s *Server) handleTunPacket(pkt []byte) {
	if len(pkt) < 20 || pkt[0]>>4 != 4 {
		return
	}
	dstIP := net.IPv4(pkt[16], pkt[17], pkt[18], pkt[19]).String()

	s.mu.RLock()
	sess := s.byTunIP[dstIP]
	s.mu.RUnlock()

	if sess == nil {
		// Нет сессии — отбрасываем
		return
	}

	wire, err := sess.obfsTx.Wrap(pkt)
	if err != nil {
		log.Printf("[Sirin Server] Ошибка obfsTx.Wrap для туннельного пакета клиенту %s: %v", sess.RemoteAddr(), err)
		return
	}

	if sess.streamLink != nil {
		if err := sess.streamLink.WriteFrame(wire); err != nil {
			log.Printf("[Sirin Server] Ошибка отправки туннельного пакета по TCP клиенту %s: %v", sess.RemoteAddr(), err)
		}
	} else if s.conn != nil && sess.addr != nil {
		if _, err := s.conn.WriteToUDP(wire, sess.addr); err != nil {
			log.Printf("[Sirin Server] Ошибка отправки туннельного пакета клиенту %s: %v", sess.addr.String(), err)
		}
	}
}

func (s *Server) tcpAcceptLoop() {
	defer s.wg.Done()

	for {
		conn, err := s.tcpListener.Accept()
		if err != nil {
			select {
			case <-s.stopCh:
				return
			default:
				log.Printf("[Sirin Server] Ошибка Accept TCP: %v", err)
				continue
			}
		}

		go s.handleTCPConn(conn)
	}
}

func (s *Server) handleTCPConn(conn net.Conn) {
	defer conn.Close()

	remoteAddr := conn.RemoteAddr().String()

	// 1. Проверить, что соединение от 127.0.0.1 (от Xray)
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil || host != "127.0.0.1" {
		log.Printf("[Sirin Server] TCP from non-local %s — drop", remoteAddr)
		return
	}

	link := NewStreamLink(conn)

	// 2. Читаем первый кадр: msg1
	pkt, err := link.ReadFrame()
	if err != nil {
		log.Printf("[Sirin Server] Ошибка чтения первого TCP-кадра от %s: %v", remoteAddr, err)
		return
	}

	// 3. Подбор UUID для первого кадра
	var matchedUUID string
	var matchedKey []byte
	var msg1Payload []byte

	s.wrapMu.RLock()
	keysCopy := make(map[string][]byte, len(s.wrapKeys))
	for k, v := range s.wrapKeys {
		keysCopy[k] = v
	}
	s.wrapMu.RUnlock()

	for uuid, key := range keysCopy {
		state, err := obfs.NewState(key)
		if err != nil {
			continue
		}
		plain, err := state.Unwrap(pkt)
		if err != nil {
			continue
		}
		if len(plain) == 49 {
			matchedUUID = uuid
			matchedKey = key
			msg1Payload = plain
			break
		}
	}

	if matchedUUID == "" {
		log.Printf("[Sirin Server] TCP-пакет от %s отброшен: нет подходящего UUID", remoteAddr)
		return
	}

	log.Printf("[Sirin Server] Обнаружен входящий TCP-сеанс от %s (UUID: %s)", remoteAddr, matchedUUID)

	// 4. Инициализация Noise Responder
	resp, err := noise.NewResponder(s.staticPriv)
	if err != nil {
		log.Printf("[Sirin Server] Ошибка NewResponder: %v", err)
		return
	}

	// 5. Обработка msg1
	p1, err := resp.ReadMsg1(msg1Payload)
	if err != nil {
		log.Printf("[Sirin Server] Сбой проверки msg1: %v", err)
		return
	}
	if len(p1) == 0 || p1[0] != noise.ProtocolVersion {
		log.Printf("[Sirin Server] Неверная версия в msg1: %v", p1)
		return
	}

	// 6. Формирование msg2
	msg2, err := resp.WriteMsg2([]byte{})
	if err != nil {
		log.Printf("[Sirin Server] Сбой формирования msg2: %v", err)
		return
	}

	// 7. Инициализация obfs состояний
	obfsRx, err := obfs.NewState(matchedKey)
	if err != nil {
		return
	}
	obfsTx, err := obfs.NewState(matchedKey)
	if err != nil {
		return
	}

	// 8. Упаковка msg2 и отправка клиенту в streamlink кадре
	wireMsg2, err := obfsTx.Wrap(msg2)
	if err != nil {
		log.Printf("[Sirin Server] Сбой упаковки msg2: %v", err)
		return
	}
	if err := link.WriteFrame(wireMsg2); err != nil {
		log.Printf("[Sirin Server] Ошибка отправки msg2 по TCP: %v", err)
		return
	}

	// 9. Завершение рукопожатия Noise
	if err := resp.Split(); err != nil {
		log.Printf("[Sirin Server] Ошибка Split: %v", err)
		return
	}

	// 10. Создание сессии
	sess := &Session{
		tcpConn:    conn,
		streamLink: link,
		remoteStr:  remoteAddr,
		clientUUID: matchedUUID,
		wrapKey:    matchedKey,
		obfsRx:     obfsRx,
		obfsTx:     obfsTx,
		noise:      resp,
		state:      StateHandshakeSent,
		lastSeen:   time.Now(),
	}

	s.mu.Lock()
	s.sessions[remoteAddr] = sess
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.sessions, remoteAddr)
		if sess.tunIP != nil {
			delete(s.byTunIP, sess.tunIP.String())
		}
		sess.SetState(StateClosed)
		s.mu.Unlock()
		log.Printf("[Sirin Server] TCP-сессия %s завершена", remoteAddr)
	}()

	log.Printf("[Sirin Server] Msg2 отправлен TCP-клиенту %s, ожидаю AUTH", remoteAddr)

	// 11. Цикл чтения последующих кадров (AUTH, keepalive, IPv4)
	for {
		frame, err := link.ReadFrame()
		if err != nil {
			select {
			case <-s.stopCh:
				return
			default:
				return
			}
		}

		sess.Touch()

		plain, err := sess.obfsRx.Unwrap(frame)
		if err != nil {
			continue
		}

		st := sess.State()
		switch st {
		case StateHandshakeSent:
			decrypted, err := sess.noise.DecryptAuth(plain)
			if err != nil {
				log.Printf("[Sirin Server] Ошибка DecryptAuth от %s: %v", remoteAddr, err)
				return
			}

			authStr := string(decrypted)
			parts := strings.Split(authStr, "|")
			if len(parts) < 2 || parts[0] != "AUTH" {
				log.Printf("[Sirin Server] Некорректный синтаксис AUTH от %s: %q", remoteAddr, authStr)
				return
			}

			clientUUID := parts[1]
			if clientUUID != sess.clientUUID {
				log.Printf("[Sirin Server] Несовпадение UUID в AUTH (%s != %s)", clientUUID, sess.clientUUID)
				return
			}

			seq := s.clientIPSeq.Add(1)
			tunIP := net.IPv4(10, 77, 77, byte(seq))
			sess.mu.Lock()
			sess.tunIP = tunIP
			sess.state = StateAuthenticated
			sess.mu.Unlock()

			s.mu.Lock()
			s.byTunIP[tunIP.String()] = sess
			s.mu.Unlock()

			okResp := fmt.Sprintf("OK|%s|%s|ffffff00|echo", tunIP.String(), s.cfg.TunIP)
			encResp, err := sess.noise.EncryptResponse([]byte(okResp))
			if err != nil {
				log.Printf("[Sirin Server] Ошибка шифрования ответа OK: %v", err)
				return
			}

			wireResp, err := sess.obfsTx.Wrap(encResp)
			if err != nil {
				log.Printf("[Sirin Server] Ошибка упаковки ответа OK: %v", err)
				return
			}

			if err := link.WriteFrame(wireResp); err != nil {
				log.Printf("[Sirin Server] Ошибка отправки OK по TCP: %v", err)
				return
			}

			log.Printf("[Sirin Server] TCP-клиент %s успешно авторизован! Выделен IP %s", remoteAddr, tunIP.String())

		case StateAuthenticated:
			if len(plain) < 20 {
				// Keepalive метка: отвечаем зеркальным эхо клиенту
				echoWire, err := sess.obfsTx.Wrap(plain)
				if err == nil {
					_ = link.WriteFrame(echoWire)
				}
				continue
			}

			// Разбор IP пакета
			version := plain[0] >> 4
			if version == 4 && len(plain) >= 20 {
				if s.tunEnabled {
					if _, err := s.tun.Write(plain); err != nil {
						log.Printf("[Sirin Server] tun write from %s: %v", remoteAddr, err)
					}
				} else {
					dstIP := net.IPv4(plain[16], plain[17], plain[18], plain[19]).String()
					log.Printf("[Sirin Server] IPv4 from %s: %d bytes dst=%s (no TUN)",
						remoteAddr, len(plain), dstIP)
				}
			}
		}
	}
}

func (s *Server) sessionCleanupLoop() {
	defer s.wg.Done()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			ttl := time.Duration(s.cfg.SessionTTL) * time.Second
			now := time.Now()

			s.mu.Lock()
			for k, sess := range s.sessions {
				if now.Sub(sess.LastSeen()) > ttl {
					delete(s.sessions, k)
					if sess.tunIP != nil {
						delete(s.byTunIP, sess.tunIP.String())
					}
					sess.SetState(StateClosed)
					if sess.tcpConn != nil {
						_ = sess.tcpConn.Close()
					}
					log.Printf("[Sirin Server] Сессия %s закрыта по таймауту неактивности", k)
				}
			}
			s.mu.Unlock()
		}
	}
}
