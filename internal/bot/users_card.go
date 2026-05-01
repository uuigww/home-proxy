package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-telegram/bot/models"

	"github.com/uuigww/home-proxy/internal/store"
)

// protoKind identifies which protocol an action targets.
type protoKind int

const (
	protoVLESS protoKind = iota + 1
	protoSOCKS
	protoMTProto
)

// parseUserID decodes a callback payload of the form "42".
func parseUserID(s string) (int64, error) {
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse user id %q: %w", s, err)
	}
	return id, nil
}

// showUserCard renders the single-user screen with toggle/disable/delete
// buttons.
func (b *Bot) showUserCard(ctx context.Context, update *models.Update, payload string) error {
	tgID := updateTGID(update)
	id, err := parseUserID(payload)
	if err != nil {
		return err
	}
	sess, err := b.sessions.Get(ctx, tgID)
	if err != nil {
		return err
	}
	sess.TGID = tgID
	sess.Screen = "user_card"
	if sess.ChatID == 0 {
		sess.ChatID = updateChatID(update)
	}
	lang := b.adminLang(ctx, tgID)

	u, err := b.deps.Store.GetUser(ctx, id)
	if err != nil {
		return fmt.Errorf("get user %d: %w", id, err)
	}

	text := renderUserCard(b, lang, u)
	kb := renderUserCardKeyboard(b, lang, u)
	return b.sessions.Edit(ctx, b.tg, &sess, text, kb)
}

// renderUserCard composes the informational body for a user card.
func renderUserCard(b *Bot, lang string, u store.User) string {
	status := b.deps.I18n.T(lang, "users.card.enabled")
	if !u.Enabled {
		status = b.deps.I18n.T(lang, "users.card.disabled")
	}
	lines := []string{
		"<b>" + b.deps.I18n.T(lang, "users.card.title", u.Name) + "</b> · " + status,
		"",
	}
	if u.VLESSUUID != nil {
		lines = append(lines, b.deps.I18n.T(lang, "users.card.vless_on"))
	} else {
		lines = append(lines, b.deps.I18n.T(lang, "users.card.vless_off"))
	}
	if u.SOCKSUser != nil {
		lines = append(lines, b.deps.I18n.T(lang, "users.card.socks_on"))
	} else {
		lines = append(lines, b.deps.I18n.T(lang, "users.card.socks_off"))
	}
	if b.deps.Cfg.MTProtoEnabled {
		if u.MTProtoEnabled {
			lines = append(lines, b.deps.I18n.T(lang, "users.card.mtproto_on"))
		} else {
			lines = append(lines, b.deps.I18n.T(lang, "users.card.mtproto_off"))
		}
	}
	lines = append(lines, b.deps.I18n.T(lang, "users.card.limit", limitHuman(b, lang, u.LimitBytes)))
	lines = append(lines, b.deps.I18n.T(lang, "users.card.used", humanBytes(u.UsedBytes), percent(u.UsedBytes, u.LimitBytes)))
	lines = append(lines, b.deps.I18n.T(lang, "users.card.created", u.CreatedAt.Format("2006-01-02")))
	return strings.Join(lines, "\n")
}

