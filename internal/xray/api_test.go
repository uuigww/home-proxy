package xray

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// mustUnmarshal decodes a JSON string into a map for field-by-field asserts
// so we don't depend on Go map iteration order in the encoded payload.
func mustUnmarshal(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("invalid JSON payload %q: %v", s, err)
	}
	return m
}

func newTestClient() *CLIClient { return NewCLIClient("", "") }

func TestDefaults(t *testing.T) {
	c := NewCLIClient("", "")
	if c.XrayBin != "xray" {
		t.Errorf("expected default XrayBin=xray, got %q", c.XrayBin)
	}
	if c.APIAddr != "127.0.0.1:10085" {
		t.Errorf("expected default APIAddr=127.0.0.1:10085, got %q", c.APIAddr)
	}
	if c.vlessTag() != VLESSInboundTag {
		t.Errorf("expected default VLESS tag %q", VLESSInboundTag)
	}
	if c.socksTag() != SOCKSInboundTag {
		t.Errorf("expected default SOCKS tag %q", SOCKSInboundTag)
	}
	c2 := NewCLIClient("/usr/local/bin/xray", "10.0.0.1:9000")
	if c2.XrayBin != "/usr/local/bin/xray" || c2.APIAddr != "10.0.0.1:9000" {
		t.Errorf("NewCLIClient did not honour explicit args: %+v", c2)
	}
}

func TestBuildStatsCmd(t *testing.T) {
	got := newTestClient().buildStatsCmd("alice")
	want := []string{"api", "statsquery", "-s", "127.0.0.1:10085", "-pattern", "user>>>alice>>>traffic>>>"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildStatsCmd mismatch:\n got:  %v\n want: %v", got, want)
	}
}

func TestBuildResetArgs(t *testing.T) {
	got := newTestClient().buildResetArgs()
	want := []string{"api", "stats", "-s", "127.0.0.1:10085", "-reset"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildResetArgs mismatch:\n got:  %v\n want: %v", got, want)
	}
}

func TestBuildPingArgs(t *testing.T) {
	got := newTestClient().buildPingArgs()
	want := []string{"api", "statsquery", "-s", "127.0.0.1:10085", "-pattern", ""}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildPingArgs mismatch:\n got:  %v\n want: %v", got, want)
	}
}

func TestBuildAddVLESSArgs(t *testing.T) {
	got := newTestClient().buildAddVLESSArgs("abc-uuid", "alice")
	if len(got) != 5 {
		t.Fatalf("expected 5 args, got %d: %v", len(got), got)
	}
	if got[0] != "api" || got[1] != "adu" || got[2] != "-s" || got[3] != "127.0.0.1:10085" {
		t.Errorf("unexpected arg prefix: %v", got[:4])
	}
	m := mustUnmarshal(t, got[4])
	inbounds, ok := m["inbounds"].([]any)
	if !ok || len(inbounds) != 1 {
		t.Fatalf("expected one inbound entry, got %v", m["inbounds"])
	}
	ib := inbounds[0].(map[string]any)
	if ib["tag"] != VLESSInboundTag {
		t.Errorf("expected tag=%q, got %v", VLESSInboundTag, ib["tag"])
	}
	if ib["protocol"] != "vless" {
		t.Errorf("expected protocol=vless, got %v", ib["protocol"])
	}
	settings := ib["settings"].(map[string]any)
	clients, ok := settings["clients"].([]any)
	if !ok || len(clients) != 1 {
		t.Fatalf("expected one client, got %v", settings["clients"])
	}
	client := clients[0].(map[string]any)
	if client["email"] != "alice" {
		t.Errorf("expected email alice, got %v", client["email"])
	}
	if client["id"] != "abc-uuid" {
		t.Errorf("expected id=abc-uuid, got %v", client["id"])
	}
	if client["flow"] != "xtls-rprx-vision" {
		t.Errorf("expected flow xtls-rprx-vision, got %v", client["flow"])
	}
}

func TestBuildRemoveVLESSArgs(t *testing.T) {
	got := newTestClient().buildRemoveVLESSArgs("alice")
	want := []string{"api", "rmu", "-s", "127.0.0.1:10085", "-tag=" + VLESSInboundTag, "alice"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildRemoveVLESSArgs mismatch:\n got:  %v\n want: %v", got, want)
	}
}

