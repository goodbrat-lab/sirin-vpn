package server

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
)

// tcpFrameMax — максимальный размер кадра streamlink (совпадает с клиентом).
const tcpFrameMax = 2048

// StreamLink оборачивает потоковое TCP-соединение для кадрирования пакетов obfs.
// Формат кадра на проводе:
//
//	[2 байта длины, Big-Endian][ровно столько байт — obfs-пакет]
//
// Длина считает только тело; сами 2 байта длины в неё не входят.
type StreamLink struct {
	conn net.Conn
	wmu  sync.Mutex
	rbuf []byte
}

// NewStreamLink создаёт новый экземпляр StreamLink для TCP-соединения.
func NewStreamLink(conn net.Conn) *StreamLink {
	return &StreamLink{
		conn: conn,
		rbuf: make([]byte, tcpFrameMax),
	}
}

// ReadFrame считывает один полный obfs-пакет из потока.
func (l *StreamLink) ReadFrame() ([]byte, error) {
	var lenBuf [2]byte
	if _, err := io.ReadFull(l.conn, lenBuf[:]); err != nil {
		return nil, err
	}
	n := int(binary.BigEndian.Uint16(lenBuf[:]))
	if n == 0 || n > tcpFrameMax {
		return nil, fmt.Errorf("кадр TCP: длина %d вне диапазона [1..%d] — поток разошёлся", n, tcpFrameMax)
	}

	pkt := make([]byte, n)
	if _, err := io.ReadFull(l.conn, pkt); err != nil {
		return nil, err
	}
	return pkt, nil
}

// WriteFrame упаковывает obfs-пакет в кадр streamlink и отправляет его в соединение одним вызовом.
func (l *StreamLink) WriteFrame(wire []byte) error {
	if len(wire) > tcpFrameMax {
		return fmt.Errorf("кадр TCP: пакет %d байт больше потолка %d", len(wire), tcpFrameMax)
	}

	frame := make([]byte, 2+len(wire))
	binary.BigEndian.PutUint16(frame[:2], uint16(len(wire)))
	copy(frame[2:], wire)

	l.wmu.Lock()
	defer l.wmu.Unlock()

	nw, err := l.conn.Write(frame)
	if err != nil {
		return err
	}
	if nw != len(frame) {
		return fmt.Errorf("кадр TCP: неполная запись %d из %d байт", nw, len(frame))
	}
	return nil
}

// Close закрывает подлежащее TCP-соединение.
func (l *StreamLink) Close() error {
	return l.conn.Close()
}
