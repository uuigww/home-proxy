package xray

import (
	"strings"
	"testing"
)

const sampleWgcfProfile = `[Interface]
PrivateKey = YOUR_PRIVATE_KEY_BASE64=
Address = 172.16.0.2/32
Address = 2606:4700:110:8a36:df92:102a:9602:fa18/128
DNS = 1.1.1.1
MTU = 1280

[Peer]
PublicKey = bmXOC+F1FxEMF9dyiK2H5/1SUtzH0JuVo51h2wPfgyo=
AllowedIPs = 0.0.0.0/0
AllowedIPs = ::/0
Endpoint = engage.cloudflareclient.com:2408
`

func TestParseWgcfProfile_Happy(t *testing.T) {
	peer, err := ParseWgcfProfile(strings.NewReader(sampleWgcfProfile))
	if err != nil {
		t.Fatalf("ParseWgcfProfile: %v", err)
	}
	if peer.PrivateKey != "YOUR_PRIVATE_KEY_BASE64=" {
		t.Errorf("PrivateKey = %q", peer.PrivateKey)
	}
	if peer.PeerPublicKey != "bmXOC+F1FxEMF9dyiK2H5/1SUtzH0JuVo51h2wPfgyo=" {
		t.Errorf("PeerPublicKey = %q", peer.PeerPublicKey)
	}
	if peer.IPv4 != "172.16.0.2/32" {
		t.Errorf("IPv4 = %q", peer.IPv4)
	}
	if peer.IPv6 != "2606:4700:110:8a36:df92:102a:9602:fa18/128" {
		t.Errorf("IPv6 = %q", peer.IPv6)
	}
	if peer.Endpoint != "engage.cloudflareclient.com:2408" {
		t.Errorf("Endpoint = %q", peer.Endpoint)
	}
	if peer.MTU != 1280 {
		t.Errorf("MTU = %d", peer.MTU)
	}
}

func TestParseWgcfProfile_MissingPeer(t *testing.T) {
	body := `[Interface]
PrivateKey = abc=
Address = 172.16.0.2/32
`
	_, err := ParseWgcfProfile(strings.NewReader(body))
	if err == nil {
		t.Fatal("expected error for missing [Peer], got nil")
	}
	if !strings.Contains(err.Error(), "[Peer]") {
		t.Errorf("error should mention [Peer], got %q", err)
	}
}

func TestParseWgcfProfile_Malformed(t *testing.T) {
	body := `[Interface]
this is not an ini entry
`
	_, err := ParseWgcfProfile(strings.NewReader(body))
	if err == nil {
		t.Fatal("expected error for malformed input, got nil")
	}
}

func TestParseWgcfProfile_MissingRequired(t *testing.T) {
	// Has [Peer] but no PublicKey.
	body := `[Interface]
PrivateKey = abc=
Address = 172.16.0.2/32

[Peer]
Endpoint = engage.cloudflareclient.com:2408
`
	_, err := ParseWgcfProfile(strings.NewReader(body))
	if err == nil {
		t.Fatal("expected error for missing Peer.PublicKey, got nil")
	}
}

func TestParseWgcfProfile_EntryBeforeSection(t *testing.T) {
	body := "PrivateKey = abc=\n[Interface]\n"
	_, err := ParseWgcfProfile(strings.NewReader(body))
	if err == nil {
		t.Fatal("expected error for entry outside section, got nil")
	}
}