func TestCustomVLESSTagHonoured(t *testing.T) {
	c := &CLIClient{XrayBin: "xray", APIAddr: "127.0.0.1:10085", VLESSTag: "my-v", SOCKSTag: "my-s"}
	v := c.buildAddVLESSArgs("u", "e")
	m := mustUnmarshal(t, v[4])
	ib := m["inbounds"].([]any)[0].(map[string]any)
	if ib["tag"] != "my-v" {
		t.Errorf("expected tag=my-v, got %v", ib["tag"])
	}
	rm := c.buildRemoveVLESSArgs("e")
	if rm[4] != "-tag=my-v" {
		t.Errorf("expected -tag=my-v, got %v", rm[4])
	}
}

func TestCustomAPIAddrInArgs(t *testing.T) {
	c := NewCLIClient("", "10.0.0.5:10086")
	got := c.buildPingArgs()
	if got[3] != "10.0.0.5:10086" {
		t.Errorf("expected custom api addr, got %v", got[3])
	}
}

func TestClassifyNotFound(t *testing.T) {
	positives := []string{
		"ERROR user not found\n",
		"something: no such user",
		"User Does Not Exist",
		"error: no matched user alice",
		"rpc error: user not exist",
	}
	for _, s := range positives {
		if !classifyNotFound(s) {
			t.Errorf("expected classifyNotFound to match %q", s)
		}
	}
	for _, s := range []string{"", "connection refused", "context deadline exceeded", "permission denied"} {
		if classifyNotFound(s) {
			t.Errorf("expected classifyNotFound NOT to match %q", s)
		}
	}
}

func TestParseUserStats(t *testing.T) {
	raw := []byte(`{"stat":[
        {"name":"user>>>alice>>>traffic>>>uplink","value":1234},
        {"name":"user>>>alice>>>traffic>>>downlink","value":5678}
    ]}`)
	up, down, err := parseUserStats(raw, "alice")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if up != 1234 || down != 5678 {
		t.Errorf("expected (1234,5678), got (%d,%d)", up, down)
	}
}

func TestParseUserStatsOnlyUplink(t *testing.T) {
	raw := []byte(`{"stat":[{"name":"user>>>bob>>>traffic>>>uplink","value":42}]}`)
	up, down, err := parseUserStats(raw, "bob")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if up != 42 || down != 0 {
		t.Errorf("expected (42,0), got (%d,%d)", up, down)
	}
}

func TestParseUserStatsWrongEmail(t *testing.T) {
	raw := []byte(`{"stat":[{"name":"user>>>bob>>>traffic>>>uplink","value":42}]}`)
	_, _, err := parseUserStats(raw, "alice")
	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestParseUserStatsEmpty(t *testing.T) {
	for _, in := range []string{"", "   ", `{"stat":[]}`} {
		_, _, err := parseUserStats([]byte(in), "alice")
		if !errors.Is(err, ErrUserNotFound) {
			t.Errorf("input %q: expected ErrUserNotFound, got %v", in, err)
		}
	}
}

func TestParseUserStatsInvalidJSON(t *testing.T) {
	_, _, err := parseUserStats([]byte("not json"), "alice")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if errors.Is(err, ErrUserNotFound) {
		t.Fatal("invalid JSON should not collapse to ErrUserNotFound")
	}
	if !strings.Contains(err.Error(), "decode") {
		t.Errorf("expected decode-error wrap, got %v", err)
	}
}

func TestInterfaceCompliance(t *testing.T) {
	var _ Client = (*CLIClient)(nil)
}

func TestValidationErrors(t *testing.T) {
	c := newTestClient()
	ctx := context.Background()
	cases := []struct {
		name string
		fn   func() error
	}{
		{"AddVLESS empty uuid", func() error { return c.AddVLESSUser(ctx, "", "e") }},
		{"AddVLESS empty email", func() error { return c.AddVLESSUser(ctx, "u", "") }},
		{"RemoveVLESS empty", func() error { return c.RemoveVLESSUser(ctx, "") }},
		{"AddSOCKS empty user", func() error { return c.AddSOCKSUser(ctx, "", "p", "e") }},
		{"AddSOCKS empty pass", func() error { return c.AddSOCKSUser(ctx, "u", "", "e") }},
		{"AddSOCKS empty email", func() error { return c.AddSOCKSUser(ctx, "u", "p", "") }},
		{"RemoveSOCKS empty", func() error { return c.RemoveSOCKSUser(ctx, "") }},
		{"GetUserStats empty", func() error { _, _, e := c.GetUserStats(ctx, ""); return e }},
	}
	for _, tc := range cases {
		if err := tc.fn(); err == nil {
			t.Errorf("%s: expected validation error, got nil", tc.name)
		}
	}
}
