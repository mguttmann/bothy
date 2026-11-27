// SPDX-License-Identifier: Apache-2.0

package computer

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Frame kinds of the local channel to the helper (docs/spec/27-computer-use.md 9.4).
const (
	// KindJSON is a frame with one UTF-8 JSON object.
	KindJSON byte = 0x01
	// KindImage is a frame with the bytes of one image, sent directly after the done that
	// announces it.
	KindImage byte = 0x02
)

// lenSize is the size of the big-endian length prefix of a channel frame.
const lenSize = 4

// Ops of the local channel (docs/spec/27-computer-use.md 9.4).
const (
	OpHello    = "hello"
	OpState    = "state"
	OpStop     = "stop"
	OpShow     = "show"
	OpHide     = "hide"
	OpCapture  = "capture"
	OpDo       = "do"
	OpPosition = "position"
	OpRelease  = "release"
	OpDisable  = "disable"
	OpDone     = "done"
)

// Errors of the channel codec; each closes the channel.
var (
	// ErrFrameTooLong is a frame above host_service.computer_frame_max_bytes or an empty one.
	ErrFrameTooLong = errors.New("computer: channel frame length out of range")
	// ErrChannelMalformed is a frame of an unknown kind or a JSON object against the rules of its
	// op.
	ErrChannelMalformed = errors.New("computer: malformed channel frame")
)

// Permissions are the operating system permissions of the helper: granted, denied or unknown.
type Permissions struct {
	Capture string `json:"capture"`
	Input   string `json:"input"`
}

// Hello is the first frame of a helper connection.
type Hello struct {
	Op            string      `json:"op"`
	Enabled       bool        `json:"enabled"`
	Generation    int64       `json:"generation"`
	OSAccount     string      `json:"os_account"`
	Permissions   Permissions `json:"permissions"`
	HelperVersion string      `json:"helper_version"`
	Signature     string      `json:"signature"`
	Locked        bool        `json:"locked"`
}

// StateMsg carries the members of hello that changed, and secure_desktop (Windows).
type StateMsg struct {
	Op            string       `json:"op"`
	Enabled       *bool        `json:"enabled,omitempty"`
	Generation    *int64       `json:"generation,omitempty"`
	OSAccount     *string      `json:"os_account,omitempty"`
	Permissions   *Permissions `json:"permissions,omitempty"`
	HelperVersion *string      `json:"helper_version,omitempty"`
	Signature     *string      `json:"signature,omitempty"`
	Locked        *bool        `json:"locked,omitempty"`
	SecureDesktop *bool        `json:"secure_desktop,omitempty"`
}

// Stop is a stop at the machine: Esc or the stop button of the indicator.
type Stop struct {
	Op        string `json:"op"`
	SessionID string `json:"session_id"`
	Detail    string `json:"detail"`
}

// Show tells the helper to show the indicator, start the Esc tap and keep the display awake.
type Show struct {
	Op        string `json:"op"`
	SessionID string `json:"session_id"`
	AgentName string `json:"agent_name"`
}

// Hide is the reverse of Show.
type Hide struct {
	Op        string `json:"op"`
	SessionID string `json:"session_id"`
}

// Rect is a region in screen space.
type Rect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// Capture asks for an image of the screen, or of Region (zoom).
type Capture struct {
	Op      string `json:"op"`
	Seq     int64  `json:"seq"`
	MaxEdge int64  `json:"max_edge"`
	Format  string `json:"format"`
	Quality int64  `json:"quality"`
	Region  *Rect  `json:"region,omitempty"`
}

// Do asks for one input action; X and Y are in the platform's screen space.
type Do struct {
	Op        string   `json:"op"`
	Seq       int64    `json:"seq"`
	Action    string   `json:"action"`
	X         *float64 `json:"x,omitempty"`
	Y         *float64 `json:"y,omitempty"`
	Modifiers []string `json:"modifiers,omitempty"`
	Direction string   `json:"direction,omitempty"`
	Amount    int64    `json:"amount,omitempty"`
	Text      string   `json:"text,omitempty"`
	Keys      string   `json:"keys,omitempty"`
	Repeat    int64    `json:"repeat,omitempty"`
}

// Position asks for the pointer position.
type Position struct {
	Op  string `json:"op"`
	Seq int64  `json:"seq"`
}

// Release asks the helper to release every pressed key and button now.
type Release struct {
	Op string `json:"op"`
}

// Disable switches the enablement off.
type Disable struct {
	Op         string `json:"op"`
	Generation int64  `json:"generation"`
}

// AppID is the identity of the frontmost app as the helper reports it.
type AppID struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Signer string `json:"signer"`
}

