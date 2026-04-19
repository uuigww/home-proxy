package deploy

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// SSHClient is a thin wrapper around *ssh.Client that knows its target host
// and exposes helpers tailored to the deploy wizard's needs.
type SSHClient struct {
	conn *ssh.Client
	host string
}

// Dial opens an SSH connection to the server described by p. It loads (or
// creates) the user's known_hosts file and prompts interactively the first
// time a host key is seen.
func Dial(ctx context.Context, p Params) (*SSHClient, error) {
	return DialWithPrompt(ctx, p, defaultHostKeyPrompt)
}

// DialWithPrompt is like Dial but lets callers (and tests) inject a custom
// first-seen host-key prompt.
func DialWithPrompt(ctx context.Context, p Params, prompt HostKeyPromptFn) (*SSHClient, error) {
	if p.Host == "" {
		return nil, errors.New("host is required")
	}
	if p.Port == 0 {
		p.Port = 22
	}
	if p.User == "" {
		p.User = "root"
	}

	khPath, err := KnownHostsPath()
	if err != nil {
		return nil, err
	}
	hkCb, err := HostKeyCallback(khPath, prompt)
	if err != nil {
		return nil, err
	}

	auth, err := buildAuthMethods(p)
	if err != nil {
		return nil, err
	}

	cfg := &ssh.ClientConfig{
		User:            p.User,
		Auth:            auth,
		HostKeyCallback: hkCb,
		Timeout:         15 * time.Second,
	}

	addr := net.JoinHostPort(p.Host, strconv.Itoa(p.Port))
	type dialResult struct {
		c   *ssh.Client
		err error
	}
	ch := make(chan dialResult, 1)
	go func() {
		c, err := ssh.Dial("tcp", addr, cfg)
		ch <- dialResult{c, err}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-ch:
		if res.err != nil {
			return nil, fmt.Errorf("ssh dial %s: %w", addr, res.err)
		}
		return &SSHClient{conn: res.c, host: p.Host}, nil
	}
}

// Host returns the hostname this client was dialled to.
func (c *SSHClient) Host() string { return c.host }

// Client exposes the underlying *ssh.Client for callers that need raw access
// (for example the SFTP subsystem).
func (c *SSHClient) Client() *ssh.Client { return c.conn }

// Close terminates the SSH connection.
func (c *SSHClient) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

// Run executes a short command on the remote host and returns its captured
// stdout and stderr. It aborts early if ctx is cancelled.
func (c *SSHClient) Run(ctx context.Context, cmd string) (string, string, error) {
	if c == nil || c.conn == nil {
		return "", "", errors.New("ssh client is not connected")
	}
	sess, err := c.conn.NewSession()
	if err != nil {
		return "", "", fmt.Errorf("new ssh session: %w", err)
	}
	defer sess.Close()

	var stdout, stderr bytes.Buffer
	sess.Stdout = &stdout
	sess.Stderr = &stderr

	done := make(chan error, 1)
	go func() { done <- sess.Run(cmd) }()

	select {
	case <-ctx.Done():
		_ = sess.Signal(ssh.SIGKILL)
		_ = sess.Close()
		return stdout.String(), stderr.String(), ctx.Err()
	case err := <-done:
		if err != nil {
			return stdout.String(), stderr.String(), fmt.Errorf("remote command: %w", err)
		}
		return stdout.String(), stderr.String(), nil
	}
}

// RunStreaming executes a command with a PTY attached and streams its stdout
// (and stderr-merged) output line-by-line to out. Suitable for long-running
// installer runs where the user wants to watch progress.
func (c *SSHClient) RunStreaming(ctx context.Context, cmd string, out io.Writer) error {
	if c == nil || c.conn == nil {
		return errors.New("ssh client is not connected")
	}
	sess, err := c.conn.NewSession()
	if err != nil {
		return fmt.Errorf("new ssh session: %w", err)
	}
	defer sess.Close()

	modes := ssh.TerminalModes{
		ssh.ECHO:          0,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := sess.RequestPty("xterm", 80, 40, modes); err != nil {
		return fmt.Errorf("request pty: %w", err)
	}

	pipe, err := sess.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	sess.Stderr = out

	if err := sess.Start(cmd); err != nil {
		return fmt.Errorf("start remote command: %w", err)
	}

	copyDone := make(chan struct{})
	go func() {
		defer close(copyDone)
		scanner := bufio.NewScanner(pipe)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			fmt.Fprintln(out, scanner.Text())
		}
	}()

	waitDone := make(chan error, 1)
	go func() { waitDone <- sess.Wait() }()

	select {
	case <-ctx.Done():
		_ = sess.Signal(ssh.SIGKILL)
		_ = sess.Close()
		<-copyDone
		return ctx.Err()
	case err := <-waitDone:
		<-copyDone
		if err != nil {
			return fmt.Errorf("remote command: %w", err)
		}
		return nil
	}
}

// buildAuthMethods assembles ssh.AuthMethod values from the deploy params.
// Password and key material are never persisted; they live only in the
// AuthMethod closures until the connection completes.
func buildAuthMethods(p Params) ([]ssh.AuthMethod, error) {
	switch p.AuthMethod {
	case AuthPassword:
		if p.Password == "" {
			return nil, errors.New("password is empty")
		}
		return []ssh.AuthMethod{ssh.Password(p.Password)}, nil
	case AuthKey:
		if p.KeyPath == "" {
			return nil, errors.New("ssh key path is empty")
		}
		data, err := os.ReadFile(p.KeyPath)
		if err != nil {
			return nil, fmt.Errorf("read ssh key: %w", err)
		}
		var signer ssh.Signer
		if p.KeyPass != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(data, []byte(p.KeyPass))
		} else {
			signer, err = ssh.ParsePrivateKey(data)
		}
		if err != nil {
			return nil, fmt.Errorf("parse ssh key: %w", err)
		}
		return []ssh.AuthMethod{ssh.PublicKeys(signer)}, nil
	case AuthAgent:
		sock := os.Getenv("SSH_AUTH_SOCK")
		if sock == "" {
			return nil, errors.New("SSH_AUTH_SOCK is not set; ssh-agent unavailable")
		}
		conn, err := net.Dial("unix", sock)
		if err != nil {
			return nil, fmt.Errorf("dial ssh-agent: %w", err)
		}
		ag := agent.NewClient(conn)
		return []ssh.AuthMethod{ssh.PublicKeysCallback(ag.Signers)}, nil
	default:
		return nil, fmt.Errorf("unknown auth method: %d", p.AuthMethod)
	}
}

// defaultHostKeyPrompt prints the fingerprint to stderr and reads y/N from stdin.
func defaultHostKeyPrompt(host, fingerprint string) (bool, error) {
	fmt.Fprintf(os.Stderr, "The authenticity of host %q can't be established.\n", host)
	fmt.Fprintf(os.Stderr, "Key fingerprint is %s.\n", fingerprint)
	fmt.Fprint(os.Stderr, "Accept and save to known_hosts? [y/N]: ")
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	if len(line) == 0 {
		return false, nil
	}
	switch line[0] {
	case 'y', 'Y':
		return true, nil
	}
	return false, nil
}
