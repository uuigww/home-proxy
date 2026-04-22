package deploy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/uuigww/home-proxy/internal/deploy/bootstrap"
)

// AuthMethod picks how the deploy wizard authenticates its outbound SSH.
type AuthMethod int

const (
	// AuthUnset marks AuthMethod as not yet selected. The wizard treats it as
	// a "please prompt" signal.
	AuthUnset AuthMethod = iota
	// AuthPassword authenticates with a plaintext password.
	AuthPassword
	// AuthKey authenticates with a private key file on disk.
	AuthKey
	// AuthAgent authenticates via an already-running ssh-agent.
	AuthAgent
)

// String renders an AuthMethod for diagnostics.
func (a AuthMethod) String() string {
	switch a {
	case AuthPassword:
		return "password"
	case AuthKey:
		return "key"
	case AuthAgent:
		return "agent"
	default:
		return "unset"
	}
}

// Params captures everything the wizard needs to run. Fields left zero are
// filled in by PromptMissing when the process is interactive.
type Params struct {
	Host       string
	Port       int
	User       string
	AuthMethod AuthMethod
	Password   string
	KeyPath    string
	KeyPass    string
	BotToken   string
	Admins     []int64
	Lang       string
}

// Wizard orchestrates the deploy pipeline. It owns the Params struct and
// streams progress to a pluggable io.Writer.
type Wizard struct {
	params Params
	out    io.Writer
	in     io.Reader

	// dialFn is the hook used by Run to open an SSH connection. Tests can
	// replace it to bypass the network.
	dialFn func(ctx context.Context, p Params) (sshConn, error)

	// localBinCandidates are the paths the wizard probes for a locally-built
	// home-proxy binary to upload. Tests override this to control behaviour.
	localBinCandidates []string
}

// sshConn is the minimal SSH surface used by Run. SSHClient satisfies it.
type sshConn interface {
	Run(ctx context.Context, cmd string) (string, string, error)
	RunStreaming(ctx context.Context, cmd string, out io.Writer) error
	Upload(local, remote string, mode os.FileMode) error
	UploadBytes(data []byte, remote string, mode os.FileMode) error
	Close() error
}

// New returns a Wizard configured to write progress to os.Stdout and read
// prompts from os.Stdin.
func New(params Params) *Wizard {
	return &Wizard{
		params:             params,
		out:                os.Stdout,
		in:                 os.Stdin,
		dialFn:             defaultDial,
		localBinCandidates: []string{"./home-proxy", "./bin/home-proxy"},
	}
}

// SetOutput redirects wizard output to w. Must be called before Run.
func (w *Wizard) SetOutput(out io.Writer) {
	if out != nil {
		w.out = out
	}
}

// Params returns a copy of the wizard's current parameters (mainly for tests).
func (w *Wizard) Params() Params { return w.params }

// PromptMissing fills in any unset Params fields via interactive prompts.
// Fully populated Params are a no-op.
func (w *Wizard) PromptMissing(ctx context.Context) error {
	_ = ctx // survey/v2 doesn't accept a Context; Ctrl-C is handled by the TUI.
	return Prompt(&w.params)
}

// Validate checks the Params in-place and returns a descriptive error on the
// first violation found.
func (w *Wizard) Validate() error { return w.params.Validate() }

// Validate is the free-standing validator used by tests and the command layer.
func (p Params) Validate() error {
	if strings.TrimSpace(p.Host) == "" {
		return errors.New("host is required")
	}
	if p.Port < 1 || p.Port > 65535 {
		return fmt.Errorf("port %d out of range", p.Port)
	}
	if strings.TrimSpace(p.User) == "" {
		return errors.New("user is required")
	}
	switch p.AuthMethod {
	case AuthPassword:
		if p.Password == "" {
			return errors.New("password is required")
		}
	case AuthKey:
		if p.KeyPath == "" {
			return errors.New("ssh key path is required")
		}
	case AuthAgent:
		// nothing to validate here.
	default:
		return errors.New("auth method is not set")
	}
	if !botTokenRE.MatchString(p.BotToken) {
		return errors.New("bot token is not in NNN:AA... shape")
	}
	if len(p.Admins) == 0 {
		return errors.New("at least one admin id is required")
	}
	switch p.Lang {
	case "ru", "en":
	default:
		return fmt.Errorf("lang must be ru or en, got %q", p.Lang)
	}
	return nil
}

// botTokenRE matches the Telegram bot-token shape (NNNNN:alnum_-).
var botTokenRE = regexp.MustCompile(`^\d{5,}:[A-Za-z0-9_\-]{20,}$`)

