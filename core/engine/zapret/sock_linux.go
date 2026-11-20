//go:build linux

package zapret

import (
	"encoding/binary"
	"net"
	"syscall"
	"unsafe"
)

// sendOOB отправляет data через обычный write, затем 1 байт TCP Urgent (MSG_OOB).
// Анализаторы ТСПУ (stateful DPI) склеивают OOB-байт с TLS-потоком (ломая SNI),
// тогда как целевой TLS-сервер (SO_OOBINLINE=0 по умолчанию) отбрасывает его.
//
// ИСПРАВЛЕНИЕ: старая реализация копировала data+oobByte в один buf и отправляла
// всё с флагом MSG_OOB — это некорректно. MSG_OOB должен содержать ровно 1 байт
// срочных данных, отправляемый отдельным syscall. Результат: TLS-запись на сервере
// получалась повреждённой (лишний байт в конце), соединение падало с TLS Alert.
func sendOOB(conn net.Conn, data []byte, oobByte byte) error {
	tc, ok := conn.(*net.TCPConn)
	if !ok {
		_, err := conn.Write(data)
		return err
	}
	rc, err := tc.SyscallConn()
	if err != nil {
		_, werr := conn.Write(data)
		return werr
	}

	// Шаг 1: отправляем data обычным write (без MSG_OOB).
	if _, err := tc.Write(data); err != nil {
		return err
	}

	// Шаг 2: отправляем ровно 1 байт с флагом MSG_OOB.
	oobBuf := [1]byte{oobByte}
	var sendErr error
	cerr := rc.Write(func(fd uintptr) bool {
		_, _, errno := syscall.Syscall6(
			syscall.SYS_SENDTO,
			fd,
			uintptr(unsafe.Pointer(&oobBuf[0])),
			1,
			uintptr(syscall.MSG_OOB),
			0,
			0,
		)
		if errno == syscall.EAGAIN || errno == syscall.EWOULDBLOCK {
			return false
		}
		if errno != 0 {
			sendErr = errno
		}
		return true
	})
	if cerr != nil {
		return cerr
	}
	return sendErr
}

// setConnTTL временно меняет IP_TTL на TCP-сокете.
func setConnTTL(conn net.Conn, ttl int) error {
	tc, ok := conn.(*net.TCPConn)
	if !ok {
		return nil
	}
	rc, err := tc.SyscallConn()
	if err != nil {
		return err
	}
	var sockErr error
	_ = rc.Control(func(fd uintptr) {
		sockErr = syscall.SetsockoptInt(int(fd), syscall.IPPROTO_IP, syscall.IP_TTL, ttl)
	})
	return sockErr
}

// setPacketConnTTL временно меняет IP_TTL на UDP-сокете (для QUIC/UDP Fake Burst).
func setPacketConnTTL(pc net.PacketConn, ttl int) error {
	sc, ok := pc.(interface {
		SyscallConn() (syscall.RawConn, error)
	})
	if !ok {
		return nil
	}
	rc, err := sc.SyscallConn()
	if err != nil {
		return err
	}
	var sockErr error
	_ = rc.Control(func(fd uintptr) {
		sockErr = syscall.SetsockoptInt(int(fd), syscall.IPPROTO_IP, syscall.IP_TTL, ttl)
	})
	return sockErr
}

