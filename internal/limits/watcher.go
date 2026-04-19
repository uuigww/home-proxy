package limits

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"syscall"
	"time"

	"github.com/uuigww/home-proxy/internal/store"
	"github.com/uuigww/home-proxy/internal/xray"
)

// Config tunes the watcher's timers. Zero values fall back to sane
// production defaults inside New.
type Config struct {
	// PollInterval is how often the traffic poller reads per-user
	// stats from Xray. Default: 60s.
	PollInterval time.Duration
	// HealthInterval is how often the health poller pings Xray and
	// checks disk / reality key age. Default: 30s.
	HealthInterval time.Duration
	// DigestHour is the fallback daily-digest hour (UTC-local on the
	// server) when an admin has no per-admin preference. Default: 9.
	DigestHour int
	// DataDir is the on-disk location the daemon uses for SQLite +
	// state. Default: "/var/lib/home-proxy".
	DataDir string
}

// Watcher bundles the three background loops (traffic, health,
// digests) that together enforce the per-user quotas and surface
// operational health to admins.
type Watcher struct {
	store *store.Store
	xray  xray.Client
	notif Notifier
	cfg   Config
	log   *slog.Logger

	// prevStats remembers the last (up,down) Xray reported per
	// user, keyed by store user ID. We use it to compute deltas
	// between polls.
	prevStatsMu sync.Mutex
	prevStats   map[int64]stats

	// firedEighty tracks which users already had their 80%
	// notification fired for the current quota period. It is reset
	// whenever the user's LimitBytes is increased (quota bump) or
	// UsedBytes goes back below 80%.
	firedEightyMu sync.Mutex
	firedEighty   map[int64]bool

	// dedup coalesces repeated host/reality/xray notifications.
	dedup *Dedup

	// lastDigestSent remembers the UTC calendar day of the last
	// daily digest we pushed to a given admin, so the minute-
	// granularity scheduler does not duplicate-send.
	lastDigestMu   sync.Mutex
	lastDigestSent map[int64]string // tgID -> "2026-04-19"

	// xrayPingFails is an in-memory counter for consecutive xray
	// ping failures, used to fire xray.api.unreachable after 3.
	xrayPingFails int

	// mountPoints are the paths probed by the disk-free check.
	mountPoints []string

	// now lets tests inject a deterministic clock.
	now func() time.Time
}

// stats is one snapshot of Xray's per-user counters.
type stats struct {
	up, down int64
}

// New returns a Watcher wired to the supplied dependencies. The
// Watcher is inert until Start is called.
func New(s *store.Store, x xray.Client, n Notifier, cfg Config, log *slog.Logger) *Watcher {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 60 * time.Second
	}
	if cfg.HealthInterval <= 0 {
		cfg.HealthInterval = 30 * time.Second
	}
	if cfg.DigestHour < 0 || cfg.DigestHour > 23 {
		cfg.DigestHour = 9
	}
	if cfg.DataDir == "" {
		cfg.DataDir = "/var/lib/home-proxy"
	}
	if log == nil {
		log = slog.Default()
	}
	return &Watcher{
		store:          s,
		xray:           x,
		notif:          n,
		cfg:            cfg,
		log:            log.With(slog.String("component", "limits.watcher")),
		prevStats:      make(map[int64]stats),
		firedEighty:    make(map[int64]bool),
		dedup:          NewDedup(),
		lastDigestSent: make(map[int64]string),
		mountPoints:    []string{cfg.DataDir, "/"},
		now:            func() time.Time { return time.Now().UTC() },
	}
}

