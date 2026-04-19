package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-telegram/bot/models"

	"github.com/uuigww/home-proxy/internal/store"
)

// userListPageSize is the number of users shown per page in the list.
const userListPageSize = 5

// showUsersList renders the paginated list of users. pageStr is the raw
// callback payload ("" or a decimal integer); out-of-range values clamp.
func (b *Bot) showUsersList(ctx context.Context, update *models.Update, pageStr string) error {
	tgID := updateTGID(update)
	sess, err := b.sessions.Get(ctx, tgID)
	if err != nil {
		return err
	}
	sess.TGID = tgID
	sess.Screen = "users_list"
	if sess.ChatID == 0 {
		sess.ChatID = updateChatID(update)
	}

	lang := b.adminLang(ctx, tgID)
	page, _ := strconv.Atoi(pageStr)
	if page < 0 {
		page = 0
	}

	users, err := b.deps.Store.ListUsers(ctx, true)
	if err != nil {
		return fmt.Errorf("list users: %w", err)
	}

	totalPages := (len(users) + userListPageSize - 1) / userListPageSize
	if totalPages == 0 {
		totalPages = 1
	}
	if page >= totalPages {
		page = totalPages - 1
	}

	start := page * userListPageSize
	end := start + userListPageSize
	if end > len(users) {
		end = len(users)
	}
	visible := users[start:end]

	header := b.deps.I18n.T(lang, "users.list.title", page+1, totalPages)
	var body strings.Builder
	body.WriteString("<b>")
	body.WriteString(header)
	body.WriteString("</b>\n")
	if len(users) == 0 {
		body.WriteString(b.deps.I18n.T(lang, "users.list.empty"))
	}

	rows := [][]models.InlineKeyboardButton{}
	for _, u := range visible {
		rows = append(rows, kbRow(btn(userListRow(b, lang, u), CBUserCard+itoa(u.ID))))
	}

	nav := []models.InlineKeyboardButton{}
	if page > 0 {
		nav = append(nav, btn(b.deps.I18n.T(lang, "users.list.page_prev"), CBUsersList+itoa(int64(page-1))))
	}
	if page+1 < totalPages {
		nav = append(nav, btn(b.deps.I18n.T(lang, "users.list.page_next"), CBUsersList+itoa(int64(page+1))))
	}
	if len(nav) > 0 {
		rows = append(rows, nav)
	}

	rows = append(rows, kbRow(btn(b.deps.I18n.T(lang, "users.list.add"), CBAddStart)))
	rows = append(rows, backRow(b.deps.I18n.T(lang, "menu.back")))

	return b.sessions.Edit(ctx, b.tg, &sess, body.String(), markup(rows...))
}

// userListRow renders a single row "👤 name · status · used/limit".
func userListRow(b *Bot, lang string, u store.User) string {
	status := "✅"
	if !u.Enabled {
		status = "🚫"
	}
	return fmt.Sprintf("👤 %s · %s · %s/%s",
		u.Name, status,
		humanBytes(u.UsedBytes), limitHuman(b, lang, u.LimitBytes),
	)
}

// itoa converts a signed integer to its decimal representation — a typed
// convenience wrapper so callsites don't need to import strconv.
func itoa[T ~int | ~int64](v T) string {
	return strconv.FormatInt(int64(v), 10)
}
