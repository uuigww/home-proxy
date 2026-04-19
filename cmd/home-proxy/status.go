package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	var (
		host, user, password, keyPath string
		port                          int
	)

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Check remote home-proxy server status over SSH",
		RunE: func(cmd *cobra.Command, _ []string) error {
			// TODO (M7): SSH and show `systemctl is-active home-proxy`, xray version, users count.
			fmt.Printf("home-proxy status — stub. host=%s user=%s\n", host, user)
			return nil
		},
	}

	cmd.Flags().StringVar(&host, "host", "", "Remote server IP / hostname")
	cmd.Flags().StringVar(&user, "user", "root", "SSH user")
	cmd.Flags().IntVar(&port, "port", 22, "SSH port")
	cmd.Flags().StringVar(&password, "password", "", "SSH password")
	cmd.Flags().StringVar(&keyPath, "key", "", "Path to SSH private key")

	return cmd
}
