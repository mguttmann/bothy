// SPDX-License-Identifier: Apache-2.0

package computermcp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// answerPoll is how often the request client looks for the answer file. The runner answers
// within its own HTTP round trip; the value is a protocol constant of this binary, not a limit
// (D-CU-05-6), and the wait itself is bounded by BOTHY_COMPUTER_TIMEOUT_MS.
const answerPoll = 20 * time.Millisecond

// Suffixes of the file contract of 27 10.2.
const (
	requestSuffix = ".request.json"
	answerSuffix  = ".answer.json"
	tmpSuffix     = ".tmp"
)

// errTimeout is a request the runner did not answer within the timeout.
var errTimeout = errors.New("computermcp: no answer of the runner")

// Desktop is one item of listComputerDesktops (27 12.2).
type Desktop struct {
	HostID   string `json:"host_id"`
	Name     string `json:"name"`
	Platform string `json:"platform"`
	Online   bool   `json:"online"`
	Locked   bool   `json:"locked"`
	Busy     bool   `json:"busy"`
}

// Grant is the answer body of openComputerSession (27 12.2).
type Grant struct {
	SessionID      string `json:"session_id"`
	Token          string `json:"token"`
	AttachPath     string `json:"attach_path"`
	NoisePublicKey string `json:"noise_public_key"`
	KeyFingerprint string `json:"key_fingerprint"`
	KeyGeneration  int64  `json:"key_generation"`
	ExpiresAt      string `json:"expires_at"`
}

// hostKey returns the 32 bytes of noise_public_key (strict base64url without padding, 24 5.5).
func (g Grant) hostKey() ([32]byte, error) {
	var k [32]byte
	b, err := base64.RawURLEncoding.Strict().DecodeString(g.NoisePublicKey)
	if err != nil || len(b) != len(k) {
		return k, errors.New("computermcp: noise_public_key is not a 32-byte key")
	}
	copy(k[:], b)
	return k, nil
}

// Refusal is an answer with ok false (27 10.2 item 3).
type Refusal struct {
	Status int     `json:"status"`
	Code   string  `json:"code"`
	Reason *string `json:"reason"`
}

// ToolCode maps the refusal to a code of 27 7 by its code and reason (27 12.4); a refusal this
// table does not name is host_offline, the code that tells the agent the desktop cannot be
// reached now (D-CU-05-7).
func (r Refusal) ToolCode() string {
	reason := ""
	if r.Reason != nil {
		reason = *r.Reason
	}
	switch {
	case reason == "computer_not_owner_run" || reason == "computer_tool_class" || r.Code == "invocation_token_invalid" || r.Code == "request_refused":
		return CodeNotOwnerRun
	case reason == "computer_not_enabled" || r.Code == "not_found" || r.Status == 404:
		return CodeNotEnabled
	case reason == "computer_busy" || reason == "computer_run_busy":
		return CodeBusy
	default:
		return CodeHostOffline
	}
}

// requestClient writes requests into the runner's folder and waits for the answers.
type requestClient struct {
	dir, nonce string
	timeout    time.Duration
	random     io.Reader
}

// listRequest and openRequest are the two request bodies of 27 10.2 item 2.
type listRequest struct {
	V     int    `json:"v"`
	Nonce string `json:"nonce"`
	Op    string `json:"op"`
}

type openRequest struct {
	V          int    `json:"v"`
	Nonce      string `json:"nonce"`
	Op         string `json:"op"`
	HostID     string `json:"host_id"`
	Cek        string `json:"client_ephemeral_key_hash"`
	PinChanged bool   `json:"pin_changed"`
}

// answer is the envelope of an answer file.
type answer struct {
	V      int             `json:"v"`
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result"`
	Status int             `json:"status"`
	Code   string          `json:"code"`
	Reason *string         `json:"reason"`
}

