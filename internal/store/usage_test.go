package store

import (
	"context"
	"testing"
	"time"
)

func TestUsageHistoryAndFKCascade(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	u := &User{Name: "dave", Enabled: true}
	if err := s.CreateUser(ctx, u); err != nil {
		t.Fatalf("create: %v", err)
	}

	day := time.Date(2026, 4, 1, 12, 34, 56, 0, time.UTC)
	if err := s.RecordUsageDay(ctx, u.ID, day, 100, 200); err != nil {
		t.Fatalf("record 1: %v", err)
	}
	if err := s.RecordUsageDay(ctx, u.ID, day, 50, 0); err != nil {
		t.Fatalf("record 2: %v", err)
	}
	if err := s.RecordUsageDay(ctx, u.ID, day.Add(24*time.Hour), 10, 20); err != nil {
		t.Fatalf("record 3: %v", err)
	}

	rows, err := s.GetUsageSince(ctx, u.ID, day.Add(-48*time.Hour))
	if err != nil {
		t.Fatalf("get usage: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 days, got %d", len(rows))
	}
	if rows[0].UplinkBytes != 150 || rows[0].DownlinkBytes != 200 {
		t.Fatalf("day-1 aggregation wrong: %+v", rows[0])
	}
	if rows[1].UplinkBytes != 10 || rows[1].DownlinkBytes != 20 {
		t.Fatalf("day-2 aggregation wrong: %+v", rows[1])
	}

	top, err := s.GetTopUsersByUsage(ctx, 10, day.Add(-48*time.Hour))
	if err != nil {
		t.Fatalf("top users: %v", err)
	}
	if len(top) != 1 {
		t.Fatalf("expected 1 top user, got %d", len(top))
	}
	if top[0].TotalBytes != 150+200+10+20 {
		t.Fatalf("expected total=380, got %d", top[0].TotalBytes)
	}

	// Delete user, verify cascade wipes usage_history.
	if err := s.DeleteUser(ctx, u.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	var n int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM usage_history WHERE user_id = ?`, u.ID).Scan(&n); err != nil {
		t.Fatalf("count cascaded rows: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected usage_history cascade wipe, still %d rows", n)
	}

	// Negative bytes rejected.
	u2 := &User{Name: "erin", Enabled: true}
	if err := s.CreateUser(ctx, u2); err != nil {
		t.Fatalf("create erin: %v", err)
	}
	if err := s.RecordUsageDay(ctx, u2.ID, day, -1, 0); err == nil {
		t.Fatal("expected error for negative bytes")
	}

	// GetTopUsersByUsage with n<=0 returns nil.
	got, err := s.GetTopUsersByUsage(ctx, 0, day)
	if err != nil {
		t.Fatalf("top 0: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil for n=0, got %+v", got)
	}
}
