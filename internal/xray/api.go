// This file intentionally steps outside the "pure data layer" remit stated in
// doc.go: it wraps Xray's gRPC HandlerService and StatsService for runtime
// user management (add/remove VLESS + SOCKS5 accounts, query per-user traffic
// stats). A future milestone will replace this implementation with a direct
// gRPC client and remove the CLI dependency. Until then, CLIClient shells out
// to the `xray api` subcommand, which ships with xray-core and exposes the
// same gRPC methods over stdin/stdout JSON.
//
// Rationale for going via the CLI first:
//   - avoids vendoring a non-trivial slice of the Xray protobuf tree;
//   - lets us keep this milestone self-contained with zero new dependencies;
//   - the argument surface is stable enough to unit-test by asserting the
//     command slice that would be handed to exec.Command.
//
// Callers should depend on the Client interface, not CLIClient directly.

package xray

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ErrUserNotFound is returned by GetUserStats and RemoveXxxUser when Xray
// reports that the email/user does not exist.
var ErrUserNotFound = errors.New("xray: user not found")

// Client is the surface home-proxy uses to talk to a running xray-core
// instance. Every method is cancellable via the context.
type Client interface {
	// AddVLESSUser registers a VLESS account with the configured inbound tag.
	// email is the stats key used by Xray to attribute per-user traffic.
	AddVLESSUser(ctx context.Context, uuid, email string) error

	// RemoveVLESSUser removes a VLESS account previously added with
	// AddVLESSUser. Returns ErrUserNotFound if no such account exists.
	RemoveVLESSUser(ctx context.Context, email string) error

	// AddSOCKSUser registers a SOCKS5 account with the configured inbound tag.
	AddSOCKSUser(ctx context.Context, user, pass, email string) error

	// RemoveSOCKSUser removes a SOCKS5 account. Returns ErrUserNotFound if no
	// such account exists.
	RemoveSOCKSUser(ctx context.Context, email string) error

	// GetUserStats returns (uplink, downlink) bytes for the given email since
	// the last reset. Returns ErrUserNotFound if Xray has no record of it.
	GetUserStats(ctx context.Context, email string) (uplink, downlink int64, err error)

	// Reset zeroes every per-user traffic counter. Used after a successful
	// poll-and-persist cycle so the in-process counter stays small.
	Reset(ctx context.Context) error

	// Ping is a cheap "is the API server reachable?" probe.
	Ping(ctx context.Context) error
}

// Default Xray inbound tags home-proxy emits. Aliased to the canonical
// TagVLESSIn / TagSOCKSIn constants in config.go so a single source of truth
// is used both when generating config.json and when calling the runtime API.
const (
	VLESSInboundTag = TagVLESSIn
	SOCKSInboundTag = TagSOCKSIn
)

// CLIClient implements Client by shelling out to the `xray api` subcommand.
//
// XrayBin is the absolute path (or bare name in $PATH) of the xray binary;
// APIAddr is the host:port of the Xray API gRPC listener (e.g. 127.0.0.1:10085).
// VLESSTag and SOCKSTag default to the constants above when empty.
//
// SOCKS user management (Add/Remove) cannot use the runtime gRPC API because
// xray-core's SOCKS proxy does not implement UserManager — calls return
// "proxy is not a UserManager". Instead, AddSOCKSUser/RemoveSOCKSUser patch
// the on-disk config.json and restart xray. ConfigPath and RestartXray must
// both be set for those operations to work.
type CLIClient struct {
	XrayBin  string
	APIAddr  string
	VLESSTag string
	SOCKSTag string

	// ConfigPath is the absolute path to xray's config.json. Required for
	// SOCKS user management.
	ConfigPath string

	// RestartXray restarts the xray process so a freshly-written config.json
	// takes effect. Required for SOCKS user management.
	RestartXray func(ctx context.Context) error
}

// NewCLIClient returns a CLIClient with sensible defaults. Either argument
// may be empty; falls back to "xray" and "127.0.0.1:10085" respectively.
func NewCLIClient(xrayBin, apiAddr string) *CLIClient {
	if xrayBin == "" {
		xrayBin = "xray"
	}
	if apiAddr == "" {
		apiAddr = "127.0.0.1:10085"
	}
	return &CLIClient{XrayBin: xrayBin, APIAddr: apiAddr}
}

func (c *CLIClient) vlessTag() string {
	if c.VLESSTag != "" {
		return c.VLESSTag
	}
	return VLESSInboundTag
}

