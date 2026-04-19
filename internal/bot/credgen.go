package bot

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// newUUID returns a lowercase hex UUID v4 string. The crypto/rand source is
// used so collisions are effectively impossible.
func newUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	hexBuf := make([]byte, 32)
	hex.Encode(hexBuf, b[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		string(hexBuf[0:8]), string(hexBuf[8:12]), string(hexBuf[12:16]),
		string(hexBuf[16:20]), string(hexBuf[20:32]))
}

// newSOCKSPass generates a 16-char hex password for SOCKS5 auth.
func newSOCKSPass() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	out := make([]byte, 16)
	hex.Encode(out, b[:])
	return string(out)
}
