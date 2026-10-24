package main

import (
	_ "embed"
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"sort"
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
	return formatRouteArgsTable(dst, wan, "")
}

// formatRouteArgsTable формирует аргументы команды `ip route replace <dst> ... table <tbl>`
func formatRouteArgsTable(dst string, wan wanInfo, table string) []string {
	dst = strings.TrimSpace(dst)
	if dst == "" {
		return nil
	}
	var args []string
	if isPointToPoint(wan.iface) && wan.iface != "" {
		args = []string{"route", "replace", dst, "dev", wan.iface}
	} else if wan.gw != "" && wan.iface != "" {
		args = []string{"route", "replace", dst, "via", wan.gw, "dev", wan.iface}
	} else if wan.iface != "" {
		args = []string{"route", "replace", dst, "dev", wan.iface}
	} else if wan.gw != "" {
		args = []string{"route", "replace", dst, "via", wan.gw}
	}
	if len(args) > 0 && table != "" {
		args = append(args, "table", table)
	}
	return args
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
func parseDefaultRoute(output []byte) wanInfo {
	scanner := bufio.NewScanner(bytes.NewReader(output))
	var bestWan wanInfo
	bestMetric := -1

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
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

// parseProcNetRoute разбирает текстовый формат /proc/net/route на Linux.
func parseProcNetRoute(data []byte) wanInfo {
	scanner := bufio.NewScanner(bytes.NewReader(data))
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

// detectWan определяет физический WAN-интерфейс и шлюз по умолчанию.
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

	if data, err := os.ReadFile("/proc/net/route"); err == nil && len(data) > 0 {
		wan := parseProcNetRoute(data)
		if wan.iface != "" {
			return wan
		}
	}

	return wanInfo{}
}

type cidrRange struct {
	start uint32
	end   uint32
	cidr  string
}

// ipToUint32 преобразует IPv4 в uint32 для вычисления диапазонов.
func ipToUint32(ip net.IP) uint32 {
	ip4 := ip.To4()
	if ip4 == nil {
		return 0
	}
	return binary.BigEndian.Uint32(ip4)
}

// mergeCIDRs объединяет и дедуплицирует список CIDR, исключая дубликаты
// и подсети, полностью перекрываемые более крупными диапазонами.
// Это критично для nftables sets с флагом interval.
func mergeCIDRs(rawCIDRs []string) []string {
	var ranges []cidrRange
	for _, raw := range rawCIDRs {
		raw = strings.TrimSpace(raw)
		if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		if !strings.Contains(raw, "/") {
			raw = raw + "/32"
		}
		ip, ipNet, err := net.ParseCIDR(raw)
		if err != nil || ip.To4() == nil {
			continue
		}

		ones, bits := ipNet.Mask.Size()
		if bits != 32 {
			continue
		}
		start := ipToUint32(ipNet.IP)
		var mask uint32 = 0xFFFFFFFF
		if ones < 32 {
			mask = ^(uint32(1)<<(32-ones) - 1)
		}
		end := start | (^mask)

		ranges = append(ranges, cidrRange{
			start: start,
			end:   end,
			cidr:  ipNet.String(),
		})
	}

	if len(ranges) == 0 {
		return nil
	}

	// Сортировка: по start ASC, а при равном start — по end DESC (более крупная сеть первой)
	sort.Slice(ranges, func(i, j int) bool {
		if ranges[i].start != ranges[j].start {
			return ranges[i].start < ranges[j].start
		}
		return ranges[i].end > ranges[j].end
	})

	var merged []cidrRange
	for _, r := range ranges {
		if len(merged) == 0 {
			merged = append(merged, r)
			continue
		}
		curr := merged[len(merged)-1]
		// Если r лежит полностью внутри текущей сети, пропускаем (уже покрыто)
		if r.start >= curr.start && r.end <= curr.end {
			continue
		}
		merged = append(merged, r)
	}

	result := make([]string, len(merged))
	for i, m := range merged {
		result[i] = m.cidr
	}
	return result
}

// buildSplitCidrs формирует итоговый список CIDR для split tunneling.
func buildSplitCidrs(cfg *Config) []string {
	var raw []string

	// 1. RuDirect диапазоны (если включены)
	if cfg.IsRuDirectActive() {
		scanner := bufio.NewScanner(strings.NewReader(ruRangesData))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" && !strings.HasPrefix(line, "#") {
				raw = append(raw, line)
			}
		}
		log.Printf("[split] RuDirect: добавлено %d диапазонов РФ", len(raw))
	} else if cfg.SplitTunnel != nil && cfg.SplitTunnel.Mode == "only" {
		if cfg.SplitTunnel.UseRuDirect != nil && *cfg.SplitTunnel.UseRuDirect {
			log.Printf("[split] ПРЕДУПРЕЖДЕНИЕ: Опция use_ru_direct игнорируется в режиме 'only'")
		}
	}

	// 2. Дополнительные CIDR из конфигурации
	if cfg.SplitTunnel != nil && len(cfg.SplitTunnel.CIDRs) > 0 {
		raw = append(raw, cfg.SplitTunnel.CIDRs...)
		log.Printf("[split] Дополнительные CIDR: %d адресов", len(cfg.SplitTunnel.CIDRs))
	}

	// 3. Домены (резолвятся в IPv4)
	if cfg.SplitTunnel != nil && len(cfg.SplitTunnel.Domains) > 0 {
		resolvedCount := 0
		for _, dom := range cfg.SplitTunnel.Domains {
			dom = strings.TrimSpace(dom)
			if dom == "" {
				continue
			}
			ips, err := net.LookupIP(dom)
			if err != nil {
				log.Printf("[split] Не удалось зарезолвить домен %s: %v", dom, err)
				continue
			}
			var domIPs []string
			for _, ip := range ips {
				if ip4 := ip.To4(); ip4 != nil {
					raw = append(raw, ip4.String()+"/32")
					domIPs = append(domIPs, ip4.String())
					resolvedCount++
				}
			}
			log.Printf("[split] Домен %s зарезолвлен в IP: %v", dom, domIPs)
		}
		log.Printf("[split] Домены: зарезолвлено %d IPv4-адресов", resolvedCount)
	}

	merged := mergeCIDRs(raw)
	log.Printf("[split] Итоговый split-список: %d непересекающихся диапазонов", len(merged))
	return merged
}