// Start runs the three sub-loops concurrently and blocks until ctx
// is cancelled. The first non-nil error from any loop is returned;
// context cancellation is not treated as an error.
func (w *Watcher) Start(ctx context.Context) error {
	var wg sync.WaitGroup
	errCh := make(chan error, 3)

	run := func(name string, fn func(context.Context) error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := fn(ctx); err != nil && !errors.Is(err, context.Canceled) {
				w.log.Error("loop exited with error", slog.String("loop", name), slog.Any("err", err))
				errCh <- fmt.Errorf("%s: %w", name, err)
			}
		}()
	}

	run("traffic", w.runTrafficLoop)
	run("health", w.runHealthLoop)
	run("scheduled", w.runScheduledLoop)

	wg.Wait()
	close(errCh)
	var firstErr error
	for err := range errCh {
		if firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// runTrafficLoop polls Xray every cfg.PollInterval, updates
// per-user used_bytes + usage_history, and fires quota notifications.
func (w *Watcher) runTrafficLoop(ctx context.Context) error {
	t := time.NewTicker(w.cfg.PollInterval)
	defer t.Stop()
	// Fire once at startup so the first sample primes prevStats
	// instead of forcing admins to wait a full interval.
	w.pollTraffic(ctx)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			w.pollTraffic(ctx)
		}
	}
}

// pollTraffic reads stats for every enabled user with a known Xray
// email, diffs against the previous sample, persists the delta and
// fires quota notifications when thresholds are crossed.
func (w *Watcher) pollTraffic(ctx context.Context) {
	users, err := w.store.ListUsers(ctx, true)
	if err != nil {
		w.log.Error("list users", slog.Any("err", err))
		return
	}
	today := w.now().Truncate(24 * time.Hour)
	for _, u := range users {
		if ctx.Err() != nil {
			return
		}
		if !u.Enabled {
			continue
		}
		if u.VLESSUUID == nil && u.SOCKSUser == nil {
			continue
		}
		up, down, err := w.xray.GetUserStats(ctx, u.Name)
		if err != nil {
			if errors.Is(err, xray.ErrUserNotFound) {
				w.log.Debug("no xray stats yet", slog.String("user", u.Name))
				continue
			}
			w.log.Warn("get user stats", slog.String("user", u.Name), slog.Any("err", err))
			continue
		}
		w.prevStatsMu.Lock()
		prev := w.prevStats[u.ID]
		dUp := Delta(prev.up, up)
		dDown := Delta(prev.down, down)
		w.prevStats[u.ID] = stats{up: up, down: down}
		w.prevStatsMu.Unlock()

		if dUp == 0 && dDown == 0 {
			continue
		}
		if err := w.store.AddUserUsage(ctx, u.ID, dUp, dDown); err != nil {
			w.log.Warn("add user usage", slog.String("user", u.Name), slog.Any("err", err))
			continue
		}
		if err := w.store.RecordUsageDay(ctx, u.ID, today, dUp, dDown); err != nil {
			w.log.Warn("record usage day", slog.String("user", u.Name), slog.Any("err", err))
		}

		oldUsed := u.UsedBytes
		newUsed := u.UsedBytes + dUp + dDown
		w.handleQuota(ctx, u, oldUsed, newUsed)
	}
}

