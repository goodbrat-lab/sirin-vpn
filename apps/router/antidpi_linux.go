//go:build linux

package main

import (
	"fmt"
	"net"
	"syscall"
	"unsafe"
)

const (
	// SO_ORIGINAL_DST из netfilter_ipv4.h для извлечения исходного IP при REDIRECT
	SO_ORIGINAL_DST = 80
)

// getOriginalDst извлекает первоначальный адрес назначения TCP-соединения,
// перенаправленного через nftables/iptables REDIRECT.
func getOriginalDst(conn *net.TCPConn) (*net.TCPAddr, error) {
	rawConn, err := conn.SyscallConn()
	if err != nil {
		return nil, fmt.Errorf("SyscallConn: %w", err)
	}

	var addr syscall.RawSockaddrInet4
	var sockErr error

	err = rawConn.Control(func(fd uintptr) {
		size := uint32(unsafe.Sizeof(addr))
		_, _, errno := syscall.Syscall6(
			syscall.SYS_GETSOCKOPT,
			fd,
			syscall.SOL_IP,
			SO_ORIGINAL_DST,
			uintptr(unsafe.Pointer(&addr)),
			uintptr(unsafe.Pointer(&size)),
			0,
		)
		if errno != 0 {
			sockErr = errno
		}
	})

	if err != nil {
		return nil, fmt.Errorf("Control: %w", err)
	}
	if sockErr != nil {
		return nil, fmt.Errorf("getsockopt SO_ORIGINAL_DST: %w", sockErr)
	}

	ip := net.IPv4(addr.Addr[0], addr.Addr[1], addr.Addr[2], addr.Addr[3])
	portBytes := (*[2]byte)(unsafe.Pointer(&addr.Port))
	port := int(portBytes[0])<<8 | int(portBytes[1])

	return &net.TCPAddr{IP: ip, Port: port}, nil
}
