package main

import (
	_ "embed"
	"bufio"
	"bytes"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"sirin/engine/desktop"
)

//go:embed configs/ru-ranges.txt
var ruRangesData string

type wanInfo struct {
	iface string
	gw    string
}

// isPointToPoint проверяет, является ли интерфейс точкой-точкой (PPPoE, ppp*).
// Для таких интерфейсов маршрут в Linux должен создаваться без шлюза via (dev pppoe-wan).
func isPointToPoint(ifaceName string) bool {
	if strings.HasPrefix(ifaceName, "ppp") {
		return true
	}
	if iface, err := net.InterfaceByName(ifaceName); err == nil {
		if iface.Flags&net.FlagPointToPoint != 0 {
			return true
		}
	}
	return false
}

// formatRouteArgs формирует аргументы команды `ip route replace <dst> ...`
// с учётом типа интерфейса (PPPoE / Point-to-Point требует dev без via).
func formatRouteArgs(dst string, wan wanInfo) []string {
	dst = strings.TrimSpace(dst)
	if dst == "" {
		return nil
	}
	if isPointToPoint(wan.iface) && wan.iface != "" {
		return []string{"route", "replace", dst, "dev", wan.iface}
	}
	if wan.gw != "" && wan.iface != "" {
		return []string{"route", "replace", dst, "via", wan.gw, "dev", wan.iface}
	}
	if wan.iface != "" {
		return []string{"route", "replace", dst, "dev", wan.iface}
	}
	if wan.gw != "" {
		return []string{"route", "replace", dst, "via", wan.gw}
	}
	return nil
}

// checkTproxyConflict проверяет наличие правил TPROXY (sing-box, xray, passwall)
// в файрволе роутера и выдаёт предупреждение в лог, если они могут перехватывать трафик.
func checkTproxyConflict(serverHosts []string) {
	hasTproxy := false
	source := ""

	// 1. Проверяем nftables (OpenWrt 22+)
	if out, err := exec.Command("nft", "list", "ruleset").Output(); err == nil && len(out) > 0 {
		if strings.Contains(strings.ToLower(string(out)), "tproxy") {
			hasTproxy = true
			source = "nftables"
		}
	}

	// 2. Проверяем iptables (OpenWrt 21 и старые/legacy таблицы)
	if !hasTproxy {
		if out, err := exec.Command("iptables-save").Output(); err == nil && len(out) > 0 {
			if strings.Contains(strings.ToUpper(string(out)), "TPROXY") {
				hasTproxy = true
				source = "iptables"
			}
		}
	}

	if hasTproxy {
		log.Printf("[tproxy] ВНИМАНИЕ: Обнаружен TPROXY (%s, активен sing-box/xray/passwall)!", source)
		log.Printf("[tproxy] Исходящие UDP-пакеты могут перехватываться sing-box и приводить к зависанию туннеля.")
		log.Printf("[tproxy] Решение: перед запуском Sirin остановите sing-box:")
		log.Printf("[tproxy]   /etc/init.d/sing-box stop")
		if len(serverHosts) > 0 {
			log.Printf("[tproxy] Либо добавьте исключение в sing-box для адреса сервера: %s", strings.Join(serverHosts, ", "))
		}
	}
}

// parseDefaultRoute разбирает вывод команды `ip route show default`.
// Корректно определяет интерфейсы с PPPoE (например, dev pppoe-wan без via),
// а также выбирает маршрут с наименьшей метрикой, если маршрутов по умолчанию несколько.
func parseDefaultRoute(output []byte) wanInfo {
	scanner := bufio.NewScanner(bytes.NewReader(output))
	var bestWan wanInfo
	bestMetric := -1

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// Проверяем строки маршрутов по умолчанию
		if !strings.HasPrefix(line, "default") && !strings.HasPrefix(line, "0.0.0.0/0") {
			continue
		}

		fields := strings.Fields(line)
		var gw, dev string
		metric := 0
		hasMetric := false

		for i := 0; i < len(fields); i++ {
			if fields[i] == "via" && i+1 < len(fields) {
				gw = fields[i+1]
			}
			if fields[i] == "dev" && i+1 < len(fields) {
				dev = fields[i+1]
			}
			if fields[i] == "metric" && i+1 < len(fields) {
				if m, err := strconv.Atoi(fields[i+1]); err == nil {
					metric = m
					hasMetric = true
				}
			}
		}

		if dev != "" {
			// Если метрика не указана явно, в Linux по умолчанию metric 0
			if !hasMetric {
				metric = 0
			}
			if bestMetric == -1 || metric < bestMetric {
				bestMetric = metric
				bestWan = wanInfo{iface: dev, gw: gw}
			}
		}
	}
	return bestWan
}

