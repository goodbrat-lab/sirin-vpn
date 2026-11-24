//go:build windows

package protector

import (
	"encoding/binary"
	"fmt"
	"sync/atomic"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// WindowsProtector реализует engine.Protector для Windows.
// Использует setsockopt(IP_UNICAST_IF / IPV6_UNICAST_IF) для привязки
// сокета к физическому адаптеру (не к туннелю). Потокобезопасен при смене сети после сна.
type WindowsProtector struct {
	ifaceIndex atomic.Uint32 // индекс физического интерфейса
}

func New(ifaceIndex uint32) *WindowsProtector {
	p := &WindowsProtector{}
	p.ifaceIndex.Store(ifaceIndex)
	return p
}

// SetInterfaceIndex атомарно обновляет индекс физического интерфейса (например, после сна или смены Wi-Fi/Ethernet).
func (p *WindowsProtector) SetInterfaceIndex(ifaceIndex uint32) {
	if p != nil {
		p.ifaceIndex.Store(ifaceIndex)
	}
}

// InterfaceIndex возвращает текущий индекс защищаемого физического интерфейса.
func (p *WindowsProtector) InterfaceIndex() uint32 {
	if p == nil {
		return 0
	}
	return p.ifaceIndex.Load()
}

// Protect реализует engine.Protector (Protect(fd int32) bool).
// Привязывает сокет к физ. адаптеру, чтобы трафик VPN не завернулся в сам туннель.
func (p *WindowsProtector) Protect(fd int32) bool {
	idx := p.InterfaceIndex()
	if idx == 0 {
		return true
	}
	const IP_UNICAST_IF = 31
	const IPV6_UNICAST_IF = 31

	// IPv4: IP_UNICAST_IF принимает DWORD в сетевом порядке байт (network byte order)
	var idxBE uint32
	binary.BigEndian.PutUint32((*[4]byte)(unsafe.Pointer(&idxBE))[:], idx)

	err4 := windows.Setsockopt(windows.Handle(uintptr(fd)), windows.IPPROTO_IP, IP_UNICAST_IF, (*byte)(unsafe.Pointer(&idxBE)), 4)

	// IPv6: IPV6_UNICAST_IF принимает DWORD в хостовом порядке байт
	idxHost := idx
	_ = windows.Setsockopt(windows.Handle(uintptr(fd)), windows.IPPROTO_IPV6, IPV6_UNICAST_IF, (*byte)(unsafe.Pointer(&idxHost)), 4)

	return err4 == nil
}

// ProtectConn — привязка сокета через syscall.RawConn (на случай net.Dialer.Control)
func (p *WindowsProtector) ProtectConn(network, address string, c syscall.RawConn) error {
	var sockErr error
	err := c.Control(func(fd uintptr) {
		if !p.Protect(int32(fd)) {
			sockErr = fmt.Errorf("setsockopt IP_UNICAST_IF failed for fd=%d, ifaceIndex=%d", fd, p.InterfaceIndex())
		}
	})
	if err != nil {
		return err
	}
	return sockErr
}
