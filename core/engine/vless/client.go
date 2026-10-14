package vless

import (
	"context"
	"fmt"
	"net"
)

// Client представляет клиент VLESS поверх Reality.
type Client struct {
	uuid [16]byte
	cfg  *RealityConfig
}

// NewClient создаёт новый экземпляр клиента VLESS.
func NewClient(uuid [16]byte, cfg *RealityConfig) *Client {
	return &Client{
		uuid: uuid,
		cfg:  cfg,
	}
}

// DialTCP устанавливает защищённое соединение через VLESS Reality с поддержкой flow="xtls-rprx-vision".
func (c *Client) DialTCP(ctx context.Context, target string) (net.Conn, error) {
	conn, err := Dial(ctx, c.cfg)
	if err != nil {
		return nil, fmt.Errorf("reality dial: %w", err)
	}

	req := &Request{
		UUID:   c.uuid,
		Cmd:    CmdTCP,
		Target: target,
		Flow:   AddonFlowVision,
	}

	hdr, err := req.Encode()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("encode vless header: %w", err)
	}

	return NewVisionConn(conn, hdr, c.uuid), nil
}

// DialUDP устанавливает VLESS-туннель для передачи UDP трафика.
func (c *Client) DialUDP(ctx context.Context, target string) (net.Conn, error) {
	conn, err := Dial(ctx, c.cfg)
	if err != nil {
		return nil, fmt.Errorf("reality dial: %w", err)
	}

	req := &Request{
		UUID:   c.uuid,
		Cmd:    CmdUDP,
		Target: target,
		Flow:   "",
	}

	hdr, err := req.Encode()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("encode vless header: %w", err)
	}

	return NewVisionConn(conn, hdr, c.uuid), nil
}