// renderUserCardKeyboard builds the per-user action keyboard.
func renderUserCardKeyboard(b *Bot, lang string, u store.User) *models.InlineKeyboardMarkup {
	id := itoa(u.ID)
	vlessLabel := b.deps.I18n.T(lang, "users.card.vless_on")
	if u.VLESSUUID == nil {
		vlessLabel = b.deps.I18n.T(lang, "users.card.vless_off")
	}
	socksLabel := b.deps.I18n.T(lang, "users.card.socks_on")
	if u.SOCKSUser == nil {
		socksLabel = b.deps.I18n.T(lang, "users.card.socks_off")
	}
	toggleRow := kbRow(
		btn(vlessLabel, CBUserToggleV+id),
		btn(socksLabel, CBUserToggleS+id),
	)
	var mtprotoRow []models.InlineKeyboardButton
	if b.deps.Cfg.MTProtoEnabled {
		mtprotoLabel := b.deps.I18n.T(lang, "users.card.mtproto_off")
		if u.MTProtoEnabled {
			mtprotoLabel = b.deps.I18n.T(lang, "users.card.mtproto_on")
		}
		mtprotoRow = kbRow(btn(mtprotoLabel, CBUserToggleMTProto+id))
	}
	actionRow := kbRow(
		btn(b.deps.I18n.T(lang, "users.card.links"), CBUserLinks+id),
		btn(b.deps.I18n.T(lang, "users.card.qr"), CBUserQR+id),
	)
	var stateRow []models.InlineKeyboardButton
	if u.Enabled {
		stateRow = kbRow(
			btn(b.deps.I18n.T(lang, "users.card.disable"), CBUserDisable+id),
			btn(b.deps.I18n.T(lang, "users.card.delete"), CBUserDelete+id),
		)
	} else {
		stateRow = kbRow(
			btn(b.deps.I18n.T(lang, "users.card.enable"), CBUserEnable+id),
			btn(b.deps.I18n.T(lang, "users.card.delete"), CBUserDelete+id),
		)
	}
	rows := [][]models.InlineKeyboardButton{toggleRow}
	if mtprotoRow != nil {
		rows = append(rows, mtprotoRow)
	}
	rows = append(rows,
		actionRow,
		stateRow,
		kbRow(btn(b.deps.I18n.T(lang, "menu.back"), CBUsersList+"0")),
	)
	return markup(rows...)
}

// toggleUserProto flips VLESS or SOCKS on the given user atomically.
//
// DB is updated first, then xray's config.json is rebuilt and xray restarted;
// on xray failure the DB change is rolled back. An info notification is fired
// on success for other admins.
func (b *Bot) toggleUserProto(ctx context.Context, update *models.Update, payload string, kind protoKind) error {
	tgID := updateTGID(update)
	id, err := parseUserID(payload)
	if err != nil {
		return err
	}
	u, err := b.deps.Store.GetUser(ctx, id)
	if err != nil {
		return err
	}
	oldProtos := protoList(u)

	newUser := u
	switch kind {
	case protoVLESS:
		if u.VLESSUUID == nil {
			uuid := newUUID()
			newUser.VLESSUUID = &uuid
		} else {
			newUser.VLESSUUID = nil
		}
	case protoSOCKS:
		if u.SOCKSUser == nil {
			user := newUser.Name
			pass := newSOCKSPass()
			newUser.SOCKSUser = &user
			newUser.SOCKSPass = &pass
		} else {
			newUser.SOCKSUser = nil
			newUser.SOCKSPass = nil
		}
	}

	if err := b.applyUserChange(ctx, &u, &newUser, b.reapplyXrayConfig, b.reapplyXrayConfig); err != nil {
		b.deps.Log.Error("bot: toggle proto", "err", err, "user", u.Name)
		return err
	}
	newProtos := protoList(newUser)
	b.notifyInfo(ctx, update, "user.protocols_changed", tgID, u.Name, oldProtos, newProtos)
	return b.showUserCard(ctx, update, payload)
}

// setUserEnabled toggles the enabled flag atomically. The xray config is
// rebuilt from DB state and xray restarted so disabled users are dropped from
// the live config and re-enabled users are reinstated.
func (b *Bot) setUserEnabled(ctx context.Context, update *models.Update, payload string, enabled bool) error {
	tgID := updateTGID(update)
	id, err := parseUserID(payload)
	if err != nil {
		return err
	}
	u, err := b.deps.Store.GetUser(ctx, id)
	if err != nil {
		return err
	}
	if u.Enabled == enabled {
		return b.showUserCard(ctx, update, payload)
	}
	newUser := u
	newUser.Enabled = enabled

	if err := b.applyUserChange(ctx, &u, &newUser, b.reapplyXrayConfig, b.reapplyXrayConfig); err != nil {
		b.deps.Log.Error("bot: set enabled", "err", err)
		return err
	}
	if enabled {
		b.notifyInfo(ctx, update, "user.enabled", tgID, u.Name)
	} else {
		b.notifyInfo(ctx, update, "user.disabled", tgID, u.Name)
	}
	return b.showUserCard(ctx, update, payload)
}

