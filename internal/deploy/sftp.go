package deploy

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"

	"github.com/pkg/sftp"
)

// Upload copies a local file to remote via SFTP with the given mode.
func (c *SSHClient) Upload(local, remote string, mode os.FileMode) error {
	if c == nil || c.conn == nil {
		return errors.New("ssh client is not connected")
	}
	f, err := os.Open(local)
	if err != nil {
		return fmt.Errorf("open local file: %w", err)
	}
	defer f.Close()
	return c.uploadReader(f, remote, mode)
}

// UploadBytes writes data to remote via SFTP with the given mode.
func (c *SSHClient) UploadBytes(data []byte, remote string, mode os.FileMode) error {
	if c == nil || c.conn == nil {
		return errors.New("ssh client is not connected")
	}
	return c.uploadReader(bytes.NewReader(data), remote, mode)
}

// uploadReader is the shared implementation for Upload and UploadBytes.
func (c *SSHClient) uploadReader(r io.Reader, remote string, mode os.FileMode) error {
	client, err := sftp.NewClient(c.conn)
	if err != nil {
		return fmt.Errorf("open sftp session: %w", err)
	}
	defer client.Close()

	dir := path.Dir(remote)
	if dir != "" && dir != "." && dir != "/" {
		if err := client.MkdirAll(dir); err != nil {
			return fmt.Errorf("create remote dir %q: %w", dir, err)
		}
	}

	dst, err := client.OpenFile(remote, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return fmt.Errorf("open remote file %q: %w", remote, err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, r); err != nil {
		return fmt.Errorf("write remote file %q: %w", remote, err)
	}
	if err := client.Chmod(remote, mode); err != nil {
		return fmt.Errorf("chmod remote file %q: %w", remote, err)
	}
	return nil
}