// buildLocalCidrs формирует список локальных сетей и адресов серверов,
// которые никогда не должны туннелироваться через VPN.
func buildLocalCidrs(serverHosts []string) []string {
	locals := []string{
		"127.0.0.0/8",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"224.0.0.0/4",
		"255.255.255.255/32",
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
		for _, ip := range ips {
			if ipObj := net.ParseIP(ip); ipObj != nil && ipObj.To4() != nil {
				locals = append(locals, ipObj.To4().String()+"/32")
			}
		}
	}

	return mergeCIDRs(locals)
}

// setupRouting настраивает виртуальный интерфейс TUN, маршруты в обход к серверу,
// правила маркировки split tunneling через nftables и policy routing (таблицы 100 и 200).
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

	// 1. Идемпотентный сброс предыдущих правил
	cleanupRouting(cfg, tun, serverHosts, wan)

	// 2. Поднять TUN-интерфейс и задать IP
	runCmd("ip", "link", "set", tun.name, "up", "mtu", fmt.Sprintf("%d", mtu))
	runCmd("ip", "addr", "replace", fmt.Sprintf("%s/%d", tunIP, prefix), "dev", tun.name)

	// 3. Прямые host-маршруты к серверу через WAN (в таблице main)
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

	// 4. Policy routing:
	// Таблица 100: маршрут по умолчанию через VPN TUN (mark 0x1)
	runCmd("ip", "route", "replace", "default", "dev", tun.name, "table", "100")

	// Таблица 200: маршрут по умолчанию через физический WAN (mark 0x2)
	wanArgs := formatRouteArgsTable("default", wan, "200")
	if len(wanArgs) > 0 {
		runCmd("ip", wanArgs...)
	}

	// 5. Правила ip rule:
	runCmd("ip", "rule", "add", "fwmark", "0x1", "table", "100", "pref", "10100")
	runCmd("ip", "rule", "add", "fwmark", "0x2", "table", "200", "pref", "10200")

	// 6. Включить IPv4 forwarding в ядре
	_ = os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1"), 0644)

	// 7. Сборка split и локальных списков CIDR
	splitCIDRs := buildSplitCidrs(cfg)
	localCIDRs := buildLocalCidrs(serverHosts)

	lanIface := cfg.LanInterface
	if lanIface == "" {
		lanIface = "br-lan"
	}

	mode := "bypass"
	if cfg.SplitTunnel != nil && cfg.SplitTunnel.Mode != "" {
		mode = cfg.SplitTunnel.Mode
	}

	// 8. Настройка nftables правил маркировки и NAT
	if err := applySplitTunnelNftables(tun.name, lanIface, mode, localCIDRs, splitCIDRs); err != nil {
		log.Printf("[route] nftables ошибка: %v. Пробуем iptables fallback...", err)
		setupIptables(tun.name, lanIface)
	}

	// 9. Настройка Kill Switch (inet sirin_killswitch)
	if cfg.IsKillSwitchEnabled() {
		var allowedDest []string
		allowedDest = append(allowedDest, serverHosts...)
		allowedDest = append(allowedDest, cfg.KillSwitchExceptions...)
		allowedDest = append(allowedDest, getWanHostRoutes(wan.iface)...)
		if err := applyKillSwitchNftables(lanIface, wan.iface, mode, allowedDest); err != nil {
			log.Printf("[route] Ошибка настройки Kill Switch: %v", err)
		} else {
			log.Printf("[route] Kill Switch активирован (LAN: %s, WAN: %s, исключений: %d)", lanIface, wan.iface, len(mergeCIDRs(allowedDest)))
		}
	} else {
		log.Println("[route] Kill Switch отключен (kill_switch = false)")
		cleanupKillSwitch()
	}

	log.Printf("[route] Маршрутизация активирована (режим: %s, split_cidrs: %d)", mode, len(splitCIDRs))
	return nil
}

