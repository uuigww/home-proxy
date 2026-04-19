package bot

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/uuigww/home-proxy/internal/config"
	"github.com/uuigww/home-proxy/internal/i18n"
	"github.com/uuigww/home-proxy/internal/store"
	"github.com/uuigww/home-proxy/internal/xray"
)

// Deps is the collection of collaborators the Bot needs.
type Deps struct {
	Store       *store.Store
	Xray        xray.Client
	I18n        *i18n.Bundle
	Admins      []int64
	DefaultLang string
	Cfg         config.Config
	Log         *slog.Logger
}

// Bot is the Telegram bot wrapper.
//
// The zero value is not usable; construct via New.
type Bot struct {
	deps     Deps
	tg       *bot.Bot
	sessions *SessionMgr
	notifier *Notifier
}

// New wires a Telegram client around deps and registers all handlers.
func New(ctx context.Context, token string, deps Deps) (*Bot, error) {
	if deps.Log == nil {
		deps.Log = slog.Default()
	}
	if deps.I18n == nil {
		return nil, fmt.Errorf("bot: i18n bundle is required")
	}
	if deps.Store == nil {
		return nil, fmt.Errorf("bot: store is required")
	}
	if token == "" {
		return nil, fmt.Errorf("bot: empty token")
	}

	b := &Bot{
		deps:     deps,
		sessions: NewSessionMgr(deps.Store),
	}

	opts := []bot.Option{
		bot.WithDefaultHandler(b.defaultHandler),
		bot.WithMiddlewares(b.adminMiddleware, b.recoverMiddleware),
	}
	tg, err := bot.New(token, opts...)
	if err != nil {
		return nil, fmt.Errorf("bot: new: %w", err)
	}
	b.tg = tg
	b.notifier = NewNotifier(b.tg, deps.Store, deps.I18n, deps.Admins, deps.DefaultLang, deps.Log)

	tg.RegisterHandler(bot.HandlerTypeMessageText, "/start", bot.MatchTypeExact, b.handleStart)
	tg.RegisterHandler(bot.HandlerTypeMessageText, "/menu", bot.MatchTypeExact, b.handleStart)

	return b, nil
}

// Start begins polling Telegram. It blocks until ctx is cancelled. The
// notifier's quiet-hour outbox is also flushed in the background.
func (b *Bot) Start(ctx context.Context) error {
	if b == nil || b.tg == nil {
		return fmt.Errorf("bot: not initialised")
	}
	go b.notifier.flushQuietLoop(ctx)
	b.tg.Start(ctx)
	return nil
}

// Notifier returns the bot's event notifier.
func (b *Bot) Notifier() *Notifier { return b.notifier }

// isAdmin returns true when tgID appears in deps.Admins.
func (b *Bot) isAdmin(tgID int64) bool {
	for _, id := range b.deps.Admins {
		if id == tgID {
			return true
		}
	}
	return false
}

// updateTGID extracts the originating Telegram user id from any update,
// regardless of whether it's a message, callback or channel post.
func updateTGID(u *models.Update) int64 {
	if u == nil {
		return 0
	}
	switch {
	case u.Message != nil && u.Message.From != nil:
		return u.Message.From.ID
	case u.CallbackQuery != nil:
		return u.CallbackQuery.From.ID
	}
	return 0
}

// updateChatID returns the chat id where the update originated.
func updateChatID(u *models.Update) int64 {
	if u == nil {
		return 0
	}
	if u.Message != nil {
		return u.Message.Chat.ID
	}
	if u.CallbackQuery != nil {
		if m := u.CallbackQuery.Message.Message; m != nil {
			return m.Chat.ID
		}
	}
	return 0
}

// adminMiddleware silently drops every update from a non-admin. Admins fall
// through to the downstream handler.
func (b *Bot) adminMiddleware(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, tg *bot.Bot, update *models.Update) {
		tgID := updateTGID(update)
		if tgID == 0 {
			return
		}
		if !b.isAdmin(tgID) {
			b.deps.Log.Debug("bot: ignoring non-admin update", "tg_id", tgID)
			return
		}
		next(ctx, tg, update)
	}
}

// recoverMiddleware captures panics inside handlers so one bad update can't
// take the daemon down.
func (b *Bot) recoverMiddleware(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, tg *bot.Bot, update *models.Update) {
		defer func() {
			if r := recover(); r != nil {
				b.deps.Log.Error("bot: panic in handler", "recovered", r)
			}
		}()
		next(ctx, tg, update)
	}
}

// defaultHandler is invoked for every update that didn't match a specific
// registered handler. It dispatches callbacks by prefix and treats any plain
// text message as potential wizard input.
func (b *Bot) defaultHandler(ctx context.Context, tg *bot.Bot, update *models.Update) {
	if update == nil {
		return
	}
	if update.CallbackQuery != nil {
		if err := b.dispatchCallback(ctx, update); err != nil {
			b.deps.Log.Error("bot: callback failed", "err", err, "data", update.CallbackQuery.Data)
			b.answerCallback(ctx, update.CallbackQuery.ID, b.tr(ctx, update, "err.generic"))
		}
		return
	}
	if update.Message != nil && update.Message.Text != "" {
		if err := b.handleText(ctx, update); err != nil {
			b.deps.Log.Error("bot: text failed", "err", err)
		}
		return
	}
}

// handleStart renders the main menu and initialises the session.
func (b *Bot) handleStart(ctx context.Context, tg *bot.Bot, update *models.Update) {
	if err := b.showMainMenu(ctx, update, true); err != nil {
		b.deps.Log.Error("bot: /start", "err", err)
	}
}

