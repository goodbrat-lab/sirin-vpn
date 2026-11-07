//go:build windows

package route

import (
	"fmt"
	"net"
	"os/exec"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modiphlpapi                  = windows.NewLazySystemDLL("iphlpapi.dll")
	procCreateIpForwardEntry2    = modiphlpapi.NewProc("CreateIpForwardEntry2")
	procDeleteIpForwardEntry2    = modiphlpapi.NewProc("DeleteIpForwardEntry2")
	procInitializeIpForwardEntry = modiphlpapi.NewProc("InitializeIpForwardEntry")

	routeMu      sync.Mutex
	activeRoutes []RouteEntry
)

const (
	errObjectAlreadyExists = windows.Errno(5010)
	errElementNotFound     = windows.Errno(1168)
	errFileNotFound        = windows.Errno(2)
)

func initializeIpForwardEntry(row *windows.MibIpForwardRow2) {
	procInitializeIpForwardEntry.Call(uintptr(unsafe.Pointer(row)))
}

func createIpForwardEntry2(row *windows.MibIpForwardRow2) error {
	r1, _, _ := procCreateIpForwardEntry2.Call(uintptr(unsafe.Pointer(row)))
	if r1 != 0 {
		errno := windows.Errno(r1)
		if errno == errObjectAlreadyExists {
			return nil
		}
		return errno
	}
	return nil
}

func deleteIpForwardEntry2(row *windows.MibIpForwardRow2) error {
	r1, _, _ := procDeleteIpForwardEntry2.Call(uintptr(unsafe.Pointer(row)))
	if r1 != 0 {
		errno := windows.Errno(r1)
		if errno == errElementNotFound || errno == errFileNotFound {
			return nil
		}
		return errno
	}
	return nil
}

// GetPhysicalDefaultGateway находит шлюз по умолчанию на физическом адаптере.
func GetPhysicalDefaultGateway() (net.IP, uint32, error) {
	var table *windows.MibIpForwardTable2
	err := windows.GetIpForwardTable2(windows.AF_INET, &table)
	if err != nil {
		return nil, 0, fmt.Errorf("GetIpForwardTable2: %w", err)
	}
	defer windows.FreeMibTable(unsafe.Pointer(table))

	entries := unsafe.Slice(&table.Table[0], table.NumEntries)
	var bestGw net.IP
	var bestIfIndex uint32
	var bestMetric uint32 = 0xFFFFFFFF
	found := false

	for _, row := range entries {
		// Дефолтный маршрут имеет длину префикса 0 (0.0.0.0/0)
		if row.DestinationPrefix.PrefixLength == 0 {
			inet4 := (*windows.RawSockaddrInet4)(unsafe.Pointer(&row.NextHop))
			gw := net.IPv4(inet4.Addr[0], inet4.Addr[1], inet4.Addr[2], inet4.Addr[3])
			if !gw.IsUnspecified() && !gw.IsLoopback() {
				if row.Metric < bestMetric {
					bestMetric = row.Metric
					bestGw = gw
					bestIfIndex = row.InterfaceIndex
					found = true
				}
			}
		}
	}

	if !found {
		return nil, 0, fmt.Errorf("physical default gateway not found")
	}
	return bestGw, bestIfIndex, nil
}

// addRoute добавляет маршрут через Win32 IP Helper API.
func addRoute(entry RouteEntry) error {
	var row windows.MibIpForwardRow2
	initializeIpForwardEntry(&row)
	row.InterfaceIndex = entry.IfIndex
	row.Metric = entry.Metric
	row.Protocol = 3 // MIB_IPPROTO_NETMGMT

	dst4 := entry.Dst.To4()
	if dst4 == nil {
		return fmt.Errorf("IPv4 required: %v", entry.Dst)
	}

	dstInet := (*windows.RawSockaddrInet4)(unsafe.Pointer(&row.DestinationPrefix.Prefix))
	dstInet.Family = windows.AF_INET
	copy(dstInet.Addr[:], dst4)
	row.DestinationPrefix.PrefixLength = uint8(entry.PrefixLen)

	if entry.NextHop != nil {
		nh4 := entry.NextHop.To4()
		if nh4 != nil {
			nextHop := (*windows.RawSockaddrInet4)(unsafe.Pointer(&row.NextHop))
			nextHop.Family = windows.AF_INET
			copy(nextHop.Addr[:], nh4)
		}
	}

	return createIpForwardEntry2(&row)
}

// deleteRoute удаляет маршрут через Win32 IP Helper API.
func deleteRoute(entry RouteEntry) error {
	var row windows.MibIpForwardRow2
	initializeIpForwardEntry(&row)
	row.InterfaceIndex = entry.IfIndex

	dst4 := entry.Dst.To4()
	if dst4 == nil {
		return nil
	}

	dstInet := (*windows.RawSockaddrInet4)(unsafe.Pointer(&row.DestinationPrefix.Prefix))
	dstInet.Family = windows.AF_INET
	copy(dstInet.Addr[:], dst4)
	row.DestinationPrefix.PrefixLength = uint8(entry.PrefixLen)

	if entry.NextHop != nil {
		nh4 := entry.NextHop.To4()
		if nh4 != nil {
			nextHop := (*windows.RawSockaddrInet4)(unsafe.Pointer(&row.NextHop))
			nextHop.Family = windows.AF_INET
			copy(nextHop.Addr[:], nh4)
		}
	}

	return deleteIpForwardEntry2(&row)
}

