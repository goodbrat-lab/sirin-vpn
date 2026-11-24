//go:build !linux

package main

type linuxProtector struct {
	wanIface string
	wanGW    string
}

func newProtector(wanIface, wanGW string) *linuxProtector {
	return &linuxProtector{wanIface: wanIface, wanGW: wanGW}
}
func (p *linuxProtector) UpdateWAN(wanIface, wanGW string) { p.wanIface = wanIface; p.wanGW = wanGW }
func (p *linuxProtector) CurrentWAN() (string, string)     { return p.wanIface, p.wanGW }
func (p *linuxProtector) Protect(fd int32) bool            { return true }
func (p *linuxProtector) ExcludeHost(host string)          {}
