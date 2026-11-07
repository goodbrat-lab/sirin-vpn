package main

import (
	"fmt"
	"net"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

var domainRegex = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)

// PortRule представляет нормализованное правило для порта (протокол + одиночный порт или диапазон).
type PortRule struct {
	Proto string // "tcp", "udp", "both"
	Range string // "80" или "8080-8090"
}

type SplitTunnelConfig struct {
	Mode        string   `toml:"mode"`          // "bypass" (default) или "only"
	UseRuDirect *bool    `toml:"use_ru_direct"` // по умолчанию true для bypass
	CIDRs       []string `toml:"cidrs"`         // список IP / CIDR
	Domains     []string `toml:"domains"`       // список доменов
	Ports       []string `toml:"-"`             // нормализованный список портов (например ["80", "443", "tcp:8080", "udp:27000-27050"])
	RawPorts    []any    `toml:"ports"`         // сырые значения портов из TOML (поддерживает числа и строки)
	PortMode    string   `toml:"port_mode"`     // режим разделения портов: "only" (default при указании портов) или "bypass"
}

type Config struct {
	// Подключение
	Gateway  string `toml:"gateway"`   // crp.guseynovia.ru или IP шлюза
	Password string `toml:"password"`  // UUID клиента
	DeviceID string `toml:"device_id"` // router-01

	// TUN
	TunName string `toml:"tun_name"` // sirin0
	TunMTU  int    `toml:"tun_mtu"`  // 1280
	TunIP   string `toml:"tun_ip"`   // 10.77.77.2
	TunMask string `toml:"tun_mask"` // 255.255.255.0

	// Маршрутизация
	LanInterface string `toml:"lan_interface"` // br-lan
	WanInterface string `toml:"wan_interface"` // wan / eth0
	WanGateway   string `toml:"wan_gateway"`   // шлюз WAN провайдера (если не указан, определяется автоматически)

	// API (опционально — если не указан ladder, берём из /v1/params)
	APIURL string `toml:"api_url"` // https://crp.guseynovia.ru

	// Dead Drop (резервный канал обновления параметров через Яндекс Диск или S3 при блокировке центрального API)
	DeadDropURLs []string `toml:"dead_drop_urls,omitempty"`

	// Fallback: если API недоступен, использовать статичный ladder
	StaticLadder       []LadderEntry `toml:"ladder"`
	PreferStaticLadder bool          `toml:"prefer_static_ladder,omitempty"` // принудительно использовать [[ladder]] из конфига вместо API

	// Split tunneling (обратная совместимость с v1.1.2)
	RuDirect    *bool              `toml:"ru_direct,omitempty"`
	SplitTunnel *SplitTunnelConfig `toml:"split_tunnel,omitempty"`

	// Kill Switch (блокировка трафика LAN в WAN при обрыве туннеля)
	KillSwitch           *bool    `toml:"kill_switch,omitempty"`
	KillSwitchExceptions []string `toml:"kill_switch_exceptions,omitempty"`

	// OpSec & Fingerprinting (методика Vektor T13: маскировка стека и защита от утечек)
	NormalizeTTL *bool `toml:"normalize_ttl,omitempty"` // нормализация TTL на выходе из туннеля (по умолчанию true)
	TargetTTL    int   `toml:"target_ttl,omitempty"`    // целевое значение TTL (по умолчанию 64)
	InterceptDNS *bool `toml:"intercept_dns,omitempty"` // принудительный перехват DNS (порт 53) на dnsmasq (по умолчанию true)
	BlockDoT     *bool `toml:"block_dot,omitempty"`      // блокировка обхода через DNS-over-TLS (порт 853) (по умолчанию true)

	// Anti-DPI (прямой доступ к YouTube и Discord на полной скорости провайдера без VPN)
	AntiDPI *AntiDPIConfig `toml:"antidpi,omitempty"`

	// Whitelist Fallback (аварийный обход блокировок через белые списки: WebRTC TURN VK Calls / DION / S3)
	WhitelistFallback *WhitelistFallbackConfig `toml:"whitelist_fallback,omitempty"`
}

