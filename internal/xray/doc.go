// Package xray generates and inspects configuration for an embedded Xray-core
// instance.
//
// Responsibilities:
//   - model the subset of Xray's config.json we actually emit (VLESS+Reality
//     inbound, SOCKS5 inbound, freedom/wireguard/blackhole outbounds, routing,
//     stats/policy, gRPC API);
//   - generate Reality X25519 keypairs and short IDs (reality.go);
//   - parse wgcf-generated WireGuard profiles for the Cloudflare Warp outbound
//     (warp.go).
//
// Nothing in this package talks to the network or spawns processes — it is a
// pure data layer. The daemon (internal/bot, cmd/home-proxy/serve) combines
// these helpers with the SQLite store to render /usr/local/etc/xray/config.json
// and hot-reload users via the Xray gRPC HandlerService.
package xray
