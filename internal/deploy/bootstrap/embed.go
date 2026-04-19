// Package bootstrap embeds static assets (installer script and systemd unit)
// that the deploy wizard uploads to a fresh remote server. Embedding lets the
// deploy binary ship self-contained without requiring network access to the
// project repo at deploy time.
package bootstrap

import "embed"

// FS holds the files uploaded by the deploy wizard.
//
//go:embed install.sh home-proxy.service
var FS embed.FS
