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

	routeMu        sync.Mutex
	activeRoutes   []RouteEntry
	lastCfg        Config
	hasActiveSetup bool
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

// getForwardTableSnapshot читает текущую IPv4-таблицу маршрутов ядра Windows за один системный вызов,
// одновременно определяя физический шлюз по умолчанию (исключая excludeIfIndex) и множество всех активных маршрутов.
func getForwardTableSnapshot(excludeIfIndex uint32) (net.IP, uint32, map[string]bool, error) {
	var table *windows.MibIpForwardTable2
	err := windows.GetIpForwardTable2(windows.AF_INET, &table)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("GetIpForwardTable2: %w", err)
	}
	defer windows.FreeMibTable(unsafe.Pointer(table))

	present := make(map[string]bool, int(table.NumEntries))
	if table.NumEntries == 0 {
		return nil, 0, present, fmt.Errorf("physical default gateway not found")
	}

	entries := unsafe.Slice(&table.Table[0], table.NumEntries)
	var bestGw net.IP
	var bestIfIndex uint32
	var bestMetric uint32 = 0xFFFFFFFF
	found := false

	for _, row := range entries {
		dstInet := (*windows.RawSockaddrInet4)(unsafe.Pointer(&row.DestinationPrefix.Prefix))
		nhInet := (*windows.RawSockaddrInet4)(unsafe.Pointer(&row.NextHop))
		dstIP := net.IPv4(dstInet.Addr[0], dstInet.Addr[1], dstInet.Addr[2], dstInet.Addr[3])
		nhIP := net.IPv4(nhInet.Addr[0], nhInet.Addr[1], nhInet.Addr[2], nhInet.Addr[3])
		pLen := int(row.DestinationPrefix.PrefixLength)

		present[RouteKey(RouteEntry{
			IfIndex:   row.InterfaceIndex,
			Dst:       dstIP,
			PrefixLen: pLen,
			NextHop:   nhIP,
		})] = true

		// Дефолтный маршрут имеет длину префикса 0 (0.0.0.0/0)
		if pLen == 0 && (excludeIfIndex == 0 || row.InterfaceIndex != excludeIfIndex) {
			if !nhIP.IsUnspecified() && !nhIP.IsLoopback() {
				if row.Metric < bestMetric {
					bestMetric = row.Metric
					bestGw = nhIP
					bestIfIndex = row.InterfaceIndex
					found = true
				}
			}
		}
	}

	if !found {
		return nil, 0, present, fmt.Errorf("physical default gateway not found")
	}
	return bestGw, bestIfIndex, present, nil
}

// GetPhysicalDefaultGatewayExcluding находит шлюз по умолчанию на физическом адаптере, игнорируя указанный интерфейс туннеля.
func GetPhysicalDefaultGatewayExcluding(excludeIfIndex uint32) (net.IP, uint32, error) {
	gw, ifIdx, _, err := getForwardTableSnapshot(excludeIfIndex)
	return gw, ifIdx, err
}

// GetPhysicalDefaultGateway находит шлюз по умолчанию на физическом адаптере.
func GetPhysicalDefaultGateway() (net.IP, uint32, error) {
	return GetPhysicalDefaultGatewayExcluding(0)
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

	// Получаем физический шлюз (исключая интерфейс Wintun)
	physGw, physIf, _ := GetPhysicalDefaultGatewayExcluding(uint32(cfg.IfIndex))

	var domainIPs []net.IP
	if len(cfg.CustomDomains) > 0 {
		domainIPs = ResolveDomains(cfg.CustomDomains)
	}

	routeMu.Lock()
	defer routeMu.Unlock()

	if cfg.Mode != "only" {
		// РЕЖИМ BYPASS (по умолчанию): базовые /1 маршруты в туннель
		cmds := [][]string{
			{"route", "add", "0.0.0.0", "mask", "128.0.0.0", cfg.Gateway, "metric", "1", "if", fmt.Sprint(cfg.IfIndex)},
			{"route", "add", "128.0.0.0", "mask", "128.0.0.0", cfg.Gateway, "metric", "1", "if", fmt.Sprint(cfg.IfIndex)},
		}
		for _, args := range cmds {
			if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
				return fmt.Errorf("route add %v: %w (%s)", args, err, out)
			}
		}
	}

	toAdd := BuildDesiredRoutes(cfg, physGw, physIf, domainIPs)

	// Быстрая пакетная установка всех специфичных маршрутов
	for _, entry := range toAdd {
		if err := addRoute(entry); err == nil {
			activeRoutes = append(activeRoutes, entry)
		}
	}
	lastCfg = cfg
	hasActiveSetup = true

	return nil
}

