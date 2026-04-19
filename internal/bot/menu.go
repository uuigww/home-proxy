package bot

import (
	"context"
	"fmt"
	"time"

	"github.com/go-telegram/bot/models"

	"github.com/uuigww/home-proxy/internal/store"
)

// showMainMenu renders the home screen with the summary line and the four
// top-level action buttons.
//
// When firstInteraction is true the session is (re-)seeded with chat_id so
// subsequent edits target the right chat.
func (b *Bot) showMainMenu(ctx context.Context, update *models.Update, firstInteraction bool) error {
	tgID := updateTGID(update)
	sess, err := b.sessions.Get(ctx, tgID)
	if err != nil {
		return err
	}
	sess.TGID = tgID
	sess.Screen = "main"
	if firstInteraction {
		sess.ChatID = updateChatID(update)
		sess.MessageID = 0
		ClearWizard(&sess)
	} else if sess.ChatID == 0 {
		sess.ChatID = updateChatID(update)
	}

	lang := b.adminLang(ctx, tgID)
	summary, err := b.mainSummary(ctx, lang)
	if err != nil {
		b.deps.Log.Warn("bot: main summary", "err", err)
		summary = ""
	}

	text := fmt.Sprintf("<b>%s</b>\n%s",
		b.deps.I18n.T(lang, "menu.title"),
		summary,
	)

	kb := markup(
		kbRow(
			btn(b.deps.I18n.T(lang, "menu.users"), CBUsersList+"0"),
			btn(b.deps.I18n.T(lang, "menu.stats"), CBStatsMain),
		),
		kbRow(
			btn(b.deps.I18n.T(lang, "menu.server"), CBServerMain),
			btn(b.deps.I18n.T(lang, "menu.help"), CBHelp),
		),
	)
	return b.sessions.Edit(ctx, b.tg, &sess, text, kb)
}

// mainSummary builds the "Active: N · Today: X GB" one-liner.
func (b *Bot) mainSummary(ctx context.Context, lang string) (string, error) {
	users, err := b.deps.Store.ListUsers(ctx, false)
	if err != nil {
		return "", err
	}
	active := 0
	for _, u := range users {
		if u.Enabled {
			active++
		}
	}
	today := todayTraffic(ctx, b.deps.Store)
	return b.deps.I18n.T(lang, "menu.summary", active, humanBytes(today)), nil
}

// todayTraffic sums uplink+downlink across all users for the current UTC day.
// Failures degrade silently to 0 — the summary should never block the menu.
func todayTraffic(ctx context.Context, s *store.Store) int64 {
	if s == nil {
		return 0
	}
	day := time.Now().UTC().Truncate(24 * time.Hour)
	users, err := s.ListUsers(ctx, true)
	if err != nil {
		return 0
	}
	var total int64
	for _, u := range users {
		rows, err := s.GetUsageSince(ctx, u.ID, day)
		if err != nil {
			continue
		}
		for _, r := range rows {
			total += r.UplinkBytes + r.DownlinkBytes
		}
	}
	return total
}

// showHelp renders the help screen.
func (b *Bot) showHelp(ctx context.Context, update *models.Update) error {
	tgID := updateTGID(update)
	sess, err := b.sessions.Get(ctx, tgID)
	if err != nil {
		return err
	}
	sess.TGID = tgID
	sess.Screen = "help"
	if sess.ChatID == 0 {
		sess.ChatID = updateChatID(update)
	}
	lang := b.adminLang(ctx, tgID)
	text := fmt.Sprintf("<b>%s</b>\n\n%s",
		b.deps.I18n.T(lang, "help.title"),
		b.deps.I18n.T(lang, "help.body"))
	kb := markup(backRow(b.deps.I18n.T(lang, "menu.back")))
	return b.sessions.Edit(ctx, b.tg, &sess, text, kb)
}
