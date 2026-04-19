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
	}
	for name, c := range cases {
		if err := c.Validate(); err == nil {
			t.Errorf("%s: expected error, got nil", name)
		}
	}
}