// Screen is the geometry of the screen in screen space; Scale is pixels per unit.
type Screen struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Scale  float64 `json:"scale"`
}

// XY is a point in screen space.
type XY struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// ImageInfo announces the image frame that follows a done.
type ImageInfo struct {
	Format string `json:"format"`
	Width  int64  `json:"width"`
	Height int64  `json:"height"`
	Bytes  int64  `json:"bytes"`
}

// Done answers capture, do and position.
type Done struct {
	Op     string     `json:"op"`
	Seq    int64      `json:"seq"`
	Code   string     `json:"code"`
	App    AppID      `json:"app"`
	Screen Screen     `json:"screen"`
	Cursor *XY        `json:"cursor,omitempty"`
	Image  *ImageInfo `json:"image,omitempty"`
}

// ReadFrame reads one channel frame from r: the kind byte and the content. A length of zero or
// above maxBytes wraps ErrFrameTooLong, an unknown kind ErrChannelMalformed; the content is not
// read then, and the caller closes the channel.
func ReadFrame(r io.Reader, maxBytes int) (byte, []byte, error) {
	var head [lenSize]byte
	if _, err := io.ReadFull(r, head[:]); err != nil {
		return 0, nil, err
	}
	n := binary.BigEndian.Uint32(head[:])
	if n == 0 || uint64(n) > uint64(maxBytes) { //nolint:gosec // G115: maxBytes is positive
		return 0, nil, fmt.Errorf("%w: %d bytes", ErrFrameTooLong, n)
	}
	var kind [1]byte
	if _, err := io.ReadFull(r, kind[:]); err != nil {
		return 0, nil, err
	}
	if kind[0] != KindJSON && kind[0] != KindImage {
		return 0, nil, fmt.Errorf("%w: kind 0x%02x", ErrChannelMalformed, kind[0])
	}
	body := make([]byte, n-1)
	if _, err := io.ReadFull(r, body); err != nil {
		return 0, nil, err
	}
	return kind[0], body, nil
}

// EncodeFrame returns the wire form of one channel frame; a frame above maxBytes wraps
// ErrFrameTooLong.
func EncodeFrame(kind byte, body []byte, maxBytes int) ([]byte, error) {
	if len(body) >= maxBytes {
		return nil, fmt.Errorf("%w: %d bytes", ErrFrameTooLong, 1+len(body))
	}
	n := 1 + len(body)
	out := make([]byte, lenSize+n)
	binary.BigEndian.PutUint32(out, uint32(n)) //nolint:gosec // G115: n is at most maxBytes, an int of the configuration
	out[lenSize] = kind
	copy(out[lenSize+1:], body)
	return out, nil
}

// EncodeMsg returns the channel frame of one JSON message.
func EncodeMsg(v any, maxBytes int) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("computer: encode channel message: %w", err)
	}
	return EncodeFrame(KindJSON, b, maxBytes)
}

// DecodeMsg decodes the JSON object of a KindJSON frame by its op into its type (Hello, StateMsg,
// Stop, Show, Hide, Capture, Do, Position, Release, Disable or Done). An unknown op, an unknown
// member, a null or trailing data wraps ErrChannelMalformed.
func DecodeMsg(body []byte) (string, any, error) {
	var env struct {
		Op string `json:"op"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return "", nil, fmt.Errorf("%w: no object with op", ErrChannelMalformed)
	}
	var v any
	switch env.Op {
	case OpHello:
		v = &Hello{}
	case OpState:
		v = &StateMsg{}
	case OpStop:
		v = &Stop{}
	case OpShow:
		v = &Show{}
	case OpHide:
		v = &Hide{}
	case OpCapture:
		v = &Capture{}
	case OpDo:
		v = &Do{}
	case OpPosition:
		v = &Position{}
	case OpRelease:
		v = &Release{}
	case OpDisable:
		v = &Disable{}
	case OpDone:
		v = &Done{}
	default:
		return "", nil, fmt.Errorf("%w: unknown op", ErrChannelMalformed)
	}
	if bytes.Contains(body, []byte("null")) && hasNull(body) {
		return "", nil, fmt.Errorf("%w: null member", ErrChannelMalformed)
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return "", nil, fmt.Errorf("%w: %s body", ErrChannelMalformed, env.Op)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return "", nil, fmt.Errorf("%w: trailing data", ErrChannelMalformed)
	}
	return env.Op, v, nil
}

// hasNull reports a JSON null anywhere in the object b.
func hasNull(b []byte) bool {
	dec := json.NewDecoder(bytes.NewReader(b))
	for {
		tok, err := dec.Token()
		if err != nil {
			return false
		}
		if tok == nil {
			return true
		}
	}
}
