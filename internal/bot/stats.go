package bot

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-telegram/bot/models"
)

// showStats renders the statistics screen.
//
// Data sources:
//   - total traffic = sum of usage_history rows (today, across all users)
//   - active/disabled counts from ListUsers
//   - top-3 users by bytes from GetTopUsersByUsage
func (b *Bot) showStats(ctx context.Context, update *models.Update) error {
	tgID := updateTGID(update)
	sess, err := b.sessions.Get(ctx, tgID)
	if err != nil {
		return err
	}
	sess.TGID = tgID
	sess.Screen = "stats"
	if sess.ChatID == 0 {
		sess.ChatID = updateChatID(update)
	}
	lang := b.adminLang(ctx, tgID)

	var up, down int64
	day := time.Now().UTC().Truncate(24 * time.Hour)
	users, err := b.deps.Store.ListUsers(ctx, true)
	if err != nil {
		return err
	}
	active, total := 0, len(users)
	for _, u := range users {
		if u.Enabled {
			active++
		}
		rows, err := b.deps.Store.GetUsageSince(ctx, u.ID, day)
		if err != nil {
			continue
		}
		for _, r := range rows {
			up += r.UplinkBytes
			down += r.DownlinkBytes
		}
	}
	top, _ := b.deps.Store.GetTopUsersByUsage(ctx, 3, day)

	var sb strings.Builder
	fmt.Fprintf(&sb, "<b>%s</b>\n", b.deps.I18n.T(lang, "stats.title"))
	sb.WriteString(b.deps.I18n.T(lang, "stats.total", humanBytes(up+down), humanBytes(up), humanBytes(down)))
	sb.WriteString("\n")
	sb.WriteString(b.deps.I18n.T(lang, "stats.active", active, total))
	sb.WriteString("\n\n")
	if len(top) == 0 {
		sb.WriteString(b.deps.I18n.T(lang, "stats.no_data"))
	} else {
		sb.WriteString(b.deps.I18n.T(lang, "stats.top_users"))
		sb.WriteString("\n")
		for _, t := range top {
			sb.WriteString(b.deps.I18n.T(lang, "digest.daily.top_line", t.Name, humanBytes(t.TotalBytes)))
			sb.WriteString("\n")
		}
	}

	kb := markup(backRow(b.deps.I18n.T(lang, "menu.back")))
	return b.sessions.Edit(ctx, b.tg, &sess, sb.String(), kb)
}