// parseProcNetRoute разбирает текстовый формат /proc/net/route на Linux,
// находя интерфейс для шлюза 00000000 (0.0.0.0/0).
func parseProcNetRoute(data []byte) wanInfo {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	// Первая строка — заголовки (Iface Destination Gateway Flags ...)
	if scanner.Scan() {
		_ = scanner.Text()
	}

	var bestWan wanInfo
	bestMetric := -1

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		iface := fields[0]
		dest := fields[1]
		gwHex := fields[2]
		metric := 0
		if len(fields) >= 7 {
			if m, err := strconv.Atoi(fields[6]); err == nil {
				metric = m
			}
		}

		if dest == "00000000" {
			var gw string
			if len(gwHex) == 8 && gwHex != "00000000" {
				if b, err := strconv.ParseUint(gwHex, 16, 32); err == nil {
					gw = fmt.Sprintf("%d.%d.%d.%d", byte(b), byte(b>>8), byte(b>>16), byte(b>>24))
				}
			}
			if bestMetric == -1 || metric < bestMetric {
				bestMetric = metric
				bestWan = wanInfo{iface: iface, gw: gw}
			}
		}
	}
	return bestWan
}

// detectWan определяет физический WAN-интерфейс и шлюз по умолчанию
func detectWan() wanInfo {
	out, err := exec.Command("ip", "-4", "route", "show", "default").Output()
	if err != nil || len(out) == 0 {
		out, _ = exec.Command("ip", "route", "show", "default").Output()
	}
	if len(out) > 0 {
		wan := parseDefaultRoute(out)
		if wan.iface != "" {
			return wan
		}
	}

	// Запасной вариант через /proc/net/route
	if data, err := os.ReadFile("/proc/net/route"); err == nil && len(data) > 0 {
		wan := parseProcNetRoute(data)
		if wan.iface != "" {
			return wan
		}
	}

	return wanInfo{}
}

// setupRouting настраивает IP-адрес на TUN, host-маршруты к серверу в обход TUN,
// маршруты по умолчанию 0.0.0.0/1 и 128.0.0.0/1, RuDirect (split tunneling),
// а также NAT/Forwarding для LAN.
func setupRouting(cfg *Config, tun *tunDevice, info desktop.TunnelInfo, serverHosts []string, wan wanInfo) error {
	tunIP := cfg.TunIP
	prefix := 24
	if info.IP != "" {
		tunIP = info.IP
		if info.PrefixLen > 0 {
			prefix = info.PrefixLen
		}
	}
	if tunIP == "" {
		tunIP = "10.77.77.2"
	}

	mtu := cfg.TunMTU
	if info.MTU > 0 {
		mtu = int(info.MTU)
	}

	log.Printf("[route] Настройка интерфейса %s: IP=%s/%d, MTU=%d", tun.name, tunIP, prefix, mtu)

	// 1. Поднять TUN-интерфейс и задать IP
	runCmd("ip", "link", "set", tun.name, "up", "mtu", fmt.Sprintf("%d", mtu))
	runCmd("ip", "addr", "replace", fmt.Sprintf("%s/%d", tunIP, prefix), "dev", tun.name)

	// 2. Исключить серверные IP из туннеля (прямой маршрут через WAN)
	for _, host := range serverHosts {
		host = strings.TrimSpace(host)
		if host == "" {
			continue
		}
		ips, err := net.LookupHost(host)
		if err != nil {
			ips = []string{host}
		}
		for _, ipStr := range ips {
			args := formatRouteArgs(ipStr+"/32", wan)
			if len(args) > 0 {
				runCmd("ip", args...)
			}
		}
	}

	// 3. Маршруты по умолчанию через TUN (/1 не затирает дефолтный шлюз физического WAN)
	runCmd("ip", "route", "replace", "0.0.0.0/1", "dev", tun.name)
	runCmd("ip", "route", "replace", "128.0.0.0/1", "dev", tun.name)

	// 4. Split tunneling: прямой маршрут для российских диапазонов IP через WAN
	if cfg.RuDirect {
		if err := setupRuDirect(wan); err != nil {
			log.Printf("[route] WARN: RuDirect: %v", err)
		}
	}

	// 5. Включить IPv4 forwarding в ядре
	_ = os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1"), 0644)

	// 6. Настроить Firewall: masquerade и forwarding для трафика домашней сети LAN -> TUN
	lanIface := cfg.LanInterface
	if lanIface == "" {
		lanIface = "br-lan"
	}

	// Попытка использовать nftables (OpenWrt 22+)
	nftErr := setupNftables(tun.name, lanIface)
	if nftErr != nil {
		log.Printf("[route] nftables недоступен (%v), пробуем iptables...", nftErr)
		setupIptables(tun.name, lanIface)
	}

	log.Println("[route] Маршрутизация и NAT успешно активированы")
	return nil
}

