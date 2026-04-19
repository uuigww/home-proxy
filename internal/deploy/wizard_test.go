package deploy

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

// validParams returns a Params value that passes Validate. Individual tests
// mutate one field at a time to assert the relevant error fires.
func validParams() Params {
	return Params{
		Host:       "1.2.3.4",
		Port:       22,
		User:       "root",
		AuthMethod: AuthPassword,
		Password:   "hunter2",
		BotToken:   "123456789:AAABBBCCCDDDEEEFFFGGGHHHIIIJJJKKK",
		Admins:     []int64{111, 222},
		Lang:       "ru",
	}
}

func TestParamsValidate_Ok(t *testing.T) {
	if err := validParams().Validate(); err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
}

func TestParamsValidate_MissingHost(t *testing.T) {
	p := validParams()
	p.Host = ""
	if err := p.Validate(); err == nil {
		t.Fatal("expected error for missing host")
	}
}

func TestParamsValidate_PortRange(t *testing.T) {
	for _, port := range []int{0, -1, 70000} {
		p := validParams()
		p.Port = port
		if err := p.Validate(); err == nil {
			t.Fatalf("expected error for port %d", port)
		}
	}
}

func TestParamsValidate_BadBotToken(t *testing.T) {
	p := validParams()
	p.BotToken = "not-a-token"
	if err := p.Validate(); err == nil {
		t.Fatal("expected error for malformed bot token")
	}
}

func TestParamsValidate_EmptyAdmins(t *testing.T) {
	p := validParams()
	p.Admins = nil
	if err := p.Validate(); err == nil {
		t.Fatal("expected error for empty admins")
	}
}

func TestParamsValidate_BadLang(t *testing.T) {
	p := validParams()
	p.Lang = "de"
	if err := p.Validate(); err == nil {
		t.Fatal("expected error for non-ru/en lang")
	}
}

func TestParamsValidate_AuthPasswordMissingPassword(t *testing.T) {
	p := validParams()
	p.Password = ""
	if err := p.Validate(); err == nil {
		t.Fatal("expected error for missing password")
	}
}

func TestParamsValidate_AuthKeyMissingPath(t *testing.T) {
	p := validParams()
	p.AuthMethod = AuthKey
	p.Password = ""
	p.KeyPath = ""
	if err := p.Validate(); err == nil {
		t.Fatal("expected error for missing key path")
	}
}

func TestParamsValidate_AuthUnset(t *testing.T) {
	p := validParams()
	p.AuthMethod = AuthUnset
	if err := p.Validate(); err == nil {
		t.Fatal("expected error for AuthUnset")
	}
}

func TestParseAdmins_OK(t *testing.T) {
	ids, err := ParseAdmins(" 1, 22 ,333 ")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(ids) != 3 || ids[0] != 1 || ids[1] != 22 || ids[2] != 333 {
		t.Fatalf("bad ids: %v", ids)
	}
}