// list asks for the desktops of the run's owner. It returns the desktops, or a refusal, or an
// error (errTimeout, a broken answer).
func (c *requestClient) list(ctx context.Context) ([]Desktop, *Refusal, error) {
	res, ref, err := c.do(ctx, listRequest{V: 1, Nonce: c.nonce, Op: "list"})
	if err != nil || ref != nil {
		return nil, ref, err
	}
	var body struct {
		Items []Desktop `json:"items"`
	}
	if err := strictDecode(res, &body); err != nil || body.Items == nil {
		return nil, nil, errors.New("computermcp: the desktop list of the runner is malformed")
	}
	return body.Items, nil, nil
}

// open asks to open a session on hostID with the hash of the ephemeral key.
func (c *requestClient) open(ctx context.Context, hostID, cek string, pinChanged bool) (Grant, *Refusal, error) {
	res, ref, err := c.do(ctx, openRequest{V: 1, Nonce: c.nonce, Op: "open", HostID: hostID, Cek: cek, PinChanged: pinChanged})
	if err != nil || ref != nil {
		return Grant{}, ref, err
	}
	var g Grant
	if err := strictDecode(res, &g); err != nil || g.SessionID == "" || g.Token == "" || g.AttachPath == "" || g.NoisePublicKey == "" {
		return Grant{}, nil, errors.New("computermcp: the session grant of the runner is malformed")
	}
	return g, nil, nil
}

// do writes one request atomically (a .tmp file, then a rename in the same folder), waits for the
// answer at most the timeout, reads it and deletes both files. A timeout leaves both files for
// the runner's sweep (27 10.2 item 4).
func (c *requestClient) do(ctx context.Context, req any) (json.RawMessage, *Refusal, error) {
	id, err := newUUID(c.random)
	if err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, nil, err
	}
	reqPath := filepath.Join(c.dir, id+requestSuffix)
	if err := writeAtomic(reqPath, reqPath+tmpSuffix, body); err != nil {
		return nil, nil, fmt.Errorf("computermcp: write request: %w", err)
	}
	ansPath := filepath.Join(c.dir, id+answerSuffix)
	deadline := time.NewTimer(c.timeout)
	defer deadline.Stop()
	tick := time.NewTicker(answerPoll)
	defer tick.Stop()
	for {
		b, err := os.ReadFile(ansPath)
		if err == nil {
			// The request goes first: while it exists without an answer the runner would take
			// it again.
			_ = os.Remove(reqPath)
			_ = os.Remove(ansPath)
			return parseAnswer(b)
		}
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-deadline.C:
			return nil, nil, errTimeout
		case <-tick.C:
		}
	}
}

// parseAnswer checks the envelope: v 1, no unknown member, a result with ok true, a code with ok
// false.
func parseAnswer(b []byte) (json.RawMessage, *Refusal, error) {
	var a answer
	if err := strictDecode(b, &a); err != nil || a.V != 1 {
		return nil, nil, errors.New("computermcp: the answer of the runner is malformed")
	}
	if a.OK {
		if len(a.Result) == 0 {
			return nil, nil, errors.New("computermcp: the answer of the runner has no result")
		}
		return a.Result, nil, nil
	}
	if a.Code == "" {
		return nil, nil, errors.New("computermcp: the refusal of the runner has no code")
	}
	return nil, &Refusal{Status: a.Status, Code: a.Code, Reason: a.Reason}, nil
}

// strictDecode decodes one JSON value without unknown members and without trailing data.
func strictDecode(b []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if dec.More() {
		return errors.New("trailing data")
	}
	return nil
}

// writeAtomic writes b to path with mode 0600 through the new file tmp in the same folder and a
// rename, so a reader sees the old or the new content, never a part.
func writeAtomic(path, tmp string, b []byte) error {
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, werr := f.Write(b)
	cerr := f.Close()
	if werr != nil || cerr != nil {
		_ = os.Remove(tmp)
		return errors.Join(werr, cerr)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// newUUID returns a random UUID (version 4, RFC 9562) from random.
func newUUID(random io.Reader) (string, error) {
	var b [16]byte
	if _, err := io.ReadFull(random, b[:]); err != nil {
		return "", fmt.Errorf("computermcp: random: %w", err)
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
