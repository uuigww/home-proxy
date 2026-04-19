package bot

import (
	"testing"

	"github.com/uuigww/home-proxy/internal/store"
)

func TestWizardRoundTrip(t *testing.T) {
	sess := store.Session{TGID: 1}
	ws := WizardState{
		Kind: wizardKindAddUser,
		Step: 2,
		Data: map[string]any{
			"name":  "alice",
			"vless": true,
			"socks": false,
		},
	}
	if err := SaveWizard(&sess, ws); err != nil {
		t.Fatalf("save: %v", err)
	}
	got := LoadWizard(sess)
	if got.Kind != ws.Kind {
		t.Errorf("Kind = %q, want %q", got.Kind, ws.Kind)
	}
	if got.Step != ws.Step {
		t.Errorf("Step = %d, want %d", got.Step, ws.Step)
	}
	if got.Data["name"] != "alice" {
		t.Errorf("name = %v", got.Data["name"])
	}
	if v, _ := got.Data["vless"].(bool); !v {
		t.Errorf("vless = %v, want true", got.Data["vless"])
	}
	if v, _ := got.Data["socks"].(bool); v {
		t.Errorf("socks = %v, want false", got.Data["socks"])
	}
}

func TestClearWizard(t *testing.T) {
	sess := store.Session{TGID: 1, WizardJSON: `{"kind":"add_user","step":3}`}
	ClearWizard(&sess)
	if sess.WizardJSON != "{}" {
		t.Fatalf("ClearWizard: %q", sess.WizardJSON)
	}
	ws := LoadWizard(sess)
	if ws.Kind != "" {
		t.Fatalf("expected empty wizard after clear, got %+v", ws)
	}
}

func TestLoadWizardEmpty(t *testing.T) {
	sess := store.Session{WizardJSON: ""}
	ws := LoadWizard(sess)
	if ws.Kind != "" || ws.Step != 0 {
		t.Fatalf("expected zero wizard, got %+v", ws)
	}
}