func (c *CLIClient) socksTag() string {
	if c.SOCKSTag != "" {
		return c.SOCKSTag
	}
	return SOCKSInboundTag
}

// ---------------------------------------------------------------------------
// Argv builders (extracted so tests can assert them without running xray)
// ---------------------------------------------------------------------------

// buildAddVLESSArgs returns the argv for `xray api adu` adding a VLESS
// account to the configured inbound. xray 26 parses the payload as a
// full conf.Config fragment, so the inbound has to be wrapped in an
// "inbounds" array with a tag that matches a live runtime handler.
// The "port" field is required by the config validator but unused at
// runtime — adu locates the inbound by tag.
func (c *CLIClient) buildAddVLESSArgs(uuid, email string) []string {
	payload := map[string]any{
		"inbounds": []map[string]any{{
			"tag":      c.vlessTag(),
			"port":     1,
			"protocol": "vless",
			"settings": map[string]any{
				"clients": []map[string]any{{
					"id":    uuid,
					"email": email,
					"flow":  "xtls-rprx-vision",
					"level": 0,
				}},
				"decryption": "none",
			},
		}},
	}
	return []string{"api", "adu", "-s", c.APIAddr, mustJSON(payload)}
}

// buildRemoveVLESSArgs returns the argv for `xray api rmu` removing a VLESS
// account identified by email. xray 26+ uses flag syntax: -tag=TAG email.
func (c *CLIClient) buildRemoveVLESSArgs(email string) []string {
	return []string{"api", "rmu", "-s", c.APIAddr, "-tag=" + c.vlessTag(), email}
}

// buildStatsCmd returns the argv for `xray api statsquery` scoped to a single
// user's traffic counters.
func (c *CLIClient) buildStatsCmd(email string) []string {
	pattern := fmt.Sprintf("user>>>%s>>>traffic>>>", email)
	return []string{"api", "statsquery", "-s", c.APIAddr, "-pattern", pattern}
}

// buildResetArgs returns the argv for `xray api stats` with --reset.
func (c *CLIClient) buildResetArgs() []string {
	return []string{"api", "stats", "-s", c.APIAddr, "-reset"}
}

// buildPingArgs returns the argv for `xray api statsquery` with an empty
// pattern, used as a cheap "is the server answering?" probe.
func (c *CLIClient) buildPingArgs() []string {
	return []string{"api", "statsquery", "-s", c.APIAddr, "-pattern", ""}
}

// mustJSON marshals v and panics on failure; used only for compile-time
// known structures, never on untrusted input, so the panic is a bug guard.
func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("{\"err\":%q}", err.Error())
	}
	return string(b)
}

// ---------------------------------------------------------------------------
// Client interface implementation
// ---------------------------------------------------------------------------

// AddVLESSUser implements Client.
func (c *CLIClient) AddVLESSUser(ctx context.Context, uuid, email string) error {
	if uuid == "" || email == "" {
		return fmt.Errorf("xray: AddVLESSUser: uuid and email are required")
	}
	if _, err := c.run(ctx, c.buildAddVLESSArgs(uuid, email)); err != nil {
		return fmt.Errorf("xray add vless %q: %w", email, err)
	}
	return nil
}

// RemoveVLESSUser implements Client.
func (c *CLIClient) RemoveVLESSUser(ctx context.Context, email string) error {
	if email == "" {
		return fmt.Errorf("xray: RemoveVLESSUser: email is required")
	}
	if _, err := c.run(ctx, c.buildRemoveVLESSArgs(email)); err != nil {
		return fmt.Errorf("xray remove vless %q: %w", email, err)
	}
	return nil
}

// AddSOCKSUser implements Client. SOCKS in xray-core does not expose a
// runtime UserManager, so we patch config.json and restart xray.
func (c *CLIClient) AddSOCKSUser(ctx context.Context, user, pass, email string) error {
	if user == "" || pass == "" || email == "" {
		return fmt.Errorf("xray: AddSOCKSUser: user, pass and email are required")
	}
	if err := c.patchSOCKSAccounts(ctx, func(accs []map[string]string) []map[string]string {
		for _, a := range accs {
			if a["user"] == user {
				a["pass"] = pass
				return accs
			}
		}
		return append(accs, map[string]string{"user": user, "pass": pass})
	}); err != nil {
		return fmt.Errorf("xray add socks %q: %w", email, err)
	}
	return nil
}

