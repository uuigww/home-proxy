package xray

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// splitHostPort parses "host:port" into its parts, returning a typed port.
func splitHostPort(addr string) (string, int, error) {
	h, p, err := net.SplitHostPort(addr)
	if err != nil {
		return "", 0, err
	}
	port, err := strconv.Atoi(p)
	if err != nil {
		return "", 0, fmt.Errorf("port %q: %w", p, err)
	}
	if port <= 0 {
		return "", 0, fmt.Errorf("port must be > 0, got %d", port)
	}
	return h, port, nil
}

// buildAPIInbound renders the dokodemo-door inbound that exposes Xray's gRPC
// API to the daemon over loopback.
func buildAPIInbound(host string, port int) (Inbound, error) {
	settings, err := json.Marshal(map[string]any{
		"address": "127.0.0.1",
	})
	if err != nil {
		return Inbound{}, err
	}
	return Inbound{
		Tag:      TagAPIIn,
		Listen:   host,
		Port:     port,
		Protocol: "dokodemo-door",
		Settings: settings,
	}, nil
}

// vlessClient is one element of settings.clients for a VLESS inbound.
type vlessClient struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Flow  string `json:"flow,omitempty"`
	Level int    `json:"level"`
}

// buildVLESSInbound renders the VLESS+Reality inbound. Only enabled users with
// a non-empty VLESSUUID are added as clients.
func buildVLESSInbound(in GenInput) (Inbound, error) {
	clients := []vlessClient{}
	for _, u := range in.Users {
		if !u.Enabled || strings.TrimSpace(u.VLESSUUID) == "" {
			continue
		}
		clients = append(clients, vlessClient{
			ID:    u.VLESSUUID,
			Email: u.Name,
			Flow:  "xtls-rprx-vision",
			Level: 0,
		})
	}
	settings, err := json.Marshal(map[string]any{
		"clients":    clients,
		"decryption": "none",
	})
	if err != nil {
		return Inbound{}, err
	}

	dest := in.Reality.Dest
	if dest == "" {
		dest = "www.google.com:443"
	}
	serverName := in.Reality.ServerName
	if serverName == "" {
		serverName = "www.google.com"
	}

	return Inbound{
		Tag:      TagVLESSIn,
		Port:     in.RealityPort,
		Protocol: "vless",
		Settings: settings,
		StreamSettings: &StreamSettings{
			Network:  "tcp",
			Security: "reality",
			RealitySettings: &RealitySettings{
				Show:        false,
				Dest:        dest,
				Xver:        0,
				ServerNames: []string{serverName},
				PrivateKey:  in.Reality.PrivateKey,
				ShortIds:    []string{in.Reality.ShortID},
			},
		},
		Sniffing: &Sniffing{
			Enabled:      true,
			DestOverride: []string{"http", "tls", "quic"},
		},
	}, nil
}

// socksAccount is one element of settings.accounts for a SOCKS inbound.
type socksAccount struct {
	User string `json:"user"`
	Pass string `json:"pass"`
}

// buildSOCKSInbound renders the authenticated SOCKS5 inbound.
func buildSOCKSInbound(in GenInput) (Inbound, error) {
	accounts := []socksAccount{}
	for _, u := range in.Users {
		if !u.Enabled {
			continue
		}
		if strings.TrimSpace(u.SOCKSUser) == "" || strings.TrimSpace(u.SOCKSPass) == "" {
			continue
		}
		accounts = append(accounts, socksAccount{User: u.SOCKSUser, Pass: u.SOCKSPass})
	}
	settings, err := json.Marshal(map[string]any{
		"auth":     "password",
		"accounts": accounts,
		"udp":      true,
	})
	if err != nil {
		return Inbound{}, err
	}
	return Inbound{
		Tag:      TagSOCKSIn,
		Port:     in.SOCKSPort,
		Protocol: "socks",
		Settings: settings,
		Sniffing: &Sniffing{
			Enabled:      true,
			DestOverride: []string{"http", "tls", "quic"},
		},
	}, nil
}

// wireguardPeer mirrors Xray's wireguard outbound peer object.
type wireguardPeer struct {
	PublicKey  string   `json:"publicKey"`
	Endpoint   string   `json:"endpoint"`
	AllowedIPs []string `json:"allowedIPs,omitempty"`
	KeepAlive  int      `json:"keepAlive,omitempty"`
}

// buildOutbounds renders the ordered outbound list: direct first (default),
// warp for Google traffic, and blackhole for the block rule.
func buildOutbounds(w WarpPeer, hasWarp bool) ([]Outbound, error) {
	directSettings, err := json.Marshal(map[string]any{
		"domainStrategy": "UseIP",
	})
	if err != nil {
		return nil, err
	}

	blockSettings, err := json.Marshal(map[string]any{
		"response": map[string]any{"type": "none"},
	})
	if err != nil {
		return nil, err
	}

	out := []Outbound{
		{Tag: TagDirect, Protocol: "freedom", Settings: directSettings},
	}

	if hasWarp {
		addresses := []string{}
		if strings.TrimSpace(w.IPv4) != "" {
			addresses = append(addresses, w.IPv4)
		}
		if strings.TrimSpace(w.IPv6) != "" {
			addresses = append(addresses, w.IPv6)
		}
		warpPayload := map[string]any{
			"secretKey": w.PrivateKey,
			"address":   addresses,
			"peers": []wireguardPeer{
				{
					PublicKey:  w.PeerPublicKey,
					Endpoint:   w.Endpoint,
					AllowedIPs: []string{"0.0.0.0/0", "::/0"},
					KeepAlive:  25,
				},
			},
			"mtu": warpMTU(w.MTU),
		}
		if len(w.Reserved) > 0 {
			warpPayload["reserved"] = w.Reserved
		}
		warpSettings, err := json.Marshal(warpPayload)
		if err != nil {
			return nil, err
		}
		out = append(out, Outbound{Tag: TagWarp, Protocol: "wireguard", Settings: warpSettings})
	}

	out = append(out, Outbound{Tag: TagBlock, Protocol: "blackhole", Settings: blockSettings})
	return out, nil
}

// warpMTU returns 1280 if mtu is unset; Cloudflare's wgcf default.
func warpMTU(mtu int) int {
	if mtu <= 0 {
		return 1280
	}
	return mtu
}

// buildRouting returns the routing block: api traffic first, Google/AI domains
// to warp, everything else falls through to direct via the last rule.
func buildRouting(hasWarp bool) Routing {
	rules := []RoutingRule{
		{
			Type:        "field",
			InboundTag:  []string{TagAPIIn},
			OutboundTag: TagAPI,
		},
	}
	if hasWarp {
		rules = append(rules, RoutingRule{
			Type:        "field",
			Domain:      append([]string(nil), warpRouteDomains...),
			OutboundTag: TagWarp,
		})
	}
	rules = append(rules, RoutingRule{
		Type:        "field",
		Network:     "tcp,udp",
		OutboundTag: TagDirect,
	})
	return Routing{
		DomainStrategy: "IPIfNonMatch",
		Rules:          rules,
	}
}
