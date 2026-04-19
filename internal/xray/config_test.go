package xray

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
)

func sampleInput() GenInput {
	return GenInput{
		Users: []User{
			{
				Name:      "alice",
				VLESSUUID: "11111111-1111-1111-1111-111111111111",
				SOCKSUser: "alice",
				SOCKSPass: "p1",
				Enabled:   true,
			},
			{
				Name:      "bob",
				VLESSUUID: "22222222-2222-2222-2222-222222222222",
				SOCKSUser: "bob",
				SOCKSPass: "p2",
				Enabled:   true,
			},
			{
				Name:      "carol-disabled",
				VLESSUUID: "33333333-3333-3333-3333-333333333333",
				SOCKSUser: "carol",
				SOCKSPass: "p3",
				Enabled:   false,
			},
			{
				Name:      "dave-socks-only",
				SOCKSUser: "dave",
				SOCKSPass: "p4",
				Enabled:   true,
			},
		},
		Reality: Reality{
			Dest:       "www.google.com:443",
			ServerName: "www.google.com",
			PrivateKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
			PublicKey:  "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB",
			ShortID:    "deadbeef",
		},
		Warp: WarpPeer{
			PrivateKey:    "warp-priv=",
			PeerPublicKey: "warp-pub=",
			IPv4:          "172.16.0.2/32",
			IPv6:          "2606:4700:110:8a36:df92:102a:9602:fa18/128",
			Endpoint:      "engage.cloudflareclient.com:2408",
			MTU:           1280,
		},
		SOCKSPort:   1080,
		RealityPort: 443,
		API:         "127.0.0.1:10085",
	}
}

// findInbound returns a pointer to the first inbound with the given tag.
func findInbound(cfg Config, tag string) *Inbound {
	for i := range cfg.Inbounds {
		if cfg.Inbounds[i].Tag == tag {
			return &cfg.Inbounds[i]
		}
	}
	return nil
}

func TestGenerate_ClientsAndAccounts(t *testing.T) {
	cfg, err := Generate(sampleInput())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	vless := findInbound(cfg, TagVLESSIn)
	if vless == nil {
		t.Fatal("VLESS inbound missing")
	}
	var vlessSettings struct {
		Clients    []vlessClient `json:"clients"`
		Decryption string        `json:"decryption"`
	}
	if err := json.Unmarshal(vless.Settings, &vlessSettings); err != nil {
		t.Fatalf("unmarshal vless settings: %v", err)
	}
	if vlessSettings.Decryption != "none" {
		t.Errorf("vless decryption = %q, want none", vlessSettings.Decryption)
	}
	if got := len(vlessSettings.Clients); got != 2 {
		t.Errorf("vless clients = %d, want 2 (alice, bob); got %+v", got, vlessSettings.Clients)
	}
	for _, c := range vlessSettings.Clients {
		if c.Email == "carol-disabled" {
			t.Errorf("disabled user leaked into VLESS clients: %+v", c)
		}
		if c.Email == "dave-socks-only" {
			t.Errorf("SOCKS-only user leaked into VLESS clients: %+v", c)
		}
	}

	socks := findInbound(cfg, TagSOCKSIn)
	if socks == nil {
		t.Fatal("SOCKS inbound missing")
	}
	var socksSettings struct {
		Auth     string         `json:"auth"`
		Accounts []socksAccount `json:"accounts"`
		UDP      bool           `json:"udp"`
	}
	if err := json.Unmarshal(socks.Settings, &socksSettings); err != nil {
		t.Fatalf("unmarshal socks settings: %v", err)
	}
	if socksSettings.Auth != "password" {
		t.Errorf("socks auth = %q", socksSettings.Auth)
	}
	if !socksSettings.UDP {
		t.Errorf("socks UDP = false, want true")
	}
	if got := len(socksSettings.Accounts); got != 3 {
		t.Errorf("socks accounts = %d, want 3 (alice, bob, dave)", got)
	}
	for _, a := range socksSettings.Accounts {
		if a.User == "carol" {
			t.Errorf("disabled user leaked into SOCKS accounts: %+v", a)
		}
	}
}

func TestGenerate_DisabledUserSkipped(t *testing.T) {
	in := sampleInput()
	// Disable all except carol; carol is already disabled → expect zero clients/accounts.
	for i := range in.Users {
		in.Users[i].Enabled = false
	}
	cfg, err := Generate(in)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	vless := findInbound(cfg, TagVLESSIn)
	socks := findInbound(cfg, TagSOCKSIn)

	var vs struct {
		Clients []vlessClient `json:"clients"`
	}
	var ss struct {
		Accounts []socksAccount `json:"accounts"`
	}
	_ = json.Unmarshal(vless.Settings, &vs)
	_ = json.Unmarshal(socks.Settings, &ss)

	if len(vs.Clients) != 0 {
		t.Errorf("VLESS clients = %d, want 0 (all disabled)", len(vs.Clients))
	}
	if len(ss.Accounts) != 0 {
		t.Errorf("SOCKS accounts = %d, want 0 (all disabled)", len(ss.Accounts))
	}
}

