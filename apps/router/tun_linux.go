//go:build linux

package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"sync"

	"golang.org/x/sys/unix"
)

type tunDevice struct {
	file      *os.File
	name      string
	mtuVal    int
	writeMu   sync.Mutex
	closeCh   chan struct{}
	closeOnce sync.Once
}

func openTun(name string, mtu int) (*tunDevice, error) {
	if mtu <= 0 {
		mtu = 1280
	}
	f, err := os.OpenFile("/dev/net/tun", os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("open /dev/net/tun: %w", err)
	}

	ifr, err := unix.NewIfreq(name)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("NewIfreq %s: %w", name, err)
	}

	ifr.SetUint16(unix.IFF_TUN | unix.IFF_NO_PI)
	if err := unix.IoctlIfreq(int(f.Fd()), unix.TUNSETIFF, ifr); err != nil {
		f.Close()
		return nil, fmt.Errorf("ioctl TUNSETIFF: %w", err)
	}

	actualName := ifr.Name()
	if actualName == "" {
		actualName = name
	}

	return &tunDevice{
		file:    f,
		name:    actualName,
		mtuVal:  mtu,
		closeCh: make(chan struct{}),
	}, nil
}

// ReadBatch возвращает пачку кадров в формате engine.PacketFlow:
// [2 байта длины BE][пакет IP]. Возврат nil сигнализирует движку о закрытии потока.
func (t *tunDevice) ReadBatch() []byte {
	select {
	case <-t.closeCh:
		return nil
	default:
	}

	buf := make([]byte, 2+t.mtuVal)
	n, err := t.file.Read(buf[2:])
	if err != nil || n <= 0 {
		return nil
	}
	binary.BigEndian.PutUint16(buf[:2], uint16(n))
	return buf[:2+n]
}

// WriteBatch принимает ровно один кадр [2 байта длины BE][пакет IP].
// Возвращает false, если поток закрыт или запись не удалась.
func (t *tunDevice) WriteBatch(b []byte) bool {
	if len(b) < 2 {
		return false
	}
	pktLen := int(binary.BigEndian.Uint16(b[:2]))
	if len(b) < 2+pktLen {
		return false
	}

	select {
	case <-t.closeCh:
		return false
	default:
	}

	t.writeMu.Lock()
	defer t.writeMu.Unlock()

	_, err := t.file.Write(b[2 : 2+pktLen])
	return err == nil
}

func (t *tunDevice) Close() {
	t.closeOnce.Do(func() {
		close(t.closeCh)
		_ = t.file.Close()
	})
}
