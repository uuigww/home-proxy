package xray

import (
	"encoding/hex"
	"testing"
)

func TestGenerateReality_KeyLengths(t *testing.T) {
	r, err := GenerateReality()
	if err != nil {
		t.Fatalf("GenerateReality: %v", err)
	}
	if got := len(r.PrivateKey); got != 43 {
		t.Errorf("PrivateKey length = %d, want 43 (%q)", got, r.PrivateKey)
	}
	if got := len(r.PublicKey); got != 43 {
		t.Errorf("PublicKey length = %d, want 43 (%q)", got, r.PublicKey)
	}
}

func TestGenerateReality_ShortID(t *testing.T) {
	r, err := GenerateReality()
	if err != nil {
		t.Fatalf("GenerateReality: %v", err)
	}
	if got := len(r.ShortID); got != 8 {
		t.Fatalf("ShortID length = %d, want 8 (%q)", got, r.ShortID)
	}
	if _, err := hex.DecodeString(r.ShortID); err != nil {
		t.Errorf("ShortID %q is not valid hex: %v", r.ShortID, err)
	}
}

func TestGenerateReality_Defaults(t *testing.T) {
	r, err := GenerateReality()
	if err != nil {
		t.Fatalf("GenerateReality: %v", err)
	}
	if r.Dest != "www.google.com:443" {
		t.Errorf("Dest = %q, want www.google.com:443", r.Dest)
	}
	if r.ServerName != "www.google.com" {
		t.Errorf("ServerName = %q, want www.google.com", r.ServerName)
	}
}

func TestGenerateReality_UniquePerCall(t *testing.T) {
	a, err := GenerateReality()
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	b, err := GenerateReality()
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if a.PrivateKey == b.PrivateKey {
		t.Errorf("two calls produced identical PrivateKey %q", a.PrivateKey)
	}
	if a.PublicKey == b.PublicKey {
		t.Errorf("two calls produced identical PublicKey %q", a.PublicKey)
	}
	if a.ShortID == b.ShortID {
		t.Errorf("two calls produced identical ShortID %q", a.ShortID)
	}
}
