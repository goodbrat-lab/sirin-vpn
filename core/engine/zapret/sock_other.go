//go:build !linux && !windows

package zapret

import (
	"net"
	"syscall"
)

func sendOOB(conn net.Conn, data []byte, oobByte byte) error {
	_, err := conn.Write(data)
	return err
}

func setConnTTL(conn net.Conn, ttl int) error {
	tc, ok := conn.(*net.TCPConn)
	if !ok {
		return nil
	}
	rc, err := tc.SyscallConn()
	if err != nil {
		return err
	}
	var sockErr error
	_ = rc.Control(func(fd uintptr) {
		sockErr = syscall.SetsockoptInt(int(fd), syscall.IPPROTO_IP, syscall.IP_TTL, ttl)
	})
	return sockErr
}

func setPacketConnTTL(pc net.PacketConn, ttl int) error {
	sc, ok := pc.(interface {
		SyscallConn() (syscall.RawConn, error)
	})
	if !ok {
		return nil
	}
	rc, err := sc.SyscallConn()
	if err != nil {
		return err
	}
	var sockErr error
	_ = rc.Control(func(fd uintptr) {
		sockErr = syscall.SetsockoptInt(int(fd), syscall.IPPROTO_IP, syscall.IP_TTL, ttl)
	})
	return sockErr
}

func trySendRawFakeTCP(conn net.Conn, fakePayload []byte, ttl int) bool {
	return false
}
