// SPDX-License-Identifier: Apache-2.0

package demo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// The files of the exchange directory (16 16.5). The command writes the first two with mode 0600;
// the runner of the stack writes the third.
const (
	// AgentsFile holds the instance and every agent with role, token and private DPoP key: the
	// input of the demo agent.
	AgentsFile = "agents.json"
	// HostPairingFile holds the host, its pairing code and its session users: the input of
	// bothy-host install and bothy-host pair on the demo host.
	HostPairingFile = "host-pairing.json"
	// HostPairedFile holds the fingerprint bothy-host pair printed: what the owner types.
	HostPairedFile = "host-paired.json"
)

// exchangePoll is the interval at which the command looks for an answer of the stack (the paired
// host, the connected agents). The overall wait is bounded by the named values of the data set.
const exchangePoll = time.Second

// AgentsDoc is the content of AgentsFile.
type AgentsDoc struct {
	InstanceID string       `json:"instance_id"`
	Values     AgentValues  `json:"values"`
	Agents     []AgentEntry `json:"agents"`
}

// AgentValues are the named values of the data set the demo agent uses (16 16.3).
type AgentValues struct {
	WorkerSteps       int    `json:"worker_steps"`
	StepIntervalMS    int64  `json:"step_interval_ms"`
	WatcherIntervalMS int64  `json:"watcher_interval_ms"`
	TokensIn          int64  `json:"tokens_in"`
	TokensOut         int64  `json:"tokens_out"`
}

// AgentEntry is one agent of AgentsDoc: handle, role, model and the currency of its price, ID and
// its credentials.
type AgentEntry struct {
	Handle         string         `json:"handle"`
	Role           string         `json:"role"`
	Provider       string         `json:"provider"`
	Model          string         `json:"model"`
	Currency       string         `json:"currency"`
	AgentID        string         `json:"agent_id"`
	Token          string         `json:"token"`
	DPoPPrivateJWK map[string]any `json:"dpop_private_jwk"`
}

// HostPairingDoc is the content of HostPairingFile.
type HostPairingDoc struct {
	HostID       string   `json:"host_id"`
	PairingCode  string   `json:"pairing_code"`
	SessionUsers []string `json:"session_users"`
}

// HostPairedDoc is the content of HostPairedFile.
type HostPairedDoc struct {
	Fingerprint string `json:"fingerprint"`
}

// writePrivate writes v as JSON to name in dir with mode 0600 through a temporary file and a
// rename, so a reader never sees half a file.
func writePrivate(dir, name string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+name+".*")
	if err != nil {
		return fmt.Errorf("demo: write %s: %w", name, err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("demo: write %s: %w", name, err)
	}
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("demo: write %s: %w", name, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("demo: write %s: %w", name, err)
	}
	if err := os.Rename(tmp.Name(), filepath.Join(dir, name)); err != nil {
		return fmt.Errorf("demo: write %s: %w", name, err)
	}
	return nil
}

// errWaitExpired is the error of a wait of the exchange that ran past its bound.
var errWaitExpired = errors.New("demo: the wait ran past its bound")

// waitFor calls check every exchangePoll until it reports done, fails, ctx ends or bound passes.
// what names the wait in the error.
func waitFor(ctx context.Context, sleep func(context.Context, time.Duration) error, bound time.Duration, what string,
	check func() (bool, error)) error {
	deadline := time.Now().Add(bound)
	for {
		done, err := check()
		if err != nil || done {
			return err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%w: %s after %v", errWaitExpired, what, bound)
		}
		if err := sleep(ctx, exchangePoll); err != nil {
			return err
		}
	}
}

// readPaired reads HostPairedFile of dir; done is false while it does not exist.
func readPaired(dir string) (HostPairedDoc, bool, error) {
	var doc HostPairedDoc
	b, err := os.ReadFile(filepath.Join(dir, HostPairedFile)) // #nosec G304 -- the exchange directory of the run
	if errors.Is(err, fs.ErrNotExist) {
		return doc, false, nil
	}
	if err != nil {
		return doc, false, fmt.Errorf("demo: read %s: %w", HostPairedFile, err)
	}
	if err := json.Unmarshal(b, &doc); err != nil || doc.Fingerprint == "" {
		return doc, false, fmt.Errorf("demo: %s has no fingerprint", HostPairedFile)
	}
	return doc, true, nil
}

// checkExchangeDir refuses a directory that group or others can access: it receives tokens and
// keys (16 16.5).
func checkExchangeDir(dir string) error {
	fi, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("demo: the exchange directory: %w", err)
	}
	if !fi.IsDir() {
		return errors.New("demo: the exchange directory is no directory")
	}
	if fi.Mode().Perm()&0o077 != 0 {
		return errors.New("demo: the exchange directory must have mode 0700")
	}
	return nil
}
