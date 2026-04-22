// Package config loads the daemon configuration from /etc/home-proxy/config.toml.
package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config is the on-disk daemon configuration.
//
// Only bot_token, admins and default_lang are required; the rest have sane
// defaults so a minimal config.toml fits on a postcard.
type Config struct {
	BotToken    string  `toml:"bot_token"`
	Admins      []int64 `toml:"admins"`
	DefaultLang string  `toml:"default_lang"`

	DataDir           string `toml:"data_dir"`
	XrayAPI           string `toml:"xray_api"`
	XrayConfig        string `toml:"xray_config"`
	XrayVLESSTag      string `toml:"xray_vless_tag"`
	XraySocksTag      string `toml:"xray_socks_tag"`
	ServerHost        string `toml:"server_host"`
	RealityDest       string `toml:"reality_dest"`
	RealityServerName string `toml:"reality_server_name"`
	SOCKSPort         int    `toml:"socks_port"`
	RealityPort       int    `toml:"reality_port"`

	// MTProto (optional, opt-in at install time).
	//
	// When MTProtoEnabled is true, the daemon assumes a companion mtg
	// (9seconds/mtg) systemd unit is managing /etc/home-proxy/mtg.toml on
	// MTProtoPort with the Fake-TLS host MTProtoFakeTLSHost. The bot exposes
	// the tg://proxy link in the Links and Server screens.
	MTProtoEnabled     bool   `toml:"mtproto_enabled"`
	MTProtoPort        int    `toml:"mtproto_port"`
	MTProtoFakeTLSHost string `toml:"mtproto_fake_tls_host"`
}

// Defaults returns a Config pre-filled with production defaults.
func Defaults() Config {
	return Config{
		DefaultLang:        "ru",
		DataDir:            "/var/lib/home-proxy",
		XrayAPI:            "127.0.0.1:10085",
		XrayConfig:         "/usr/local/etc/xray/config.json",
		RealityDest:        "www.google.com:443",
		RealityServerName:  "www.google.com",
		SOCKSPort:          1080,
		RealityPort:        443,
		MTProtoEnabled:     false,
		MTProtoPort:        8443,
		MTProtoFakeTLSHost: "www.google.com",
	}
}

// Load reads and validates a config file from disk.
func Load(path string) (Config, error) {
	c := Defaults()
	data, err := os.ReadFile(path)
	if err != nil {
		return c, fmt.Errorf("read config %q: %w", path, err)
	}
	if _, err := toml.Decode(string(data), &c); err != nil {
		return c, fmt.Errorf("parse config %q: %w", path, err)
	}
	if err := c.Validate(); err != nil {
		return c, err
	}
	return c, nil
}

// Validate checks that required fields are present and well-formed.
func (c Config) Validate() error {
	if strings.TrimSpace(c.BotToken) == "" {
		return fmt.Errorf("bot_token is required")
	}
	if len(c.Admins) == 0 {
		return fmt.Errorf("at least one admin Telegram ID is required")
	}
	switch c.DefaultLang {
	case "ru", "en":
	default:
		return fmt.Errorf("default_lang must be 'ru' or 'en', got %q", c.DefaultLang)
	}
	if c.MTProtoEnabled {
		if c.MTProtoPort < 1 || c.MTProtoPort > 65535 {
			return fmt.Errorf("mtproto_port must be 1..65535, got %d", c.MTProtoPort)
		}
		if strings.TrimSpace(c.MTProtoFakeTLSHost) == "" {
			return fmt.Errorf("mtproto_fake_tls_host is required when mtproto_enabled=true")
		}
	}
	return nil
}
