package vless

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// VisionConn — обёртка net.Conn для соединений XTLS Vision.
// При первой записи отправляет заголовок VLESS с flow="xtls-rprx-vision" вместе с полезной нагрузкой.
// При чтении отсекает заголовок ответа сервера VLESS и распаковывает padding XTLS Vision.
type VisionConn struct {
	net.Conn

	uuid [16]byte

	wmu        sync.Mutex
	headerSent bool
	header     []byte

	rmu             sync.Mutex
	responseRead    bool
	initialUUIDRead bool
	paddingDone     bool
	readBuf         []byte
}

// NewVisionConn создаёт VisionConn с подготовленным заголовком VLESS и UUID клиента.
func NewVisionConn(conn net.Conn, header []byte, uuid [16]byte) *VisionConn {
	return &VisionConn{
		Conn:   conn,
		header: header,
		uuid:   uuid,
	}
}

// Write отправляет данные. При первой отправке заголовок VLESS объединяется с данными
// в один пакет, чтобы избежать лишней фрагментации на сетевом уровне.
func (v *VisionConn) Write(b []byte) (int, error) {
	v.wmu.Lock()
	defer v.wmu.Unlock()

	if !v.headerSent {
		packet := make([]byte, len(v.header)+len(b))
		copy(packet, v.header)
		copy(packet[len(v.header):], b)

		n, err := v.Conn.Write(packet)
		if err != nil {
			return 0, err
		}
		v.headerSent = true

		if n < len(v.header) {
			return 0, fmt.Errorf("partial write of vless header: %d of %d", n, len(v.header))
		}
		return n - len(v.header), nil
	}

	return v.Conn.Write(b)
}

// Read считывает данные из соединения, предварительно отсекая ответный заголовок VLESS
// и прозрачно снимая XTLS Vision padding.
func (v *VisionConn) Read(b []byte) (int, error) {
	v.rmu.Lock()
	defer v.rmu.Unlock()

	if !v.responseRead {
		// Читаем заголовок ответа сервера: [версия: 1 байт][длина addon: 1 байт]
		var respHeader [2]byte
		if _, err := io.ReadFull(v.Conn, respHeader[:]); err != nil {
			return 0, fmt.Errorf("read vless response header: %w", err)
		}

		addonLen := int(respHeader[1])
		if addonLen > 0 {
			addonBuf := make([]byte, addonLen)
			if _, err := io.ReadFull(v.Conn, addonBuf); err != nil {
				return 0, fmt.Errorf("read vless response addon: %w", err)
			}
		}
		v.responseRead = true
	}

	// Если есть уже прочитанные и распакованные данные в буфере, отдаём их
	if len(v.readBuf) > 0 {
		n := copy(b, v.readBuf)
		v.readBuf = v.readBuf[n:]
		return n, nil
	}

	// Если padding завершён, читаем напрямую из underlying conn
	if v.paddingDone {
		return v.Conn.Read(b)
	}

	// Если дошли сюда, распаковываем Vision padding блоки
	for !v.paddingDone && len(v.readBuf) == 0 {
		if !v.initialUUIDRead {
			// В начале даунлинка Vision передаёт 16 байт UUID пользователя
			var uuidBuf [16]byte
			if _, err := io.ReadFull(v.Conn, uuidBuf[:]); err != nil {
				return 0, fmt.Errorf("read vision initial uuid: %w", err)
			}
			v.initialUUIDRead = true
		}

		// Заголовок блока: [command: 1 byte][contentLen: 2 bytes][paddingLen: 2 bytes]
		var blockHdr [5]byte
		if _, err := io.ReadFull(v.Conn, blockHdr[:]); err != nil {
			return 0, fmt.Errorf("read vision block header: %w", err)
		}

		command := blockHdr[0]
		contentLen := int(binary.BigEndian.Uint16(blockHdr[1:3]))
		paddingLen := int(binary.BigEndian.Uint16(blockHdr[3:5]))

		// Читаем payload
		if contentLen > 0 {
			content := make([]byte, contentLen)
			if _, err := io.ReadFull(v.Conn, content); err != nil {
				return 0, fmt.Errorf("read vision payload (%d bytes): %w", contentLen, err)
			}
			v.readBuf = append(v.readBuf, content...)
		}

		// Читаем и отбрасываем padding
		if paddingLen > 0 {
			if _, err := io.CopyN(io.Discard, v.Conn, int64(paddingLen)); err != nil {
				return 0, fmt.Errorf("discard vision padding (%d bytes): %w", paddingLen, err)
			}
		}

		// command == 0 (CommandPaddingContinue) -> продолжать padding
		// command != 0 (CommandPaddingEnd или CommandPaddingDirect) -> padding окончен
		if command != 0 {
			v.paddingDone = true
		}
	}

	if len(v.readBuf) > 0 {
		n := copy(b, v.readBuf)
		v.readBuf = v.readBuf[n:]
		return n, nil
	}

	return v.Conn.Read(b)
}

func (v *VisionConn) Close() error {
	return v.Conn.Close()
}

func (v *VisionConn) SetDeadline(t time.Time) error {
	return v.Conn.SetDeadline(t)
}

func (v *VisionConn) SetReadDeadline(t time.Time) error {
	return v.Conn.SetReadDeadline(t)
}

func (v *VisionConn) SetWriteDeadline(t time.Time) error {
	return v.Conn.SetWriteDeadline(t)
}