// dispatchCallback routes inline-callback events to per-screen handlers.
func (b *Bot) dispatchCallback(ctx context.Context, update *models.Update) error {
	cq := update.CallbackQuery
	if cq == nil {
		return nil
	}
	defer b.answerCallback(ctx, cq.ID, "")
	data := cq.Data

	switch {
	case data == CBMainMenu:
		return b.showMainMenu(ctx, update, false)
	case data == CBHelp:
		return b.showHelp(ctx, update)
	case strings.HasPrefix(data, CBUsersList):
		return b.showUsersList(ctx, update, data[len(CBUsersList):])
	case strings.HasPrefix(data, CBUserCard):
		return b.showUserCard(ctx, update, data[len(CBUserCard):])
	case strings.HasPrefix(data, CBUserToggleV):
		return b.toggleUserProto(ctx, update, data[len(CBUserToggleV):], protoVLESS)
	case strings.HasPrefix(data, CBUserToggleS):
		return b.toggleUserProto(ctx, update, data[len(CBUserToggleS):], protoSOCKS)
	case strings.HasPrefix(data, CBUserEnable):
		return b.setUserEnabled(ctx, update, data[len(CBUserEnable):], true)
	case strings.HasPrefix(data, CBUserDisable):
		return b.setUserEnabled(ctx, update, data[len(CBUserDisable):], false)
	case strings.HasPrefix(data, CBUserLinks):
		return b.showUserLinks(ctx, update, data[len(CBUserLinks):])
	case strings.HasPrefix(data, CBUserQR):
		return b.showUserQR(ctx, update, data[len(CBUserQR):])
	case strings.HasPrefix(data, CBUserDelete):
		return b.confirmDeleteUser(ctx, update, data[len(CBUserDelete):])
	case strings.HasPrefix(data, CBUserDelYes):
		return b.deleteUser(ctx, update, data[len(CBUserDelYes):])
	case strings.HasPrefix(data, CBUserDelNo):
		return b.showUserCard(ctx, update, data[len(CBUserDelNo):])

	case data == CBAddStart:
		return b.addWizardStart(ctx, update)
	case data == CBAddProtoVLESS:
		return b.addWizardToggleProto(ctx, update, protoVLESS)
	case data == CBAddProtoSOCKS:
		return b.addWizardToggleProto(ctx, update, protoSOCKS)
	case data == CBAddNext:
		return b.addWizardNext(ctx, update)
	case data == CBAddLimit10:
		return b.addWizardPickLimit(ctx, update, 10)
	case data == CBAddLimit50:
		return b.addWizardPickLimit(ctx, update, 50)
	case data == CBAddLimit100:
		return b.addWizardPickLimit(ctx, update, 100)
	case data == CBAddLimitInf:
		return b.addWizardPickLimit(ctx, update, 0)
	case data == CBAddLimitCustom:
		return b.addWizardPromptCustom(ctx, update)
	case data == CBAddCancel:
		return b.addWizardCancel(ctx, update)

	case data == CBStatsMain:
		return b.showStats(ctx, update)
	case data == CBServerMain:
		return b.showServer(ctx, update)
	case data == CBServerRotate:
		return b.answerInfo(ctx, update, "err.generic")
	case data == CBServerUpdateGeo:
		return b.answerInfo(ctx, update, "err.generic")
	case data == CBServerNotifications:
		return b.showNotifSettings(ctx, update)
	case data == CBNotifToggleCritical:
		return b.toggleNotifPref(ctx, update, "critical")
	case data == CBNotifToggleImportant:
		return b.toggleNotifPref(ctx, update, "important")
	case data == CBNotifToggleInfo:
		return b.toggleNotifPref(ctx, update, "info")
	case data == CBNotifToggleOthers:
		return b.toggleNotifPref(ctx, update, "others")
	case data == CBNotifToggleSecurity:
		return b.toggleNotifPref(ctx, update, "security")
	case data == CBNotifToggleDaily:
		return b.toggleNotifPref(ctx, update, "daily")
	case data == CBNoop:
		return nil
	}
	return fmt.Errorf("unknown callback %q", data)
}

// answerCallback acknowledges a callback_query with optional user-visible
// text. Errors are logged but never returned — failure here is non-fatal.
func (b *Bot) answerCallback(ctx context.Context, id, text string) {
	if id == "" {
		return
	}
	if _, err := b.tg.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{CallbackQueryID: id, Text: text}); err != nil {
		b.deps.Log.Debug("bot: answer callback failed", "err", err)
	}
}

// answerInfo sends a transient toast using a translated key.
func (b *Bot) answerInfo(ctx context.Context, update *models.Update, key string) error {
	if update.CallbackQuery != nil {
		b.answerCallback(ctx, update.CallbackQuery.ID, b.tr(ctx, update, key))
	}
	return nil
}

// tr is the bound translator for the admin who triggered update.
func (b *Bot) tr(ctx context.Context, update *models.Update, key string, args ...any) string {
	lang := b.adminLang(ctx, updateTGID(update))
	return b.deps.I18n.T(lang, key, args...)
}

// adminLang returns the admin's preferred language, falling back to the
// daemon default. Missing prefs rows fall back to DefaultLang.
func (b *Bot) adminLang(ctx context.Context, tgID int64) string {
	if tgID == 0 {
		return b.deps.DefaultLang
	}
	p, err := b.deps.Store.GetAdminPrefs(ctx, tgID)
	if err == nil && p.Lang != "" {
		return p.Lang
	}
	if b.deps.DefaultLang != "" {
		return b.deps.DefaultLang
	}
	return "ru"
}
