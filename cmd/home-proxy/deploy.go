package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/uuigww/home-proxy/internal/deploy"
)

// deployFlags mirrors the CLI surface expected by users and tooling. The flag
// names must not change without updating docs and scripts.
type deployFlags struct {
	host     string
	user     string
	password string
	keyPath  string
	keyPass  string
	useAgent bool
	port     int
	botToken string
	admins   string
	lang     string
	nonInter bool
}

// newDeployCmd wires the `home-proxy deploy` subcommand.
func newDeployCmd() *cobra.Command {
	f := &deployFlags{}

	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Install home-proxy on a remote server over SSH (local wizard)",
		Long: "Run an interactive wizard (or use flags) to provision a fresh " +
			"remote server with Xray+Reality, SOCKS5, and the home-proxy " +
			"Telegram bot. Everything happens over SSH from your machine.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDeploy(cmd.Context(), f, cmd.OutOrStdout())
		},
	}

	cmd.Flags().StringVar(&f.host, "host", "", "Remote server IP / hostname")
	cmd.Flags().StringVar(&f.user, "user", "root", "SSH user")
	cmd.Flags().IntVar(&f.port, "port", 22, "SSH port")
	cmd.Flags().StringVar(&f.password, "password", "", "SSH password (prefer --key or --agent)")
	cmd.Flags().StringVar(&f.keyPath, "key", "", "Path to SSH private key")
	cmd.Flags().StringVar(&f.keyPass, "key-pass", "", "Passphrase for the SSH key")
	cmd.Flags().BoolVar(&f.useAgent, "agent", false, "Use ssh-agent")
	cmd.Flags().StringVar(&f.botToken, "bot-token", "", "Telegram bot token")
	cmd.Flags().StringVar(&f.admins, "admins", "", "Comma-separated admin Telegram IDs")
	cmd.Flags().StringVar(&f.lang, "lang", "ru", "Bot UI language: ru | en")
	cmd.Flags().BoolVar(&f.nonInter, "yes", false, "Non-interactive mode; fail if anything is missing")

	return cmd
}

// runDeploy is the RunE body, broken out so it is testable with arbitrary
// flag structs and output writers.
func runDeploy(parent context.Context, f *deployFlags, out io.Writer) error {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	params, err := flagsToParams(f)
	if err != nil {
		return err
	}

	w := deploy.New(params)
	w.SetOutput(out)

	if !f.nonInter && hasMissing(params) {
		if err := w.PromptMissing(ctx); err != nil {
			return fmt.Errorf("collect inputs: %w", err)
		}
		// Refresh the local copy for validation reporting.
		params = w.Params()
	}

	if err := params.Validate(); err != nil {
		return fmt.Errorf("invalid deploy params: %w", err)
	}

	if err := w.Run(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			fmt.Fprintln(os.Stderr, "aborted.")
			return err
		}
		return err
	}
	return nil
}

// flagsToParams converts CLI flags into a deploy.Params struct, parsing
// comma-separated admin IDs along the way.
func flagsToParams(f *deployFlags) (deploy.Params, error) {
	p := deploy.Params{
		Host:     strings.TrimSpace(f.host),
		Port:     f.port,
		User:     strings.TrimSpace(f.user),
		Password: f.password,
		KeyPath:  strings.TrimSpace(f.keyPath),
		KeyPass:  f.keyPass,
		BotToken: strings.TrimSpace(f.botToken),
		Lang:     strings.TrimSpace(f.lang),
	}

	// Pick an auth method. Precedence: agent > key > password > unset.
	switch {
	case f.useAgent:
		p.AuthMethod = deploy.AuthAgent
	case f.keyPath != "":
		p.AuthMethod = deploy.AuthKey
	case f.password != "":
		p.AuthMethod = deploy.AuthPassword
	default:
		p.AuthMethod = deploy.AuthUnset
	}

	if strings.TrimSpace(f.admins) != "" {
		ids, err := deploy.ParseAdmins(f.admins)
		if err != nil {
			return p, fmt.Errorf("--admins: %w", err)
		}
		p.Admins = ids
	}

	return p, nil
}

// hasMissing reports whether any required field is still empty and therefore
// warrants an interactive prompt.
func hasMissing(p deploy.Params) bool {
	if p.Host == "" || p.User == "" || p.Port == 0 {
		return true
	}
	if p.AuthMethod == deploy.AuthUnset {
		return true
	}
	switch p.AuthMethod {
	case deploy.AuthPassword:
		if p.Password == "" {
			return true
		}
	case deploy.AuthKey:
		if p.KeyPath == "" {
			return true
		}
	}
	if p.BotToken == "" || len(p.Admins) == 0 || p.Lang == "" {
		return true
	}
	return false
}