// WhitelistFallbackConfig задаёт параметры аварийного резервного канала через
// отечественные WebRTC TURN-релеи (VK Calls / DION) и белые списки при жестких блокировках.
type WhitelistFallbackConfig struct {
	Enabled     bool     `toml:"enabled"`                // включен ли аварийный канал (по умолчанию true)
	Mode        string   `toml:"mode"`                   // "auto" (fallback при отказе direct/stream) или "always" (только whitelist)
	GatewayPort int      `toml:"gateway_port,omitempty"` // порт шлюза (по умолчанию 56005)
	UseDTLS     bool     `toml:"use_dtls,omitempty"`     // использовать DTLS поверх TURN
	TurnBackup  string   `toml:"turn_backup,omitempty"`  // запасной TURN адрес (например "193.203.43.1:19302")
	TurnRealm   string   `toml:"turn_realm,omitempty"`   // realm (по умолчанию "vkcalls")
	RelayTarget string   `toml:"relay_target,omitempty"` // запасной IP адрес назначения шлюза
	VKHashes    []string `toml:"vk_hashes,omitempty"`    // ссылки/хеши VK Calls (https://vk.com/call/join/...)
	DionSlugs   []string `toml:"dion_slugs,omitempty"`   // ссылки/slug комнат DION (https://dion.vc/event/...)
}

func DefaultWhitelistFallbackConfig() WhitelistFallbackConfig {
	return WhitelistFallbackConfig{
		Enabled:     true,
		Mode:        "auto",
		GatewayPort: 56005,
		UseDTLS:     false,
		TurnBackup:  "193.203.43.1:19302",
		TurnRealm:   "vkcalls",
	}
}

func (c *Config) GetWhitelistFallbackConfig() WhitelistFallbackConfig {
	if c.WhitelistFallback != nil {
		cfg := *c.WhitelistFallback
		if cfg.GatewayPort <= 0 {
			cfg.GatewayPort = 56005
		}
		if cfg.TurnBackup == "" {
			cfg.TurnBackup = "193.203.43.1:19302"
		}
		if cfg.TurnRealm == "" {
			cfg.TurnRealm = "vkcalls"
		}
		if cfg.Mode == "" {
			cfg.Mode = "auto"
		}
		return cfg
	}
	return DefaultWhitelistFallbackConfig()
}

func (c *Config) IsWhitelistFallbackEnabled() bool {
	if c.WhitelistFallback != nil {
		return c.WhitelistFallback.Enabled
	}
	return true
}

// OpSecOptions задаёт параметры маскировки сетевого стека и защиты от утечек (методика Vektor T13).
type OpSecOptions struct {
	NormalizeTTL bool // фиксация TTL на egress туннеля
	TargetTTL    int  // целевое значение TTL (по умолчанию 64)
	InterceptDNS bool // принудительный перехват порта 53 на локальный dnsmasq
	BlockDoT     bool // блокировка порта 853 (DNS-over-TLS)
}

func DefaultOpSecOptions() OpSecOptions {
	return OpSecOptions{
		NormalizeTTL: true,
		TargetTTL:    64,
		InterceptDNS: true,
		BlockDoT:     true,
	}
}

func (c *Config) GetOpSecOptions() OpSecOptions {
	opts := DefaultOpSecOptions()
	if c.NormalizeTTL != nil {
		opts.NormalizeTTL = *c.NormalizeTTL
	}
	if c.TargetTTL > 0 && c.TargetTTL <= 255 {
		opts.TargetTTL = c.TargetTTL
	}
	if c.InterceptDNS != nil {
		opts.InterceptDNS = *c.InterceptDNS
	}
	if c.BlockDoT != nil {
		opts.BlockDoT = *c.BlockDoT
	}
	return opts
}

