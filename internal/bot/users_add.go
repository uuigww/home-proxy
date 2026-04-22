package bot

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-telegram/bot/models"

	"github.com/uuigww/home-proxy/internal/store"
)

// wizardKindAddUser is the Kind value for the add-user wizard.
const wizardKindAddUser = "add_user"

// validUserName is the relaxed name pattern the wizard accepts.
var validUserName = regexp.MustCompile(`^[A-Za-z0-9_-]{1,32}$`)

// addWizardStart kicks off the add-user wizard at step 1 (name prompt).
func (b *Bot) addWizardStart(ctx context.Context, update *models.Update) error {
	tgID := updateTGID(update)
	sess, err := b.sessions.Get(ctx, tgID)
	if err != nil {
		return err
	}
	sess.TGID = tgID
	if sess.ChatID == 0 {
		sess.ChatID = updateChatID(update)
	}
	sess.Screen = "wizard_add"

	ws := WizardState{Kind: wizardKindAddUser, Step: 1, Data: map[string]any{
		"vless": true,
		"socks": true,
	}}
	if err := SaveWizard(&sess, ws); err != nil {
		return err
	}
	lang := b.adminLang(ctx, tgID)
	text := fmt.Sprintf("<b>%s</b>\n%s",
		b.deps.I18n.T(lang, "wizard.add.step1_title"),
		b.deps.I18n.T(lang, "wizard.add.step1_prompt"))
	kb := markup(kbRow(btn(b.deps.I18n.T(lang, "menu.cancel"), CBAddCancel)))
	return b.sessions.Edit(ctx, b.tg, &sess, text, kb)
}

// handleText processes free-text messages — only relevant when an add-user
// wizard is in step 1 (awaiting a name).
func (b *Bot) handleText(ctx context.Context, update *models.Update) error {
	tgID := updateTGID(update)
	sess, err := b.sessions.Get(ctx, tgID)
	if err != nil {
		return err
	}
	if sess.ChatID == 0 {
		sess.ChatID = updateChatID(update)
	}
	ws := LoadWizard(sess)
	if ws.Kind != wizardKindAddUser {
		// Out-of-context plain text: just re-render the main menu so the
		// single-message UX holds.
		return b.showMainMenu(ctx, update, false)
	}
	lang := b.adminLang(ctx, tgID)
	text := strings.TrimSpace(update.Message.Text)

	switch ws.Step {
	case 1:
		if !validUserName.MatchString(text) {
			msg := fmt.Sprintf("<b>%s</b>\n%s\n<i>%s</i>",
				b.deps.I18n.T(lang, "wizard.add.step1_title"),
				b.deps.I18n.T(lang, "wizard.add.step1_prompt"),
				b.deps.I18n.T(lang, "err.invalid_name"))
			return b.sessions.Edit(ctx, b.tg, &sess, msg, markup(kbRow(btn(b.deps.I18n.T(lang, "menu.cancel"), CBAddCancel))))
		}
		if _, err := b.deps.Store.GetUserByName(ctx, text); err == nil {
			msg := fmt.Sprintf("<b>%s</b>\n%s\n<i>%s</i>",
				b.deps.I18n.T(lang, "wizard.add.step1_title"),
				b.deps.I18n.T(lang, "wizard.add.step1_prompt"),
				b.deps.I18n.T(lang, "err.user_exists"))
			return b.sessions.Edit(ctx, b.tg, &sess, msg, markup(kbRow(btn(b.deps.I18n.T(lang, "menu.cancel"), CBAddCancel))))
		} else if !errors.Is(err, store.ErrNotFound) {
			return err
		}
		ws.Data["name"] = text
		ws.Step = 2
		if err := SaveWizard(&sess, ws); err != nil {
			return err
		}
		return b.renderAddStep2(ctx, update, &sess, ws)
	case 3:
		// Custom limit input (GB).
		gb, err := strconv.Atoi(text)
		if err != nil || gb < 0 {
			return b.renderAddStep3(ctx, update, &sess, ws)
		}
		return b.addWizardPickLimit(ctx, update, int64(gb))
	}
	return b.showMainMenu(ctx, update, false)
}

// addWizardToggleProto flips a protocol flag in the wizard data.
func (b *Bot) addWizardToggleProto(ctx context.Context, update *models.Update, kind protoKind) error {
	tgID := updateTGID(update)
	sess, err := b.sessions.Get(ctx, tgID)
	if err != nil {
		return err
	}
	ws := LoadWizard(sess)
	if ws.Kind != wizardKindAddUser {
		return b.showMainMenu(ctx, update, false)
	}
	var key string
	switch kind {
	case protoVLESS:
		key = "vless"
	case protoSOCKS:
		key = "socks"
	case protoMTProto:
		if !b.deps.Cfg.MTProtoEnabled {
			// MTProto not installed on this server: silently ignore.
			return b.renderAddStep2(ctx, update, &sess, ws)
		}
		key = "mtproto"
	}
	cur, _ := ws.Data[key].(bool)
	ws.Data[key] = !cur
	if err := SaveWizard(&sess, ws); err != nil {
		return err
	}
	return b.renderAddStep2(ctx, update, &sess, ws)
}

