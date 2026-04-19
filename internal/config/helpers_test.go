package config

import "os"

func writeBytes(path string, b []byte) error {
	return os.WriteFile(path, b, 0o600)
}
