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
	"strings"

	"sirin/engine/desktop"
)

//go:embed configs/ru-ranges.txt
var ruRangesData string

type wanInfo struct {
	iface string
	gw    string
}

// detectWan определяет физический WAN-интерфейс и шлюз по умолчанию
func detectWan() wanInfo {
	out, err := exec.Command("ip", "route", "show", "default").Output()
	if err != nil || len(out) == 0 {
		return wanInfo{}
	}

	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		fields := strings.Fields(line)
		// default via 192.168.1.1 dev eth0 ...
		var gw, dev string
		for i := 0; i < len(fields)-1; i++ {
			if fields[i] == "via" {
				gw = fields[i+1]
			}
			if fields[i] == "dev" {
				dev = fields[i+1]
			}
		}
		if dev != "" {
			return wanInfo{iface: dev, gw: gw}
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
			if wan.gw != "" && wan.iface != "" {
				runCmd("ip", "route", "replace", ipStr+"/32", "via", wan.gw, "dev", wan.iface)
			} else if wan.iface != "" {
				runCmd("ip", "route", "replace", ipStr+"/32", "dev", wan.iface)
			} else if wan.gw != "" {
				runCmd("ip", "route", "replace", ipStr+"/32", "via", wan.gw)
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
		if wan.gw != "" && wan.iface != "" {
			runCmd("ip", "route", "replace", cidr, "via", wan.gw, "dev", wan.iface)
		} else if wan.iface != "" {
			runCmd("ip", "route", "replace", cidr, "dev", wan.iface)
		} else if wan.gw != "" {
			runCmd("ip", "route", "replace", cidr, "via", wan.gw)
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