// Reconcile идемпотентно и без разрыва соединения восстанавливает маршруты туннеля и Split Tunneling
// после выхода ОС из сна (Sleep/Hibernate) или смены физического сетевого интерфейса (Wi-Fi ↔ Ethernet/Модем).
// Новые маршруты всегда добавляются ДО удаления устаревших, исключая окно утечки трафика.
func Reconcile() (added int, removed int, physIf uint32, err error) {
	routeMu.Lock()
	if !hasActiveSetup {
		routeMu.Unlock()
		return 0, 0, 0, nil
	}
	cfg := lastCfg
	routeMu.Unlock()

	var domainIPs []net.IP
	if len(cfg.CustomDomains) > 0 {
		domainIPs = ResolveDomains(cfg.CustomDomains)
	}

	routeMu.Lock()
	defer routeMu.Unlock()
	if !hasActiveSetup {
		return 0, 0, 0, nil
	}
	cfg = lastCfg
	wintunIf := uint32(cfg.IfIndex)
	wintunGw := net.ParseIP(cfg.Gateway).To4()

	physGw, detectedIf, present, snapErr := getForwardTableSnapshot(wintunIf)
	if present == nil {
		return 0, 0, 0, snapErr
	}

	// 1. В режиме bypass проверяем сохранность базовых /1 маршрутов Wintun
	if cfg.Mode != "only" && wintunIf > 0 && wintunGw != nil {
		halfRoutes := []RouteEntry{
			{IfIndex: wintunIf, Dst: net.IPv4(0, 0, 0, 0), PrefixLen: 1, NextHop: wintunGw, Metric: 1},
			{IfIndex: wintunIf, Dst: net.IPv4(128, 0, 0, 0), PrefixLen: 1, NextHop: wintunGw, Metric: 1},
		}
		for _, hr := range halfRoutes {
			if !present[RouteKey(hr)] {
				if aerr := addRoute(hr); aerr == nil {
					added++
				}
			}
		}
	}

	// Если физический шлюз ещё не поднялся после сна (Wi-Fi ассоциируется), не удаляем старые исключения раньше времени
	if cfg.Mode != "only" && (snapErr != nil || detectedIf == 0 || physGw == nil) {
		return added, 0, 0, nil
	}

	desired := BuildDesiredRoutes(cfg, physGw, detectedIf, domainIPs)
	toAdd, toRemove := DiffRoutes(desired, present, activeRoutes)
	if len(toAdd) == 0 && len(toRemove) == 0 {
		return added, 0, detectedIf, nil
	}

	// 2. Сначала добавляем все недостающие маршруты на актуальный интерфейс
	var nextActive []RouteEntry
	addedKeys := make(map[string]bool, len(desired))
	for _, entry := range desired {
		k := RouteKey(entry)
		if present[k] {
			nextActive = append(nextActive, entry)
			addedKeys[k] = true
			continue
		}
		if aerr := addRoute(entry); aerr == nil {
			added++
			nextActive = append(nextActive, entry)
			addedKeys[k] = true
		}
	}

	// 3. Только после установки новых маршрутов убираем устаревшие со старого интерфейса/шлюза
	for _, stale := range toRemove {
		if !addedKeys[RouteKey(stale)] {
			if derr := deleteRoute(stale); derr == nil {
				removed++
			}
		}
	}
	activeRoutes = nextActive
	return added, removed, detectedIf, nil
}

// Cleanup удаляет все установленные маршруты туннеля и исключений.
func Cleanup() error {
	routeMu.Lock()
	defer routeMu.Unlock()

	hasActiveSetup = false
	lastCfg = Config{}

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
