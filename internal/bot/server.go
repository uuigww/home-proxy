package bot

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/go-telegram/bot/models"

	"github.com/uuigww/home-proxy/internal/store"
)

// showServer renders the ⚙️ Server status + actions screen.
func (b *Bot) showServer(ctx context.Context, update *models.Update) error {
	tgID := updateTGID(update)
	sess, err := b.sessions.Get(ctx, tgID)
	if err != nil {
		return err
	}
	sess.TGID = tgID
	sess.Screen = "server"
	if sess.ChatID == 0 {
		sess.ChatID = updateChatID(update)
	}
	lang := b.adminLang(ctx, tgID)

	xrayLbl := b.deps.I18n.T(lang, "server.xray_online")
	if b.deps.Xray != nil {
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		if err := b.deps.Xray.Ping(pingCtx); err != nil {
			xrayLbl = b.deps.I18n.T(lang, "server.xray_offline")
		}
		cancel()
	}

	realityLbl := "—"
	reality, err := b.deps.Store.GetRealityKeys(ctx)
	if err == nil {
		age := time.Since(reality.CreatedAt)
		realityLbl = b.deps.I18n.T(lang, "server.reality_age", humanDuration(age))
	} else if !errors.Is(err, store.ErrNotFound) {
		b.deps.Log.Warn("bot: reality keys", "err", err)
	}

	warpLbl := b.deps.I18n.T(lang, "server.warp_missing")
	if _, err := b.deps.Store.GetWarpPeer(ctx); err == nil {
		warpLbl = b.deps.I18n.T(lang, "server.warp_ok")
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "<b>%s</b>\n", b.deps.I18n.T(lang, "server.title"))
	sb.WriteString(b.deps.I18n.T(lang, "server.xray_status", xrayLbl))
	sb.WriteString("\n")
	sb.WriteString(b.deps.I18n.T(lang, "server.reality_keys", realityLbl))
	sb.WriteString("\n")
	sb.WriteString(b.deps.I18n.T(lang, "server.warp_status", warpLbl))
	sb.WriteString("\n")
	b.appendMTProtoStatus(ctx, &sb, lang)

	rows := [][]models.InlineKeyboardButton{
		kbRow(btn(b.deps.I18n.T(lang, "server.rotate_reality"), CBServerRotate)),
	}
	if b.deps.Cfg.MTProtoEnabled {
		rows = append(rows, kbRow(btn(b.deps.I18n.T(lang, "server.mtproto_rotate"), CBServerRotateMTProto)))
	}
	rows = append(rows,
		kbRow(btn(b.deps.I18n.T(lang, "server.update_geo"), CBServerUpdateGeo)),
		kbRow(btn(b.deps.I18n.T(lang, "server.notifications"), CBServerNotifications)),
		backRow(b.deps.I18n.T(lang, "menu.back")),
	)
	kb := markup(rows...)
	return b.sessions.Edit(ctx, b.tg, &sess, sb.String(), kb)
}

// appendMTProtoStatus renders a compact MTProto info block on the server
// screen — status, port, Fake-TLS host, truncated secret tail — or a single
// "not installed" hint when the operator didn't opt in at install time.
func (b *Bot) appendMTProtoStatus(ctx context.Context, sb *strings.Builder, lang string) {
	if !b.deps.Cfg.MTProtoEnabled {
		sb.WriteString(b.deps.I18n.T(lang, "server.mtproto_disabled"))
		sb.WriteString("\n")
		return
	}
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	status := b.deps.I18n.T(lang, "server.xray_offline")
	if out, err := exec.CommandContext(pingCtx, "systemctl", "is-active", "home-proxy-mtg.service").Output(); err == nil {
		if strings.TrimSpace(string(out)) == "active" {
			status = b.deps.I18n.T(lang, "server.xray_online")
		}
	}
	sb.WriteString(b.deps.I18n.T(lang, "server.mtproto_status", status))
	sb.WriteString("\n")
	sb.WriteString(b.deps.I18n.T(lang, "server.mtproto_port", b.deps.Cfg.MTProtoPort))
	sb.WriteString("\n")
	sb.WriteString(b.deps.I18n.T(lang, "server.mtproto_fake_tls", b.deps.Cfg.MTProtoFakeTLSHost))
	sb.WriteString("\n")
	if mtg, err := b.deps.Store.GetMTGConfig(ctx); err == nil && mtg.Secret != "" {
		sb.WriteString(b.deps.I18n.T(lang, "server.mtproto_secret", truncateSecret(mtg.Secret)))
		sb.WriteString("\n")
	}
}