func (c *Config) GetDeadDropURLs() []string {
	var urls []string
	for _, u := range c.DeadDropURLs {
		u = strings.TrimSpace(u)
		if u != "" {
			urls = append(urls, u)
		}
	}
	return urls
}

func (c *Config) GetParamsCachePath() string {
	return "/tmp/sirin_params_cache.json"
}

func (c *Config) GetAntiDPIConfig() AntiDPIConfig {
	if c.AntiDPI != nil {
		cfg := *c.AntiDPI
		if cfg.Port <= 0 {
			cfg.Port = DefaultAntiDPIPort
		}
		return cfg
	}
	return DefaultAntiDPIConfig()
}

func (c *Config) IsAntiDPIEnabled() bool {
	if c.AntiDPI != nil {
		return c.AntiDPI.Enabled
	}
	return true
}

type LadderEntry struct {
	Name       string   `toml:"name"`
	Transport  string   `toml:"transport"` // udp | vless | xhttp | relay
	Host       string   `toml:"host"`
	Port       int      `toml:"port"`
	SNI        string   `toml:"sni,omitempty"`
	SNIs       []string `toml:"snis,omitempty"`
	PBK        string   `toml:"pbk,omitempty"`
	SID        string   `toml:"sid,omitempty"`
	Path       string   `toml:"path,omitempty"`
	Mode       string   `toml:"mode,omitempty"`
	DTLS       bool     `toml:"dtls,omitempty"`
	TurnBackup string   `toml:"turn_backup,omitempty"`
	TurnRealm  string   `toml:"turn_realm,omitempty"`
	VKHashes   []string `toml:"vk_hashes,omitempty"`
	DionSlugs  []string `toml:"dion_slugs,omitempty"`
}

// ValidateCIDR проверяет и нормализует строку IPv4 CIDR или одиночный IP.
// Возвращает нормализованную форму (например "1.1.1.1/32" или "10.0.0.0/8").
func ValidateCIDR(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("пустая строка CIDR")
	}
	if !strings.Contains(s, "/") {
		ip := net.ParseIP(s)
		if ip == nil {
			return "", fmt.Errorf("неверный IP-адрес: %q", s)
		}
		if ip.To4() == nil {
			return "", fmt.Errorf("поддерживается только IPv4: %q", s)
		}
		return ip.To4().String() + "/32", nil
	}

	ip, ipNet, err := net.ParseCIDR(s)
	if err != nil {
		return "", fmt.Errorf("неверный формат CIDR %q: %w", s, err)
	}
	if ip.To4() == nil {
		return "", fmt.Errorf("поддерживается только IPv4: %q", s)
	}
	return ipNet.String(), nil
}

// ValidateDomain проверяет корректность доменного имени.
func ValidateDomain(d string) (string, error) {
	d = strings.ToLower(strings.TrimSpace(d))
	if d == "" {
		return "", fmt.Errorf("пустое имя домена")
	}
	if len(d) > 253 {
		return "", fmt.Errorf("доменное имя слишком длинное (> 253 симв.): %q", d)
	}
	if !domainRegex.MatchString(d) {
		return "", fmt.Errorf("недопустимый формат доменного имени: %q", d)
	}
	return d, nil
}

// NormalizeMode проверяет режим split tunneling (bypass или only).
func NormalizeMode(m string) (string, error) {
	m = strings.ToLower(strings.TrimSpace(m))
	if m == "" || m == "bypass" {
		return "bypass", nil
	}
	if m == "only" {
		return "only", nil
	}
	return "", fmt.Errorf("недопустимый режим split_tunnel: %q (допустимы 'bypass' или 'only')", m)
}

// NormalizePortMode проверяет режим разделения портов (only или bypass).
func NormalizePortMode(m string, hasPorts bool) (string, error) {
	m = strings.ToLower(strings.TrimSpace(m))
	if m == "" {
		if hasPorts {
			return "only", nil // по умолчанию для указанных портов — направлять только их в VPN
		}
		return "", nil
	}
	if m == "only" || m == "bypass" {
		return m, nil
	}
	return "", fmt.Errorf("недопустимый режим port_mode: %q (допустимы 'only' или 'bypass')", m)
}

