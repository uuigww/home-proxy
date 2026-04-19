package xray

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Config is the root Xray config.json document we render.
//
// Only the subset we actually produce is modelled — no VMess, no mKCP, no
// generic TLS transport. Fields are exported so callers can inspect or tweak a
// generated config before marshalling.
type Config struct {
	Log       LogConfig   `json:"log"`
	API       APIConfig   `json:"api"`
	Inbounds  []Inbound   `json:"inbounds"`
	Outbounds []Outbound  `json:"outbounds"`
	Routing   Routing     `json:"routing"`
	Stats     StatsConfig `json:"stats"`
	Policy    Policy      `json:"policy"`
}

// LogConfig is the top-level "log" section.
type LogConfig struct {
	LogLevel string `json:"loglevel"`
}

// APIConfig is the top-level "api" section; the matching inbound uses Tag.
type APIConfig struct {
	Tag      string   `json:"tag"`
	Services []string `json:"services"`
}

// StatsConfig is the top-level "stats" section — Xray requires an empty object
// to enable the stats subsystem.
type StatsConfig struct{}

// Policy configures per-user and system-wide stats collection.
type Policy struct {
	Levels map[string]PolicyLevel `json:"levels"`
	System PolicySystem           `json:"system"`
}

// PolicyLevel is a single entry in policy.levels (keyed by level number).
type PolicyLevel struct {
	StatsUserUplink   bool `json:"statsUserUplink"`
	StatsUserDownlink bool `json:"statsUserDownlink"`
}

// PolicySystem toggles system-wide traffic counters.
type PolicySystem struct {
	StatsInboundUplink   bool `json:"statsInboundUplink"`
	StatsInboundDownlink bool `json:"statsInboundDownlink"`
}

// Inbound is one entry in the Xray inbounds array.
//
// Protocol-specific fields live in Settings (a raw JSON blob) and StreamSettings
// so one Go type covers dokodemo-door, VLESS+Reality and SOCKS5.
type Inbound struct {
	Tag            string          `json:"tag"`
	Listen         string          `json:"listen,omitempty"`
	Port           int             `json:"port"`
	Protocol       string          `json:"protocol"`
	Settings       json.RawMessage `json:"settings"`
	StreamSettings *StreamSettings `json:"streamSettings,omitempty"`
	Sniffing       *Sniffing       `json:"sniffing,omitempty"`
}

// StreamSettings holds transport + security for an inbound/outbound.
type StreamSettings struct {
	Network         string           `json:"network"`
	Security        string           `json:"security,omitempty"`
	RealitySettings *RealitySettings `json:"realitySettings,omitempty"`
}

// RealitySettings is the realitySettings sub-object of streamSettings.
type RealitySettings struct {
	Show         bool     `json:"show"`
	Dest         string   `json:"dest"`
	Xver         int      `json:"xver"`
	ServerNames  []string `json:"serverNames"`
	PrivateKey   string   `json:"privateKey"`
	MinClientVer string   `json:"minClientVer,omitempty"`
	MaxClientVer string   `json:"maxClientVer,omitempty"`
	MaxTimeDiff  int      `json:"maxTimeDiff,omitempty"`
	ShortIds     []string `json:"shortIds"`
}

// Sniffing enables destination sniffing for routing.
type Sniffing struct {
	Enabled      bool     `json:"enabled"`
	DestOverride []string `json:"destOverride"`
}

// Outbound is one entry in the Xray outbounds array.
type Outbound struct {
	Tag            string          `json:"tag"`
	Protocol       string          `json:"protocol"`
	Settings       json.RawMessage `json:"settings,omitempty"`
	StreamSettings *StreamSettings `json:"streamSettings,omitempty"`
}

// Routing holds domainStrategy and the ordered list of rules.
type Routing struct {
	DomainStrategy string        `json:"domainStrategy"`
	Rules          []RoutingRule `json:"rules"`
}

// RoutingRule is a single Xray routing rule. Only the fields we actually emit
// are modelled; omitempty keeps unused slots out of config.json.
type RoutingRule struct {
	Type        string   `json:"type"`
	Domain      []string `json:"domain,omitempty"`
	IP          []string `json:"ip,omitempty"`
	Port        string   `json:"port,omitempty"`
	Network     string   `json:"network,omitempty"`
	InboundTag  []string `json:"inboundTag,omitempty"`
	OutboundTag string   `json:"outboundTag"`
}