func setupRuDirect(wan wanInfo) error {
	if wan.iface == "" && wan.gw == "" {
		return fmt.Errorf("WAN не определён — RuDirect невозможен")
	}
	scanner := bufio.NewScanner(strings.NewReader(ruRangesData))
	count := 0
	for scanner.Scan() {
		cidr := strings.TrimSpace(scanner.Text())
		if cidr == "" || strings.HasPrefix(cidr, "#") {
			continue
		}
		args := formatRouteArgs(cidr, wan)
		if len(args) > 0 {
			runCmd("ip", args...)
		}
		count++
	}
	log.Printf("[route] RuDirect: %d CIDR добавлено напрямую через WAN", count)
	return nil
}

func cleanupRuDirect(wan wanInfo) {
	scanner := bufio.NewScanner(strings.NewReader(ruRangesData))
	count := 0
	for scanner.Scan() {
		cidr := strings.TrimSpace(scanner.Text())
		if cidr == "" || strings.HasPrefix(cidr, "#") {
			continue
		}
		runCmd("ip", "route", "del", cidr)
		count++
	}
	log.Printf("[route] RuDirect: %d CIDR удалено", count)
}

func cleanupRouting(cfg *Config, tun *tunDevice, serverHosts []string, wan wanInfo) {
	log.Println("[route] Сброс маршрутов и правил файрвола...")

	runCmd("ip", "route", "del", "0.0.0.0/1")
	runCmd("ip", "route", "del", "128.0.0.0/1")

	if cfg.RuDirect {
		cleanupRuDirect(wan)
	}

	for _, host := range serverHosts {
		host = strings.TrimSpace(host)
		if host == "" {
			continue
		}
		ips, err := net.LookupHost(host)
		if err != nil {
			ips = []string{host}
		}
		for _, ipStr := range ips {
			runCmd("ip", "route", "del", ipStr+"/32")
		}
	}

	// Удалить nftables таблицу
	_ = exec.Command("nft", "delete", "table", "ip", "sirin").Run()

	// Удалить iptables правила
	lanIface := cfg.LanInterface
	if lanIface == "" {
		lanIface = "br-lan"
	}
	_ = exec.Command("iptables", "-t", "nat", "-D", "POSTROUTING", "-o", tun.name, "-j", "MASQUERADE").Run()
	_ = exec.Command("iptables", "-D", "FORWARD", "-i", lanIface, "-o", tun.name, "-j", "ACCEPT").Run()
	_ = exec.Command("iptables", "-D", "FORWARD", "-i", tun.name, "-o", lanIface, "-m", "state", "--state", "ESTABLISHED,RELATED", "-j", "ACCEPT").Run()
}

func setupNftables(tunName, lanIface string) error {
	cmds := [][]string{
		{"add", "table", "ip", "sirin"},
		{"add", "chain", "ip", "sirin", "postrouting", "{ type nat hook postrouting priority 100; }"},
		{"add", "rule", "ip", "sirin", "postrouting", "oifname", tunName, "masquerade"},
		{"add", "chain", "ip", "sirin", "forward", "{ type filter hook forward priority 0; }"},
		{"add", "rule", "ip", "sirin", "forward", "iifname", lanIface, "oifname", tunName, "accept"},
		{"add", "rule", "ip", "sirin", "forward", "iifname", tunName, "oifname", lanIface, "ct", "state", "established,related", "accept"},
	}

	for _, args := range cmds {
		cmd := exec.Command("nft", args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("nft %v: %w (%s)", args, err, strings.TrimSpace(string(out)))
		}
	}
	return nil
}

func setupIptables(tunName, lanIface string) {
	runCmd("iptables", "-t", "nat", "-I", "POSTROUTING", "1", "-o", tunName, "-j", "MASQUERADE")
	runCmd("iptables", "-I", "FORWARD", "1", "-i", lanIface, "-o", tunName, "-j", "ACCEPT")
	runCmd("iptables", "-I", "FORWARD", "2", "-i", tunName, "-o", lanIface, "-m", "state", "--state", "ESTABLISHED,RELATED", "-j", "ACCEPT")
}

func runCmd(name string, args ...string) {
	cmd := exec.Command(name, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("[cmd] %s %s: %v (%s)", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
}