func TestParseAdmins_BadValue(t *testing.T) {
	if _, err := ParseAdmins("1,not-a-number,3"); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestParseAdmins_Empty(t *testing.T) {
	if _, err := ParseAdmins("   "); err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestAuthMethodString(t *testing.T) {
	if AuthPassword.String() != "password" {
		t.Fatalf("bad: %s", AuthPassword.String())
	}
	if AuthKey.String() != "key" {
		t.Fatalf("bad: %s", AuthKey.String())
	}
	if AuthAgent.String() != "agent" {
		t.Fatalf("bad: %s", AuthAgent.String())
	}
	if AuthUnset.String() != "unset" {
		t.Fatalf("bad: %s", AuthUnset.String())
	}
}

// fakeConn is an in-memory sshConn implementation for Run tests.
type fakeConn struct {
	runs          []string
	runOutputs    map[string]string
	runErrs       map[string]error
	streamed      []string
	streamErr     error
	uploads       []string
	uploadsBytes  []string
	uploadErr     error
	uploadBytesEr error
	closeCalled   bool
}

func (f *fakeConn) Run(_ context.Context, cmd string) (string, string, error) {
	f.runs = append(f.runs, cmd)
	if f.runErrs != nil {
		if err, ok := f.runErrs[cmd]; ok {
			return "", "", err
		}
	}
	if f.runOutputs != nil {
		if out, ok := f.runOutputs[cmd]; ok {
			return out, "", nil
		}
	}
	return "", "", nil
}

func (f *fakeConn) RunStreaming(_ context.Context, cmd string, _ io.Writer) error {
	f.streamed = append(f.streamed, cmd)
	return f.streamErr
}

func (f *fakeConn) Upload(local, remote string, _ os.FileMode) error {
	f.uploads = append(f.uploads, local+"→"+remote)
	return f.uploadErr
}

func (f *fakeConn) UploadBytes(_ []byte, remote string, _ os.FileMode) error {
	f.uploadsBytes = append(f.uploadsBytes, remote)
	return f.uploadBytesEr
}

func (f *fakeConn) Close() error {
	f.closeCalled = true
	return nil
}

// TestWizardRun_Happy exercises the full eight-step pipeline with a fake
// SSH connection and asserts every expected side effect happened.
func TestWizardRun_Happy(t *testing.T) {
	fc := &fakeConn{
		runOutputs: map[string]string{
			"uname -sm": "Linux x86_64",
		},
	}

	// Pre-populate a "remote" getMe response.
	fc.runOutputs[anyGetMeCmd(validParams().BotToken)] = `{"ok":true,"result":{"id":1}}`

	w := &Wizard{
		params: validParams(),
		out:    &bytes.Buffer{},
		dialFn: func(_ context.Context, _ Params) (sshConn, error) { return fc, nil },
		localBinCandidates: []string{
			"/nonexistent/home-proxy",
		},
	}

	if err := w.Run(context.Background()); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if !fc.closeCalled {
		t.Error("expected Close to be called")
	}
	if len(fc.uploadsBytes) < 2 {
		t.Errorf("expected at least 2 byte uploads, got %d", len(fc.uploadsBytes))
	}
	if len(fc.streamed) != 1 {
		t.Errorf("expected one streamed command, got %d", len(fc.streamed))
	}
	if !strings.Contains(fc.streamed[0], "home-proxy-install.sh") {
		t.Errorf("streamed command should invoke installer: %q", fc.streamed[0])
	}
}

// anyGetMeCmd returns the exact shell command the wizard runs for getMe so
// the fake can seed its map.
func anyGetMeCmd(token string) string {
	return "curl -s -m 10 https://api.telegram.org/bot" + shellQuote(token) + "/getMe"
}

func TestWizardRun_FailsFastOnValidation(t *testing.T) {
	w := &Wizard{
		params: Params{}, // intentionally empty
		out:    &bytes.Buffer{},
		dialFn: func(_ context.Context, _ Params) (sshConn, error) {
			t.Fatal("dial should not be called on validation failure")
			return nil, nil
		},
	}
	err := w.Run(context.Background())
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestWizardRun_GetMeFailure(t *testing.T) {
	fc := &fakeConn{
		runOutputs: map[string]string{
			"uname -sm": "Linux x86_64",
		},
	}
	// getMe returns a response without "ok":true
	fc.runOutputs[anyGetMeCmd(validParams().BotToken)] = `{"ok":false,"description":"Unauthorized"}`

	w := &Wizard{
		params:             validParams(),
		out:                &bytes.Buffer{},
		dialFn:             func(_ context.Context, _ Params) (sshConn, error) { return fc, nil },
		localBinCandidates: []string{"/nonexistent"},
	}
	err := w.Run(context.Background())
	if err == nil {
		t.Fatal("expected getMe failure")
	}
}

// TestPromptMissing_NoOpWhenFull confirms fully-populated Params skip all
// interactive prompts (which would otherwise block, not being wired to a TTY).
func TestPromptMissing_NoOpWhenFull(t *testing.T) {
	w := New(validParams())
	if err := w.PromptMissing(context.Background()); err != nil {
		t.Fatalf("expected no-op prompt to succeed, got %v", err)
	}
}

// TestNew_DefaultWriters ensures New wires output and input to standard
// streams without panicking.
func TestNew_DefaultWriters(t *testing.T) {
	w := New(Params{})
	if w.out == nil {
		t.Fatal("default output must not be nil")
	}
	if w.in == nil {
		t.Fatal("default input must not be nil")
	}
}

// TestBuildInstallerCmd_ArgsQuoted makes sure admin list and language are
// passed quoted, so values with commas or spaces would survive.
func TestBuildInstallerCmd_ArgsQuoted(t *testing.T) {
	cmd := buildInstallerCmd(validParams())
	want := []string{"--bot-token", "--admins", "--lang", "111,222", "'ru'"}
	for _, w := range want {
		if !strings.Contains(cmd, w) {
			t.Errorf("expected %q in %q", w, cmd)
		}
	}
}

func TestShellQuote_Escape(t *testing.T) {
	q := shellQuote("a'b")
	if !strings.HasPrefix(q, "'") || !strings.HasSuffix(q, "'") {
		t.Fatalf("not quoted: %q", q)
	}
	if !strings.Contains(q, `'\''`) {
		t.Fatalf("single-quote not escaped: %q", q)
	}
}

// Ensure detectOSArch surfaces a clear error on blank output.
func TestDetectOSArch_Blank(t *testing.T) {
	fc := &fakeConn{runOutputs: map[string]string{"uname -sm": "   "}}
	if _, err := detectOSArch(context.Background(), fc); err == nil {
		t.Fatal("expected error on blank uname output")
	}
}

// Ensure detectOSArch wraps remote errors.
func TestDetectOSArch_RemoteError(t *testing.T) {
	fc := &fakeConn{runErrs: map[string]error{"uname -sm": errors.New("exit 1")}}
	if _, err := detectOSArch(context.Background(), fc); err == nil {
		t.Fatal("expected error")
	}
}
