package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

type deployFlags struct {
	host      string
	user      string
	password  string
	keyPath   string
	keyPass   string
	useAgent  bool
	port      int
	botToken  string
	admins    string
	lang      string
	nonInter  bool
}

func newDeployCmd() *cobra.Command {
	f := &deployFlags{}

	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Install home-proxy on a remote server over SSH (local wizard)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			// TODO (M7): run interactive wizard (if no flags) or non-interactive deploy.
			fmt.Printf("home-proxy deploy — stub.\n  host=%s user=%s lang=%s\n",
				f.host, f.user, f.lang)
			return nil
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
