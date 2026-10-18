//go:build linux

package main

import (
	"os/exec"
	"strings"

	"golang.org/x/sys/unix"
)

type linuxProtector struct {
	wanIface string
	wanGW    string
}

func newProtector(wanIface, wanGW string) *linuxProtector {
	return &linuxProtector{
		wanIface: strings.TrimSpace(wanIface),
		wanGW:    strings.TrimSpace(wanGW),
	}
}

// Protect привязывает исходящий сокет VPN-движка к физическому WAN-интерфейсу,
// исключая зацикливание трафика туннеля через самого себя.
func (p *linuxProtector) Protect(fd int32) bool {
	if p.wanIface != "" {
		_ = unix.BindToDevice(int(fd), p.wanIface)
	}
	_ = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_MARK, 0x1)
	return true
}

// ExcludeHost добавляет прямой маршрут /32 к серверу через WAN-шлюз.
func (p *linuxProtector) ExcludeHost(host string) {
	host = strings.TrimSpace(host)
	if p.wanGW != "" && host != "" {
		_ = exec.Command("ip", "route", "replace", host+"/32", "via", p.wanGW).Run()
	}
}
