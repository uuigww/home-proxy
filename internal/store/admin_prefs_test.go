package store

import (
	"context"
	"errors"
	"testing"
)

func TestAdminPrefsUpsertIdempotent(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	p := NewDefaultAdminPrefs(42, "en")
	if err := s.UpsertAdminPrefs(ctx, p); err != nil {
		t.Fatalf("insert: %v", err)
	}
	got1, err := s.GetAdminPrefs(ctx, 42)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got1.Lang != "en" || !got1.NotifyCritical || !got1.NotifyDaily || got1.DigestHour != 9 {
		t.Fatalf("unexpected prefs: %+v", got1)
	}

	// Re-upsert changes fields without creating a new row.
	p.Lang = "ru"
	p.DigestHour = 21
	if err := s.UpsertAdminPrefs(ctx, p); err != nil {
		t.Fatalf("re-upsert: %v", err)
	}
	got2, _ := s.GetAdminPrefs(ctx, 42)
	if got2.Lang != "ru" || got2.DigestHour != 21 {
		t.Fatalf("update not persisted: %+v", got2)
	}

	all, err := s.ListAdminPrefs(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected 1 row, got %d", len(all))
	}

	if err := s.UpsertAdminPrefs(ctx, AdminPrefs{}); err == nil {
		t.Fatal("expected error for missing tg_id")
	}
	if _, err := s.GetAdminPrefs(ctx, 777); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestNewDefaultAdminPrefs(t *testing.T) {
	p := NewDefaultAdminPrefs(1, "")
	if p.TGID != 1 {
		t.Errorf("TGID not set")
	}
	if p.Lang != "ru" {
		t.Errorf("expected default lang 'ru', got %q", p.Lang)
	}
	if !p.NotifyCritical || !p.NotifyImportant || !p.NotifyInfo || !p.NotifySecurity || !p.NotifyDaily {
		t.Errorf("expected all default notification toggles on: %+v", p)
	}
	if p.NotifyInfoOthersOnly || p.NotifyNonadminSpam {
		t.Errorf("expected opt-in flags off by default: %+v", p)
	}
	if p.DigestHour != 9 || p.QuietFromHour != 23 || p.QuietToHour != 7 {
		t.Errorf("unexpected hour defaults: %+v", p)
	}
	if p.UpdatedAt.IsZero() {
		t.Errorf("UpdatedAt should be set")
	}

	p2 := NewDefaultAdminPrefs(2, "en")
	if p2.Lang != "en" {
		t.Errorf("expected explicit lang 'en', got %q", p2.Lang)
	}
}