// toggleUserMTProto flips the MTProto UI flag for a user.
//
// Unlike toggleUserProto, there is no Xray side-effect: mtg is a separate
// process that uses a single server-wide secret. The DB flag only controls
// whether the bot exposes the tg://proxy link for this user.
func (b *Bot) toggleUserMTProto(ctx context.Context, update *models.Update, payload string) error {
	tgID := updateTGID(update)
	if !b.deps.Cfg.MTProtoEnabled {
		return b.answerInfo(ctx, update, "server.mtproto_disabled")
	}
	id, err := parseUserID(payload)
	if err != nil {
		return err
	}
	u, err := b.deps.Store.GetUser(ctx, id)
	if err != nil {
		return err
	}
	oldProtos := protoList(u)
	newOn := !u.MTProtoEnabled
	if err := b.deps.Store.SetUserMTProtoEnabled(ctx, id, newOn); err != nil {
		b.deps.Log.Error("bot: toggle mtproto", "err", err, "user", u.Name)
		return err
	}
	u.MTProtoEnabled = newOn
	newProtos := protoList(u)
	b.notifyInfo(ctx, update, "user.protocols_changed", tgID, u.Name, oldProtos, newProtos)
	return b.showUserCard(ctx, update, payload)
}

// confirmDeleteUser shows the "are you sure" screen for deletion.
func (b *Bot) confirmDeleteUser(ctx context.Context, update *models.Update, payload string) error {
	tgID := updateTGID(update)
	id, err := parseUserID(payload)
	if err != nil {
		return err
	}
	u, err := b.deps.Store.GetUser(ctx, id)
	if err != nil {
		return err
	}
	sess, err := b.sessions.Get(ctx, tgID)
	if err != nil {
		return err
	}
	sess.TGID = tgID
	sess.Screen = "user_delete"
	if sess.ChatID == 0 {
		sess.ChatID = updateChatID(update)
	}
	lang := b.adminLang(ctx, tgID)
	text := b.deps.I18n.T(lang, "users.card.confirm_delete", u.Name)
	kb := markup(
		kbRow(
			btn(b.deps.I18n.T(lang, "users.card.confirm_yes"), CBUserDelYes+payload),
			btn(b.deps.I18n.T(lang, "users.card.confirm_no"), CBUserDelNo+payload),
		),
	)
	return b.sessions.Edit(ctx, b.tg, &sess, text, kb)
}

// deleteUser removes the user from the store then rebuilds xray's config so
// the dropped user is no longer authenticated. On xray-rebuild failure the
// user is restored to keep the two stores in sync.
func (b *Bot) deleteUser(ctx context.Context, update *models.Update, payload string) error {
	tgID := updateTGID(update)
	id, err := parseUserID(payload)
	if err != nil {
		return err
	}
	u, err := b.deps.Store.GetUser(ctx, id)
	if err != nil {
		return err
	}
	if err := b.deps.Store.DeleteUser(ctx, id); err != nil {
		return err
	}
	if err := b.reapplyXrayConfig(ctx); err != nil {
		// Best-effort restore: re-create with a fresh row id.
		restored := u
		restored.ID = 0
		if rbErr := b.deps.Store.CreateUser(ctx, &restored); rbErr != nil {
			b.deps.Log.Error("bot: rollback delete failed", "err", rbErr, "user", u.Name)
		}
		b.deps.Log.Error("bot: xray apply on delete", "err", err, "user", u.Name)
		return err
	}
	b.notifyInfo(ctx, update, "user.deleted", tgID, u.Name)
	return b.showUsersList(ctx, update, "0")
}

