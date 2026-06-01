package ipc

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// DefaultSocketPath returns the socket path used when neither a CLI
// flag nor $SEA_NOISE_SOCKET is set. Resolves to ~/.sea-noise/daemon.sock.
//
// On Darwin and Linux a Unix-domain socket's sun_path is capped at
// roughly 100 bytes (104 on macOS, 108 on Linux); long $HOME values
// can violate that. ValidateSocketPath checks the budget so callers
// fail fast at startup with a clear message rather than at bind time.
func DefaultSocketPath() (string, error) {
	if env := os.Getenv("SEA_NOISE_SOCKET"); env != "" {
		return env, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("ipc: resolve home dir: %w", err)
	}
	return filepath.Join(home, ".sea-noise", "daemon.sock"), nil
}

// ErrSocketPathTooLong is returned when a configured socket path
// exceeds the platform's sun_path budget. See .tempor/KNOWN-DELTA.md.
var ErrSocketPathTooLong = errors.New("ipc: socket path exceeds sun_path limit")

// ErrSocketInUse is returned by Server.Start when the configured
// socket path is already serving another daemon. The contract in
// DAEMON.md is "refuse to start, exit non-zero" — there is no --force.
var ErrSocketInUse = errors.New("ipc: socket already in use by another daemon")

// maxSunPath is the conservative limit covering both Darwin (104)
// and Linux (108). The final byte is reserved for the null
// terminator on most platforms.
const maxSunPath = 103

// ValidateSocketPath returns ErrSocketPathTooLong if path will not
// fit in sun_path on the current platform.
func ValidateSocketPath(path string) error {
	if len(path) > maxSunPath {
		return fmt.Errorf("%w: %d > %d (%s)", ErrSocketPathTooLong, len(path), maxSunPath, path)
	}
	return nil
}
