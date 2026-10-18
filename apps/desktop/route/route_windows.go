//go:build windows

package route

import (
	"fmt"
	"os/exec"
)

// Setup включает full tunnel через Wintun.
// gateway — IP шлюза туннеля (например, 10.77.77.1)
// ifIndex — индекс Wintun интерфейса
func Setup(gateway string, ifIndex int) error {
	// 1. Cleanup старых маршрутов (если прошлый запуск упал)
	_ = cleanup()

	// 2. Добавить два /1 маршрута
	cmds := [][]string{
		{"route", "add", "0.0.0.0", "mask", "128.0.0.0", gateway, "metric", "1", "if", fmt.Sprint(ifIndex)},
		{"route", "add", "128.0.0.0", "mask", "128.0.0.0", gateway, "metric", "1", "if", fmt.Sprint(ifIndex)},
	}
	for _, args := range cmds {
		if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
			return fmt.Errorf("route add %v: %w (%s)", args, err, out)
		}
	}
	return nil
}

// Cleanup удаляет маршруты туннеля.
func Cleanup() error { return cleanup() }

func cleanup() error {
	exec.Command("route", "delete", "0.0.0.0", "mask", "128.0.0.0").Run()
	exec.Command("route", "delete", "128.0.0.0", "mask", "128.0.0.0").Run()
	return nil
}

// SetupDNS настраивает DNS на Wintun интерфейсе.
func SetupDNS(ifaceName string, dns []string) error {
	if len(dns) == 0 {
		return nil
	}
	// Первый DNS — static, остальные — add
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

// CleanupDNS восстанавливает DNS (auto).
func CleanupDNS(ifaceName string) error {
	args := []string{"interface", "ip", "set", "dnsservers", "name=" + ifaceName, "dhcp"}
	exec.Command("netsh", args...).Run()
	return nil
}
