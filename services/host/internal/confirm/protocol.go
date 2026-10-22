// SPDX-License-Identifier: Apache-2.0

package confirm

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
)

// Kinds of a request.
const (
	// KindOpen asks before the PTY of a new session starts.
	KindOpen = "open"
	// KindAttach asks before the scrollback and the live output of a session go to a device.
	KindAttach = "attach"
)

// Ops of the protocol (brief M2-45 Design note 15 b).
const (
	opConfirm = "confirm"
	opCancel  = "cancel"
	opAnswer  = "answer"
)

// maxLine bounds one line including its line feed on both sides. It is a protocol constant, not
// a product limit: the largest message is a confirm with a profile and a session user name, far
// below it.
const maxLine = 4096

// Request is what the person at the machine is asked: open or attach, the profile and the session
// user; nothing else (no content, no path, no person ID).
type Request struct {
	// Kind is KindOpen or KindAttach.
	Kind string
	// Profile is the profile name of the session (docs/spec/24-terminal.md 11).
	Profile string
	// SessionUser is the operating system user the session runs as (claim osu).
	SessionUser string
}

// errMalformed marks a line that breaks the protocol.
var errMalformed = errors.New("confirm: malformed message")

// errLineTooLong marks a line longer than maxLine.
var errLineTooLong = errors.New("confirm: line too long")

// wire is a decoded line; a nil pointer is a missing member or a JSON null.
type wire struct {
	Op          *string `json:"op"`
	ID          *string `json:"id"`
	Kind        *string `json:"kind"`
	Profile     *string `json:"profile"`
	SessionUser *string `json:"session_user"`
	TimeoutMS   *int64  `json:"timeout_ms"`
	Allow       *bool   `json:"allow"`
}

type confirmMsg struct {
	Op          string `json:"op"`
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Profile     string `json:"profile"`
	SessionUser string `json:"session_user"`
	TimeoutMS   int64  `json:"timeout_ms"`
}

type cancelMsg struct {
	Op string `json:"op"`
	ID string `json:"id"`
}

type answerMsg struct {
	Op    string `json:"op"`
	ID    string `json:"id"`
	Allow bool   `json:"allow"`
}

// encodeLine returns v as one JSON line.
func encodeLine(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		// The message types hold only strings, integers and booleans.
		panic("confirm: encode: " + err.Error())
	}
	return append(b, '\n')
}

// readLine returns the next line of r without its line feed. r must have been created with
// bufio.NewReaderSize(conn, maxLine), so a longer line fills the buffer and is refused.
func readLine(r *bufio.Reader) ([]byte, error) {
	line, err := r.ReadSlice('\n')
	if errors.Is(err, bufio.ErrBufferFull) {
		return nil, errLineTooLong
	}
	if err != nil {
		return nil, err
	}
	return line[:len(line)-1], nil
}

// decodeLine decodes one line strictly: exactly one object, no unknown member, nothing after it.
func decodeLine(line []byte) (wire, error) {
	var w wire
	dec := json.NewDecoder(bytes.NewReader(line))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&w); err != nil {
		return wire{}, errMalformed
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return wire{}, errMalformed
	}
	if w.Op == nil || w.ID == nil || *w.ID == "" {
		return wire{}, errMalformed
	}
	return w, nil
}

// decodeAnswer accepts exactly {"op":"answer","id","allow"}.
func decodeAnswer(line []byte) (id string, allow bool, err error) {
	w, err := decodeLine(line)
	if err != nil {
		return "", false, err
	}
	if *w.Op != opAnswer || w.Allow == nil || w.Kind != nil || w.Profile != nil || w.SessionUser != nil || w.TimeoutMS != nil {
		return "", false, errMalformed
	}
	return *w.ID, *w.Allow, nil
}

// serviceMsg is a decoded message of the service: a confirm with its request and timeout, or a
// cancel.
type serviceMsg struct {
	op        string
	id        string
	req       Request
	timeoutMS int64
}

// decodeService accepts exactly a confirm with every member valid, or a cancel.
func decodeService(line []byte) (serviceMsg, error) {
	w, err := decodeLine(line)
	if err != nil {
		return serviceMsg{}, err
	}
	switch *w.Op {
	case opCancel:
		if w.Kind != nil || w.Profile != nil || w.SessionUser != nil || w.TimeoutMS != nil || w.Allow != nil {
			return serviceMsg{}, errMalformed
		}
		return serviceMsg{op: opCancel, id: *w.ID}, nil
	case opConfirm:
		if w.Kind == nil || (*w.Kind != KindOpen && *w.Kind != KindAttach) || w.Profile == nil || *w.Profile == "" ||
			w.SessionUser == nil || *w.SessionUser == "" || w.TimeoutMS == nil || *w.TimeoutMS <= 0 || w.Allow != nil {
			return serviceMsg{}, errMalformed
		}
		return serviceMsg{op: opConfirm, id: *w.ID, req: Request{Kind: *w.Kind, Profile: *w.Profile, SessionUser: *w.SessionUser},
			timeoutMS: *w.TimeoutMS}, nil
	}
	return serviceMsg{}, errMalformed
}

// newID returns a random UUID (version 4) for one request.
func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:]) // crypto/rand.Read never fails (Go 1.24 and later)
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}
