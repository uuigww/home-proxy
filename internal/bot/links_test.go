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

func TestBuildMTProtoLink(t *testing.T) {
	got := BuildMTProtoLink("www.google.com", 8443, "eedeadbeef")
	want := "tg://proxy?server=www.google.com&port=8443&secret=eedeadbeef"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	share := BuildMTProtoShareLink("www.google.com", 8443, "eedeadbeef")
	if !strings.HasPrefix(share, "https://t.me/proxy?") {
		t.Fatalf("unexpected share prefix: %q", share)
	}
	for _, want := range []string{"server=www.google.com", "port=8443", "secret=eedeadbeef"} {
		if !strings.Contains(share, want) {
			t.Errorf("share missing %q in %s", want, share)
		}
	}
}

func TestBuildMTProtoLinkEmpty(t *testing.T) {
	if got := BuildMTProtoLink("", 1, "s"); got != "" {
		t.Errorf("expected empty for empty host, got %q", got)
	}
	if got := BuildMTProtoLink("h", 1, ""); got != "" {
		t.Errorf("expected empty for empty secret, got %q", got)
	}
	if got := BuildMTProtoShareLink("", 1, "s"); got != "" {
		t.Errorf("share: expected empty for empty host, got %q", got)
	}
	if got := BuildMTProtoShareLink("h", 1, ""); got != "" {
		t.Errorf("share: expected empty for empty secret, got %q", got)
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
