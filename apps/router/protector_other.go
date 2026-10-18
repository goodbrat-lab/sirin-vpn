//go:build !linux

package main

type linuxProtector struct{}

func newProtector(wanIface, wanGW string) *linuxProtector { return &linuxProtector{} }
func (p *linuxProtector) Protect(fd int32) bool          { return true }
func (p *linuxProtector) ExcludeHost(host string)        {}