// addWizardNext advances from step 2 (protocols) to step 3 (limit).
func (b *Bot) addWizardNext(ctx context.Context, update *models.Update) error {
	tgID := updateTGID(update)
	sess, err := b.sessions.Get(ctx, tgID)
	if err != nil {
		return err
	}
	ws := LoadWizard(sess)
	if ws.Kind != wizardKindAddUser || ws.Step < 2 {
		return b.showMainMenu(ctx, update, false)
	}
	vless, _ := ws.Data["vless"].(bool)
	socks, _ := ws.Data["socks"].(bool)
	mtproto, _ := ws.Data["mtproto"].(bool)
	if !vless && !socks && !mtproto {
		// Re-render step 2 with an inline error hint.
		return b.renderAddStep2Err(ctx, update, &sess, ws, "wizard.add.proto_none")
	}
	ws.Step = 3
	if err := SaveWizard(&sess, ws); err != nil {
		return err
	}
	return b.renderAddStep3(ctx, update, &sess, ws)
}

// addWizardPickLimit finalises the wizard with the chosen monthly limit
// (gigabytes; 0 = unlimited).
func (b *Bot) addWizardPickLimit(ctx context.Context, update *models.Update, gb int64) error {
	tgID := updateTGID(update)
	sess, err := b.sessions.Get(ctx, tgID)
	if err != nil {
		return err
	}
	ws := LoadWizard(sess)
	if ws.Kind != wizardKindAddUser {
		return b.showMainMenu(ctx, update, false)
	}
	ws.Data["limit_gb"] = float64(gb)
	if err := SaveWizard(&sess, ws); err != nil {
		return err
	}
	return b.finishAddWizard(ctx, update, &sess, ws)
}

// addWizardPromptCustom re-renders step 3 with an instruction to enter a
// number in chat.
func (b *Bot) addWizardPromptCustom(ctx context.Context, update *models.Update) error {
	tgID := updateTGID(update)
	sess, err := b.sessions.Get(ctx, tgID)
	if err != nil {
		return err
	}
	ws := LoadWizard(sess)
	if ws.Kind != wizardKindAddUser {
		return b.showMainMenu(ctx, update, false)
	}
	lang := b.adminLang(ctx, tgID)
	ws.Step = 3
	if err := SaveWizard(&sess, ws); err != nil {
		return err
	}
	text := fmt.Sprintf("<b>%s</b>\n%s",
		b.deps.I18n.T(lang, "wizard.add.step3_title"),
		b.deps.I18n.T(lang, "wizard.add.limit_custom"))
	kb := markup(kbRow(btn(b.deps.I18n.T(lang, "menu.cancel"), CBAddCancel)))
	return b.sessions.Edit(ctx, b.tg, &sess, text, kb)
}

// addWizardCancel aborts the wizard and returns to the main menu.
func (b *Bot) addWizardCancel(ctx context.Context, update *models.Update) error {
	tgID := updateTGID(update)
	sess, err := b.sessions.Get(ctx, tgID)
	if err != nil {
		return err
	}
	ClearWizard(&sess)
	if err := b.sessions.Save(ctx, sess); err != nil {
		return err
	}
	return b.showMainMenu(ctx, update, false)
}

// renderAddStep2 paints the protocols picker.
func (b *Bot) renderAddStep2(ctx context.Context, update *models.Update, sess *store.Session, ws WizardState) error {
	return b.renderAddStep2Err(ctx, update, sess, ws, "")
}

// renderAddStep2Err is like renderAddStep2 but appends an error line.
func (b *Bot) renderAddStep2Err(ctx context.Context, update *models.Update, sess *store.Session, ws WizardState, errKey string) error {
	lang := b.adminLang(ctx, updateTGID(update))
	vless, _ := ws.Data["vless"].(bool)
	socks, _ := ws.Data["socks"].(bool)
	mtproto, _ := ws.Data["mtproto"].(bool)
	text := fmt.Sprintf("<b>%s</b>\n%s",
		b.deps.I18n.T(lang, "wizard.add.step2_title"),
		b.deps.I18n.T(lang, "wizard.add.step2_prompt"))
	if errKey != "" {
		text += "\n<i>" + b.deps.I18n.T(lang, errKey) + "</i>"
	}
	rows := [][]models.InlineKeyboardButton{
		kbRow(btn(mark(vless)+b.deps.I18n.T(lang, "wizard.add.proto_vless"), CBAddProtoVLESS)),
		kbRow(btn(mark(socks)+b.deps.I18n.T(lang, "wizard.add.proto_socks"), CBAddProtoSOCKS)),
	}
	if b.deps.Cfg.MTProtoEnabled {
		rows = append(rows, kbRow(btn(mark(mtproto)+b.deps.I18n.T(lang, "wizard.add.proto_mtproto"), CBAddProtoMTProto)))
	}
	rows = append(rows, kbRow(
		btn(b.deps.I18n.T(lang, "menu.next"), CBAddNext),
		btn(b.deps.I18n.T(lang, "menu.cancel"), CBAddCancel),
	))
	kb := markup(rows...)
	return b.sessions.Edit(ctx, b.tg, sess, text, kb)
}

