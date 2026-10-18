//go:build windows

package tun

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wintun"
)

// Interface — реализация engine.PacketFlow для Windows через Wintun.
type Interface struct {
	name      string
	mtu       int
	adapter   *wintun.Adapter
	session   wintun.Session
	readWait  windows.Handle
	closeCh   chan struct{}
	closeOnce sync.Once
	writeMu   sync.Mutex
}

// New создаёт Wintun adapter и session.
// name — имя интерфейса (например, "Sirin")
// mtu — MTU интерфейса (например, 1280)
func New(name string, mtu int, dllPath ...string) (*Interface, error) {
	path := ""
	if len(dllPath) > 0 {
		path = dllPath[0]
	}
	if err := ensureWintun(path); err != nil {
		return nil, err
	}

	adapter, err := wintun.CreateAdapter(name, "Sirin VPN", nil)
	if err != nil {
		var openErr error
		adapter, openErr = wintun.OpenAdapter(name)
		if openErr != nil {
			return nil, fmt.Errorf("CreateAdapter: %w (OpenAdapter: %v)", err, openErr)
		}
	}

	session, err := adapter.StartSession(0x400000) // 4 MB ring
	if err != nil {
		adapter.Close()
		return nil, fmt.Errorf("StartSession: %w", err)
	}

	iface := &Interface{
		name:     name,
		mtu:      mtu,
		adapter:  adapter,
		session:  session,
		readWait: session.ReadWaitEvent(),
		closeCh:  make(chan struct{}),
	}
	return iface, nil
}

func (i *Interface) Name() string { return i.name }
func (i *Interface) MTU() int     { return i.mtu }

// InterfaceIndex возвращает индекс Windows-интерфейса Wintun.
func (i *Interface) InterfaceIndex() (int, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return 0, err
	}
	for _, ifi := range ifaces {
		if strings.EqualFold(ifi.Name, i.name) {
			return ifi.Index, nil
		}
	}
	return 0, fmt.Errorf("Wintun interface %q not found", i.name)
}

// DeleteAdapter отключает Wintun адаптер в системе.
func DeleteAdapter(name string) error {
	cmd := exec.Command("netsh", "interface", "set", "interface", "name="+name, "admin=disable")
	return cmd.Run()
}

// ReadBatch блокирует до появления пакетов и отдаёт их одним куском:
// кадр [2 байта длины BE][пакет IP]. Возврат nil означает закрытие.
func (i *Interface) ReadBatch() []byte {
	for {
		select {
		case <-i.closeCh:
			return nil
		default:
		}

		packet, err := i.session.ReceivePacket()
		if err == nil {
			pktLen := len(packet)
			frame := make([]byte, 2+pktLen)
			binary.BigEndian.PutUint16(frame[:2], uint16(pktLen))
			copy(frame[2:], packet)
			i.session.ReleaseReceivePacket(packet)
			return frame
		}

		// Ждём пакета или таймаута проверки закрытия
		s, _ := windows.WaitForSingleObject(i.readWait, 250)
		if s == windows.WAIT_FAILED {
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// WriteBatch принимает ровно один кадр [2 байта длины BE][пакет IP].
// Возвращает false, если поток закрыт.
func (i *Interface) WriteBatch(b []byte) bool {
	if len(b) < 2 {
		return false
	}
	pktLen := int(binary.BigEndian.Uint16(b[:2]))
	if len(b) < 2+pktLen {
		return false
	}
	pktData := b[2 : 2+pktLen]

	select {
	case <-i.closeCh:
		return false
	default:
	}

	i.writeMu.Lock()
	defer i.writeMu.Unlock()

	packet, err := i.session.AllocateSendPacket(pktLen)
	if err != nil {
		return false
	}
	copy(packet, pktData)
	i.session.SendPacket(packet)
	return true
}

func (i *Interface) Close() {
	i.closeOnce.Do(func() {
		close(i.closeCh)
		i.session.End()
		i.adapter.Close()
	})
}

func ensureWintun(dllPath string) error {
	if dllPath != "" {
		if _, err := os.Stat(dllPath); err == nil {
			return nil
		}
	}
	exe, err := os.Executable()
	if err == nil {
		alt := filepath.Join(filepath.Dir(exe), "wintun.dll")
		if _, err := os.Stat(alt); err == nil {
			return nil
		}
	}
	if _, err := os.Stat(filepath.Join("build", "windows", "wintun.dll")); err == nil {
		return nil
	}
	return nil
}
