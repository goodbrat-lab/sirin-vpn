//go:build !linux

package main

import (
	"errors"
	"net"
)

// getOriginalDst — stub-заглушка для сборки и модульного тестирования на Windows/macOS.
func getOriginalDst(conn *net.TCPConn) (*net.TCPAddr, error) {
	return nil, errors.New("SO_ORIGINAL_DST поддерживается исключительно на платформе Linux")
}
