package store

import (
	"context"
	"errors"
	"testing"
)

func TestSessionUpsertGetDelete(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	sess := Session{
		TGID: 7, ChatID: 99, MessageID: 555,
		Screen: "home", WizardJSON: `{"step":1}`,
	}
	if err := s.UpsertSession(ctx, sess); err != nil {
		t.Fatalf("insert session: %v", err)
	}
	got, err := s.GetSession(ctx, 7)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Screen != "home" || got.MessageID != 555 || got.WizardJSON != `{"step":1}` {
		t.Fatalf("unexpected session: %+v", got)
	}

	// Replace fields via upsert.
	sess.Screen = "users"
	sess.MessageID = 556
	if err := s.UpsertSession(ctx, sess); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ = s.GetSession(ctx, 7)
	if got.Screen != "users" || got.MessageID != 556 {
		t.Fatalf("update not persisted: %+v", got)
	}

	// Empty WizardJSON normalised to "{}".
	sess2 := Session{TGID: 8, ChatID: 1, MessageID: 1, Screen: "x"}
	if err := s.UpsertSession(ctx, sess2); err != nil {
		t.Fatalf("insert empty: %v", err)
	}
	got2, _ := s.GetSession(ctx, 8)
	if got2.WizardJSON != "{}" {
		t.Fatalf("expected WizardJSON normalised to '{}', got %q", got2.WizardJSON)
	}

	// Delete + idempotent delete.
	if err := s.DeleteSession(ctx, 7); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.GetSession(ctx, 7); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if err := s.DeleteSession(ctx, 7); err != nil {
		t.Fatalf("second delete should be nil, got %v", err)
	}

	if err := s.UpsertSession(ctx, Session{}); err == nil {
		t.Fatal("expected error for missing tg_id")
	}
}
