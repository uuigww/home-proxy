package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/uuigww/home-proxy/internal/version"
)

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "home-proxy",
		Short: "Self-hosted Xray+Reality+SOCKS5 proxy managed from Telegram",
		Long: "home-proxy runs an Xray-core instance (VLESS+Reality, SOCKS5) on your " +
			"server and exposes all administration through a Telegram bot.\n\n" +
			"Subcommands:\n" +
			"  serve      — run the daemon (systemd-managed, on the server)\n" +
			"  deploy     — local wizard that installs home-proxy on a remote server via SSH\n" +
			"  status     — query remote server status over SSH\n" +
			"  uninstall  — remove home-proxy from a remote server over SSH",
		Version:       version.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(newServeCmd())
	root.AddCommand(newDeployCmd())
	root.AddCommand(newStatusCmd())
	root.AddCommand(newUninstallCmd())

	return root
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
