package server

import (
	"net"
	"sync"
	"time"

	"sirin/server/internal/noise"
	"sirin/server/internal/obfs"
)

type State int

const (
	StateNew State = iota
	StateHandshakeSent
	StateAuthenticated
	StateClosed
)

func (s State) String() string {
	switch s {
	case StateNew:
		return "NEW"
	case StateHandshakeSent:
		return "HANDSHAKE_SENT"
	case StateAuthenticated:
		return "AUTHENTICATED"
	case StateClosed:
		return "CLOSED"
	}
	return "UNKNOWN"
}

type Session struct {
	mu sync.Mutex

	addr       *net.UDPAddr
	clientUUID string
	wrapKey    []byte

	obfsRx *obfs.State
	obfsTx *obfs.State
	noise  *noise.Responder

	tunIP    net.IP
	state    State
	lastSeen time.Time
}

func (s *Session) Touch() {
	s.mu.Lock()
	s.lastSeen = time.Now()
	s.mu.Unlock()
}

func (s *Session) LastSeen() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastSeen
}

func (s *Session) State() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

func (s *Session) SetState(st State) {
	s.mu.Lock()
	s.state = st
	s.mu.Unlock()
}

func (s *Session) TunIP() net.IP {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tunIP
}
