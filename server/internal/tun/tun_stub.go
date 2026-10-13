//go:build !linux

package tun

import "errors"

type Device struct{}

func Open(name string, mtu int) (*Device, error) {
	return nil, errors.New("TUN не поддерживается на этой платформе")
}

func (d *Device) Name() string                 { return "" }
func (d *Device) Read(p []byte) (int, error)   { return 0, errors.New("unsupported") }
func (d *Device) Write(p []byte) (int, error)  { return 0, errors.New("unsupported") }
func (d *Device) Close() error                 { return nil }
func (d *Device) Configure(ip, m string) error { return errors.New("unsupported") }
func (d *Device) AddRoute(cidr string) error   { return errors.New("unsupported") }
