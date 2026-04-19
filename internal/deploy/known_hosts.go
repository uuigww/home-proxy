package deploy

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// userConfigDir is a package-level alias so tests can override the location
// of the per-user config directory without touching the real filesystem.
var userConfigDir = os.UserConfigDir

// KnownHostsPath returns the absolute path of the home-proxy known_hosts file
// inside the user's config directory, creating the parent directory if needed.
func KnownHostsPath() (string, error) {
	base, err := userConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate user config dir: %w", err)
	}
	dir := filepath.Join(base, "home-proxy")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create config dir: %w", err)
	}
	return filepath.Join(dir, "known_hosts"), nil
}

// HostKeyPromptFn is called when an unknown host key is encountered. Return
// true to accept and persist, false to refuse the connection.
type HostKeyPromptFn func(host, fingerprint string) (bool, error)

// HostKeyCallback returns an ssh.HostKeyCallback backed by the known_hosts
// file at path. On first encounter of an unknown host, promptFn is invoked
// and — if it returns true — the key is appended to the file. Mismatches are
// rejected outright.
func HostKeyCallback(path string, promptFn HostKeyPromptFn) (ssh.HostKeyCallback, error) {
	if path == "" {
		return nil, errors.New("known_hosts path is empty")
	}
	if promptFn == nil {
		return nil, errors.New("host-key prompt is required")
	}
	if err := ensureFile(path); err != nil {
		return nil, err
	}
	base, err := knownhosts.New(path)
	if err != nil {
		return nil, fmt.Errorf("load known_hosts: %w", err)
	}

	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := base(hostname, remote, key)
		if err == nil {
			return nil
		}
		var keyErr *knownhosts.KeyError
		if !errors.As(err, &keyErr) {
			return err
		}
		if len(keyErr.Want) > 0 {
			// The host is known but the key changed → refuse.
			return fmt.Errorf("host key mismatch for %s: %w", hostname, err)
		}
		// First-seen host: ask the user.
		fp := ssh.FingerprintSHA256(key)
		ok, perr := promptFn(hostname, fp)
		if perr != nil {
			return fmt.Errorf("host-key prompt: %w", perr)
		}
		if !ok {
			return fmt.Errorf("host %s rejected by user", hostname)
		}
		return appendKnownHost(path, hostname, remote, key)
	}, nil
}

// ensureFile creates path (and its parent) with 0600 perms if it does not exist.
func ensureFile(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat known_hosts: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create known_hosts dir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create known_hosts: %w", err)
	}
	return f.Close()
}

// appendKnownHost appends a host/key line to the known_hosts file.
func appendKnownHost(path, hostname string, remote net.Addr, key ssh.PublicKey) error {
	addrs := []string{knownhosts.Normalize(hostname)}
	if remote != nil {
		if ra := knownhosts.Normalize(remote.String()); ra != addrs[0] {
			addrs = append(addrs, ra)
		}
	}
	line := knownhosts.Line(addrs, key)
	if !strings.HasSuffix(line, "\n") {
		line += "\n"
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		return fmt.Errorf("open known_hosts for append: %w", err)
	}
	defer f.Close()
	buf := bytes.NewBufferString(line)
	if _, err := buf.WriteTo(f); err != nil {
		return fmt.Errorf("append known_hosts: %w", err)
	}
	return nil
}
