//go:build !windows

package route

import (
	"fmt"
	"net"
)

func GetPhysicalDefaultGateway() (net.IP, uint32, error) {
	return nil, 0, fmt.Errorf("only supported on windows")
}

func Setup(cfg Config) error {
	return fmt.Errorf("only supported on windows")
}

func Cleanup() error {
	return nil
}

func ActiveRouteCount() int {
	return 0
}

func SetupDNS(ifaceName string, dns []string) error {
	return nil
}

func CleanupDNS(ifaceName string) error {
	return nil
}