// ValidatePort проверяет и нормализует строку порта или диапазона портов.
// Поддерживает: "80", "80-443", "tcp:80", "udp:53", "udp:27000-27050", "tcp:8000-8080".
func ValidatePort(s string) (PortRule, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return PortRule{}, fmt.Errorf("пустая строка порта")
	}

	proto := "both"
	if strings.HasPrefix(s, "tcp:") {
		proto = "tcp"
		s = strings.TrimPrefix(s, "tcp:")
	} else if strings.HasPrefix(s, "udp:") {
		proto = "udp"
		s = strings.TrimPrefix(s, "udp:")
	}

	s = strings.TrimSpace(s)
	if s == "" {
		return PortRule{}, fmt.Errorf("отсутствует номер порта после префикса протокола")
	}

	if strings.Contains(s, "-") {
		parts := strings.Split(s, "-")
		if len(parts) != 2 {
			return PortRule{}, fmt.Errorf("неверный формат диапазона портов: %q", s)
		}
		start, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil || start < 1 || start > 65535 {
			return PortRule{}, fmt.Errorf("неверный начальный порт в диапазоне %q: должен быть от 1 до 65535", s)
		}
		end, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil || end < 1 || end > 65535 {
			return PortRule{}, fmt.Errorf("неверный конечный порт в диапазоне %q: должен быть от 1 до 65535", s)
		}
		if start >= end {
			return PortRule{}, fmt.Errorf("начальный порт должен быть строго меньше конечного в диапазоне %q", s)
		}
		return PortRule{Proto: proto, Range: fmt.Sprintf("%d-%d", start, end)}, nil
	}

	val, err := strconv.Atoi(s)
	if err != nil || val < 1 || val > 65535 {
		return PortRule{}, fmt.Errorf("неверный номер порта %q: должен быть от 1 до 65535", s)
	}

	return PortRule{Proto: proto, Range: strconv.Itoa(val)}, nil
}

// Validate проверяет и нормализует параметры раздельного туннелирования.
func (s *SplitTunnelConfig) Validate() error {
	mode, err := NormalizeMode(s.Mode)
	if err != nil {
		return err
	}
	s.Mode = mode

	var validCIDRs []string
	for _, c := range s.CIDRs {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		norm, err := ValidateCIDR(c)
		if err != nil {
			return err
		}
		validCIDRs = append(validCIDRs, norm)
	}
	s.CIDRs = validCIDRs

	var validDomains []string
	for _, d := range s.Domains {
		d = strings.TrimSpace(d)
		if d == "" {
			continue
		}
		norm, err := ValidateDomain(d)
		if err != nil {
			return err
		}
		validDomains = append(validDomains, norm)
	}
	s.Domains = validDomains

	// Если RawPorts заполнен (из TOML могут приходить int64 или string)
	if len(s.RawPorts) > 0 {
		var strPorts []string
		for _, rp := range s.RawPorts {
			switch v := rp.(type) {
			case int64:
				strPorts = append(strPorts, strconv.FormatInt(v, 10))
			case int:
				strPorts = append(strPorts, strconv.Itoa(v))
			case string:
				strPorts = append(strPorts, v)
			default:
				strPorts = append(strPorts, fmt.Sprint(v))
			}
		}
		s.Ports = strPorts
	}

	var validPorts []string
	for _, p := range s.Ports {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		rule, err := ValidatePort(p)
		if err != nil {
			return err
		}
		if rule.Proto != "both" {
			validPorts = append(validPorts, rule.Proto+":"+rule.Range)
		} else {
			validPorts = append(validPorts, rule.Range)
		}
	}
	s.Ports = validPorts

	portMode, err := NormalizePortMode(s.PortMode, len(s.Ports) > 0)
	if err != nil {
		return err
	}
	s.PortMode = portMode

	return nil
}

