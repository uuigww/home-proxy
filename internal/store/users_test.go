package store

import (
	"context"
	"errors"
	"testing"
)

func TestUserCRUD(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	u := &User{
		Name:       "alice",
		VLESSUUID:  ptr("11111111-1111-1111-1111-111111111111"),
		SOCKSUser:  ptr("alice"),
		SOCKSPass:  ptr("secret"),
		LimitBytes: 100,
		Enabled:    true,
	}
	if err := s.CreateUser(ctx, u); err != nil {
		t.Fatalf("create: %v", err)
	}
	if u.ID == 0 {
		t.Fatalf("expected non-zero id after create")
	}

	got, err := s.GetUser(ctx, u.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "alice" || got.LimitBytes != 100 || !got.Enabled {
		t.Fatalf("unexpected user round-trip: %+v", got)
	}
	if got.VLESSUUID == nil || *got.VLESSUUID != *u.VLESSUUID {
		t.Fatalf("VLESSUUID not preserved: %+v", got.VLESSUUID)
	}

	byName, err := s.GetUserByName(ctx, "alice")
	if err != nil {
		t.Fatalf("get by name: %v", err)
	}
	if byName.ID != u.ID {
		t.Fatalf("GetUserByName returned id %d, want %d", byName.ID, u.ID)
	}

	u2 := &User{Name: "bob", Enabled: false}
	if err := s.CreateUser(ctx, u2); err != nil {
		t.Fatalf("create bob: %v", err)
	}

	enabledOnly, err := s.ListUsers(ctx, false)
	if err != nil {
		t.Fatalf("list enabled: %v", err)
	}
	if len(enabledOnly) != 1 || enabledOnly[0].Name != "alice" {
		t.Fatalf("expected [alice], got %+v", enabledOnly)
	}

	all, err := s.ListUsers(ctx, true)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 users, got %d", len(all))
	}

	got.LimitBytes = 999
	got.Enabled = false
	if err := s.UpdateUser(ctx, &got); err != nil {
		t.Fatalf("update: %v", err)
	}
	got2, err := s.GetUser(ctx, u.ID)
	if err != nil {
		t.Fatalf("reget: %v", err)
	}
	if got2.LimitBytes != 999 || got2.Enabled {
		t.Fatalf("update not persisted: %+v", got2)
	}

	if err := s.SetUserEnabled(ctx, u.ID, true); err != nil {
		t.Fatalf("set enabled: %v", err)
	}
	got3, _ := s.GetUser(ctx, u.ID)
	if !got3.Enabled {
		t.Fatalf("SetUserEnabled didn't take effect")
	}

	if err := s.DeleteUser(ctx, u.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.GetUser(ctx, u.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
	if err := s.DeleteUser(ctx, u.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete twice: expected ErrNotFound, got %v", err)
	}
}

func TestUpdateUserMissingIDRejected(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	if err := s.UpdateUser(ctx, &User{Name: "x"}); err == nil {
		t.Fatal("expected error for missing id")
	}
	if err := s.UpdateUser(ctx, nil); err == nil {
		t.Fatal("expected error for nil user")
	}
	if err := s.CreateUser(ctx, nil); err == nil {
		t.Fatal("expected error for nil user in create")
	}
}

func TestUserMTProtoRoundTrip(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	u := &User{Name: "dan", Enabled: true, MTProtoEnabled: true}
	if err := s.CreateUser(ctx, u); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := s.GetUser(ctx, u.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !got.MTProtoEnabled {
		t.Fatalf("MTProtoEnabled not persisted on create")
	}

	if err := s.SetUserMTProtoEnabled(ctx, u.ID, false); err != nil {
		t.Fatalf("set mtproto off: %v", err)
	}
	got2, _ := s.GetUser(ctx, u.ID)
	if got2.MTProtoEnabled {
		t.Fatalf("SetUserMTProtoEnabled(false) did not take effect")
	}
	if err := s.SetUserMTProtoEnabled(ctx, u.ID, true); err != nil {
		t.Fatalf("set mtproto on: %v", err)
	}
	got3, _ := s.GetUser(ctx, u.ID)
	if !got3.MTProtoEnabled {
		t.Fatalf("SetUserMTProtoEnabled(true) did not take effect")
	}

	// UpdateUser round-trip preserves the flag.
	got3.MTProtoEnabled = false
	if err := s.UpdateUser(ctx, &got3); err != nil {
		t.Fatalf("update: %v", err)
	}
	got4, _ := s.GetUser(ctx, u.ID)
	if got4.MTProtoEnabled {
		t.Fatalf("UpdateUser did not persist MTProtoEnabled=false")
	}

	if err := s.SetUserMTProtoEnabled(ctx, 99999, true); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestAddUserUsageAdditive(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	u := &User{Name: "carol", Enabled: true}
	if err := s.CreateUser(ctx, u); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := s.AddUserUsage(ctx, u.ID, 100, 50); err != nil {
		t.Fatalf("add 1: %v", err)
	}
	if err := s.AddUserUsage(ctx, u.ID, 200, 0); err != nil {
		t.Fatalf("add 2: %v", err)
	}
	got, _ := s.GetUser(ctx, u.ID)
	if got.UsedBytes != 350 {
		t.Fatalf("expected used_bytes=350, got %d", got.UsedBytes)
	}

	if err := s.AddUserUsage(ctx, u.ID, -1, 0); err == nil {
		t.Fatal("expected error for negative bytes")
	}
	if err := s.AddUserUsage(ctx, 99999, 10, 10); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