func TestGenerate_RoutingRules(t *testing.T) {
	cfg, err := Generate(sampleInput())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if cfg.Routing.DomainStrategy != "IPIfNonMatch" {
		t.Errorf("domainStrategy = %q", cfg.Routing.DomainStrategy)
	}

	// Find the rule whose outbound is "warp" and verify it carries the required
	// domain hints.
	var warpRule *RoutingRule
	for i := range cfg.Routing.Rules {
		if cfg.Routing.Rules[i].OutboundTag == TagWarp {
			warpRule = &cfg.Routing.Rules[i]
			break
		}
	}
	if warpRule == nil {
		t.Fatal("no routing rule with outboundTag=warp")
	}
	required := []string{
		"geosite:google",
		"geosite:youtube",
		"domain:generativelanguage.googleapis.com",
	}
	for _, want := range required {
		found := false
		for _, d := range warpRule.Domain {
			if d == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("warp rule missing domain %q; rule=%+v", want, warpRule)
		}
	}

	// API rule must be first and point to api outbound.
	first := cfg.Routing.Rules[0]
	if first.OutboundTag != TagAPI {
		t.Errorf("first routing rule outboundTag = %q, want api", first.OutboundTag)
	}
	if len(first.InboundTag) != 1 || first.InboundTag[0] != TagAPIIn {
		t.Errorf("first routing rule inboundTag = %v, want [api-in]", first.InboundTag)
	}

	// Default fallthrough: last rule points to direct.
	last := cfg.Routing.Rules[len(cfg.Routing.Rules)-1]
	if last.OutboundTag != TagDirect {
		t.Errorf("last routing rule outboundTag = %q, want direct", last.OutboundTag)
	}
}

func TestGenerate_APIInbound(t *testing.T) {
	cfg, err := Generate(sampleInput())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	api := findInbound(cfg, TagAPIIn)
	if api == nil {
		t.Fatal("api inbound missing")
	}
	if api.Listen != "127.0.0.1" {
		t.Errorf("api inbound listen = %q, want 127.0.0.1", api.Listen)
	}
	if api.Port != 10085 {
		t.Errorf("api inbound port = %d, want 10085", api.Port)
	}
	if api.Protocol != "dokodemo-door" {
		t.Errorf("api inbound protocol = %q", api.Protocol)
	}
	if cfg.API.Tag != TagAPI {
		t.Errorf("api config tag = %q", cfg.API.Tag)
	}
	wantSvc := map[string]bool{"HandlerService": true, "StatsService": true}
	for _, s := range cfg.API.Services {
		if !wantSvc[s] {
			t.Errorf("unexpected api service %q", s)
		}
		delete(wantSvc, s)
	}
	if len(wantSvc) != 0 {
		t.Errorf("missing api services: %v", wantSvc)
	}
}

func TestGenerate_StatsAndPolicy(t *testing.T) {
	cfg, err := Generate(sampleInput())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	lvl0, ok := cfg.Policy.Levels["0"]
	if !ok {
		t.Fatal("policy.levels.0 missing")
	}
	if !lvl0.StatsUserUplink || !lvl0.StatsUserDownlink {
		t.Errorf("policy.levels.0 user stats not enabled: %+v", lvl0)
	}
	if !cfg.Policy.System.StatsInboundUplink || !cfg.Policy.System.StatsInboundDownlink {
		t.Errorf("policy.system inbound stats not enabled: %+v", cfg.Policy.System)
	}
}

func TestGenerate_JSONRoundTrip(t *testing.T) {
	cfg, err := Generate(sampleInput())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got Config
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// json.RawMessage whitespace can differ after round-trip, so compare by
	// re-marshalling both sides and diffing the bytes.
	a, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("remarshal original: %v", err)
	}
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("remarshal roundtrip: %v", err)
	}
	if !bytes.Equal(a, b) {
		t.Fatalf("JSON round-trip lost data.\nbefore: %s\nafter:  %s", a, b)
	}

	// Sanity: structural equality on the decoded tree after normalising
	// RawMessages via a second marshal/unmarshal pair.
	var na, nb any
	if err := json.Unmarshal(a, &na); err != nil {
		t.Fatalf("normalise a: %v", err)
	}
	if err := json.Unmarshal(b, &nb); err != nil {
		t.Fatalf("normalise b: %v", err)
	}
	if !reflect.DeepEqual(na, nb) {
		t.Errorf("normalised trees differ")
	}
}

func TestGenerate_ValidationErrors(t *testing.T) {
	cases := map[string]func(in *GenInput){
		"no-reality-port": func(in *GenInput) { in.RealityPort = 0 },
		"no-socks-port":   func(in *GenInput) { in.SOCKSPort = 0 },
		"no-api":          func(in *GenInput) { in.API = "" },
		"no-reality-key":  func(in *GenInput) { in.Reality.PrivateKey = "" },
		"no-short-id":     func(in *GenInput) { in.Reality.ShortID = "" },
	}
	for name, mutate := range cases {
		in := sampleInput()
		mutate(&in)
		if _, err := Generate(in); err == nil {
			t.Errorf("%s: expected error, got nil", name)
		}
	}
}
