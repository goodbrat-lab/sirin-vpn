package main

import (
	"bufio"
	"context"
	"encoding/binary"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func validPSNCfg() *PSNProxyConfig {
	return &PSNProxyConfig{Type: "SOCKS5", Host: " 1.2.3.4 ", Port: 8000, Username: "u", Password: "p"}
}

func TestPSNProxyValidateDefaults(t *testing.T) {
	p := validPSNCfg()
	if err := p.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if p.Type != "socks5" || p.Host != "1.2.3.4" || p.Via != "wan" || p.Fallback != "vpn" {
		t.Fatalf("неверная нормализация: %+v", p)
	}
	if p.ListenPort != DefaultPSNProxyListenPort || p.DNSPort != DefaultPSNProxyDNSPort || p.UpstreamDNS != "1.1.1.1" {
		t.Fatalf("неверные порты/DNS по умолчанию: %+v", p)
	}

	bad := []*PSNProxyConfig{
		{Type: "ftp", Host: "1.2.3.4", Port: 1},
		{Type: "http", Host: "", Port: 1},
		{Type: "http", Host: "h", Port: 70000},
	}
	for _, b := range bad {
		if err := b.Validate(); err == nil {
			t.Errorf("ожидалась ошибка для %+v", b)
		}
	}
}

func TestPSNProxyHostMatching(t *testing.T) {
	p := validPSNCfg()
	_ = p.Validate()
	for _, h := range []string{"store.playstation.com", "web.np.playstation.com", "ca.account.sony.com", "client-api.arkoselabs.com", "playstation.net"} {
		if !p.IsPSNHost(h) {
			t.Errorf("%s должен идти через прокси", h)
		}
	}
	for _, h := range []string{"gs2.ww.prod.dl.playstation.net", "youtube.com", "notplaystation.com", ""} {
		if p.IsPSNHost(h) {
			t.Errorf("%s не должен идти через прокси", h)
		}
	}
	p.IncludeDownloads = true
	if !p.IsPSNHost("gs2.ww.prod.dl.playstation.net") {
		t.Errorf("при include_downloads CDN загрузок должен идти через прокси")
	}
}

func TestPSNProxyFilterAndDnsmasq(t *testing.T) {
	p := validPSNCfg()
	_ = p.Validate()
	out := p.FilterOutPSNDomains([]string{"openai.com", "playstation.com", "sonyentertainmentnetwork.com", "spotify.com"})
	if strings.Join(out, ",") != "openai.com,spotify.com" {
		t.Fatalf("FilterOutPSNDomains: %v", out)
	}
	c := BuildDnsmasqPSNProxyContent(p)
	if !strings.Contains(c, "/127.0.0.1#56253\n") || !strings.Contains(c, "playstation.com") {
		t.Fatalf("нет перенаправления на форвардер:\n%s", c)
	}
	if !strings.Contains(c, "server=/dl.playstation.net/127.0.0.1#56254\n") {
		t.Fatalf("нет исключения CDN загрузок:\n%s", c)
	}
}

func TestPSNProxyConfigFromTOML(t *testing.T) {
	path := t.TempDir() + "/client.toml"
	data := `
gateway = "gw.example.com"
password = "x"

[psn_proxy]
enabled = true
type = "http"
host = "5.6.7.8"
port = 3128
username = "user"
password = "pass"
via = "vpn"
`
	if err := writeFileForTest(path, data); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if !cfg.IsPSNProxyEnabled() || cfg.PSNProxy.Type != "http" || cfg.PSNProxy.Via != "vpn" {
		t.Fatalf("секция psn_proxy разобрана неверно: %+v", cfg.PSNProxy)
	}
	opts := cfg.GetPSNProxyOptions()
	if !opts.Enabled || opts.ListenPort != 56200 {
		t.Fatalf("GetPSNProxyOptions: %+v", opts)
	}
}

func TestPSNProxyNftablesRules(t *testing.T) {
	rules := generateNftablesRules("sirin0", "br-lan", "bypass", nil, []string{"5.0.0.0/8"}, "", nil, nil,
		DefaultOpSecOptions(), AntiDPIConfig{Enabled: true, Port: 56100},
		ForceVPNOptions{Enabled: true, HasDomains: true}, PSNProxyOptions{Enabled: true, ListenPort: 56200})
	for _, want := range []string{
		"set psn_proxy_cidrs {",
		`iifname "br-lan" ip daddr @psn_proxy_cidrs tcp dport { 80, 443 } redirect to :56200`,
		`iifname "br-lan" ip daddr @psn_proxy_cidrs udp dport 443 reject`,
		`iifname "br-lan" tcp dport 56200 accept`,
		"tcp dport 56200 drop",
		"tcp dport 56100 drop",
	} {
		if !strings.Contains(rules, want) {
			t.Errorf("нет строки %q", want)
		}
	}
	iPSN := strings.Index(rules, "@psn_proxy_cidrs tcp dport { 80, 443 } redirect")
	iFV := strings.Index(rules, `iifname "br-lan" ip daddr @force_vpn_cidrs return`)
	iAD := strings.Index(rules, "@antidpi_cidrs tcp dport { 80, 443 } redirect")
	if iPSN < 0 || iFV < 0 || iAD < 0 || iPSN > iFV || iPSN > iAD {
		t.Fatalf("редирект PSN должен идти первым в dstnat (psn=%d fv=%d ad=%d)", iPSN, iFV, iAD)
	}

	without := generateNftablesRules("sirin0", "br-lan", "bypass", nil, nil, "", nil, nil, DefaultOpSecOptions())
	if strings.Contains(without, "psn_proxy") {
		t.Fatalf("без PSN-прокси правил psn_proxy быть не должно")
	}
}

func buildDNSResponse(name string, ips []net.IP) []byte {
	msg := []byte{0x12, 0x34, 0x81, 0x80, 0, 1, 0, byte(len(ips) + 1), 0, 0, 0, 0}
	for _, l := range strings.Split(name, ".") {
		msg = append(msg, byte(len(l)))
		msg = append(msg, l...)
	}
	msg = append(msg, 0, 0, 1, 0, 1)
	// CNAME (должен быть пропущен)
	msg = append(msg, 0xC0, 0x0C, 0, 5, 0, 1, 0, 0, 0, 60, 0, 2, 0xC0, 0x0C)
	for _, ip := range ips {
		msg = append(msg, 0xC0, 0x0C, 0, 1, 0, 1, 0, 0, 0, 60, 0, 4)
		msg = append(msg, ip.To4()...)
	}
	return msg
}

func TestParseDNSAnswerIPv4(t *testing.T) {
	resp := buildDNSResponse("store.playstation.com", []net.IP{net.ParseIP("23.222.42.136"), net.ParseIP("2.16.153.37")})
	ips := ParseDNSAnswerIPv4(resp)
	if len(ips) != 2 || ips[0].String() != "23.222.42.136" || ips[1].String() != "2.16.153.37" {
		t.Fatalf("ParseDNSAnswerIPv4: %v", ips)
	}
	if ParseDNSAnswerIPv4(resp[:20]) != nil && len(ParseDNSAnswerIPv4(resp[:20])) != 0 {
		t.Fatalf("усечённый ответ не должен давать адресов")
	}
	if len(ParseDNSAnswerIPv4([]byte{1, 2, 3})) != 0 {
		t.Fatalf("мусор не должен давать адресов")
	}
}

func TestCIDRMatcher(t *testing.T) {
	m := newCIDRMatcher([]string{"5.0.0.0/8", "77.88.8.8"})
	if !m(net.ParseIP("5.1.2.3")) || !m(net.ParseIP("77.88.8.8")) || m(net.ParseIP("8.8.8.8")) {
		t.Fatalf("CIDR matcher работает неверно")
	}
}

// fakeSOCKS5 — минимальный SOCKS5-сервер с авторизацией user/pass, отвечающий "OK:<host>:<port>".
func fakeSOCKS5(t *testing.T, user, pass string) string {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				h := make([]byte, 2)
				io.ReadFull(c, h)
				io.ReadFull(c, make([]byte, h[1]))
				c.Write([]byte{5, 2})
				v := make([]byte, 2)
				io.ReadFull(c, v)
				u := make([]byte, v[1])
				io.ReadFull(c, u)
				pl := make([]byte, 1)
				io.ReadFull(c, pl)
				p := make([]byte, pl[0])
				io.ReadFull(c, p)
				if string(u) != user || string(p) != pass {
					c.Write([]byte{1, 1})
					return
				}
				c.Write([]byte{1, 0})
				req := make([]byte, 4)
				io.ReadFull(c, req)
				var host string
				switch req[3] {
				case 3:
					l := make([]byte, 1)
					io.ReadFull(c, l)
					hb := make([]byte, l[0])
					io.ReadFull(c, hb)
					host = string(hb)
				case 1:
					ip := make([]byte, 4)
					io.ReadFull(c, ip)
					host = net.IP(ip).String()
				}
				pb := make([]byte, 2)
				io.ReadFull(c, pb)
				c.Write([]byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0})
				c.Write([]byte("OK:" + host + ":" + itoa(int(binary.BigEndian.Uint16(pb)))))
			}(c)
		}
	}()
	return ln.Addr().String()
}

