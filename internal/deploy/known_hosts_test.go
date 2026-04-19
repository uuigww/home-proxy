package deploy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withUserConfigDir swaps the package-level userConfigDir for the duration of
// a test so we can isolate KnownHostsPath from the real filesystem.
func withUserConfigDir(t *testing.T, dir string) {
	t.Helper()
	orig := userConfigDir
	userConfigDir = func() (string, error) { return dir, nil }
	t.Cleanup(func() { userConfigDir = orig })
}

func TestKnownHostsPath_CreatesDir(t *testing.T) {
	tmp := t.TempDir()
	withUserConfigDir(t, tmp)

	path, err := KnownHostsPath()
	if err != nil {
		t.Fatalf("KnownHostsPath error: %v", err)
	}
	wantDir := filepath.Join(tmp, "home-proxy")
	if !strings.HasPrefix(path, wantDir) {
		t.Errorf("expected path under %q, got %q", wantDir, path)
	}
	info, err := os.Stat(wantDir)
	if err != nil {
		t.Fatalf("dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("known_hosts parent is not a directory")
	}
}

func TestKnownHostsPath_PropagatesError(t *testing.T) {
	orig := userConfigDir
	userConfigDir = func() (string, error) { return "", os.ErrPermission }
	t.Cleanup(func() { userConfigDir = orig })

	if _, err := KnownHostsPath(); err == nil {
		t.Fatal("expected error when userConfigDir fails")
	}
}

func TestEnsureFile_CreatesEmpty(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "sub", "known_hosts")
	if err := ensureFile(p); err != nil {
		t.Fatalf("ensureFile err: %v", err)
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if info.Size() != 0 {
		t.Errorf("expected empty file, got %d bytes", info.Size())
	}
}

func TestEnsureFile_Idempotent(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "known_hosts")
	if err := os.WriteFile(p, []byte("something"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := ensureFile(p); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != "something" {
		t.Errorf("ensureFile overwrote existing content: %q", data)
	}
}

func TestHostKeyCallback_RequiresPath(t *testing.T) {
	_, err := HostKeyCallback("", func(string, string) (bool, error) { return true, nil })
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestHostKeyCallback_RequiresPrompt(t *testing.T) {
	tmp := t.TempDir()
	_, err := HostKeyCallback(filepath.Join(tmp, "known_hosts"), nil)
	if err == nil {
		t.Fatal("expected error for nil prompt")
	}
}
