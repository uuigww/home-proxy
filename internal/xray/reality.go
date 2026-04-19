package xray

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// GenerateReality creates a fresh Reality keypair and short ID.
//
// The X25519 private key is generated via crypto/ecdh (stdlib), encoded with
// base64.RawURLEncoding to match Xray's `xray x25519` output. The short ID is
// 8 random hex characters. Dest and ServerName default to www.google.com:443 /
// www.google.com — the installer can override them later.
func GenerateReality() (Reality, error) {
	curve := ecdh.X25519()
	priv, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return Reality{}, fmt.Errorf("generate x25519 key: %w", err)
	}
	pub := priv.PublicKey()

	privB := priv.Bytes()
	pubB := pub.Bytes()
	if len(privB) != 32 || len(pubB) != 32 {
		return Reality{}, fmt.Errorf("unexpected x25519 key length: priv=%d pub=%d", len(privB), len(pubB))
	}

	sid, err := randomShortID(4) // 4 bytes → 8 hex chars
	if err != nil {
		return Reality{}, err
	}

	return Reality{
		Dest:       "www.google.com:443",
		ServerName: "www.google.com",
		PrivateKey: base64.RawURLEncoding.EncodeToString(privB),
		PublicKey:  base64.RawURLEncoding.EncodeToString(pubB),
		ShortID:    sid,
	}, nil
}

// randomShortID returns a random hex string of length 2*n.
func randomShortID(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
}
