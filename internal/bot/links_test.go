package bot

import (
	"strings"
	"testing"

	"github.com/uuigww/home-proxy/internal/store"
)

func TestBuildVLESSLink(t *testing.T) {
	uuid := "11111111-2222-3333-4444-555555555555"
	u := store.User{Name: "alice", VLESSUUID: &uuid}
	r := store.RealityKeys{
		PublicKey:  "PUBKEY",
		ShortID:    "abcd",
		ServerName: "www.google.com",
	}
	link := BuildVLESSLink(u, r, "vpn.example.com", 443)
	if !strings.HasPrefix(link, "vless://"+uuid+"@vpn.example.com:443?") {
		t.Fatalf("unexpected prefix: %q", link)
	}
	for _, want := range []string{
		"security=reality",
		"sni=www.google.com",
		"pbk=PUBKEY",
		"sid=abcd",
		"flow=xtls-rprx-vision",
		"fp=chrome",
		"type=tcp",
	} {
		if !strings.Contains(link, want) {
			t.Errorf("missing %q in %s", want, link)
		}
	}
	if !strings.HasSuffix(link, "#alice") {
		t.Errorf("missing fragment: %s", link)
	}
}

func TestBuildVLESSLinkEmpty(t *testing.T) {
	u := store.User{Name: "bob"}
	if got := BuildVLESSLink(u, store.RealityKeys{}, "h", 1); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestBuildSOCKSLink(t *testing.T) {
	user, pass := "alice", "p@ss/word"
	u := store.User{SOCKSUser: &user, SOCKSPass: &pass}
	got := BuildSOCKSLink(u, "vpn.example.com", 1080)
	if !strings.HasPrefix(got, "socks5://") {
		t.Fatalf("bad prefix: %s", got)
	}
	if !strings.Contains(got, "vpn.example.com:1080") {
		t.Errorf("missing host:port: %s", got)
	}
	// Password special chars should be escaped.
	if strings.Contains(got, "p@ss/word") {
		t.Errorf("password not escaped: %s", got)
	}
}

func TestBuildSOCKSLinkEmpty(t *testing.T) {
	if got := BuildSOCKSLink(store.User{}, "h", 1); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestBuildQRPlaceholder(t *testing.T) {
	got, err := BuildQR("vless://abc")
	if err != nil {
		t.Fatalf("BuildQR: %v", err)
	}
	if string(got) != "vless://abc" {
		t.Fatalf("expected placeholder equal to input, got %q", string(got))
	}
	if _, err := BuildQR(""); err == nil {
		t.Fatalf("expected error for empty input")
	}
}