// Run executes the 8-step deploy pipeline against the remote server.
func (w *Wizard) Run(ctx context.Context) error {
	p := NewProgress(w.out, 8)

	// Step 1 — validate.
	p.Step("Validate parameters")
	if err := w.Validate(); err != nil {
		p.Fail(err)
		return err
	}
	p.OK(fmt.Sprintf("host=%s user=%s auth=%s lang=%s",
		w.params.Host, w.params.User, w.params.AuthMethod, w.params.Lang))

	// Step 2 — SSH connect.
	p.Step("Connect over SSH")
	conn, err := w.dialFn(ctx, w.params)
	if err != nil {
		p.Fail(err)
		return err
	}
	defer conn.Close()
	p.OK("connected")

	// Step 3 — detect OS/arch.
	p.Step("Detect remote OS and architecture")
	osArch, err := detectOSArch(ctx, conn)
	if err != nil {
		p.Fail(err)
		return err
	}
	p.OK(osArch)

	// Step 4 — upload binary (best effort).
	p.Step("Upload home-proxy binary")
	if err := w.uploadBinary(ctx, conn, p); err != nil {
		p.Fail(err)
		return err
	}

	// Step 5 — upload installer script.
	p.Step("Upload installer script")
	installer, err := bootstrap.FS.ReadFile("install.sh")
	if err != nil {
		p.Fail(fmt.Errorf("read embedded installer: %w", err))
		return err
	}
	if err := conn.UploadBytes(installer, "/tmp/home-proxy-install.sh", 0o755); err != nil {
		p.Fail(err)
		return err
	}
	p.OK("/tmp/home-proxy-install.sh")

	// Step 6 — upload systemd unit.
	p.Step("Upload systemd unit")
	unit, err := bootstrap.FS.ReadFile("home-proxy.service")
	if err != nil {
		p.Fail(fmt.Errorf("read embedded unit: %w", err))
		return err
	}
	if err := conn.UploadBytes(unit, "/etc/systemd/system/home-proxy.service", 0o644); err != nil {
		p.Fail(err)
		return err
	}
	p.OK("/etc/systemd/system/home-proxy.service")

	// Step 7 — run installer.
	p.Step("Run installer on the server")
	cmd := buildInstallerCmd(w.params)
	if err := conn.RunStreaming(ctx, cmd, w.out); err != nil {
		p.Fail(err)
		return err
	}
	p.OK("installer completed")

	// Step 8 — verify bot token.
	p.Step("Verify Telegram bot token")
	if err := verifyBotToken(ctx, conn, w.params.BotToken); err != nil {
		p.Fail(err)
		return err
	}
	p.OK("bot reachable")

	return nil
}

// uploadBinary probes localBinCandidates and uploads the first existing Linux
// ELF match. When none exist (or only non-Linux binaries are found), a warning
// is printed but the pipeline continues — install.sh will fetch a release.
func (w *Wizard) uploadBinary(_ context.Context, conn sshConn, p *Progress) error {
	for _, cand := range w.localBinCandidates {
		info, err := os.Stat(cand)
		if err != nil {
			continue
		}
		if info.IsDir() {
			continue
		}
		if !isLinuxELF(cand) {
			p.Info(fmt.Sprintf("skipping %s: not a Linux ELF binary", cand))
			continue
		}
		p.Info(fmt.Sprintf("uploading %s (%.1f MB)…", cand, float64(info.Size())/1024/1024))
		if err := conn.Upload(cand, "/tmp/home-proxy", 0o755); err != nil {
			return err
		}
		p.OK(fmt.Sprintf("uploaded %s → /tmp/home-proxy", cand))
		return nil
	}
	p.Info("no local Linux binary found — install.sh will download a release")
	p.OK("skipped (server-side install will download)")
	return nil
}

// isLinuxELF reports whether the file at path begins with the ELF magic bytes.
func isLinuxELF(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	var magic [4]byte
	if _, err := io.ReadFull(f, magic[:]); err != nil {
		return false
	}
	return magic[0] == 0x7f && magic[1] == 'E' && magic[2] == 'L' && magic[3] == 'F'
}

// detectOSArch runs `uname -sm` on the server and returns a trimmed label.
func detectOSArch(ctx context.Context, conn sshConn) (string, error) {
	stdout, stderr, err := conn.Run(ctx, "uname -sm")
	if err != nil {
		msg := strings.TrimSpace(stderr)
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("uname: %s", msg)
	}
	out := strings.TrimSpace(stdout)
	if out == "" {
		return "", errors.New("uname produced no output")
	}
	return out, nil
}

// buildInstallerCmd composes the remote shell invocation for the installer.
// Secrets are passed on the command line; we could switch to env vars later,
// but for now `install.sh` only consumes flags.
func buildInstallerCmd(p Params) string {
	admins := make([]string, 0, len(p.Admins))
	for _, id := range p.Admins {
		admins = append(admins, fmt.Sprintf("%d", id))
	}
	return fmt.Sprintf(
		"sudo bash /tmp/home-proxy-install.sh --bot-token %s --admins %s --lang %s",
		shellQuote(p.BotToken),
		shellQuote(strings.Join(admins, ",")),
		shellQuote(p.Lang),
	)
}

// verifyBotToken hits the Telegram getMe endpoint from the remote host and
// asserts the response contains `"ok":true`.
func verifyBotToken(ctx context.Context, conn sshConn, token string) error {
	cmd := fmt.Sprintf(
		"curl -s -m 10 https://api.telegram.org/bot%s/getMe",
		shellQuote(token),
	)
	stdout, _, err := conn.Run(ctx, cmd)
	if err != nil {
		return fmt.Errorf("curl getMe: %w", err)
	}
	if !strings.Contains(stdout, `"ok":true`) {
		return errors.New("telegram getMe did not return ok=true")
	}
	return nil
}

// shellQuote wraps a string in single quotes, escaping embedded single quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// defaultDial is the production SSH dialler used by New.
func defaultDial(ctx context.Context, p Params) (sshConn, error) {
	return Dial(ctx, p)
}
