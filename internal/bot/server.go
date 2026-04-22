package bot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/go-telegram/bot/models"

	"github.com/uuigww/home-proxy/internal/store"
	"github.com/uuigww/home-proxy/internal/version"
	"github.com/uuigww/home-proxy/internal/xray"
)

// showServer renders the ⚙️ Server status + actions screen.
func (b *Bot) showServer(ctx context.Context, update *models.Update) error {
	return b.renderServer(ctx, update, "")
}

// checkUpdates fetches the latest release tag from GitHub and re-renders the
// server screen with an update-status line appended.
func (b *Bot) checkUpdates(ctx context.Context, update *models.Update) error {
	lang := b.adminLang(ctx, updateTGID(update))

	reqCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet,
		"https://api.github.com/repos/uuigww/home-proxy/releases/latest", nil)
	if err != nil {
		return fmt.Errorf("update check: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "home-proxy-bot")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("update check: %w", err)
	}
	defer resp.Body.Close()

	var release struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("update check decode: %w", err)
	}

	latest := strings.TrimPrefix(release.TagName, "v")
	current := strings.TrimPrefix(version.Version, "v")

	var updateLine string
	if current == "dev" || latest == current {
		updateLine = b.deps.I18n.T(lang, "server.update_ok", version.Version)
	} else {
		updateLine = b.deps.I18n.T(lang, "server.update_available", version.Version, release.TagName)
	}
	return b.renderServer(ctx, update, updateLine)
}

// renderServer renders the server screen. updateLine is an optional extra line
// shown below the status block (used by checkUpdates to show update info).
func (b *Bot) renderServer(ctx context.Context, update *models.Update, updateLine string) error {
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
	sb.WriteString(b.deps.I18n.T(lang, "server.version", version.Version))
	sb.WriteString("\n")
	sb.WriteString(b.deps.I18n.T(lang, "server.xray_status", xrayLbl))
	sb.WriteString("\n")
	sb.WriteString(b.deps.I18n.T(lang, "server.reality_keys", realityLbl))
	sb.WriteString("\n")
	sb.WriteString(b.deps.I18n.T(lang, "server.warp_status", warpLbl))
	sb.WriteString("\n")
	b.appendMTProtoStatus(ctx, &sb, lang)
	if updateLine != "" {
		sb.WriteString(updateLine)
		sb.WriteString("\n")
	}

	rows := [][]models.InlineKeyboardButton{
		kbRow(btn(b.deps.I18n.T(lang, "server.rotate_reality"), CBServerRotate)),
	}
	if b.deps.Cfg.MTProtoEnabled {
		rows = append(rows, kbRow(btn(b.deps.I18n.T(lang, "server.mtproto_rotate"), CBServerRotateMTProto)))
	}
	rows = append(rows,
		kbRow(btn(b.deps.I18n.T(lang, "server.update_geo"), CBServerUpdateGeo)),
		kbRow(btn(b.deps.I18n.T(lang, "server.check_updates"), CBServerCheckUpdates),
			btn(b.deps.I18n.T(lang, "server.self_update"), CBServerSelfUpdate)),
		kbRow(btn(b.deps.I18n.T(lang, "server.notifications"), CBServerNotifications)),
		backRow(b.deps.I18n.T(lang, "menu.back")),
	)
	kb := markup(rows...)
	return b.sessions.Edit(ctx, b.tg, &sess, sb.String(), kb)
}

// rotateReality generates a fresh Reality keypair, persists it, rebuilds and
// writes the xray config, restarts xray, and re-renders the server screen.
func (b *Bot) rotateReality(ctx context.Context, update *models.Update) error {
	tgID := updateTGID(update)
	lang := b.adminLang(ctx, tgID)

	reality, err := xray.GenerateReality()
	if err != nil {
		return fmt.Errorf("generate reality: %w", err)
	}
	reality.Dest = b.deps.Cfg.RealityDest
	reality.ServerName = b.deps.Cfg.RealityServerName

	rk := store.RealityKeys{
		PrivateKey: reality.PrivateKey,
		PublicKey:  reality.PublicKey,
		ShortID:    reality.ShortID,
		Dest:       reality.Dest,
		ServerName: reality.ServerName,
	}
	if err := b.deps.Store.SaveRealityKeys(ctx, rk); err != nil {
		return fmt.Errorf("save reality keys: %w", err)
	}
	if err := b.applyXrayConfig(ctx, reality); err != nil {
		return fmt.Errorf("apply xray config: %w", err)
	}
	b.notifyInfo(ctx, update, "reality.rotated", tgID)
	return b.renderServer(ctx, update, b.deps.I18n.T(lang, "server.rotate_reality_done"))
}