// trySendRawFakeTCP пытается отправить инъекцию фейкового TCP-пакета (Fake ClientHello)
// с заведомо неверной контрольной суммой TCP (badsum) и малым TTL (например, TTL=4)
// через raw-сокет (доступно при запуске от root / CAP_NET_RAW на роутере OpenWrt).
// Для установки точного Sequence Number использует Linux TCP_REPAIR_QUEUE или отправку
// пакета-приманки, который отбрасывается конечным сервером по badsum.
func trySendRawFakeTCP(conn net.Conn, fakePayload []byte, ttl int) bool {
	tc, ok := conn.(*net.TCPConn)
	if !ok || len(fakePayload) == 0 {
		return false
	}
	laddr, ok1 := tc.LocalAddr().(*net.TCPAddr)
	raddr, ok2 := tc.RemoteAddr().(*net.TCPAddr)
	if !ok1 || !ok2 {
		return false
	}
	srcIP := laddr.IP.To4()
	dstIP := raddr.IP.To4()
	if srcIP == nil || dstIP == nil {
		return false
	}

	// Читаем текущий snd_nxt и rcv_nxt через TCP_REPAIR (если есть CAP_NET_ADMIN)
	var seqNum, ackNum uint32
	rc, err := tc.SyscallConn()
	if err != nil {
		return false
	}

	const (
		TCP_REPAIR       = 19
		TCP_REPAIR_QUEUE = 20
		TCP_QUEUE_SEQ    = 21
		TCP_SEND_QUEUE   = 1
		TCP_RECV_QUEUE   = 2
	)

	gotSeq := false
	_ = rc.Control(func(fd uintptr) {
		if syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, TCP_REPAIR, 1) == nil {
			_ = syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, TCP_REPAIR_QUEUE, TCP_SEND_QUEUE)
			var sq uint32
			slen := uint32(4)
			_, _, e1 := syscall.Syscall6(syscall.SYS_GETSOCKOPT, fd, syscall.IPPROTO_TCP, TCP_QUEUE_SEQ, uintptr(unsafe.Pointer(&sq)), uintptr(unsafe.Pointer(&slen)), 0)
			_ = syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, TCP_REPAIR_QUEUE, TCP_RECV_QUEUE)
			var rq uint32
			rlen := uint32(4)
			_, _, e2 := syscall.Syscall6(syscall.SYS_GETSOCKOPT, fd, syscall.IPPROTO_TCP, TCP_QUEUE_SEQ, uintptr(unsafe.Pointer(&rq)), uintptr(unsafe.Pointer(&rlen)), 0)
			_ = syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, TCP_REPAIR, 0)
			if e1 == 0 && e2 == 0 {
				seqNum = sq
				ackNum = rq
				gotSeq = true
			}
		}
	})
	if !gotSeq {
		return false
	}

	rawFd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, syscall.IPPROTO_RAW)
	if err != nil {
		return false
	}
	defer syscall.Close(rawFd)

	_ = syscall.SetsockoptInt(rawFd, syscall.IPPROTO_IP, syscall.IP_HDRINCL, 1)
	// Маркируем сокет 0x2, чтобы на роутере он гарантированно шёл через физический WAN
	_ = syscall.SetsockoptInt(rawFd, syscall.SOL_SOCKET, syscall.SO_MARK, 0x2)

	totalLen := 20 + 20 + len(fakePayload)
	pkt := make([]byte, totalLen)

	// IPv4 заголовок
	pkt[0] = 0x45
	binary.BigEndian.PutUint16(pkt[2:4], uint16(totalLen))
	binary.BigEndian.PutUint16(pkt[4:6], 0x1337)
	pkt[6] = 0x40 // DF
	if ttl <= 0 {
		ttl = 4
	}
	pkt[8] = byte(ttl)
	pkt[9] = syscall.IPPROTO_TCP
	copy(pkt[12:16], srcIP)
	copy(pkt[16:20], dstIP)

	// IPv4 контрольная сумма
	var ipSum uint32
	for i := 0; i < 20; i += 2 {
		ipSum += uint32(binary.BigEndian.Uint16(pkt[i : i+2]))
	}
	for (ipSum >> 16) > 0 {
		ipSum = (ipSum & 0xffff) + (ipSum >> 16)
	}
	binary.BigEndian.PutUint16(pkt[10:12], ^uint16(ipSum))

	// TCP заголовок
	tcp := pkt[20:]
	binary.BigEndian.PutUint16(tcp[0:2], uint16(laddr.Port))
	binary.BigEndian.PutUint16(tcp[2:4], uint16(raddr.Port))
	binary.BigEndian.PutUint32(tcp[4:8], seqNum)
	binary.BigEndian.PutUint32(tcp[8:12], ackNum)
	tcp[12] = 5 << 4 // Data offset = 20 байт
	tcp[13] = 0x18   // PSH | ACK
	binary.BigEndian.PutUint16(tcp[14:16], 4096)
	copy(tcp[20:], fakePayload)

	// Заведомо неверная контрольная сумма TCP (badsum, техника Zapret nfqws2):
	// DPI-анализатор на промежуточном узле не проверяет L4 checksum и парсит фейковый SNI,
	// а сетевой стек целевого сервера гарантированно отбрасывает сегмент по badsum (даже если TTL дойдёт).
	binary.BigEndian.PutUint16(tcp[16:18], 0xdead)

	var sa syscall.SockaddrInet4
	sa.Port = raddr.Port
	copy(sa.Addr[:], dstIP)

	if err := syscall.Sendto(rawFd, pkt, 0, &sa); err != nil {
		return false
	}
	return true
}