// applySplitTunnelNftables создаёт таблицу ip sirin с наборами local_cidrs и split_cidrs,
// цепочками маркировки prerouting (для LAN) и output (для роутера), а также NAT/Forward.
func applySplitTunnelNftables(tunName, lanIface, mode string, localCIDRs, splitCIDRs []string) error {
	rules := generateNftablesRules(tunName, lanIface, mode, localCIDRs, splitCIDRs)

	cmd := exec.Command("nft", "-f", "-")
	cmd.Stdin = strings.NewReader(rules)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("nft apply: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// generateNftablesRules генерирует конфигурацию nftables для split tunneling.
func generateNftablesRules(tunName, lanIface, mode string, localCIDRs, splitCIDRs []string) string {
	var b strings.Builder

	b.WriteString("table ip sirin {\n")

	// Set local_cidrs
	b.WriteString("    set local_cidrs {\n")
	b.WriteString("        type ipv4_addr\n")
	b.WriteString("        flags interval\n")
	if len(localCIDRs) > 0 {
		b.WriteString(fmt.Sprintf("        elements = { %s }\n", strings.Join(localCIDRs, ", ")))
	}
	b.WriteString("    }\n")

	// Set split_cidrs
	b.WriteString("    set split_cidrs {\n")
	b.WriteString("        type ipv4_addr\n")
	b.WriteString("        flags interval\n")
	if len(splitCIDRs) > 0 {
		b.WriteString(fmt.Sprintf("        elements = { %s }\n", strings.Join(splitCIDRs, ", ")))
	}
	b.WriteString("    }\n")

	// Mode rules
	var modeRules string
	if mode == "only" {
		// Only mode: только split_cidrs идут в VPN (0x1), всё остальное — напрямую в WAN (0x2)
		modeRules = "        ip daddr @split_cidrs meta mark set 0x00000001 return\n" +
			"        meta mark set 0x00000002\n"
	} else {
		// Bypass mode: всё в VPN (0x1), кроме split_cidrs (они идут в WAN 0x2)
		modeRules = "        ip daddr @split_cidrs meta mark set 0x00000002 return\n" +
			"        meta mark set 0x00000001\n"
	}

	// Chain prerouting (только для клиентов локальной сети lanIface!)
	// Входящие пакеты из WAN/PPPoE не должны маркироваться, чтобы ответный трафик
	// возвращался в LAN через таблицу main, а не отправлялся обратно в WAN/TUN.
	b.WriteString("    chain prerouting {\n")
	b.WriteString("        type filter hook prerouting priority mangle; policy accept;\n")
	b.WriteString(fmt.Sprintf("        iifname != \"%s\" return\n", lanIface))
	b.WriteString("        ip daddr @local_cidrs return\n")
	b.WriteString(modeRules)
	b.WriteString("    }\n")

	// Chain output (для локальных процессов роутера: dnsmasq, curl и т.д.)
	b.WriteString("    chain output {\n")
	b.WriteString("        type route hook output priority mangle; policy accept;\n")
	b.WriteString("        meta mark 0x00000002 return\n") // Protector VPN-сокета уже имеет mark 0x2
	b.WriteString("        ip daddr @local_cidrs return\n")
	b.WriteString(modeRules)
	b.WriteString("    }\n")

	// Chain postrouting (NAT masquerade на tun0)
	b.WriteString("    chain postrouting {\n")
	b.WriteString("        type nat hook postrouting priority 100; policy accept;\n")
	b.WriteString(fmt.Sprintf("        oifname \"%s\" masquerade\n", tunName))
	b.WriteString("    }\n")

	// Chain forward (фильтрация LAN -> TUN)
	b.WriteString("    chain forward {\n")
	b.WriteString("        type filter hook forward priority 0; policy accept;\n")
	b.WriteString(fmt.Sprintf("        iifname \"%s\" oifname \"%s\" accept\n", lanIface, tunName))
	b.WriteString(fmt.Sprintf("        iifname \"%s\" oifname \"%s\" ct state established,related accept\n", tunName, lanIface))
	b.WriteString("    }\n")

	b.WriteString("}\n")
	return b.String()
}

func cleanupRouting(cfg *Config, tun *tunDevice, serverHosts []string, wan wanInfo) {
	log.Println("[route] Сброс маршрутов, ip rule и nftables...")

	// 1. Удалить nftables таблицу
	_ = exec.Command("nft", "delete", "table", "ip", "sirin").Run()

	// 2. Удалить ip rule для таблиц 100 и 200
	_ = exec.Command("ip", "rule", "del", "pref", "10100").Run()
	_ = exec.Command("ip", "rule", "del", "pref", "10200").Run()

	// 3. Очистить таблицы маршрутизации 100 и 200
	_ = exec.Command("ip", "route", "flush", "table", "100").Run()
	_ = exec.Command("ip", "route", "flush", "table", "200").Run()

	// 4. Очистить legacy маршруты в table main
	runCmd("ip", "route", "del", "0.0.0.0/1")
	runCmd("ip", "route", "del", "128.0.0.0/1")

	// 5. Удалить host-маршруты к серверам в table main
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

	// 6. Удалить iptables fallback правила (если использовались)
	lanIface := cfg.LanInterface
	if lanIface == "" {
		lanIface = "br-lan"
	}
	_ = exec.Command("iptables", "-t", "nat", "-D", "POSTROUTING", "-o", tun.name, "-j", "MASQUERADE").Run()
	_ = exec.Command("iptables", "-D", "FORWARD", "-i", lanIface, "-o", tun.name, "-j", "ACCEPT").Run()
	_ = exec.Command("iptables", "-D", "FORWARD", "-i", tun.name, "-o", lanIface, "-m", "state", "--state", "ESTABLISHED,RELATED", "-j", "ACCEPT").Run()

	// 7. Удалить nftables таблицу Kill Switch
	cleanupKillSwitch()
}

// getWanHostRoutes возвращает список IPv4 хост-маршрутов (/32), привязанных к WAN-интерфейсу.
func getWanHostRoutes(wanIface string) []string {
	if wanIface == "" {
		return nil
	}
	out, err := exec.Command("ip", "route", "show", "dev", wanIface).Output()
	if err != nil {
		return nil
	}
	var hosts []string
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "default") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) > 0 {
			target := fields[0]
			ipStr := strings.TrimSuffix(target, "/32")
			if ip := net.ParseIP(ipStr); ip != nil && ip.To4() != nil {
				hosts = append(hosts, ip.To4().String())
			}
		}
	}
	return hosts
}