// applyXrayConfig rebuilds the xray config.json from current DB state and
// restarts the xray systemd unit. reality overrides whatever is in the DB so
// callers can pass freshly-generated keys before they are persisted.
func (b *Bot) applyXrayConfig(ctx context.Context, reality xray.Reality) error {
	users, err := b.deps.Store.ListUsers(ctx, true)
	if err != nil {
		return fmt.Errorf("list users: %w", err)
	}
	xUsers := make([]xray.User, 0, len(users))
	for _, u := range users {
		xu := xray.User{
			Name:    u.Name,
			Enabled: u.Enabled,
		}
		if u.VLESSUUID != nil {
			xu.VLESSUUID = *u.VLESSUUID
		}
		if u.SOCKSUser != nil {
			xu.SOCKSUser = *u.SOCKSUser
		}
		if u.SOCKSPass != nil {
			xu.SOCKSPass = *u.SOCKSPass
		}
		xUsers = append(xUsers, xu)
	}

	var warp xray.WarpPeer
	if wp, err := b.deps.Store.GetWarpPeer(ctx); err == nil {
		warp = xray.WarpPeer{
			PrivateKey:    wp.PrivateKey,
			PeerPublicKey: wp.PeerPublicKey,
			IPv4:          wp.IPv4,
			IPv6:          wp.IPv6,
			Endpoint:      wp.Endpoint,
			MTU:           wp.MTU,
		}
	}

	cfg, err := xray.Generate(xray.GenInput{
		Users:       xUsers,
		Reality:     reality,
		Warp:        warp,
		SOCKSPort:   b.deps.Cfg.SOCKSPort,
		RealityPort: b.deps.Cfg.RealityPort,
		API:         b.deps.Cfg.XrayAPI,
	})
	if err != nil {
		return fmt.Errorf("generate xray config: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal xray config: %w", err)
	}
	cfgPath := b.deps.Cfg.XrayConfig
	if cfgPath == "" {
		cfgPath = "/usr/local/etc/xray/config.json"
	}
	if err := os.WriteFile(cfgPath, data, 0644); err != nil {
		return fmt.Errorf("write xray config: %w", err)
	}
	if out, err := exec.CommandContext(ctx, "systemctl", "restart", "xray").CombinedOutput(); err != nil {
		return fmt.Errorf("restart xray: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// updateGeosite downloads the latest geoip.dat and geosite.dat from GitHub
// into the xray data directory, then restarts xray.
func (b *Bot) updateGeosite(ctx context.Context, update *models.Update) error {
	lang := b.adminLang(ctx, updateTGID(update))

	dlCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	files := []struct{ url, dest string }{
		{"https://github.com/v2fly/geoip/releases/latest/download/geoip.dat", "/usr/local/share/xray/geoip.dat"},
		{"https://github.com/v2fly/domain-list-community/releases/latest/download/dlc.dat", "/usr/local/share/xray/geosite.dat"},
	}
	for _, f := range files {
		if err := downloadFile(dlCtx, f.url, f.dest); err != nil {
			return fmt.Errorf("download %s: %w", f.url, err)
		}
	}
	if out, err := exec.CommandContext(ctx, "systemctl", "restart", "xray").CombinedOutput(); err != nil {
		return fmt.Errorf("restart xray: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	return b.renderServer(ctx, update, b.deps.I18n.T(lang, "server.geo_updated"))
}

// selfUpdate downloads the latest home-proxy binary from GitHub releases,
// replaces the running binary, and restarts the service via systemd.
//
// The edit to the Telegram message happens BEFORE the restart so the admin
// sees confirmation before the bot process is replaced.
func (b *Bot) selfUpdate(ctx context.Context, update *models.Update) error {
	lang := b.adminLang(ctx, updateTGID(update))

	dlCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// Fetch release info.
	req, _ := http.NewRequestWithContext(dlCtx, http.MethodGet,
		"https://api.github.com/repos/uuigww/home-proxy/releases/latest", nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "home-proxy-bot")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetch release: %w", err)
	}
	defer resp.Body.Close()

	var release struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("decode release: %w", err)
	}

	// Find the linux binary asset for the current GOARCH.
	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "x86_64"
	}
	wantSuffix := fmt.Sprintf("linux_%s", arch)
	var downloadURL string
	for _, a := range release.Assets {
		if strings.Contains(a.Name, wantSuffix) && strings.HasSuffix(a.Name, ".tar.gz") {
			downloadURL = a.BrowserDownloadURL
			break
		}
		// also match plain binary names
		if strings.Contains(a.Name, "linux") && strings.Contains(a.Name, arch) {
			downloadURL = a.BrowserDownloadURL
			break
		}
	}
	if downloadURL == "" {
		return fmt.Errorf("no linux/%s asset in release %s", runtime.GOARCH, release.TagName)
	}

	// Download to temp file.
	tmp, err := os.CreateTemp("", "home-proxy-update-*")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	dlReq, _ := http.NewRequestWithContext(dlCtx, http.MethodGet, downloadURL, nil)
	dlResp, err := http.DefaultClient.Do(dlReq)
	if err != nil {
		tmp.Close()
		return fmt.Errorf("download binary: %w", err)
	}
	defer dlResp.Body.Close()
	if _, err := io.Copy(tmp, dlResp.Body); err != nil {
		tmp.Close()
		return fmt.Errorf("write binary: %w", err)
	}
	tmp.Close()

	if err := os.Chmod(tmpPath, 0755); err != nil {
		return fmt.Errorf("chmod: %w", err)
	}

	// Rename over the current binary atomically.
	binPath := "/usr/local/bin/home-proxy"
	newPath := binPath + ".new"
	if err := os.Rename(tmpPath, newPath); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	if err := os.Rename(newPath, binPath); err != nil {
		return fmt.Errorf("install: %w", err)
	}

	// Tell the admin we're restarting, then do it.
	_ = b.renderServer(ctx, update,
		b.deps.I18n.T(lang, "server.self_update_restarting", release.TagName))
	exec.Command("systemctl", "restart", "home-proxy").Start() //nolint:errcheck
	return nil
}

// downloadFile fetches url and writes the body atomically to dest.
func downloadFile(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	tmp, err := os.CreateTemp("", "geo-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()
	return os.Rename(tmpPath, dest)
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
