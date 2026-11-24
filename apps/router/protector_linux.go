//go:build linux

package main

import (
	"log"
	"os/exec"
	"strings"
	"sync"

	"golang.org/x/sys/unix"
)

type linuxProtector struct {
	mu       sync.RWMutex
	wanIface string
	wanGW    string
}

func newProtector(wanIface, wanGW string) *linuxProtector {
	return &linuxProtector{
		wanIface: strings.TrimSpace(wanIface),
		wanGW:    strings.TrimSpace(wanGW),
	}
}

// UpdateWAN потокобезопасно обновляет текущий WAN-интерфейс и шлюз при переподключении PPPoE/DHCP.
func (p *linuxProtector) UpdateWAN(wanIface, wanGW string) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.wanIface = strings.TrimSpace(wanIface)
	p.wanGW = strings.TrimSpace(wanGW)
	p.mu.Unlock()
}

// CurrentWAN возвращает текущие имя WAN-интерфейса и шлюз.
func (p *linuxProtector) CurrentWAN() (string, string) {
	if p == nil {
		return "", ""
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.wanIface, p.wanGW
}

// Protect привязывает исходящий сокет VPN-движка к физическому WAN-интерфейсу,
// исключая зацикливание трафика туннеля через самого себя.
func (p *linuxProtector) Protect(fd int32) bool {
	wanIface, _ := p.CurrentWAN()
	if wanIface != "" {
		if err := unix.BindToDevice(int(fd), wanIface); err != nil {
			log.Printf("[protect] Предупреждение SO_BINDTODEVICE к %s: %v", wanIface, err)
		}
	}
	// Устанавливаем марку 0x2 (WAN), чтобы трафик самого туннеля направлялся в обход VPN
	// через таблицу маршрутизации 200 и не перехватывался правилами маркировки туннеля.
	_ = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_MARK, 0x2)
	return true
}

// ExcludeHost добавляет прямой маршрут /32 к серверу через WAN-интерфейс/шлюз.
func (p *linuxProtector) ExcludeHost(host string) {
	host = strings.TrimSpace(host)
	if host == "" {
		return
	}
	wanIface, wanGW := p.CurrentWAN()
	args := formatRouteArgs(host+"/32", wanInfo{iface: wanIface, gw: wanGW})
	if len(args) > 0 {
		log.Printf("[protect] ExcludeHost: ip %s", strings.Join(args, " "))
		_ = exec.Command("ip", args...).Run()
	}
}
