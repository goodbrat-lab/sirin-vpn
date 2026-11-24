package route

import (
	"net"
	"testing"
	"time"
)

func TestParseCIDR(t *testing.T) {
	tests := []struct {
		input     string
		expectIP  string
		expectLen int
		expectErr bool
	}{
		{"192.168.1.1/24", "192.168.1.1", 24, false},
		{"10.0.0.1", "10.0.0.1", 32, false},
		{"2.56.24.0/22", "2.56.24.0", 22, false},
		{"invalid", "", 0, true},
		{"", "", 0, true},
	}

	for _, tc := range tests {
		ip, prefixLen, err := ParseCIDR(tc.input)
		if tc.expectErr {
			if err == nil {
				t.Errorf("expected error for %q, got nil", tc.input)
			}
		} else {
			if err != nil {
				t.Errorf("unexpected error for %q: %v", tc.input, err)
			}
			if ip.String() != tc.expectIP || prefixLen != tc.expectLen {
				t.Errorf("for %q expected %s/%d, got %s/%d", tc.input, tc.expectIP, tc.expectLen, ip.String(), prefixLen)
			}
		}
	}
}

func TestGetRuCIDRs(t *testing.T) {
	start := time.Now()
	cidrs := GetRuCIDRs()
	elapsed := time.Since(start)

	t.Logf("GetRuCIDRs returned %d CIDRs in %v", len(cidrs), elapsed)
	if len(cidrs) < 8000 {
		t.Fatalf("Expected >= 8000 CIDRs, got %d", len(cidrs))
	}

	// Verify caching on second call
	start2 := time.Now()
	cidrs2 := GetRuCIDRs()
	elapsed2 := time.Since(start2)
	t.Logf("Second GetRuCIDRs call took %v", elapsed2)
	if len(cidrs2) != len(cidrs) {
		t.Fatalf("Cache mismatch")
	}
}

func TestResolveDomains(t *testing.T) {
	// Resolving localhost or common domain
	ips := ResolveDomains([]string{"127.0.0.1", "localhost"})
	t.Logf("Resolved IPs: %v", ips)
	if len(ips) == 0 {
		t.Log("Note: localhost did not resolve to IPv4 in test env")
	}
}

func TestFindPhysicalGateway(t *testing.T) {
	gw, ifIndex, err := GetPhysicalDefaultGateway()
	if err != nil {
		t.Logf("Physical gateway note: %v", err)
		return
	}
	t.Logf("Found physical gateway: %s on interface %d", gw, ifIndex)
	if gw.To4() == nil {
		t.Errorf("Gateway should be IPv4: %v", gw)
	}
}

func TestBuildDesiredAndDiffRoutes_SleepAndNetworkSwitch(t *testing.T) {
	cfg := Config{
		Gateway:     "10.77.77.1",
		IfIndex:     42,
		RuDirect:    false,
		Mode:        "bypass",
		CustomCIDRs: []string{"77.88.8.8/32", "93.158.134.0/24"},
	}
	wifiGw := net.ParseIP("192.168.1.1")
	wifiIf := uint32(12)
	domainIPs := []net.IP{net.ParseIP("213.180.204.3")}

	desiredWifi := BuildDesiredRoutes(cfg, wifiGw, wifiIf, domainIPs)
	if len(desiredWifi) != 3 {
		t.Fatalf("expected 3 desired bypass routes on Wi-Fi, got %d", len(desiredWifi))
	}

	// 1. Сценарий: после сна Windows сбросила часть маршрутов из таблицы ядра (остался только 1 из 3)
	kernelAfterSleep := map[string]bool{
		RouteKey(desiredWifi[0]): true,
	}
	toAdd, toRemove := DiffRoutes(desiredWifi, kernelAfterSleep, desiredWifi)
	if len(toAdd) != 2 || len(toRemove) != 0 {
		t.Fatalf("after sleep expected toAdd=2, toRemove=0, got toAdd=%d, toRemove=%d", len(toAdd), len(toRemove))
	}

	// 2. Сценарий: ноутбук переключился с Wi-Fi (if=12, gw=192.168.1.1) на USB-модем (if=19, gw=172.20.10.1)
	modemGw := net.ParseIP("172.20.10.1")
	modemIf := uint32(19)
	desiredModem := BuildDesiredRoutes(cfg, modemGw, modemIf, domainIPs)
	toAddSwitch, toRemoveSwitch := DiffRoutes(desiredModem, kernelAfterSleep, desiredWifi)
	if len(toAddSwitch) != 3 || len(toRemoveSwitch) != 3 {
		t.Fatalf("on interface switch expected toAdd=3, toRemove=3, got toAdd=%d, toRemove=%d",
			len(toAddSwitch), len(toRemoveSwitch))
	}

	// 3. Сценарий: режим Include ("only") направляет маршруты в Wintun (if=42)
	cfgOnly := Config{
		Gateway:     "10.77.77.1",
		IfIndex:     42,
		Mode:        "only",
		CustomCIDRs: []string{"104.18.0.0/16"},
	}
	desiredOnly := BuildDesiredRoutes(cfgOnly, wifiGw, wifiIf, []net.IP{net.ParseIP("160.79.104.10")})
	if len(desiredOnly) != 2 || desiredOnly[0].IfIndex != 42 {
		t.Fatalf("expected 2 Include routes via Wintun if=42, got %+v", desiredOnly)
	}
}

func TestHasSleepGap(t *testing.T) {
	if HasSleepGap(5010*time.Millisecond, 5000*time.Millisecond, 4*time.Second) {
		t.Fatalf("normal 10ms clock jitter must not be treated as sleep")
	}
	if !HasSleepGap(45*time.Second, 5*time.Second, 4*time.Second) {
		t.Fatalf("40s wall-clock jump over monotonic clock must be detected as sleep")
	}
}
