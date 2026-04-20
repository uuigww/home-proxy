package config

import (
	"path/filepath"
	"testing"
)

func writeTempConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "config.toml")
	if err := writeFile(p, body); err != nil {
		t.Fatal(err)
	}
	return p
}

func writeFile(path, body string) error {
	return writeBytes(path, []byte(body))
}

func TestLoad_Minimal(t *testing.T) {
	path := writeTempConfig(t, `
bot_token = "123:ABC"
admins = [111, 222]
default_lang = "ru"
`)
	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.BotToken != "123:ABC" {
		t.Errorf("BotToken = %q", c.BotToken)
	}
	if got, want := len(c.Admins), 2; got != want {
		t.Errorf("len(Admins) = %d, want %d", got, want)
	}
	// Defaults must be applied.
	if c.DataDir != "/var/lib/home-proxy" {
		t.Errorf("DataDir default not applied: %q", c.DataDir)
	}
	if c.RealityPort != 443 {
		t.Errorf("RealityPort default = %d", c.RealityPort)
	}
}

func TestValidate_Errors(t *testing.T) {
	cases := map[string]Config{
		"missing token":  {Admins: []int64{1}, DefaultLang: "ru"},
		"missing admins": {BotToken: "x", DefaultLang: "ru"},
		"bad lang":       {BotToken: "x", Admins: []int64{1}, DefaultLang: "fr"},
		"mtproto bad port": {
			BotToken: "x", Admins: []int64{1}, DefaultLang: "ru",
			MTProtoEnabled: true, MTProtoPort: 0, MTProtoFakeTLSHost: "www.google.com",
		},
		"mtproto port too high": {
			BotToken: "x", Admins: []int64{1}, DefaultLang: "ru",
			MTProtoEnabled: true, MTProtoPort: 70000, MTProtoFakeTLSHost: "www.google.com",
		},
		"mtproto missing host": {
			BotToken: "x", Admins: []int64{1}, DefaultLang: "ru",
			MTProtoEnabled: true, MTProtoPort: 8443, MTProtoFakeTLSHost: "   ",
		},
	}
	for name, c := range cases {
		if err := c.Validate(); err == nil {
			t.Errorf("%s: expected error, got nil", name)
		}
	}
}

func TestValidate_MTProtoOK(t *testing.T) {
	c := Config{
		BotToken: "x", Admins: []int64{1}, DefaultLang: "ru",
		MTProtoEnabled: true, MTProtoPort: 8443, MTProtoFakeTLSHost: "www.google.com",
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
}

func TestDefaults_MTProto(t *testing.T) {
	d := Defaults()
	if d.MTProtoEnabled {
		t.Error("MTProtoEnabled should default to false")
	}
	if d.MTProtoPort != 8443 {
		t.Errorf("MTProtoPort default = %d, want 8443", d.MTProtoPort)
	}
	if d.MTProtoFakeTLSHost != "www.google.com" {
		t.Errorf("MTProtoFakeTLSHost default = %q", d.MTProtoFakeTLSHost)
	}
}

func TestLoad_MTProtoEnabled(t *testing.T) {
	path := writeTempConfig(t, `
bot_token = "123:ABC"
admins = [111]
default_lang = "ru"
mtproto_enabled = true
mtproto_port = 9443
mtproto_fake_tls_host = "www.cloudflare.com"
`)
	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !c.MTProtoEnabled {
		t.Error("MTProtoEnabled not loaded from TOML")
	}
	if c.MTProtoPort != 9443 {
		t.Errorf("MTProtoPort = %d", c.MTProtoPort)
	}
	if c.MTProtoFakeTLSHost != "www.cloudflare.com" {
		t.Errorf("MTProtoFakeTLSHost = %q", c.MTProtoFakeTLSHost)
	}
}