// applyKillSwitchNftables создаёт таблицу inet sirin_killswitch для блокировки утечек в WAN.
func applyKillSwitchNftables(lanIface, wanIface, mode string, allowedDest []string) error {
	rules := generateKillSwitchNftablesRules(lanIface, wanIface, mode, allowedDest)

	cmd := exec.Command("nft", "-f", "-")
	cmd.Stdin = strings.NewReader(rules)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("nft apply killswitch: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// cleanupKillSwitch удаляет таблицу inet sirin_killswitch.
func cleanupKillSwitch() {
	_ = exec.Command("nft", "delete", "table", "inet", "sirin_killswitch").Run()
}

// generateKillSwitchNftablesRules генерирует конфигурацию nftables для Kill Switch.
func generateKillSwitchNftablesRules(lanIface, wanIface, mode string, allowedDest []string) string {
	if lanIface == "" {
		lanIface = "br-lan"
	}
	if wanIface == "" {
		wanIface = "pppoe-wan"
	}

	var b strings.Builder
	b.WriteString("table inet sirin_killswitch {\n")

	validDest := mergeCIDRs(allowedDest)
	if len(validDest) > 0 {
		b.WriteString("    set allowed_dest {\n")
		b.WriteString("        type ipv4_addr\n")
		b.WriteString("        flags interval\n")
		b.WriteString(fmt.Sprintf("        elements = { %s }\n", strings.Join(validDest, ", ")))
		b.WriteString("    }\n")
	}

	b.WriteString("    chain forward {\n")
	b.WriteString("        type filter hook forward priority 0; policy accept;\n")
	b.WriteString(fmt.Sprintf("        iifname != \"%s\" accept\n", lanIface))
	b.WriteString(fmt.Sprintf("        oifname != \"%s\" accept\n", wanIface))
	if len(validDest) > 0 {
		b.WriteString("        ip daddr @allowed_dest accept\n")
	}
	if mode == "bypass" {
		b.WriteString("        meta mark 0x00000002 accept\n")
	}
	b.WriteString("        meta l4proto tcp reject with tcp reset\n")
	b.WriteString("        drop\n")
	b.WriteString("    }\n")
	b.WriteString("}\n")

	return b.String()
}

func setupIptables(tunName, lanIface string) {
	runCmd("iptables", "-t", "nat", "-I", "POSTROUTING", "1", "-o", tunName, "-j", "MASQUERADE")
	runCmd("iptables", "-I", "FORWARD", "1", "-i", lanIface, "-o", tunName, "-j", "ACCEPT")
	runCmd("iptables", "-I", "FORWARD", "2", "-i", tunName, "-o", lanIface, "-m", "state", "--state", "ESTABLISHED,RELATED", "-j", "ACCEPT")
}

func runCmd(name string, args ...string) {
	cmd := exec.Command(name, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		// Игнорируем ошибки отсутствия правил при удалении
		msg := strings.TrimSpace(string(out))
		if !strings.Contains(msg, "No such process") && !strings.Contains(msg, "does not exist") && !strings.Contains(msg, "No such file") {
			log.Printf("[cmd] %s %s: %v (%s)", name, strings.Join(args, " "), err, msg)
		}
	}
}
