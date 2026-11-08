//go:build !linux

package main

import (
	"errors"
	"net"
	"syscall"
)

// getOriginalDst — stub-заглушка для сборки и модульного тестирования на Windows/macOS.
func getOriginalDst(conn *net.TCPConn) (*net.TCPAddr, error) {
	return nil, errors.New("SO_ORIGINAL_DST поддерживается исключительно на платформе Linux")
}

func markWanSocket(network, address string, c syscall.RawConn) error {
	return nil
}

func markVpnSocket(network, address string, c syscall.RawConn) error {
	return nil
}