// truncateSecret returns the last 6 hex chars of secret for visual display.
// Never log or expose the full secret — revocation relies on it being opaque.
func truncateSecret(secret string) string {
	if len(secret) <= 6 {
		return "***"
	}
	return "…" + secret[len(secret)-6:]
}

// showNotifSettings renders the 🔔 Notifications screen for the admin's own
// preferences.
func (b *Bot) showNotifSettings(ctx context.Context, update *models.Update) error {
	tgID := updateTGID(update)
	sess, err := b.sessions.Get(ctx, tgID)
	if err != nil {
		return err
	}
	sess.TGID = tgID
	sess.Screen = "notif"
	if sess.ChatID == 0 {
		sess.ChatID = updateChatID(update)
	}
	lang := b.adminLang(ctx, tgID)

	prefs, err := b.deps.Store.GetAdminPrefs(ctx, tgID)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			return err
		}
		prefs = store.NewDefaultAdminPrefs(tgID, lang)
		_ = b.deps.Store.UpsertAdminPrefs(ctx, prefs)
	}

	on := b.deps.I18n.T(lang, "notif.settings.on")
	off := b.deps.I18n.T(lang, "notif.settings.off")

	label := func(baseKey string, v bool) string {
		base := b.deps.I18n.T(lang, baseKey)
		if v {
			return base + " · " + on
		}
		return base + " · " + off
	}

	text := "<b>" + b.deps.I18n.T(lang, "notif.settings.title") + "</b>"
	kb := markup(
		kbRow(btn(label("notif.settings.critical", prefs.NotifyCritical), CBNotifToggleCritical)),
		kbRow(btn(label("notif.settings.important", prefs.NotifyImportant), CBNotifToggleImportant)),
		kbRow(btn(label("notif.settings.info", prefs.NotifyInfo), CBNotifToggleInfo)),
		kbRow(btn(label("notif.settings.info_others_only", prefs.NotifyInfoOthersOnly), CBNotifToggleOthers)),
		kbRow(btn(label("notif.settings.security", prefs.NotifySecurity), CBNotifToggleSecurity)),
		kbRow(btn(label("notif.settings.daily", prefs.NotifyDaily), CBNotifToggleDaily)),
		kbRow(btn(b.deps.I18n.T(lang, "notif.settings.digest_hour", prefs.DigestHour), CBNoop)),
		kbRow(btn(b.deps.I18n.T(lang, "notif.settings.quiet_hours", prefs.QuietFromHour, prefs.QuietToHour), CBNoop)),
		backRow(b.deps.I18n.T(lang, "menu.back")),
	)
	return b.sessions.Edit(ctx, b.tg, &sess, text, kb)
}

// toggleNotifPref flips one notification preference and re-renders.
func (b *Bot) toggleNotifPref(ctx context.Context, update *models.Update, which string) error {
	tgID := updateTGID(update)
	lang := b.adminLang(ctx, tgID)
	prefs, err := b.deps.Store.GetAdminPrefs(ctx, tgID)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			return err
		}
		prefs = store.NewDefaultAdminPrefs(tgID, lang)
	}
	switch which {
	case "critical":
		prefs.NotifyCritical = !prefs.NotifyCritical
	case "important":
		prefs.NotifyImportant = !prefs.NotifyImportant
	case "info":
		prefs.NotifyInfo = !prefs.NotifyInfo
	case "others":
		prefs.NotifyInfoOthersOnly = !prefs.NotifyInfoOthersOnly
	case "security":
		prefs.NotifySecurity = !prefs.NotifySecurity
	case "daily":
		prefs.NotifyDaily = !prefs.NotifyDaily
	}
	if err := b.deps.Store.UpsertAdminPrefs(ctx, prefs); err != nil {
		return err
	}
	return b.showNotifSettings(ctx, update)
}

// humanDuration is a coarse "age" renderer suitable for screen labels.
// Durations are printed as Nd, Nh or Nm depending on magnitude.
func humanDuration(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	days := int(d / (24 * time.Hour))
	if days > 0 {
		return fmt.Sprintf("%dd", days)
	}
	hours := int(d / time.Hour)
	if hours > 0 {
		return fmt.Sprintf("%dh", hours)
	}
	mins := int(d / time.Minute)
	if mins < 1 {
		mins = 1
	}
	return fmt.Sprintf("%dm", mins)
}