// GetSplitPorts возвращает списки уникальных портов и диапазонов для TCP и UDP.
func (s *SplitTunnelConfig) GetSplitPorts() (tcpPorts []string, udpPorts []string) {
	if s == nil || len(s.Ports) == 0 {
		return nil, nil
	}

	tcpMap := make(map[string]bool)
	udpMap := make(map[string]bool)

	for _, p := range s.Ports {
		rule, err := ValidatePort(p)
		if err != nil {
			continue
		}
		if rule.Proto == "tcp" || rule.Proto == "both" {
			tcpMap[rule.Range] = true
		}
		if rule.Proto == "udp" || rule.Proto == "both" {
			udpMap[rule.Range] = true
		}
	}

	for p := range tcpMap {
		tcpPorts = append(tcpPorts, p)
	}
	for p := range udpMap {
		udpPorts = append(udpPorts, p)
	}

	sortPortElements(tcpPorts)
	sortPortElements(udpPorts)

	return tcpPorts, udpPorts
}

// sortPortElements сортирует элементы портов и диапазонов по возрастанию начального порта.
func sortPortElements(elems []string) {
	sort.Slice(elems, func(i, j int) bool {
		startI := parseStartPort(elems[i])
		startJ := parseStartPort(elems[j])
		if startI != startJ {
			return startI < startJ
		}
		return elems[i] < elems[j]
	})
}

func parseStartPort(s string) int {
	if idx := strings.Index(s, "-"); idx != -1 {
		v, _ := strconv.Atoi(s[:idx])
		return v
	}
	v, _ := strconv.Atoi(s)
	return v
}

// IsRuDirectActive возвращает true, если RuDirect активен (только в режиме bypass).
func (c *Config) IsRuDirectActive() bool {
	if c.SplitTunnel == nil {
		return true
	}
	if c.SplitTunnel.Mode != "bypass" {
		return false
	}
	if c.SplitTunnel.UseRuDirect != nil {
		return *c.SplitTunnel.UseRuDirect
	}
	return true
}

// IsKillSwitchEnabled возвращает true, если Kill Switch активен (по умолчанию true).
func (c *Config) IsKillSwitchEnabled() bool {
	if c.KillSwitch == nil {
		return true // default true
	}
	return *c.KillSwitch
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.TunName == "" {
		cfg.TunName = "sirin0"
	}
	if cfg.TunMTU <= 0 {
		cfg.TunMTU = 1280
	}
	if cfg.APIURL == "" {
		cfg.APIURL = "https://crp.guseynovia.ru"
	}
	if cfg.LanInterface == "" {
		cfg.LanInterface = "br-lan"
	}
	if cfg.DeviceID == "" {
		cfg.DeviceID = "router-01"
	}

	// Обратная совместимость для split_tunnel:
	// Если секция [split_tunnel] отсутствует в конфиге, создаём дефолтную секцию.
	if cfg.SplitTunnel == nil {
		defaultUseRu := true
		if cfg.RuDirect != nil {
			defaultUseRu = *cfg.RuDirect
		}
		cfg.SplitTunnel = &SplitTunnelConfig{
			Mode:        "bypass",
			UseRuDirect: &defaultUseRu,
		}
	} else {
		// Если UseRuDirect не указан явно внутри [split_tunnel], смотрим на top-level ru_direct или default=true
		if cfg.SplitTunnel.UseRuDirect == nil {
			defaultUseRu := true
			if cfg.RuDirect != nil {
				defaultUseRu = *cfg.RuDirect
			}
			cfg.SplitTunnel.UseRuDirect = &defaultUseRu
		}
	}

	if err := cfg.SplitTunnel.Validate(); err != nil {
		return nil, fmt.Errorf("ошибка валидации split_tunnel: %w", err)
	}

	return &cfg, nil
}
