package route

import (
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