func fakeHTTPProxy(t *testing.T, auth string) string {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				r := bufio.NewReader(c)
				line, _ := r.ReadString('\n')
				target := strings.Fields(line)[1]
				ok := false
				for {
					l, _ := r.ReadString('\n')
					if l == "\r\n" || l == "" {
						break
					}
					if strings.HasPrefix(l, "Proxy-Authorization: Basic "+auth) {
						ok = true
					}
				}
				if !ok {
					c.Write([]byte("HTTP/1.1 407 Proxy Authentication Required\r\n\r\n"))
					return
				}
				c.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\nOK:" + target))
			}(c)
		}
	}()
	return ln.Addr().String()
}

func itoa(i int) string { return strconv.Itoa(i) }

func writeFileForTest(path, data string) error { return os.WriteFile(path, []byte(data), 0600) }

func readAllShort(t *testing.T, c net.Conn) string {
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	b, _ := io.ReadAll(c)
	return string(b)
}

func TestUpstreamSOCKS5(t *testing.T) {
	addr := fakeSOCKS5(t, "user", "secret")
	u := &upstreamProxy{typ: "socks5", addr: addr, username: "user", password: "secret", dialer: &net.Dialer{Timeout: time.Second}}
	c, err := u.Dial(context.Background(), "store.playstation.com", 443)
	if err != nil {
		t.Fatalf("SOCKS5 Dial: %v", err)
	}
	if got := readAllShort(t, c); got != "OK:store.playstation.com:443" {
		t.Fatalf("SOCKS5: прокси получил %q", got)
	}
	c.Close()

	u.password = "wrong"
	if _, err := u.Dial(context.Background(), "store.playstation.com", 443); err == nil || !strings.Contains(err.Error(), "логин или пароль") {
		t.Fatalf("ожидалась ошибка авторизации, получено %v", err)
	}
}

func TestUpstreamHTTPConnect(t *testing.T) {
	addr := fakeHTTPProxy(t, "dXNlcjpzZWNyZXQ=") // user:secret
	u := &upstreamProxy{typ: "http", addr: addr, username: "user", password: "secret", dialer: &net.Dialer{Timeout: time.Second}}
	c, err := u.Dial(context.Background(), "checkout.playstation.com", 443)
	if err != nil {
		t.Fatalf("HTTP CONNECT Dial: %v", err)
	}
	if got := readAllShort(t, c); got != "OK:checkout.playstation.com:443" {
		t.Fatalf("HTTP CONNECT: прокси получил %q", got)
	}
	c.Close()

	u.password = "wrong"
	if _, err := u.Dial(context.Background(), "checkout.playstation.com", 443); err == nil || !strings.Contains(err.Error(), "407") {
		t.Fatalf("ожидалась ошибка 407, получено %v", err)
	}
}
