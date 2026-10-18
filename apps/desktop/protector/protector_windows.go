//go:build windows

package protector

import (
	"encoding/binary"
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// WindowsProtector реализует engine.Protector для Windows.
// Использует setsockopt(IP_UNICAST_IF / IPV6_UNICAST_IF) для привязки 
// сокета к физическому адаптеру (не к туннелю).
type WindowsProtector struct {
	ifaceIndex uint32 // индекс физического интерфейса
}

func New(ifaceIndex uint32) *WindowsProtector {
	return &WindowsProtector{ifaceIndex: ifaceIndex}
}

// Protect реализует engine.Protector (Protect(fd int32) bool).
// Привязывает сокет к физ. адаптеру, чтобы трафик VPN не завернулся в сам туннель.
func (p *WindowsProtector) Protect(fd int32) bool {
	if p.ifaceIndex == 0 {
		return true
	}
	const IP_UNICAST_IF = 31
	const IPV6_UNICAST_IF = 31

	// IPv4: IP_UNICAST_IF принимает DWORD в сетевом порядке байт (network byte order)
	var idxBE uint32
	binary.BigEndian.PutUint32((*[4]byte)(unsafe.Pointer(&idxBE))[:], p.ifaceIndex)

	err4 := windows.Setsockopt(windows.Handle(uintptr(fd)), windows.IPPROTO_IP, IP_UNICAST_IF, (*byte)(unsafe.Pointer(&idxBE)), 4)

	// IPv6: IPV6_UNICAST_IF принимает DWORD в хостовом порядке байт
	idxHost := p.ifaceIndex
	_ = windows.Setsockopt(windows.Handle(uintptr(fd)), windows.IPPROTO_IPV6, IPV6_UNICAST_IF, (*byte)(unsafe.Pointer(&idxHost)), 4)

	return err4 == nil
}

// ProtectConn — привязка сокета через syscall.RawConn (на случай net.Dialer.Control)
func (p *WindowsProtector) ProtectConn(network, address string, c syscall.RawConn) error {
	var sockErr error
	err := c.Control(func(fd uintptr) {
		if !p.Protect(int32(fd)) {
			sockErr = fmt.Errorf("setsockopt IP_UNICAST_IF failed for fd=%d, ifaceIndex=%d", fd, p.ifaceIndex)
		}
	})
	if err != nil {
		return err
	}
	return sockErr
}
