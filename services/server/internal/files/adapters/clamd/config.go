// SPDX-License-Identifier: Apache-2.0

package clamd

import (
	"errors"
	"path"
	"strings"
	"time"
)

// maxChunkBytes is the largest chunk the four-byte length prefix can carry as a positive int32,
// the bound clamd reads it with.
const maxChunkBytes = 1<<31 - 1

// unixScheme is the only address form the client accepts (19 SEC-TLS-5).
const unixScheme = "unix://"

// Config is the configuration of the client, bound in internal/wire from the keys file.scan.*.
type Config struct {
	// Address is file.scan.clamd_address: unix:// followed by an absolute path.
	Address string `config:"file.scan.clamd_address"`
	// ChunkBytes is file.scan.chunk_bytes, the largest INSTREAM chunk and the bound of a reply.
	ChunkBytes int `config:"file.scan.chunk_bytes"`
	// Timeout is file.scan.timeout. The scan pass applies it to the whole pass; the client uses it
	// as the deadline of a call whose context has none.
	Timeout time.Duration `config:"file.scan.timeout"`
	// MaxConcurrent is file.scan.max_concurrent, the parallel scans of one process.
	MaxConcurrent int `config:"file.scan.max_concurrent"`
}

// ErrAddress reports a file.scan.clamd_address that is not unix:// plus an absolute path. It
// names the key, never the value.
var ErrAddress = errors.New("file.scan.clamd_address must be unix:// followed by an absolute path (19 SEC-TLS-5)")

// SocketPath returns the socket path of a clamd address, or ErrAddress. The cross-check
// clamd_address_unix of internal/wire uses it, so the configuration and the client apply one rule.
//
// Example:
//
//	p, err := clamd.SocketPath("unix:///run/clamav/clamd.sock") // "/run/clamav/clamd.sock"
func SocketPath(address string) (string, error) {
	p, ok := strings.CutPrefix(address, unixScheme)
	if !ok || !path.IsAbs(p) || path.Clean(p) != p || strings.ContainsAny(p, "\x00?#") {
		return "", ErrAddress
	}
	return p, nil
}

// check refuses values the client cannot work with; the errors name keys only.
func (c Config) check() (string, error) {
	p, err := SocketPath(c.Address)
	if err != nil {
		return "", err
	}
	switch {
	case c.ChunkBytes < 1 || c.ChunkBytes > maxChunkBytes:
		return "", errors.New("file.scan.chunk_bytes must be positive and fit the four-byte chunk length")
	case c.Timeout <= 0:
		return "", errors.New("file.scan.timeout must be positive")
	case c.MaxConcurrent < 1:
		return "", errors.New("file.scan.max_concurrent must be positive")
	}
	return p, nil
}
