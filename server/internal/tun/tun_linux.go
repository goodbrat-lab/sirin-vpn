//go:build linux

package tun

import (
	"fmt"
	"os/exec"

	"github.com/songgao/water"
)

type Device struct {
	iface *water.Interface
	name  string
}

// Open создаёт TUN-интерфейс с заданным именем.
// Настройка IP/netmask/MSS выполняется отдельно через Configure.
func Open(name string, mtu int) (*Device, error) {
	cfg := water.Config{
		DeviceType: water.TUN,
	}
	cfg.Name = name
	iface, err := water.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("water.New(%s): %w", name, err)
	}
	d := &Device{iface: iface, name: name}
	if err := d.setMTU(mtu); err != nil {
		iface.Close()
		return nil, err
	}
	return d, nil
}

func (d *Device) Name() string { return d.name }

func (d *Device) Read(p []byte) (int, error)  { return d.iface.Read(p) }
func (d *Device) Write(p []byte) (int, error) { return d.iface.Write(p) }
func (d *Device) Close() error                { return d.iface.Close() }

func (d *Device) setMTU(mtu int) error {
	cmd := exec.Command("ip", "link", "set", "dev", d.name, "mtu", fmt.Sprint(mtu))
	return cmd.Run()
}

// Configure поднимает интерфейс и назначает IP/netmask.
func (d *Device) Configure(ip, netmask string) error {
	steps := [][]string{
		{"ip", "addr", "add", ip + "/24", "dev", d.name},
		{"ip", "link", "set", "dev", d.name, "up"},
	}
	for _, args := range steps {
		cmd := exec.Command(args[0], args[1:]...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%v: %w (%s)", args, err, out)
		}
	}
	return nil
}

// AddRoute добавляет маршрут для подсети клиентов через TUN.
func (d *Device) AddRoute(cidr string) error {
	cmd := exec.Command("ip", "route", "add", cidr, "dev", d.name)
	if out, err := cmd.CombinedOutput(); err != nil {
		// Если маршрут уже есть — игнорируем
		_ = out
	}
	return nil
}