// handleQuota checks whether the (oldUsed -> newUsed) transition
// crossed the 80% or 100% threshold for u, and fires the
// appropriate notification. The 80% notification is fire-once-per-
// period, tracked by firedEighty; it is reset when the user's limit
// changes (detected externally — the bot calls w.ResetEightyFlag
// after a quota bump; MVP simply clears on quota=full as well).
func (w *Watcher) handleQuota(ctx context.Context, u store.User, oldUsed, newUsed int64) {
	ev := ClassifyQuota(oldUsed, newUsed, u.LimitBytes)
	switch ev {
	case QuotaEighty:
		w.firedEightyMu.Lock()
		already := w.firedEighty[u.ID]
		if !already {
			w.firedEighty[u.ID] = true
		}
		w.firedEightyMu.Unlock()
		if already {
			return
		}
		_ = w.notif.Notify(ctx, Event{
			ID:       "user.quota.80",
			Severity: Important,
			Params: map[string]any{
				"name":     u.Name,
				"limit":    u.LimitBytes,
				"used":     newUsed,
				"percent":  UsagePercent(newUsed, u.LimitBytes),
			},
			Buttons: [][]Button{{
				{Label: "+10 GB", Data: fmt.Sprintf("quota.add:%d:10", u.ID)},
				{Label: "+50 GB", Data: fmt.Sprintf("quota.add:%d:50", u.ID)},
			}},
		})
	case QuotaFull:
		// Disable and remove from Xray before notifying so the
		// admin's message reflects the terminal state.
		if err := w.store.SetUserEnabled(ctx, u.ID, false); err != nil {
			w.log.Error("disable user on quota", slog.String("user", u.Name), slog.Any("err", err))
		}
		if u.VLESSUUID != nil {
			if err := w.xray.RemoveVLESSUser(ctx, u.Name); err != nil && !errors.Is(err, xray.ErrUserNotFound) {
				w.log.Warn("remove vless on quota", slog.String("user", u.Name), slog.Any("err", err))
			}
		}
		if u.SOCKSUser != nil {
			if err := w.xray.RemoveSOCKSUser(ctx, u.Name); err != nil && !errors.Is(err, xray.ErrUserNotFound) {
				w.log.Warn("remove socks on quota", slog.String("user", u.Name), slog.Any("err", err))
			}
		}
		// Reset the 80% flag so a future re-enable + quota bump
		// correctly re-arms the threshold notification.
		w.firedEightyMu.Lock()
		delete(w.firedEighty, u.ID)
		w.firedEightyMu.Unlock()
		_ = w.notif.Notify(ctx, Event{
			ID:       "user.quota.100",
			Severity: Important,
			Params: map[string]any{
				"name":  u.Name,
				"limit": u.LimitBytes,
				"used":  newUsed,
			},
			Buttons: [][]Button{{
				{Label: "Re-enable", Data: fmt.Sprintf("user.enable:%d", u.ID)},
				{Label: "+10 GB & enable", Data: fmt.Sprintf("quota.add_enable:%d:10", u.ID)},
			}},
		})
	case QuotaNone:
		// Reset the 80% latch if usage dropped back under the
		// threshold (e.g. admin bumped the limit).
		if u.LimitBytes > 0 && newUsed < u.LimitBytes*80/100 {
			w.firedEightyMu.Lock()
			delete(w.firedEighty, u.ID)
			w.firedEightyMu.Unlock()
		}
	}
}

// ResetEightyFlag clears the fire-once latch for the 80% quota
// notification for userID. The bot should call this after a manual
// quota bump so the next time usage climbs back to 80% an admin
// is notified again.
func (w *Watcher) ResetEightyFlag(userID int64) {
	w.firedEightyMu.Lock()
	defer w.firedEightyMu.Unlock()
	delete(w.firedEighty, userID)
}

// runHealthLoop pings Xray, probes disk free space and checks
// Reality key age every cfg.HealthInterval.
func (w *Watcher) runHealthLoop(ctx context.Context) error {
	t := time.NewTicker(w.cfg.HealthInterval)
	defer t.Stop()
	w.pollHealth(ctx)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			w.pollHealth(ctx)
		}
	}
}

// pollHealth runs one round of health probes.
func (w *Watcher) pollHealth(ctx context.Context) {
	// Xray API liveness: 3 consecutive failures in a row trigger
	// xray.api.unreachable.
	if err := w.xray.Ping(ctx); err != nil {
		w.xrayPingFails++
		if w.xrayPingFails >= 3 && w.dedup.ShouldFire("xray.api.unreachable", 5*time.Minute) {
			_ = w.notif.Notify(ctx, Event{
				ID:       "xray.api.unreachable",
				Severity: Critical,
				Params: map[string]any{
					"attempts": w.xrayPingFails,
					"err":      err.Error(),
				},
			})
		}
	} else if w.xrayPingFails > 0 {
		w.xrayPingFails = 0
		w.dedup.Reset("xray.api.unreachable")
	}

	for _, mount := range w.mountPoints {
		free, err := diskFree(mount)
		if err != nil {
			w.log.Debug("statfs failed", slog.String("mount", mount), slog.Any("err", err))
			continue
		}
		const oneGiB = int64(1 << 30)
		if free < oneGiB && w.dedup.ShouldFire("host.disk_low:"+mount, 6*time.Hour) {
			_ = w.notif.Notify(ctx, Event{
				ID:       "host.disk_low",
				Severity: Important,
				Params: map[string]any{
					"mount": mount,
					"free":  free,
				},
			})
		}
	}

	if keys, err := w.store.GetRealityKeys(ctx); err == nil {
		age := w.now().Sub(keys.CreatedAt.UTC())
		if age > 90*24*time.Hour && w.dedup.ShouldFire("reality.key.age_warn", 7*24*time.Hour) {
			_ = w.notif.Notify(ctx, Event{
				ID:       "reality.key.age_warn",
				Severity: Important,
				Params: map[string]any{
					"age_days": int(age.Hours() / 24),
				},
			})
		}
	}

	// TODO(M5/warp): probe a well-known upstream via Warp and fire
	// warp.outbound.down on 3 consecutive failures. Deferred while
	// the Warp probe surface stabilises.
}