// User is the input row our generator consumes; it maps onto a row in the
// SQLite users table.
type User struct {
	Name       string `json:"name"`
	VLESSUUID  string `json:"vless_uuid"`
	SOCKSUser  string `json:"socks_user"`
	SOCKSPass  string `json:"socks_pass"`
	LimitBytes int64  `json:"limit_bytes"`
	Enabled    bool   `json:"enabled"`
}

// Reality holds everything needed to render the realitySettings block for the
// VLESS+Reality inbound.
type Reality struct {
	Dest       string `json:"dest"`
	ServerName string `json:"server_name"`
	PrivateKey string `json:"private_key"`
	PublicKey  string `json:"public_key"`
	ShortID    string `json:"short_id"`
}

// WarpPeer holds the Cloudflare Warp WireGuard profile we pass into the
// wireguard outbound.
type WarpPeer struct {
	PrivateKey    string `json:"private_key"`
	PeerPublicKey string `json:"peer_public_key"`
	IPv4          string `json:"ipv4"`
	IPv6          string `json:"ipv6"`
	Endpoint      string `json:"endpoint"`
	MTU           int    `json:"mtu"`
	Reserved      []byte `json:"reserved,omitempty"`
}

// GenInput is the bundle handed to Generate.
type GenInput struct {
	Users        []User   `json:"users"`
	Reality      Reality  `json:"reality"`
	Warp         WarpPeer `json:"warp"`
	SOCKSPort    int      `json:"socks_port"`
	RealityPort  int      `json:"reality_port"`
	API          string   `json:"api"` // "127.0.0.1:10085"
}

// Tag constants used across inbounds/outbounds and routing rules.
const (
	TagAPI     = "api"
	TagVLESSIn = "vless-in"
	TagSOCKSIn = "socks-in"
	TagAPIIn   = "api-in"

	TagDirect = "direct"
	TagWarp   = "warp"
	TagBlock  = "block"
)

// warpRouteDomains lists domains that must egress through the Warp outbound so
// Google refuses to captcha our VPS IP. Keep alphabetised, geosite first.
var warpRouteDomains = []string{
	"geosite:google",
	"geosite:google-ads",
	"geosite:google-play",
	"geosite:google-scholar",
	"geosite:youtube",
	"domain:aistudio.google.com",
	"domain:generativelanguage.googleapis.com",
	"domain:labs.google",
	"domain:notebooklm.google.com",
}

// Generate builds a full Config from the supplied users, Reality keys and Warp
// peer. Returns an error if required fields are missing.
func Generate(in GenInput) (Config, error) {
	if in.RealityPort <= 0 {
		return Config{}, fmt.Errorf("reality_port must be > 0")
	}
	if in.SOCKSPort <= 0 {
		return Config{}, fmt.Errorf("socks_port must be > 0")
	}
	if strings.TrimSpace(in.API) == "" {
		return Config{}, fmt.Errorf("api address is required")
	}
	if in.Reality.PrivateKey == "" || in.Reality.ShortID == "" {
		return Config{}, fmt.Errorf("reality private_key and short_id are required")
	}

	apiHost, apiPort, err := splitHostPort(in.API)
	if err != nil {
		return Config{}, fmt.Errorf("parse api addr: %w", err)
	}

	inbounds := []Inbound{}

	apiIn, err := buildAPIInbound(apiHost, apiPort)
	if err != nil {
		return Config{}, err
	}
	inbounds = append(inbounds, apiIn)

	vlessIn, err := buildVLESSInbound(in)
	if err != nil {
		return Config{}, err
	}
	inbounds = append(inbounds, vlessIn)

	socksIn, err := buildSOCKSInbound(in)
	if err != nil {
		return Config{}, err
	}
	inbounds = append(inbounds, socksIn)

	outbounds, err := buildOutbounds(in.Warp)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Log:       LogConfig{LogLevel: "warning"},
		API:       APIConfig{Tag: TagAPI, Services: []string{"HandlerService", "StatsService"}},
		Inbounds:  inbounds,
		Outbounds: outbounds,
		Routing:   buildRouting(),
		Stats:     StatsConfig{},
		Policy: Policy{
			Levels: map[string]PolicyLevel{
				"0": {StatsUserUplink: true, StatsUserDownlink: true},
			},
			System: PolicySystem{
				StatsInboundUplink:   true,
				StatsInboundDownlink: true,
			},
		},
	}
	return cfg, nil
}
