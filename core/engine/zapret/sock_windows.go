//go:build windows

package zapret

import (
	"net"
	"syscall"
	"unsafe"
)

var (
	modws2_32 = syscall.NewLazyDLL("ws2_32.dll")
	procSend  = modws2_32.NewProc("send")
)

const (
	msgOOB         = 0x1
	wsaeWouldBlock = syscall.Errno(10035)
)

// sendOOB отправляет data обычным TCP Write, затем ровно 1 срочный байт TCP Urgent (MSG_OOB) через Winsock2.
func sendOOB(conn net.Conn, data []byte, oobByte byte) error {
	tc, ok := conn.(*net.TCPConn)
	if !ok {
		_, err := conn.Write(data)
		return err
	}
	rc, err := tc.SyscallConn()
	if err != nil {
		_, werr := conn.Write(data)
		return werr
	}

	if len(data) > 0 {
		if _, err := tc.Write(data); err != nil {
			return err
		}
	}

	oobBuf := [1]byte{oobByte}
	var sendErr error
	cerr := rc.Write(func(fd uintptr) bool {
		r1, _, e1 := procSend.Call(
			fd,
			uintptr(unsafe.Pointer(&oobBuf[0])),
			1,
			uintptr(msgOOB),
		)
		if int32(r1) == -1 {
			if e1 == wsaeWouldBlock {
				return false
			}
			sendErr = e1
		}
		return true
	})
	if cerr != nil {
		return cerr
	}
	return sendErr
}

// setConnTTL временно меняет IP_TTL на TCP-сокете Windows.
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
		sockErr = syscall.SetsockoptInt(syscall.Handle(fd), syscall.IPPROTO_IP, syscall.IP_TTL, ttl)
	})
	return sockErr
}

// setPacketConnTTL временно меняет IP_TTL на UDP-сокете Windows.
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
		sockErr = syscall.SetsockoptInt(syscall.Handle(fd), syscall.IPPROTO_IP, syscall.IP_TTL, ttl)
	})
	return sockErr
}

func trySendRawFakeTCP(conn net.Conn, fakePayload []byte, ttl int) bool {
	return false
}