// runScheduledLoop drives the per-admin daily and weekly digests.
// It ticks every minute and fires when the current hh:00 matches an
// admin's configured digest_hour (+ Sunday for weekly).
func (w *Watcher) runScheduledLoop(ctx context.Context) error {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			w.sendDigestsIfDue(ctx)
		}
	}
}

// sendDigestsIfDue iterates admin prefs and sends daily/weekly
// digests whose scheduled hour just arrived.
func (w *Watcher) sendDigestsIfDue(ctx context.Context) {
	now := w.now()
	if now.Minute() != 0 {
		return // only fire on the top of the hour
	}
	prefs, err := w.store.ListAdminPrefs(ctx)
	if err != nil {
		w.log.Warn("list admin prefs", slog.Any("err", err))
		return
	}
	digest, err := computeDailyDigest(ctx, w.store, now)
	if err != nil {
		w.log.Warn("compute daily digest", slog.Any("err", err))
		return
	}
	dayKey := now.Format("2006-01-02")
	for _, p := range prefs {
		hour := p.DigestHour
		if hour < 0 || hour > 23 {
			hour = w.cfg.DigestHour
		}
		if now.Hour() != hour {
			continue
		}
		w.lastDigestMu.Lock()
		alreadySent := w.lastDigestSent[p.TGID] == dayKey
		if !alreadySent {
			w.lastDigestSent[p.TGID] = dayKey
		}
		w.lastDigestMu.Unlock()
		if alreadySent {
			continue
		}

		if p.NotifyDaily {
			_ = w.notif.Notify(ctx, Event{
				ID:       "digest.daily",
				Severity: Scheduled,
				Params:   digestParamsForAdmin(digest, p.TGID),
			})
		}
		if now.Weekday() == time.Sunday {
			_ = w.notif.Notify(ctx, Event{
				ID:       "digest.weekly",
				Severity: Scheduled,
				Params:   w.weeklyParams(ctx, p.TGID),
			})
		}
	}
}

// digestParamsForAdmin decorates the shared DailyDigest params with
// the target admin's TG id so the notifier can do per-admin routing.
func digestParamsForAdmin(d DailyDigest, tgID int64) map[string]any {
	p := d.ToParams()
	p["tg_id"] = tgID
	return p
}

// weeklyParams assembles the digest.weekly payload. It deliberately
// reuses the health probes' state (disk free, reality age) rather
// than re-fetching, to keep the digest cheap.
func (w *Watcher) weeklyParams(ctx context.Context, tgID int64) map[string]any {
	out := map[string]any{"tg_id": tgID}
	if keys, err := w.store.GetRealityKeys(ctx); err == nil {
		out["reality_age_days"] = int(w.now().Sub(keys.CreatedAt.UTC()).Hours() / 24)
	}
	for _, mount := range w.mountPoints {
		if free, err := diskFree(mount); err == nil {
			out["disk_free_"+mount] = free
		}
	}
	return out
}

// diskFree returns the number of free bytes on the filesystem that
// contains path, via syscall.Statfs. It is split out so tests can
// stub it when needed (the test file shadows this symbol).
var diskFree = func(path string) (int64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, err
	}
	// Bavail is the count of blocks available to unprivileged
	// users; multiplied by the filesystem block size it is the
	// free-space figure admins expect from `df`.
	return int64(st.Bavail) * int64(st.Bsize), nil
}
