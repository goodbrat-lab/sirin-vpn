package vless

import (
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
)

const (
	vlessVersion = 0

	CmdTCP = 1
	CmdUDP = 2
	CmdMux = 3

	AddrTypeIPv4   = 1
	AddrTypeDomain = 2
	AddrTypeIPv6   = 3

	AddonFlowVision = "xtls-rprx-vision"
)

// Request представляет заголовок клиентского запроса VLESS.
type Request struct {
	UUID   [16]byte
	Cmd    byte
	Target string // host:port
	Flow   string // например, "xtls-rprx-vision"
}

// Encode кодирует заголовок VLESS с поддержкой addons/Vision.
func (r *Request) Encode() ([]byte, error) {
	buf := make([]byte, 0, 128)
	buf = append(buf, vlessVersion)
	buf = append(buf, r.UUID[:]...)

	// Addons (flow)
	addon := encodeAddon(r.Flow)
	buf = append(buf, byte(len(addon)))
	if len(addon) > 0 {
		buf = append(buf, addon...)
	}

	buf = append(buf, r.Cmd)

	// Разбор target на хост и порт
	host, portStr, err := net.SplitHostPort(r.Target)
	if err != nil {
		return nil, fmt.Errorf("invalid target '%s': %w", r.Target, err)
	}
	portNum, err := strconv.Atoi(portStr)
	if err != nil || portNum <= 0 || portNum > 65535 {
		return nil, fmt.Errorf("invalid port '%s'", portStr)
	}

	// 2 байта порта Big-Endian
	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, uint16(portNum))
	buf = append(buf, portBytes...)

	// Тип адреса и сам адрес
	ip := net.ParseIP(host)
	if ip == nil {
		// Доменное имя
		if len(host) > 255 {
			return nil, fmt.Errorf("domain name too long: %d bytes", len(host))
		}
		buf = append(buf, AddrTypeDomain)
		buf = append(buf, byte(len(host)))
		buf = append(buf, []byte(host)...)
	} else if ip4 := ip.To4(); ip4 != nil {
		// IPv4 (4 байта)
		buf = append(buf, AddrTypeIPv4)
		buf = append(buf, ip4...)
	} else {
		// IPv6 (16 байт)
		buf = append(buf, AddrTypeIPv6)
		buf = append(buf, ip.To16()...)
	}

	return buf, nil
}

// encodeAddon кодирует flow в формат VLESS addons (protobuf wire format: field 1 string).
// Для Vision: tag 0x0a (field 1, wire type 2), length byte, flow string
func encodeAddon(flow string) []byte {
	if flow == "" {
		return nil
	}
	buf := make([]byte, 0, len(flow)+2)
	buf = append(buf, 0x0a) // tag: field 1, type 2 (length-delimited string)
	buf = append(buf, byte(len(flow)))
	buf = append(buf, []byte(flow)...)
	return buf
}

// Response представляет заголовок ответа сервера VLESS.
type Response struct {
	Version byte
	Addon   []byte
}

// DecodeResponse декодирует ответ VLESS от сервера (обычно [version=0][addon_len][addon...]).
func DecodeResponse(header []byte) (*Response, int, error) {
	if len(header) < 2 {
		return nil, 0, fmt.Errorf("vless response header too short")
	}
	ver := header[0]
	addonLen := int(header[1])
	totalLen := 2 + addonLen
	if len(header) < totalLen {
		return nil, 0, fmt.Errorf("incomplete vless response header")
	}
	var addon []byte
	if addonLen > 0 {
		addon = header[2 : 2+addonLen]
	}
	return &Response{Version: ver, Addon: addon}, totalLen, nil
}
