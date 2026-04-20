//go:build !unix

package limits

import "errors"

// diskFree is a fallback for non-unix targets (notably Windows). The real
// daemon runs on Linux under systemd, so this path only matters when the
// deploy wizard or tests are cross-compiled for developer convenience.
var diskFree = func(path string) (int64, error) {
	return 0, errors.New("diskFree: not supported on this platform")
}
