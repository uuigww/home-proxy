package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newServeCmd() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the home-proxy daemon (used by systemd on the server)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			// TODO (M3/M4/M5): load config, init store, start Xray sync, start bot, start limits watcher.
			fmt.Printf("home-proxy serve — stub. config=%s\n", configPath)
			return nil
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", "/etc/home-proxy/config.toml",
		"Path to config.toml")

	return cmd
}
