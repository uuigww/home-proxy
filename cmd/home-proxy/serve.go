package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/uuigww/home-proxy/internal/bot"
	"github.com/uuigww/home-proxy/internal/config"
	"github.com/uuigww/home-proxy/internal/i18n"
	"github.com/uuigww/home-proxy/internal/limits"
	"github.com/uuigww/home-proxy/internal/store"
	"github.com/uuigww/home-proxy/internal/xray"
)

// newServeCmd returns the root-mounted `serve` subcommand.
//
// It wires together config → store → i18n → xray client → bot and blocks on
// the Telegram long-poll until the process receives SIGINT/SIGTERM.
func newServeCmd() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the home-proxy daemon (used by systemd on the server)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runServe(cmd.Context(), configPath)
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", "/etc/home-proxy/config.toml",
		"Path to config.toml")

	return cmd
}

// runServe is the actual serve loop, factored out so it's test-friendlier.
func runServe(ctx context.Context, configPath string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{}))
	logger.Info("home-proxy: starting", "config", configPath)

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	dbPath := cfg.DataDir + "/state.db"
	st, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open store %q: %w", dbPath, err)
	}
	defer st.Close()

	bundle, err := i18n.New()
	if err != nil {
		return fmt.Errorf("init i18n: %w", err)
	}

	xc := xray.NewCLIClient("xray", cfg.XrayAPI)
	xc.VLESSTag = cfg.XrayVLESSTag
	xc.SOCKSTag = cfg.XraySocksTag
	xc.ConfigPath = cfg.XrayConfig
	xc.RestartXray = func(ctx context.Context) error {
		// Detached from caller's deadline so a slow restart can't strand
		// xray with a half-applied config.
		restartCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		out, err := exec.CommandContext(restartCtx, "systemctl", "restart", "xray").CombinedOutput()
		if err != nil {
			return fmt.Errorf("systemctl restart xray: %w (output: %s)", err, strings.TrimSpace(string(out)))
		}
		return nil
	}

	b, err := bot.New(ctx, cfg.BotToken, bot.Deps{
		Store:       st,
		Xray:        xc,
		I18n:        bundle,
		Admins:      cfg.Admins,
		DefaultLang: cfg.DefaultLang,
		Cfg:         cfg,
		Log:         logger,
	})
	if err != nil {
		return fmt.Errorf("init bot: %w", err)
	}

	watcher := limits.New(st, xc, newLimitsBridge(b.Notifier()), limits.Config{}, logger)
	go func() {
		if err := watcher.Start(ctx); err != nil && ctx.Err() == nil {
			logger.Error("limits watcher stopped", "err", err)
		}
	}()

	logger.Info("home-proxy: bot started, polling for updates")
	return b.Start(ctx)
}
