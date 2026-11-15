package engine

import (
	"bytes"
	"encoding/binary"
	"testing"
)

const (
	testTCPFlagSYN = 0x02
	testTCPFlagACK = 0x10
	testTCPFlagPSH = 0x08
)

func buildTestTCPPacket(srcIP [4]byte, srcPort uint16, dstIP [4]byte, dstPort uint16, seq, ack uint32, flags byte, payload []byte) []byte {
	totalLen := 20 + 20 + len(payload)
	pkt := make([]byte, totalLen)
	pkt[0] = 0x45
	binary.BigEndian.PutUint16(pkt[2:4], uint16(totalLen))
	pkt[6] = 0x40
	pkt[8] = 64
	pkt[9] = 6
	copy(pkt[12:16], srcIP[:])
	copy(pkt[16:20], dstIP[:])

	tcp := pkt[20:]
	binary.BigEndian.PutUint16(tcp[0:2], srcPort)
	binary.BigEndian.PutUint16(tcp[2:4], dstPort)
	binary.BigEndian.PutUint32(tcp[4:8], seq)
	binary.BigEndian.PutUint32(tcp[8:12], ack)
	tcp[12] = 5 << 4
	tcp[13] = flags
	binary.BigEndian.PutUint16(tcp[14:16], 65535)
	copy(tcp[20:], payload)
	recomputeIPv4Checksums(pkt)
	return pkt
}

func buildTestUDPPacket(srcIP [4]byte, srcPort uint16, dstIP [4]byte, dstPort uint16, payload []byte) []byte {
	totalLen := 20 + 8 + len(payload)
	pkt := make([]byte, totalLen)
	pkt[0] = 0x45
	binary.BigEndian.PutUint16(pkt[2:4], uint16(totalLen))
	pkt[6] = 0x40
	pkt[8] = 64
	pkt[9] = 17
	copy(pkt[12:16], srcIP[:])
	copy(pkt[16:20], dstIP[:])

	udp := pkt[20:]
	binary.BigEndian.PutUint16(udp[0:2], srcPort)
	binary.BigEndian.PutUint16(udp[2:4], dstPort)
	binary.BigEndian.PutUint16(udp[4:6], uint16(8+len(payload)))
	copy(udp[8:], payload)
	recomputeIPv4Checksums(pkt)
	return pkt
}

func TestRecomputeIPv4ChecksumsTCPAndUDP(t *testing.T) {
	// 1. Тест пересчёта контрольной суммы TCP-пакета после 1:1 NAT (10.77.77.2 -> 10.77.0.15)
	tcpPkt := buildTestTCPPacket(
		[4]byte{10, 77, 77, 2}, 54321,
		[4]byte{142, 250, 74, 14}, 443,
		1000, 2000, testTCPFlagACK|testTCPFlagPSH,
		[]byte("hello zapret2"),
	)
	// Меняем IP источника (как делает translateOutboundNAT)
	copy(tcpPkt[12:16], []byte{10, 77, 0, 15})
	recomputeIPv4Checksums(tcpPkt)

	// Проверяем, что сумма IPv4-заголовка даёт 0xffff (дополнение = 0)
	var ipSum uint32
	for i := 0; i < 20; i += 2 {
		ipSum += uint32(binary.BigEndian.Uint16(tcpPkt[i : i+2]))
	}
	for (ipSum >> 16) > 0 {
		ipSum = (ipSum & 0xffff) + (ipSum >> 16)
	}
	if uint16(ipSum) != 0xffff {
		t.Fatalf("неверная контрольная сумма IPv4 заголовка: 0x%04x", ipSum)
	}

	// Проверяем, что сумма TCP-сегмента вместе с псевдозаголовком даёт 0xffff
	l4 := tcpPkt[20:]
	tcpSum := pseudoHeaderSum(tcpPkt[12:16], tcpPkt[16:20], 6, len(l4)) + checksumBytes(l4)
	for (tcpSum >> 16) > 0 {
		tcpSum = (tcpSum & 0xffff) + (tcpSum >> 16)
	}
	if uint16(tcpSum) != 0xffff {
		t.Fatalf("неверная контрольная сумма TCP после NAT: 0x%04x", tcpSum)
	}

	// 2. Тест UDP-пакета
	udpPkt := buildTestUDPPacket(
		[4]byte{10, 77, 77, 2}, 51111,
		[4]byte{1, 1, 1, 1}, 53,
		[]byte{0x12, 0x34, 0x01, 0x00, 0x00, 0x01},
	)
	copy(udpPkt[12:16], []byte{10, 77, 0, 15})
	recomputeIPv4Checksums(udpPkt)
	uL4 := udpPkt[20:]
	udpSum := pseudoHeaderSum(udpPkt[12:16], udpPkt[16:20], 17, len(uL4)) + checksumBytes(uL4)
	for (udpSum >> 16) > 0 {
		udpSum = (udpSum & 0xffff) + (udpSum >> 16)
	}
	if uint16(udpSum) != 0xffff {
		t.Fatalf("неверная контрольная сумма UDP после NAT: 0x%04x", udpSum)
	}
}

func TestBuildICMPPortUnreachableForQUIC(t *testing.T) {
	quicPkt := buildTestUDPPacket(
		[4]byte{10, 77, 77, 2}, 55443,
		[4]byte{142, 250, 74, 14}, 443,
		[]byte("QUIC Initial Payload"),
	)
	icmpPkt := buildICMPPortUnreachable(quicPkt)
	if len(icmpPkt) < 28+28 {
		t.Fatalf("слишком короткий ICMP пакет: %d", len(icmpPkt))
	}
	if icmpPkt[9] != 1 {
		t.Fatalf("протокол должен быть ICMP (1), получено %d", icmpPkt[9])
	}
	// Источник ICMP должен быть IP целевого сервера (142.250.74.14), а получатель — 10.77.77.2
	if !bytes.Equal(icmpPkt[12:16], []byte{142, 250, 74, 14}) {
		t.Fatalf("неверный src IP в ICMP: %v", icmpPkt[12:16])
	}
	if !bytes.Equal(icmpPkt[16:20], []byte{10, 77, 77, 2}) {
		t.Fatalf("неверный dst IP в ICMP: %v", icmpPkt[16:20])
	}
	icmp := icmpPkt[20:]
	if icmp[0] != 3 || icmp[1] != 3 {
		t.Fatalf("ожидался ICMP Type 3 Code 3, получено Type=%d Code=%d", icmp[0], icmp[1])
	}
	cs := checksumBytes(icmp)
	for (cs >> 16) > 0 {
		cs = (cs & 0xffff) + (cs >> 16)
	}
	if uint16(cs) != 0xffff {
		t.Fatalf("неверная контрольная сумма ICMP: 0x%04x", cs)
	}
}