// renderAddStep3 paints the limit picker.
func (b *Bot) renderAddStep3(ctx context.Context, update *models.Update, sess *store.Session, _ WizardState) error {
	lang := b.adminLang(ctx, updateTGID(update))
	text := fmt.Sprintf("<b>%s</b>\n%s",
		b.deps.I18n.T(lang, "wizard.add.step3_title"),
		b.deps.I18n.T(lang, "wizard.add.step3_prompt"))
	kb := markup(
		kbRow(
			btn(b.deps.I18n.T(lang, "wizard.add.limit_10gb"), CBAddLimit10),
			btn(b.deps.I18n.T(lang, "wizard.add.limit_50gb"), CBAddLimit50),
			btn(b.deps.I18n.T(lang, "wizard.add.limit_100gb"), CBAddLimit100),
		),
		kbRow(
			btn(b.deps.I18n.T(lang, "wizard.add.limit_unlimited"), CBAddLimitInf),
			btn(b.deps.I18n.T(lang, "wizard.add.limit_custom"), CBAddLimitCustom),
		),
		kbRow(btn(b.deps.I18n.T(lang, "menu.cancel"), CBAddCancel)),
	)
	return b.sessions.Edit(ctx, b.tg, sess, text, kb)
}

// finishAddWizard creates the user in store + Xray and renders the success
// screen with copy buttons.
func (b *Bot) finishAddWizard(ctx context.Context, update *models.Update, sess *store.Session, ws WizardState) error {
	tgID := updateTGID(update)
	lang := b.adminLang(ctx, tgID)
	name, _ := ws.Data["name"].(string)
	vless, _ := ws.Data["vless"].(bool)
	socks, _ := ws.Data["socks"].(bool)
	mtproto, _ := ws.Data["mtproto"].(bool)
	gbFloat, _ := ws.Data["limit_gb"].(float64)
	limitBytes := int64(gbFloat) * 1024 * 1024 * 1024

	u := &store.User{
		Name:       name,
		LimitBytes: limitBytes,
		Enabled:    true,
	}
	if vless {
		uuid := newUUID()
		u.VLESSUUID = &uuid
	}
	if socks {
		user := name
		pass := newSOCKSPass()
		u.SOCKSUser = &user
		u.SOCKSPass = &pass
	}
	// MTProto is a UI-only flag; mtg shares a server-wide secret.
	if mtproto && b.deps.Cfg.MTProtoEnabled {
		u.MTProtoEnabled = true
	}
	if err := b.deps.Store.CreateUser(ctx, u); err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	// Apply to Xray; on failure, remove DB row and report.
	if u.VLESSUUID != nil {
		if err := b.deps.Xray.AddVLESSUser(ctx, *u.VLESSUUID, u.Name); err != nil {
			_ = b.deps.Store.DeleteUser(ctx, u.ID)
			return fmt.Errorf("xray vless: %w", err)
		}
	}
	if u.SOCKSUser != nil {
		if err := b.deps.Xray.AddSOCKSUser(ctx, *u.SOCKSUser, *u.SOCKSPass, u.Name); err != nil {
			if u.VLESSUUID != nil {
				_ = b.deps.Xray.RemoveVLESSUser(ctx, u.Name)
			}
			_ = b.deps.Store.DeleteUser(ctx, u.ID)
			return fmt.Errorf("xray socks: %w", err)
		}
	}

	ClearWizard(sess)
	sess.Screen = "user_card"
	protos := protoList(*u)
	limitStr := limitHuman(b, lang, u.LimitBytes)
	body := b.deps.I18n.T(lang, "wizard.add.done", name, protos, limitStr)
	kb := markup(
		kbRow(btn(b.deps.I18n.T(lang, "users.card.links"), CBUserLinks+itoa(u.ID))),
		kbRow(btn(b.deps.I18n.T(lang, "menu.back"), CBUsersList+"0")),
	)
	b.notifyInfo(ctx, update, "user.created", tgID, u.Name, protos, limitStr)
	return b.sessions.Edit(ctx, b.tg, sess, body, kb)
}

// mark returns a leading ✓ when on is true, else a space — used as a prefix
// on wizard toggle buttons.
func mark(on bool) string {
	if on {
		return "✓ "
	}
	return "  "
}


