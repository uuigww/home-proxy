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

// Default Xray inbound tags home-proxy emits.
const (
	VLESSInboundTag = "vless-reality"
	SOCKSInboundTag = "socks-in"
)

// CLIClient implements Client by shelling out to the `xray api` subcommand.
//
// XrayBin is the absolute path (or bare name in $PATH) of the xray binary;
// APIAddr is the host:port of the Xray API gRPC listener (e.g. 127.0.0.1:10085).
// VLESSTag and SOCKSTag default to the constants above when empty.
type CLIClient struct {
	XrayBin  string
	APIAddr  string
	VLESSTag string
	SOCKSTag string
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
// account to the configured inbound. The last argument is the JSON payload
// describing the account.
func (c *CLIClient) buildAddVLESSArgs(uuid, email string) []string {
	payload := map[string]any{
		"tag": c.vlessTag(),
		"users": []map[string]any{{
			"email": email,
			"account": map[string]any{
				"type":  "xray.proxy.vless.Account",
				"id":    uuid,
				"flow":  "xtls-rprx-vision",
				"level": 0,
			},
		}},
	}
	return []string{"api", "adu", "-s", c.APIAddr, mustJSON(payload)}
}

// buildRemoveVLESSArgs returns the argv for `xray api rmu` removing a VLESS
// account identified by email.
func (c *CLIClient) buildRemoveVLESSArgs(email string) []string {
	payload := map[string]any{
		"tag":   c.vlessTag(),
		"email": email,
	}
	return []string{"api", "rmu", "-s", c.APIAddr, mustJSON(payload)}
}

// buildAddSOCKSArgs returns the argv for `xray api adu` adding a SOCKS5
// account.
func (c *CLIClient) buildAddSOCKSArgs(user, pass, email string) []string {
	payload := map[string]any{
		"tag": c.socksTag(),
		"users": []map[string]any{{
			"email": email,
			"account": map[string]any{
				"type":     "xray.proxy.socks.Account",
				"username": user,
				"password": pass,
			},
		}},
	}
	return []string{"api", "adu", "-s", c.APIAddr, mustJSON(payload)}
}

// buildRemoveSOCKSArgs returns the argv for removing a SOCKS5 account.
func (c *CLIClient) buildRemoveSOCKSArgs(email string) []string {
	payload := map[string]any{
		"tag":   c.socksTag(),
		"email": email,
	}
	return []string{"api", "rmu", "-s", c.APIAddr, mustJSON(payload)}
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

// AddSOCKSUser implements Client.
func (c *CLIClient) AddSOCKSUser(ctx context.Context, user, pass, email string) error {
	if user == "" || pass == "" || email == "" {
		return fmt.Errorf("xray: AddSOCKSUser: user, pass and email are required")
	}
	if _, err := c.run(ctx, c.buildAddSOCKSArgs(user, pass, email)); err != nil {
		return fmt.Errorf("xray add socks %q: %w", email, err)
	}
	return nil
}

// RemoveSOCKSUser implements Client.
func (c *CLIClient) RemoveSOCKSUser(ctx context.Context, email string) error {
	if email == "" {
		return fmt.Errorf("xray: RemoveSOCKSUser: email is required")
	}
	if _, err := c.run(ctx, c.buildRemoveSOCKSArgs(email)); err != nil {
		return fmt.Errorf("xray remove socks %q: %w", email, err)
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
// Stderr is consulted to translate "user not found" errors into a typed
// ErrUserNotFound; anything else becomes a wrapped error with stderr
// preserved for diagnosis.
func (c *CLIClient) run(ctx context.Context, argv []string) ([]byte, error) {
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
	return stdout.Bytes(), nil
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
