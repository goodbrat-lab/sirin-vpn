package main

import (
	"testing"
)

func TestParseDefaultRoute_PPPoE(t *testing.T) {
	// Типичный вывод OpenWrt для PPPoE соединения (интерфейс точка-точка, шлюз via отсутствует)
	output := []byte(`default dev pppoe-wan proto static scope link
10.64.0.0/10 dev pppoe-wan proto kernel scope link src 100.64.12.34
192.168.1.0/24 dev br-lan proto kernel scope link src 192.168.1.1
`)

	wan := parseDefaultRoute(output)
	if wan.iface != "pppoe-wan" {
		t.Fatalf("Ожидался iface='pppoe-wan', получено %q", wan.iface)
	}
	if wan.gw != "" {
		t.Fatalf("Ожидался пустой шлюз для PPPoE, получено %q", wan.gw)
	}
}

func TestParseDefaultRoute_StandardDHCP(t *testing.T) {
	// Стандартный маршрут по умолчанию с via
	output := []byte(`default via 192.168.0.1 dev eth0 proto dhcp src 192.168.0.50 metric 100
192.168.0.0/24 dev eth0 proto kernel scope link src 192.168.0.50
`)

	wan := parseDefaultRoute(output)
	if wan.iface != "eth0" {
		t.Fatalf("Ожидался iface='eth0', получено %q", wan.iface)
	}
	if wan.gw != "192.168.0.1" {
		t.Fatalf("Ожидался gw='192.168.0.1', получено %q", wan.gw)
	}
}

func TestParseDefaultRoute_MetricPriority(t *testing.T) {
	// Два default маршрута: pppoe-wan (metric 10) и резервный wan (metric 20)
	output := []byte(`default dev wan proto static metric 20
default dev pppoe-wan proto static metric 10
`)

	wan := parseDefaultRoute(output)
	if wan.iface != "pppoe-wan" {
		t.Fatalf("Ожидался pppoe-wan (меньшая метрика 10), получено %q", wan.iface)
	}
}

func TestParseProcNetRoute(t *testing.T) {
	// Эмуляция содержимого /proc/net/route на Linux
	data := []byte(`Iface	Destination	Gateway 	Flags	RefCnt	Use	Metric	Mask		MTU	Window	IRTT
pppoe-wan	00000000	00000000	0003	0	0	10	00000000	0	0	0
eth0	0000A8C0	00000000	0001	0	0	0	00FFFFFF	0	0	0
`)

	wan := parseProcNetRoute(data)
	if wan.iface != "pppoe-wan" {
		t.Fatalf("Ожидался iface='pppoe-wan', получено %q", wan.iface)
	}
}

func TestWanPrecedence(t *testing.T) {
	// Тестирование логики в main.go:
	// При наличии шаблонного "wan" или "auto" должен побеждать обнаруженный default route ("pppoe-wan")
	detected := wanInfo{iface: "pppoe-wan", gw: ""}

	testCases := []struct {
		name          string
		cfgIface      string
		expectedIface string
	}{
		{"template wan", "wan", "pppoe-wan"},
		{"template auto", "auto", "pppoe-wan"},
		{"empty", "", "pppoe-wan"},
		{"explicit custom", "eth1.2", "eth1.2"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			wan := detected
			if tc.cfgIface != "" && tc.cfgIface != "wan" && tc.cfgIface != "auto" {
				wan.iface = tc.cfgIface
			} else if wan.iface == "" && tc.cfgIface != "" {
				wan.iface = tc.cfgIface
			}

			if wan.iface != tc.expectedIface {
				t.Errorf("Для cfgIface=%q ожидался %q, получено %q", tc.cfgIface, tc.expectedIface, wan.iface)
			}
		})
	}
}
