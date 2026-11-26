// SPDX-License-Identifier: Apache-2.0

package computermcp

import (
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Environment variables of the per-turn MCP file (docs/spec/27-computer-use.md 10.1). The runner
// sets these; nothing else reaches the process (D-CU-05-2 adds EnvServer, D-CU-06-12 the optional
// EnvCAFile).
const (
	// EnvDir is the request folder of the run (27 10.2 item 1).
	EnvDir = "BOTHY_COMPUTER_DIR"
	// EnvNonce is the run's nonce; every request carries it (27 10.2 item 2).
	EnvNonce = "BOTHY_COMPUTER_NONCE"
	// EnvTimeoutMS is runner.computer_request_timeout in milliseconds (27 10.2 item 4).
	EnvTimeoutMS = "BOTHY_COMPUTER_TIMEOUT_MS"
	// EnvPins is the absolute path of the pin file (27 10.4).
	EnvPins = "BOTHY_COMPUTER_PINS"
	// EnvServer is runner.server_url, the origin attach_path is relative to (D-CU-05-2).
	EnvServer = "BOTHY_COMPUTER_SERVER"
	// EnvCAFile is optional: the absolute path of a PEM bundle whose certificates the attach
	// connection trusts next to the system roots, the CA file of the runner (D-CU-06-12).
	EnvCAFile = "BOTHY_COMPUTER_CA_FILE"
)

// Config is the configuration of one server process.
type Config struct {
	// Dir is the request folder; Nonce the run's nonce.
	Dir, Nonce string
	// Timeout bounds the wait for an answer of the runner and every step of the attach.
	Timeout time.Duration
	// PinsPath is the pin file.
	PinsPath string
	// Server is the origin of the server (https, or http only on 127.0.0.1).
	Server *url.URL
	// Version is the version this binary reports in serverInfo.
	Version string
	// Log receives action kinds, codes and durations only; nil discards.
	Log *slog.Logger
	// Random is the source of the ephemeral keys and request IDs; nil means crypto/rand.
	Random io.Reader
	// HTTPClient dials the attach connection; nil means a client with RootCAs.
	HTTPClient *http.Client
	// RootCAs are the system roots plus the certificates of EnvCAFile; nil means the system
	// roots only.
	RootCAs *x509.CertPool
}

// errConfig marks a configuration error; main exits with the usage code for it.
var errConfig = errors.New("computermcp: configuration")

// ConfigFromEnv reads the variables through getenv (os.Getenv in production). EnvCAFile is
// optional: when set it must be an absolute path of a readable PEM file with at least one
// certificate, which joins the system roots. Every other variable is required: the folders and the pin file as absolute paths, the timeout as a
// positive number of milliseconds, the server as an https origin without path, query or user
// (http only for the host 127.0.0.1, like runner.server_url). The error names the variable and
// wraps errConfig; it never quotes the nonce.
//
// Example: ConfigFromEnv(os.Getenv).
func ConfigFromEnv(getenv func(string) string) (Config, error) {
	bad := func(name, why string) (Config, error) {
		return Config{}, fmt.Errorf("%w: %s %s", errConfig, name, why)
	}
	var cfg Config
	for _, p := range []struct {
		name string
		dst  *string
	}{{EnvDir, &cfg.Dir}, {EnvPins, &cfg.PinsPath}} {
		v := getenv(p.name)
		if v == "" || !filepath.IsAbs(v) {
			return bad(p.name, "must be an absolute path")
		}
		*p.dst = filepath.Clean(v)
	}
	if cfg.Nonce = getenv(EnvNonce); cfg.Nonce == "" {
		return bad(EnvNonce, "is empty")
	}
	ms, err := strconv.ParseInt(getenv(EnvTimeoutMS), 10, 64)
	if err != nil || ms <= 0 {
		return bad(EnvTimeoutMS, "must be a positive number of milliseconds")
	}
	cfg.Timeout = time.Duration(ms) * time.Millisecond
	u, err := url.Parse(getenv(EnvServer))
	if err != nil || u.Host == "" || u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return bad(EnvServer, "must be an origin like https://bothy.example.com")
	}
	switch {
	case u.Scheme == "https":
	case u.Scheme == "http" && u.Hostname() == "127.0.0.1":
	default:
		return bad(EnvServer, "must use https (http only for 127.0.0.1)")
	}
	u.Path = ""
	cfg.Server = u
	if cfg.RootCAs, err = rootsOf(getenv(EnvCAFile)); err != nil {
		return bad(EnvCAFile, err.Error())
	}
	return cfg, nil
}

// rootsOf returns nil for an empty path (the system roots), else the system roots plus the
// certificates of the PEM file path, which must be absolute.
func rootsOf(path string) (*x509.CertPool, error) {
	if path == "" {
		return nil, nil
	}
	if !filepath.IsAbs(path) {
		return nil, errors.New("must be an absolute path")
	}
	pem, err := os.ReadFile(filepath.Clean(path)) // #nosec G304 -- the runner names its own CA file (D-CU-06-12)
	if err != nil {
		return nil, errors.New("cannot be read")
	}
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	if !pool.AppendCertsFromPEM(pem) {
		return nil, errors.New("holds no PEM certificate")
	}
	return pool, nil
}

// IsConfigError reports whether err is a configuration error of ConfigFromEnv or New.
func IsConfigError(err error) bool { return errors.Is(err, errConfig) }

// attachURL returns the WebSocket URL of attach_path on the server: wss for https, ws for http.
// attach_path must be an absolute path on the same origin (27 5.1 step 8); anything else is
// refused, so an answer cannot send the token to another host.
func attachURL(server *url.URL, path string) (string, error) {
	p, err := url.Parse(path)
	if err != nil || p.Scheme != "" || p.Host != "" || p.User != nil || len(path) < 2 || path[0] != '/' || path[1] == '/' || p.RawQuery != "" || p.Fragment != "" {
		return "", errors.New("computermcp: attach_path is not an absolute path")
	}
	u := *server
	u.Scheme = "wss"
	if server.Scheme == "http" {
		u.Scheme = "ws"
	}
	u.Path = p.Path
	return u.String(), nil
}
