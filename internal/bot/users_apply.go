package bot

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-telegram/bot/models"

	"github.com/uuigww/home-proxy/internal/store"
	"github.com/uuigww/home-proxy/internal/xray"
)

// reapplyXrayConfig rebuilds xray's config.json from current DB state and
// restarts the xray systemd unit so the change takes effect.
//
// All user mutations (add/toggle/enable/delete) funnel through here instead of
// the per-user runtime API. The runtime adu API and the SOCKS-via-restart path
// are mutually incompatible: the SOCKS restart wipes any users added through
// adu, so the only consistent strategy is a single full rebuild after every
// DB write.
func (b *Bot) reapplyXrayConfig(ctx context.Context) error {
	rk, err := b.deps.Store.GetRealityKeys(ctx)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("reality keys not initialised: %w", err)
		}
		return fmt.Errorf("get reality keys: %w", err)
	}
	return b.applyXrayConfig(ctx, xray.Reality{
		PrivateKey: rk.PrivateKey,
		PublicKey:  rk.PublicKey,
		ShortID:    rk.ShortID,
		Dest:       rk.Dest,
		ServerName: rk.ServerName,
	})
}

// applyUserChange atomically updates the DB then Xray; on Xray failure the
// DB change is reverted so the two stores never diverge.
//
// `before` is the snapshot used for DB rollback, `after` is the new state
// to persist. `xrayApply` does the positive-path Xray call; `xrayRollback`
// best-effort undoes it when apply fails mid-way.
func (b *Bot) applyUserChange(
	ctx context.Context,
	before, after *store.User,
	xrayApply, xrayRollback func(context.Context) error,
) error {
	if after == nil {
		return fmt.Errorf("apply change: after is nil")
	}
	if err := b.deps.Store.UpdateUser(ctx, after); err != nil {
		return fmt.Errorf("db update: %w", err)
	}
	if xrayApply == nil {
		return nil
	}
	if err := xrayApply(ctx); err != nil {
		if before != nil {
			if rbErr := b.deps.Store.UpdateUser(ctx, before); rbErr != nil {
				b.deps.Log.Error("bot: rollback db failed", "err", rbErr)
			}
		}
		if xrayRollback != nil {
			if rbErr := xrayRollback(ctx); rbErr != nil {
				b.deps.Log.Warn("bot: rollback xray failed", "err", rbErr)
			}
		}
		return fmt.Errorf("xray apply: %w", err)
	}
	return nil
}

// protoList returns a human-readable "+"-joined list of protocols the user
// has enabled. Used in info notifications ("VLESS+SOCKS5").
func protoList(u store.User) string {
	parts := make([]string, 0, 3)
	if u.VLESSUUID != nil {
		parts = append(parts, "VLESS")
	}
	if u.SOCKSUser != nil {
		parts = append(parts, "SOCKS5")
	}
	if u.MTProtoEnabled {
		parts = append(parts, "MTProto")
	}
	if len(parts) == 0 {
		return "—"
	}
	return strings.Join(parts, "+")
}

// notifyInfo emits an informational audit-trail event if the bot has a
// notifier attached. Errors are swallowed so UI flow is never blocked by
// notification plumbing.
func (b *Bot) notifyInfo(ctx context.Context, update *models.Update, id string, actor int64, args ...any) {
	if b.notifier == nil {
		return
	}
	actorName := "admin"
	if update != nil && update.CallbackQuery != nil && update.CallbackQuery.From.Username != "" {
		actorName = update.CallbackQuery.From.Username
	} else if update != nil && update.Message != nil && update.Message.From != nil && update.Message.From.Username != "" {
		actorName = update.Message.From.Username
	}
	params := append([]any{actorName}, args...)
	ev := Event{
		ID:        id,
		Severity:  SeverityInfo,
		Params:    params,
		ActorTGID: actor,
	}
	if err := b.notifier.Notify(ctx, ev); err != nil {
		b.deps.Log.Warn("bot: notify info", "err", err, "event", id)
	}
}
