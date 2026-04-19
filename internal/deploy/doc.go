// Package deploy implements the local-side SSH deploy wizard for home-proxy.
//
// The wizard collects server credentials and Telegram bot parameters from the
// user (interactively or via flags), opens an SSH session, uploads the
// installer script and systemd unit, runs the server-side install, and
// sanity-checks that the bot token is valid. All long-running network I/O is
// context-aware so Ctrl-C aborts cleanly.
//
// The package does not perform any real install work itself: that lives in
// the server-side installer script, which is embedded under ./bootstrap and
// uploaded verbatim.
package deploy
