package bot

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/go-telegram/bot/models"

	"github.com/uuigww/home-proxy/internal/store"
)

// BuildVLESSLink renders the canonical vless:// URL for a user.
//
// The output matches the format most mobile VLESS clients expect:
//
//	vless://<uuid>@<host>:<port>?security=reality&sni=<sname>&fp=chrome
//	  &pbk=<pubkey>&sid=<shortid>&type=tcp&flow=xtls-rprx-vision#<name>
//
// host is typically the server's public DNS name; port is the Reality inbound.
// Returns an empty string when the user has no VLESS UUID.
func BuildVLESSLink(u store.User, r store.RealityKeys, host string, port int) string {
	if u.VLESSUUID == nil || host == "" {
		return ""
	}
	q := url.Values{}
	q.Set("security", "reality")
	q.Set("sni", r.ServerName)
	q.Set("fp", "chrome")
	q.Set("pbk", r.PublicKey)
	q.Set("sid", r.ShortID)
	q.Set("type", "tcp")
	q.Set("flow", "xtls-rprx-vision")
	return fmt.Sprintf("vless://%s@%s:%d?%s#%s",
		*u.VLESSUUID, host, port, q.Encode(), url.PathEscape(u.Name))
}

// BuildSOCKSLink renders a socks5:// URL for copy-paste.
//
// Returns an empty string when the user has no SOCKS credentials.
func BuildSOCKSLink(u store.User, host string, port int) string {
	if u.SOCKSUser == nil || u.SOCKSPass == nil {
		return ""
	}
	return fmt.Sprintf("socks5://%s:%s@%s:%d",
		url.QueryEscape(*u.SOCKSUser), url.QueryEscape(*u.SOCKSPass), host, port)
}

// BuildQR is a placeholder: proper PNG encoding is a TODO. Today we hand the
// raw link text back to callers so the UI can at least show it to the admin
// (who can then feed it into their client manually).
//
// TODO(M4+): inline a minimal QR PNG encoder so the bot can attach an image.
func BuildQR(data string) ([]byte, error) {
	if data == "" {
		return nil, fmt.Errorf("qr: empty data")
	}
	return []byte(data), nil
}

// showUserLinks renders the links screen for a user.
func (b *Bot) showUserLinks(ctx context.Context, update *models.Update, payload string) error {
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
	sess.Screen = "user_links"
	if sess.ChatID == 0 {
		sess.ChatID = updateChatID(update)
	}
	lang := b.adminLang(ctx, tgID)
	u, err := b.deps.Store.GetUser(ctx, id)
	if err != nil {
		return err
	}
	r, _ := b.deps.Store.GetRealityKeys(ctx)
	host := b.deps.Cfg.RealityServerName
	if host == "" {
		host = "example.com"
	}
	vl := BuildVLESSLink(u, r, host, b.deps.Cfg.RealityPort)
	sl := BuildSOCKSLink(u, host, b.deps.Cfg.SOCKSPort)

	var sb strings.Builder
	sb.WriteString("<b>")
	sb.WriteString(b.deps.I18n.T(lang, "users.card.title", u.Name))
	sb.WriteString("</b>\n")
	if vl == "" && sl == "" {
		sb.WriteString(b.deps.I18n.T(lang, "links.none"))
	}
	if vl != "" {
		sb.WriteString("\n<code>")
		sb.WriteString(htmlEscape(vl))
		sb.WriteString("</code>\n")
	}
	if sl != "" {
		sb.WriteString("\n<code>")
		sb.WriteString(htmlEscape(sl))
		sb.WriteString("</code>\n")
	}
	kb := markup(kbRow(btn(b.deps.I18n.T(lang, "menu.back"), CBUserCard+payload)))
	return b.sessions.Edit(ctx, b.tg, &sess, sb.String(), kb)
}

// showUserQR renders (for now) the same link content as the Links screen;
// callers can copy the link into a QR generator on their device until we
// inline a PNG encoder.
func (b *Bot) showUserQR(ctx context.Context, update *models.Update, payload string) error {
	// Reuse showUserLinks — this is a harmless placeholder until QR is real.
	return b.showUserLinks(ctx, update, payload)
}

// htmlEscape replaces Telegram's reserved HTML characters so we can embed
// user-provided text inside <code> tags safely.
func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