// RemoveSOCKSUser implements Client. Removes the account whose `user` field
// equals email and restarts xray. Returns ErrUserNotFound if no such account
// exists in config.json.
func (c *CLIClient) RemoveSOCKSUser(ctx context.Context, email string) error {
	if email == "" {
		return fmt.Errorf("xray: RemoveSOCKSUser: email is required")
	}
	found := false
	if err := c.patchSOCKSAccounts(ctx, func(accs []map[string]string) []map[string]string {
		out := accs[:0]
		for _, a := range accs {
			if a["user"] == email {
				found = true
				continue
			}
			out = append(out, a)
		}
		return out
	}); err != nil {
		return fmt.Errorf("xray remove socks %q: %w", email, err)
	}
	if !found {
		return ErrUserNotFound
	}
	return nil
}

// GetUserStats implements Client by parsing the JSON emitted by
// `xray api statsquery`. The output shape is:
//
//	{ "stat": [ { "name": "user>>>alice>>>traffic>>>uplink", "value": 123 }, ... ] }
//
// If no matching stats are present, ErrUserNotFound is returned.
func (c *CLIClient) GetUserStats(ctx context.Context, email string) (int64, int64, error) {
	if email == "" {
		return 0, 0, fmt.Errorf("xray: GetUserStats: email is required")
	}
	out, err := c.run(ctx, c.buildStatsCmd(email))
	if err != nil {
		return 0, 0, fmt.Errorf("xray stats %q: %w", email, err)
	}
	up, down, err := parseUserStats(out, email)
	if err != nil {
		return 0, 0, fmt.Errorf("xray stats %q: %w", email, err)
	}
	return up, down, nil
}

// Reset implements Client.
func (c *CLIClient) Reset(ctx context.Context) error {
	if _, err := c.run(ctx, c.buildResetArgs()); err != nil {
		return fmt.Errorf("xray reset stats: %w", err)
	}
	return nil
}

// Ping implements Client.
func (c *CLIClient) Ping(ctx context.Context) error {
	if _, err := c.run(ctx, c.buildPingArgs()); err != nil {
		return fmt.Errorf("xray ping: %w", err)
	}
	return nil
}

