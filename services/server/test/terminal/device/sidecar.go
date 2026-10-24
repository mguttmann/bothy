// SPDX-License-Identifier: Apache-2.0

package device

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

// SidecarError is an error answer of the Noise sidecar: Kind is the name of the sentinel error of
// internal/noise or internal/frame of the host service (ErrHandshake, ErrDecrypt, ...) or a
// protocol kind (bad_json, unknown_op, out_of_order, bad_field).
type SidecarError struct {
	Kind string
}

// Error implements error.
func (e *SidecarError) Error() string { return "noise sidecar: " + e.Kind }

// ErrSidecarGone reports a sidecar process that ended or wrote no answer.
var ErrSidecarGone = errors.New("device: the noise sidecar ended")

// Sidecar drives one process of services/host/test/noisedevice (Design note 6): the Noise
// initiator and the frames of one device session with the host service's own internal/noise and
// internal/frame. Requests and answers are JSON lines; bytes are base64url without padding. A
// Sidecar is safe for concurrent use; the calls are serialised.
type Sidecar struct {
	mu   sync.Mutex
	cmd  *exec.Cmd
	in   io.WriteCloser
	out  *bufio.Reader
	done bool
}

// BuildSidecar builds services/host/test/noisedevice for the machine of the tests into dir
// (GOTOOLCHAIN=local, CGO_ENABLED=0), as M2-42's TestHostPairAgainstServer builds bothy-host, and
// returns the path of the binary.
//
// Errors: a failed build with its output.
func BuildSidecar(ctx context.Context, root, dir string) (string, error) {
	bin := filepath.Join(dir, "noisedevice")
	cmd := exec.CommandContext(ctx, "go", "build", "-trimpath", "-o", bin, "./test/noisedevice") //nolint:gosec // G204: fixed program, the output path of the suite
	cmd.Dir = filepath.Join(root, "services", "host")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOTOOLCHAIN=local") //nolint:forbidigo // go needs PATH, HOME and the caches of the machine
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("device: build noisedevice: %w\n%s", err, out)
	}
	return bin, nil
}

// StartSidecar starts the sidecar binary bin. Its stderr is discarded: it carries no bytes of keys,
// tokens or frames, and the suite prints nothing of it.
//
// Errors: a failed start.
func StartSidecar(bin string) (*Sidecar, error) {
	cmd := exec.Command(bin) //nolint:noctx // the sidecar lives as long as the device session, not as one request
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &Sidecar{cmd: cmd, in: in, out: bufio.NewReader(out)}, nil
}

// NewSidecarIO returns a Sidecar over the given pipes without a process (the unit tests run the
// protocol against a fake).
func NewSidecarIO(in io.WriteCloser, out io.Reader) *Sidecar {
	return &Sidecar{in: in, out: bufio.NewReader(out)}
}

// call sends one request and decodes the answer into v.
func (s *Sidecar) call(req map[string]any, v any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.done {
		return ErrSidecarGone
	}
	b, err := json.Marshal(req)
	if err != nil {
		return err
	}
	if _, err := s.in.Write(append(b, '\n')); err != nil {
		return ErrSidecarGone
	}
	line, err := s.out.ReadBytes('\n')
	if err != nil {
		return ErrSidecarGone
	}
	var head struct {
		OK    bool `json:"ok"`
		Error *struct {
			Kind string `json:"kind"`
		} `json:"error"`
	}
	if err := json.Unmarshal(line, &head); err != nil {
		return fmt.Errorf("device: sidecar answer: %w", err)
	}
	if !head.OK {
		if head.Error == nil {
			return &SidecarError{Kind: "unknown"}
		}
		return &SidecarError{Kind: head.Error.Kind}
	}
	if v == nil {
		return nil
	}
	return json.Unmarshal(line, v)
}

// b64 encodes base64url without padding; unb64 decodes it.
func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func unb64(s string) ([]byte, error) { return base64.RawURLEncoding.DecodeString(s) }

// Start begins a handshake with the static public key of the host and the session and host IDs
// (the prologue of 24 5.2) and returns the ephemeral public key of the device and its hash, the
// client_ephemeral_key_hash of openTerminalSession (claim cek).
//
// Errors: a SidecarError.
func (s *Sidecar) Start(hostStatic []byte, sessionID, hostID string) (ephemeral []byte, cek string, err error) {
	var a struct {
		Ephemeral string `json:"ephemeral_public"`
		CEK       string `json:"cek"`
	}
	if err := s.call(map[string]any{"op": "start", "host_static": b64(hostStatic), "session_id": sessionID, "host_id": hostID}, &a); err != nil {
		return nil, "", err
	}
	e, err := unb64(a.Ephemeral)
	return e, a.CEK, err
}

// Message1 returns Noise message 1 with payload (the session token).
func (s *Sidecar) Message1(payload []byte) ([]byte, error) {
	return s.bytesOp("message1", "payload", payload, "message")
}

// Message2 reads Noise message 2 and returns its payload.
func (s *Sidecar) Message2(msg []byte) ([]byte, error) {
	return s.bytesOp("message2", "message", msg, "payload")
}

// Seal encrypts one encoded frame of the device.
func (s *Sidecar) Seal(frame []byte) ([]byte, error) {
	return s.bytesOp("seal", "frame", frame, "message")
}

// Open decrypts one transport message of the host and returns the encoded frame.
func (s *Sidecar) Open(msg []byte) ([]byte, error) { return s.bytesOp("open", "message", msg, "frame") }

// bytesOp sends op with one bytes field and returns the bytes field of the answer.
func (s *Sidecar) bytesOp(op, field string, b []byte, answer string) ([]byte, error) {
	var a map[string]any
	if err := s.call(map[string]any{"op": op, field: b64(b)}, &a); err != nil {
		return nil, err
	}
	v, ok := a[answer].(string)
	if !ok {
		return nil, fmt.Errorf("device: sidecar answer of %s lacks %s", op, answer)
	}
	return unb64(v)
}

// Close ends the sidecar: stdin closes, the process exits 0 at EOF.
func (s *Sidecar) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.done {
		return nil
	}
	s.done = true
	err := s.in.Close()
	if s.cmd != nil {
		if werr := s.cmd.Wait(); werr != nil && err == nil {
			err = werr
		}
	}
	return err
}

// Keypair draws the ephemeral key pair before Start and returns its public key and hash: the
// device sends client_ephemeral_key_hash in openTerminalSession before it knows the session ID
// of the prologue; Start then keeps that key.
//
// Errors: a SidecarError.
func (s *Sidecar) Keypair() (ephemeral []byte, cek string, err error) {
	var a struct {
		Ephemeral string `json:"ephemeral_public"`
		CEK       string `json:"cek"`
	}
	if err := s.call(map[string]any{"op": "keypair"}, &a); err != nil {
		return nil, "", err
	}
	e, err := unb64(a.Ephemeral)
	return e, a.CEK, err
}
