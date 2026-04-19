package xray

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// LoadWgcfProfile reads a wgcf-generated WireGuard profile from disk and
// returns the parsed WarpPeer.
func LoadWgcfProfile(path string) (WarpPeer, error) {
	f, err := os.Open(path)
	if err != nil {
		return WarpPeer{}, fmt.Errorf("open wgcf profile %q: %w", path, err)
	}
	defer f.Close()
	return ParseWgcfProfile(f)
}

// ParseWgcfProfile parses the INI-shaped wgcf profile (Interface + Peer
// sections). Multiple Address / AllowedIPs lines are collapsed; IPv4 and IPv6
// are split by presence of ':'.
func ParseWgcfProfile(r io.Reader) (WarpPeer, error) {
	var (
		peer        WarpPeer
		section     string
		sawPeer     bool
		privateKey  string
		peerPubKey  string
		endpoint    string
		mtu         int
		addresses   []string
	)

	scanner := bufio.NewScanner(r)
	// Tolerate long lines (base64 keys + AllowedIPs lists).
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	lineNo := 0
	for scanner.Scan() {
		lineNo++
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" || strings.HasPrefix(raw, "#") || strings.HasPrefix(raw, ";") {
			continue
		}
		if strings.HasPrefix(raw, "[") && strings.HasSuffix(raw, "]") {
			section = strings.ToLower(strings.TrimSpace(raw[1 : len(raw)-1]))
			if section == "peer" {
				sawPeer = true
			}
			continue
		}

		key, val, ok := splitKV(raw)
		if !ok {
			return WarpPeer{}, fmt.Errorf("line %d: malformed entry %q", lineNo, raw)
		}
		k := strings.ToLower(key)

		switch section {
		case "interface":
			switch k {
			case "privatekey":
				privateKey = val
			case "address":
				addresses = append(addresses, val)
			case "mtu":
				n, err := strconv.Atoi(val)
				if err != nil {
					return WarpPeer{}, fmt.Errorf("line %d: bad MTU %q: %w", lineNo, val, err)
				}
				mtu = n
			case "dns", "listenport", "table", "preup", "postup", "predown", "postdown", "saveconfig", "fwmark":
				// ignored — not needed for the Xray outbound
			}
		case "peer":
			switch k {
			case "publickey":
				peerPubKey = val
			case "endpoint":
				endpoint = val
			case "allowedips":
				// The Xray outbound always routes 0.0.0.0/0 + ::/0, so we drop
				// wgcf's AllowedIPs — record but don't use.
			case "presharedkey", "persistentkeepalive":
				// ignored
			}
		case "":
			return WarpPeer{}, fmt.Errorf("line %d: entry %q outside any section", lineNo, raw)
		}
	}
	if err := scanner.Err(); err != nil {
		return WarpPeer{}, fmt.Errorf("read wgcf profile: %w", err)
	}

	if !sawPeer {
		return WarpPeer{}, fmt.Errorf("missing [Peer] section")
	}
	if privateKey == "" {
		return WarpPeer{}, fmt.Errorf("missing Interface.PrivateKey")
	}
	if peerPubKey == "" {
		return WarpPeer{}, fmt.Errorf("missing Peer.PublicKey")
	}
	if endpoint == "" {
		return WarpPeer{}, fmt.Errorf("missing Peer.Endpoint")
	}

	peer.PrivateKey = privateKey
	peer.PeerPublicKey = peerPubKey
	peer.Endpoint = endpoint
	peer.MTU = mtu
	for _, a := range addresses {
		if strings.Contains(a, ":") {
			if peer.IPv6 == "" {
				peer.IPv6 = a
			}
		} else {
			if peer.IPv4 == "" {
				peer.IPv4 = a
			}
		}
	}
	return peer, nil
}

// splitKV splits "Key = Value" (with optional inline comment) into (key, value).
func splitKV(line string) (string, string, bool) {
	// Strip inline comments after '#' (not legal inside base64, but guard anyway).
	if i := strings.Index(line, "#"); i >= 0 {
		line = strings.TrimSpace(line[:i])
	}
	eq := strings.IndexByte(line, '=')
	if eq <= 0 {
		return "", "", false
	}
	// The value itself may contain '=' (base64 padding). Split on the FIRST '='.
	k := strings.TrimSpace(line[:eq])
	v := strings.TrimSpace(line[eq+1:])
	if k == "" {
		return "", "", false
	}
	return k, v, true
}