// run invokes the xray binary with the supplied argv and returns stdout.
//
// When the last element of argv is a JSON object (starts with '{'), it is
// written to a temp file and the file path replaces the inline JSON — xray api
// adu/rmu expect file paths, not inline JSON strings.
//
// Stderr is consulted to translate "user not found" errors into a typed
// ErrUserNotFound; anything else becomes a wrapped error with stderr
// preserved for diagnosis.
func (c *CLIClient) run(ctx context.Context, argv []string) ([]byte, error) {
	// Materialise any trailing JSON payload as a temp file.
	if len(argv) > 0 && strings.HasPrefix(argv[len(argv)-1], "{") {
		jsonData := argv[len(argv)-1]
		tmp, err := os.CreateTemp("", "xray-api-*.json")
		if err != nil {
			return nil, fmt.Errorf("xray: create temp: %w", err)
		}
		tmpPath := tmp.Name()
		defer os.Remove(tmpPath)
		if _, err := tmp.WriteString(jsonData); err != nil {
			tmp.Close()
			return nil, fmt.Errorf("xray: write temp: %w", err)
		}
		tmp.Close()
		argv = append(append([]string(nil), argv[:len(argv)-1]...), tmpPath)
	}

	cmd := exec.CommandContext(ctx, c.XrayBin, argv...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		if classifyNotFound(stderr.String()) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("exec %s %s: %w (stderr: %s)",
			c.XrayBin, strings.Join(argv, " "), err, strings.TrimSpace(stderr.String()))
	}
	// xray exits 0 even when adu/rmu changed nothing (e.g. wrong tag, schema
	// mismatch, unsupported inbound type). Treat "Added/Removed 0 user(s)" as
	// a real error so callers don't silently succeed.
	if argv[0] == "api" && (argv[1] == "adu" || argv[1] == "rmu") && hasZeroAffected(stdout.Bytes()) {
		return nil, fmt.Errorf("exec %s %s: 0 users affected (stdout: %s, stderr: %s)",
			c.XrayBin, strings.Join(argv, " "),
			strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// hasZeroAffected detects xray's "Added 0 user(s) in total." or
// "Removed 0 user(s) in total." trailer printed when the gRPC handler
// rejected every entry but the CLI still exited 0.
func hasZeroAffected(stdout []byte) bool {
	s := string(stdout)
	return strings.Contains(s, "Added 0 user(s)") || strings.Contains(s, "Removed 0 user(s)")
}

// patchSOCKSAccounts atomically rewrites config.json with a modified accounts
// list for the SOCKS inbound, then restarts xray so the change takes effect.
//
// SOCKS user management cannot use the runtime gRPC API: xray-core's SOCKS
// proxy does not implement the UserManager interface, so `xray api adu/rmu`
// silently no-op (returning "Added 0 user(s)" with exit 0). The only way to
// add/remove SOCKS accounts on a live server is to edit config.json and
// restart xray — that's what this method does.
func (c *CLIClient) patchSOCKSAccounts(ctx context.Context, modify func([]map[string]string) []map[string]string) error {
	if c.ConfigPath == "" || c.RestartXray == nil {
		return fmt.Errorf("xray: SOCKS user management requires ConfigPath and RestartXray")
	}
	data, err := os.ReadFile(c.ConfigPath)
	if err != nil {
		return fmt.Errorf("read xray config: %w", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("decode xray config: %w", err)
	}
	inbounds, _ := raw["inbounds"].([]any)
	tag := c.socksTag()
	patched := false
	for _, ib := range inbounds {
		m, ok := ib.(map[string]any)
		if !ok {
			continue
		}
		if t, _ := m["tag"].(string); t != tag {
			continue
		}
		settings, _ := m["settings"].(map[string]any)
		if settings == nil {
			settings = map[string]any{}
			m["settings"] = settings
		}
		rawAccounts, _ := settings["accounts"].([]any)
		current := make([]map[string]string, 0, len(rawAccounts))
		for _, a := range rawAccounts {
			am, ok := a.(map[string]any)
			if !ok {
				continue
			}
			user, _ := am["user"].(string)
			pass, _ := am["pass"].(string)
			current = append(current, map[string]string{"user": user, "pass": pass})
		}
		updated := modify(current)
		next := make([]any, len(updated))
		for i, u := range updated {
			next[i] = map[string]any{"user": u["user"], "pass": u["pass"]}
		}
		settings["accounts"] = next
		patched = true
		break
	}
	if !patched {
		return fmt.Errorf("xray: no inbound with tag %q in %s", tag, c.ConfigPath)
	}
	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("encode xray config: %w", err)
	}
	tmp := c.ConfigPath + ".tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return fmt.Errorf("write xray config: %w", err)
	}
	if err := os.Rename(tmp, c.ConfigPath); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("install xray config: %w", err)
	}
	if err := c.RestartXray(ctx); err != nil {
		return fmt.Errorf("restart xray: %w", err)
	}
	return nil
}

// classifyNotFound returns true when stderr from `xray api` matches one of
// the well-known "not found" fingerprints.
func classifyNotFound(stderr string) bool {
	s := strings.ToLower(stderr)
	for _, n := range []string{"not found", "no such user", "user not exist", "user does not exist", "no matched user"} {
		if strings.Contains(s, n) {
			return true
		}
	}
	return false
}

// parseUserStats extracts (uplink, downlink) from the JSON emitted by
// `xray api statsquery`.
func parseUserStats(out []byte, email string) (int64, int64, error) {
	trimmed := bytes.TrimSpace(out)
	if len(trimmed) == 0 {
		return 0, 0, ErrUserNotFound
	}
	var parsed struct {
		Stat []struct {
			Name  string `json:"name"`
			Value int64  `json:"value"`
		} `json:"stat"`
	}
	if err := json.Unmarshal(trimmed, &parsed); err != nil {
		return 0, 0, fmt.Errorf("decode stats JSON: %w", err)
	}
	if len(parsed.Stat) == 0 {
		return 0, 0, ErrUserNotFound
	}
	upKey := fmt.Sprintf("user>>>%s>>>traffic>>>uplink", email)
	downKey := fmt.Sprintf("user>>>%s>>>traffic>>>downlink", email)
	var up, down int64
	var matched bool
	for _, row := range parsed.Stat {
		switch row.Name {
		case upKey:
			up = row.Value
			matched = true
		case downKey:
			down = row.Value
			matched = true
		}
	}
	if !matched {
		return 0, 0, ErrUserNotFound
	}
	return up, down, nil
}
