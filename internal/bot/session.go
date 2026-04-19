package bot

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/uuigww/home-proxy/internal/store"
)

// SessionMgr persists the single-message UX state for each admin.
//
// The underlying storage is the sessions table in SQLite; SessionMgr adds
// typed helpers on top of the raw store API (wizard JSON parse/encode, a
// convenient Edit wrapper that falls back to sending a new message when an
// admin first interacts with the bot).
type SessionMgr struct {
	store *store.Store
}

// NewSessionMgr returns a manager backed by the given store.
func NewSessionMgr(s *store.Store) *SessionMgr {
	return &SessionMgr{store: s}
}

// Get returns the stored session for tgID or a fresh zero-value session when
// none exists yet. Callers can treat the returned struct as always valid.
func (m *SessionMgr) Get(ctx context.Context, tgID int64) (store.Session, error) {
	sess, err := m.store.GetSession(ctx, tgID)
	if err != nil {
		if err == store.ErrNotFound {
			return store.Session{TGID: tgID, WizardJSON: "{}"}, nil
		}
		return store.Session{}, fmt.Errorf("session get: %w", err)
	}
	return sess, nil
}

// Save upserts sess into the store.
func (m *SessionMgr) Save(ctx context.Context, sess store.Session) error {
	return m.store.UpsertSession(ctx, sess)
}

// Clear removes any stored session for tgID. Wizard state is also dropped.
func (m *SessionMgr) Clear(ctx context.Context, tgID int64) error {
	return m.store.DeleteSession(ctx, tgID)
}

// Edit rewrites the admin's single bot message in place. When no message_id
// is recorded yet (first interaction) it sends a new message and records the
// returned message_id in the session.
//
// markup is optional; pass nil for no keyboard.
func (m *SessionMgr) Edit(
	ctx context.Context,
	tg *bot.Bot,
	sess *store.Session,
	text string,
	kb *models.InlineKeyboardMarkup,
) error {
	if sess == nil {
		return fmt.Errorf("session: nil session")
	}
	if sess.ChatID == 0 {
		return fmt.Errorf("session: missing chat id")
	}
	if sess.MessageID == 0 {
		p := &bot.SendMessageParams{ChatID: sess.ChatID, Text: text, ParseMode: models.ParseModeHTML}
		if kb != nil {
			p.ReplyMarkup = kb
		}
		msg, err := tg.SendMessage(ctx, p)
		if err != nil {
			return fmt.Errorf("session: send: %w", err)
		}
		if msg != nil {
			sess.MessageID = msg.ID
		}
		return m.Save(ctx, *sess)
	}
	p := &bot.EditMessageTextParams{
		ChatID:    sess.ChatID,
		MessageID: sess.MessageID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
	}
	if kb != nil {
		p.ReplyMarkup = kb
	}
	if _, err := tg.EditMessageText(ctx, p); err != nil {
		// Telegram refuses edits on non-existent messages; fall back to send.
		p2 := &bot.SendMessageParams{ChatID: sess.ChatID, Text: text, ParseMode: models.ParseModeHTML}
		if kb != nil {
			p2.ReplyMarkup = kb
		}
		msg, err2 := tg.SendMessage(ctx, p2)
		if err2 != nil {
			return fmt.Errorf("session: edit failed and resend failed: %w (send: %v)", err, err2)
		}
		if msg != nil {
			sess.MessageID = msg.ID
		}
	}
	return m.Save(ctx, *sess)
}

// WizardState is the typed view of Session.WizardJSON.
//
// Kind identifies the active wizard ("", "add_user", ...). Step is the 0-based
// step within that wizard. Data holds any collected inputs. When Kind is
// empty the wizard is inactive.
type WizardState struct {
	Kind string         `json:"kind,omitempty"`
	Step int            `json:"step,omitempty"`
	Data map[string]any `json:"data,omitempty"`
}

// LoadWizard returns the current wizard state or an empty state if none.
func LoadWizard(sess store.Session) WizardState {
	ws := WizardState{}
	if sess.WizardJSON == "" || sess.WizardJSON == "{}" {
		return ws
	}
	_ = json.Unmarshal([]byte(sess.WizardJSON), &ws)
	if ws.Data == nil {
		ws.Data = map[string]any{}
	}
	return ws
}

// SaveWizard encodes ws onto sess.WizardJSON.
func SaveWizard(sess *store.Session, ws WizardState) error {
	if ws.Data == nil {
		ws.Data = map[string]any{}
	}
	b, err := json.Marshal(ws)
	if err != nil {
		return fmt.Errorf("wizard marshal: %w", err)
	}
	sess.WizardJSON = string(b)
	return nil
}

// ClearWizard resets sess.WizardJSON to an empty object.
func ClearWizard(sess *store.Session) {
	sess.WizardJSON = "{}"
}