// Setup настраивает маршрутизацию туннеля Sirin (Full Tunnel / RuDirect / Split Tunneling).
func Setup(cfg Config) error {
	// 1. Очистить предыдущие маршруты
	_ = Cleanup()

	wintunGw := net.ParseIP(cfg.Gateway).To4()
	wintunIf := uint32(cfg.IfIndex)

	// Получаем физический шлюз
	physGw, physIf, physErr := GetPhysicalDefaultGateway()

	routeMu.Lock()
	defer routeMu.Unlock()

	var toAdd []RouteEntry

	if cfg.Mode == "only" {
		// РЕЖИМ ONLY: трафик идёт через провайдера, кроме явно указанных CIDR и доменов.
		// Направляем кастомные CIDR в Wintun:
		for _, cidrStr := range cfg.CustomCIDRs {
			ip, prefix, err := ParseCIDR(cidrStr)
			if err == nil {
				toAdd = append(toAdd, RouteEntry{
					IfIndex:   wintunIf,
					Dst:       ip,
					PrefixLen: prefix,
					NextHop:   wintunGw,
					Metric:    1,
				})
			}
		}

		// Разрешаем домены и направляем в Wintun:
		if len(cfg.CustomDomains) > 0 {
			domainIPs := ResolveDomains(cfg.CustomDomains)
			for _, ip := range domainIPs {
				toAdd = append(toAdd, RouteEntry{
					IfIndex:   wintunIf,
					Dst:       ip,
					PrefixLen: 32,
					NextHop:   wintunGw,
					Metric:    1,
				})
			}
		}
	} else {
		// РЕЖИМ BYPASS (по умолчанию): весь трафик через VPN туннель,
		// кроме RuDirect (если включен) и кастомных CIDR/доменов.

		// 1. Базовые /1 маршруты в туннель
		cmds := [][]string{
			{"route", "add", "0.0.0.0", "mask", "128.0.0.0", cfg.Gateway, "metric", "1", "if", fmt.Sprint(cfg.IfIndex)},
			{"route", "add", "128.0.0.0", "mask", "128.0.0.0", cfg.Gateway, "metric", "1", "if", fmt.Sprint(cfg.IfIndex)},
		}
		for _, args := range cmds {
			if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
				return fmt.Errorf("route add %v: %w (%s)", args, err, out)
			}
		}

		// Если есть физический шлюз, настраиваем исключения (RuDirect и Bypass)
		if physErr == nil && physGw != nil && physIf > 0 {
			// 2. RuDirect: все 8641 RU-диапазонов направляем через физ. адаптер
			if cfg.RuDirect {
				ruCIDRs := GetRuCIDRs()
				for _, c := range ruCIDRs {
					toAdd = append(toAdd, RouteEntry{
						IfIndex:   physIf,
						Dst:       c.IP,
						PrefixLen: c.PrefixLen,
						NextHop:   physGw,
						Metric:    5,
					})
				}
			}

			// 3. Кастомные CIDR в обход VPN
			for _, cidrStr := range cfg.CustomCIDRs {
				ip, prefix, err := ParseCIDR(cidrStr)
				if err == nil {
					toAdd = append(toAdd, RouteEntry{
						IfIndex:   physIf,
						Dst:       ip,
						PrefixLen: prefix,
						NextHop:   physGw,
						Metric:    5,
					})
				}
			}

			// 4. Кастомные домены в обход VPN
			if len(cfg.CustomDomains) > 0 {
				domainIPs := ResolveDomains(cfg.CustomDomains)
				for _, ip := range domainIPs {
					toAdd = append(toAdd, RouteEntry{
						IfIndex:   physIf,
						Dst:       ip,
						PrefixLen: 32,
						NextHop:   physGw,
						Metric:    5,
					})
				}
			}
		}
	}

	// Быстрая пакетная установка всех специфичных маршрутов
	for _, entry := range toAdd {
		if err := addRoute(entry); err == nil {
			activeRoutes = append(activeRoutes, entry)
		}
	}

	return nil
}

// Cleanup удаляет все установленные маршруты туннеля и исключений.
func Cleanup() error {
	routeMu.Lock()
	defer routeMu.Unlock()

	// Удаляем маршруты /1
	exec.Command("route", "delete", "0.0.0.0", "mask", "128.0.0.0").Run()
	exec.Command("route", "delete", "128.0.0.0", "mask", "128.0.0.0").Run()

	// Удаляем все добавленные специфичные маршруты
	for i := len(activeRoutes) - 1; i >= 0; i-- {
		_ = deleteRoute(activeRoutes[i])
	}
	activeRoutes = nil

	return nil
}

// ActiveRouteCount возвращает количество активных установленных маршрутов.
func ActiveRouteCount() int {
	routeMu.Lock()
	defer routeMu.Unlock()
	return len(activeRoutes)
}

// SetupDNS настраивает DNS на Wintun интерфейсе.
func SetupDNS(ifaceName string, dns []string) error {
	if len(dns) == 0 {
		return nil
	}
	args := []string{"interface", "ip", "set", "dnsservers", "name=" + ifaceName,
		"static", dns[0], "register=none"}
	if out, err := exec.Command("netsh", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("netsh set dnsservers: %w (%s)", err, out)
	}
	for i, d := range dns[1:] {
		args := []string{"interface", "ip", "add", "dnsservers", "name=" + ifaceName,
			d, fmt.Sprintf("index=%d", i+2)}
		exec.Command("netsh", args...).Run()
	}
	return nil
}

// CleanupDNS восстанавливает DNS (dhcp).
func CleanupDNS(ifaceName string) error {
	args := []string{"interface", "ip", "set", "dnsservers", "name=" + ifaceName, "dhcp"}
	exec.Command("netsh", args...).Run()
	return nil
}
