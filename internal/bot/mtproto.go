package bot

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/go-telegram/bot/models"

	"github.com/uuigww/home-proxy/internal/store"
)

// mtgConfigPath is the on-disk location of the mtg daemon's TOML config. The
// installer writes it; the rotate handler overwrites it in place with a fresh
// secret before kicking systemd to restart the service.
const mtgConfigPath = "/etc/home-proxy/mtg.toml"

// mtgSystemdUnit is the unit file name we control for the mtg companion
// process. Kept as a package constant so tests can shadow it later without
// weaving it through every call site.
const mtgSystemdUnit = "home-proxy-mtg.service"

// rotateMTProtoSecret regenerates the Fake-TLS secret, persists it and kicks
// mtg to reload. All existing tg://proxy links become invalid after success —
// admins are expected to reshare per-user.
//
// The apply order is: run `mtg generate-secret` → save new config in DB →
// write mtg.toml on disk → `systemctl restart`. On any write failure we
// best-effort roll back the DB row to whatever was there before.
func (b *Bot) rotateMTProtoSecret(ctx context.Context, update *models.Update) error {
	tgID := updateTGID(update)
	lang := b.adminLang(ctx, tgID)
	if !b.deps.Cfg.MTProtoEnabled {
		return b.answerInfo(ctx, update, "server.mtproto_disabled")
	}

	host := b.deps.Cfg.MTProtoFakeTLSHost
	if strings.TrimSpace(host) == "" {
		return b.answerInfo(ctx, update, "err.generic")
	}

	genCtx, genCancel := context.WithTimeout(ctx, 10*time.Second)
	defer genCancel()
	secretBytes, err := exec.CommandContext(genCtx, "mtg", "generate-secret", "--hex", host).Output()
	if err != nil {
		b.deps.Log.Error("bot: mtg generate-secret", "err", err)
		return b.answerInfo(ctx, update, "err.generic")
	}
	newSecret := strings.TrimSpace(string(bytes.TrimRight(secretBytes, "\n")))
	if newSecret == "" {
		b.deps.Log.Error("bot: mtg generate-secret produced empty secret")
		return b.answerInfo(ctx, update, "err.generic")
	}

	// Snapshot the previous config so we can roll back on a file-write bail-out.
	prev, _ := b.deps.Store.GetMTGConfig(ctx)

	newCfg := store.MTGConfig{
		Secret:      newSecret,
		Port:        b.deps.Cfg.MTProtoPort,
		FakeTLSHost: host,
		UpdatedAt:   time.Now().UTC(),
	}
	if err := b.deps.Store.SaveMTGConfig(ctx, newCfg); err != nil {
		b.deps.Log.Error("bot: save mtg config", "err", err)
		return b.answerInfo(ctx, update, "err.generic")
	}

	if err := writeMTGConfigFile(mtgConfigPath, newCfg); err != nil {
		b.deps.Log.Error("bot: write mtg.toml", "err", err)
		if prev.Secret != "" {
			if rbErr := b.deps.Store.SaveMTGConfig(ctx, prev); rbErr != nil {
				b.deps.Log.Warn("bot: rollback mtg db failed", "err", rbErr)
			}
		}
		return b.answerInfo(ctx, update, "err.generic")
	}

	restartCtx, restartCancel := context.WithTimeout(ctx, 15*time.Second)
	defer restartCancel()
	if err := exec.CommandContext(restartCtx, "systemctl", "restart", mtgSystemdUnit).Run(); err != nil {
		b.deps.Log.Error("bot: restart mtg unit", "err", err)
		return b.answerInfo(ctx, update, "err.generic")
	}

	actor := "admin"
	if update != nil && update.CallbackQuery != nil && update.CallbackQuery.From.Username != "" {
		actor = update.CallbackQuery.From.Username
	}
	if b.notifier != nil {
		ev := Event{
			ID:        "mtproto.rotated",
			Severity:  SeverityInfo,
			Params:    []any{actor},
			ActorTGID: tgID,
		}
		if err := b.notifier.Notify(ctx, ev); err != nil {
			b.deps.Log.Warn("bot: notify mtproto rotated", "err", err)
		}
	}

	// Toast the actor then redraw the server screen so the updated timestamp
	// and truncated secret tail are visible.
	if update != nil && update.CallbackQuery != nil {
		b.answerCallback(ctx, update.CallbackQuery.ID, b.deps.I18n.T(lang, "server.mtproto_rotated"))
	}
	return b.showServer(ctx, update)
}

// writeMTGConfigFile overwrites path with the minimal mtg TOML needed to boot
// the daemon. Mode 0600 keeps the secret readable only by root.
func writeMTGConfigFile(path string, c store.MTGConfig) error {
	if c.Secret == "" {
		return errors.New("mtg: empty secret")
	}
	body := fmt.Sprintf("secret = %q\nbind-to = %q\n", c.Secret, fmt.Sprintf("0.0.0.0:%d", c.Port))
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
