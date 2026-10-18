//go:build !linux

package main

import "errors"

type tunDevice struct {
	name   string
	mtuVal int
}

func openTun(name string, mtu int) (*tunDevice, error) {
	return nil, errors.New("TUN device is supported only on Linux/OpenWrt")
}

func (t *tunDevice) ReadBatch() []byte        { return nil }
func (t *tunDevice) WriteBatch(b []byte) bool { return false }
func (t *tunDevice) Close()                   {}
