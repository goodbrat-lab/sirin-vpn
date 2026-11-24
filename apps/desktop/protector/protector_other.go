//go:build !windows

package protector

import (
	"sync/atomic"
	"syscall"
)

type WindowsProtector struct {
	ifaceIndex atomic.Uint32
}

func New(ifaceIndex uint32) *WindowsProtector {
	p := &WindowsProtector{}
	p.ifaceIndex.Store(ifaceIndex)
	return p
}

func (p *WindowsProtector) SetInterfaceIndex(ifaceIndex uint32) {
	if p != nil {
		p.ifaceIndex.Store(ifaceIndex)
	}
}

func (p *WindowsProtector) InterfaceIndex() uint32 {
	if p == nil {
		return 0
	}
	return p.ifaceIndex.Load()
}

func (p *WindowsProtector) Protect(fd int32) bool {
	return true
}

func (p *WindowsProtector) ProtectConn(network, address string, c syscall.RawConn) error {
	return nil
}
