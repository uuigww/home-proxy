package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newUninstallCmd() *cobra.Command {
	var (
		host, user, password, keyPath string
		port                          int
		purge                         bool
	)

	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove home-proxy from a remote server over SSH",
		RunE: func(cmd *cobra.Command, _ []string) error {
			// TODO (M7): SSH, stop service, remove unit, optionally rm -rf /var/lib/home-proxy.
			fmt.Printf("home-proxy uninstall — stub. host=%s purge=%v\n", host, purge)
			return nil
		},
	}

	cmd.Flags().StringVar(&host, "host", "", "Remote server IP / hostname")
	cmd.Flags().StringVar(&user, "user", "root", "SSH user")
	cmd.Flags().IntVar(&port, "port", 22, "SSH port")
	cmd.Flags().StringVar(&password, "password", "", "SSH password")
	cmd.Flags().StringVar(&keyPath, "key", "", "Path to SSH private key")
	cmd.Flags().BoolVar(&purge, "purge", false, "Also remove state dir (/var/lib/home-proxy)")

	return cmd
}
