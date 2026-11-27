// SPDX-License-Identifier: Apache-2.0

package frame

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
)

// The frame types of a computer session (docs/spec/27-computer-use.md 6.3). The values are
// protocol constants. A computer session also uses TypeClose (bothy-computer-mcp ends the session).
const (
	TypeCUAction     Type = 0x20 // device: one action {"seq", "action", "args", "quick"}
	TypeCUResult     Type = 0x21 // host: the result of an action
	TypeCUImageBegin Type = 0x22 // host: the start of an image
	TypeCUImageData  Type = 0x23 // host: 1 to MaxImageChunk bytes of the image
	TypeCUImageEnd   Type = 0x24 // host: the end of an image; no body
	TypeCUState      Type = 0x25 // host: the state of the session {"state", "reason"}
)

// MaxImageChunk is the most image bytes one cu_image_data frame carries: MaxLen minus the type
// byte (docs/spec/27-computer-use.md 6.3). A protocol constant.
const MaxImageChunk = MaxLen - 1

// Image formats of cu_image_begin.
const (
	FormatJPEG = "jpeg"
	FormatPNG  = "png"
)

// States of cu_state (docs/spec/27-computer-use.md 8.1).
const (
	StateActive  = "active"
	StatePaused  = "paused"
	StateStopped = "stopped"
)

// Action is the body of cu_action. Args is the JSON object of the action's arguments (27 6.4); its
// members are checked by the host service per action, not here. Quick, optional, marks a batch
// item before the last one: the host skips the automatic image and waits a short pause instead
// of computer.settle (27 6.4); only a host that names the feature quick in message 2 gets it.
type Action struct {
	Seq    int64           `json:"seq"`
	Action string          `json:"action"`
	Args   json.RawMessage `json:"args"`
	Quick  bool            `json:"quick,omitempty"`
}

// App is the identity of the frontmost app: a bundle ID or executable name with a display name.
type App struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Point is a position in pixels of a screenshot.
type Point struct {
	X int64 `json:"x"`
	Y int64 `json:"y"`
}

// Result is the body of cu_result. Image tells whether image frames follow; Width, Height and
// ScaleID name the image (zero without one); Cursor is set only for cursor_position.
type Result struct {
	Seq     int64  `json:"seq"`
	Code    string `json:"code"`
	Image   bool   `json:"image"`
	Width   int64  `json:"width"`
	Height  int64  `json:"height"`
	ScaleID int64  `json:"scale_id"`
	App     App    `json:"app"`
	Cursor  *Point `json:"cursor,omitempty"`
}

// ImageBegin is the body of cu_image_begin; Bytes is the sum of the data frames that follow.
type ImageBegin struct {
	Seq     int64  `json:"seq"`
	Format  string `json:"format"`
	Width   int64  `json:"width"`
	Height  int64  `json:"height"`
	ScaleID int64  `json:"scale_id"`
	Bytes   int64  `json:"bytes"`
}

// State is the body of cu_state: active with an empty reason, paused with locked, stopped with the
// close reason of docs/spec/27-computer-use.md 8.3.
type State struct {
	State  string `json:"state"`
	Reason string `json:"reason"`
}

// Members that several bodies share.
const (
	memberSeq     = "seq"
	memberWidth   = "width"
	memberHeight  = "height"
	memberScaleID = "scale_id"
)

// members are the required and optional members of each JSON body.
var members = map[Type]struct{ required, optional []string }{
	TypeCUAction:     {required: []string{memberSeq, "action", "args"}, optional: []string{"quick"}},
	TypeCUResult:     {required: []string{memberSeq, "code", "image", memberWidth, memberHeight, memberScaleID, "app"}, optional: []string{"cursor"}},
	TypeCUImageBegin: {required: []string{memberSeq, "format", memberWidth, memberHeight, memberScaleID, "bytes"}},
	TypeCUState:      {required: []string{"state", "reason"}},
}

func knownComputer(t Type) bool { return t >= TypeCUAction && t <= TypeCUState }

// computerFrom reports whether sender may send the computer type t: cu_action comes from the
// device (bothy-computer-mcp), every other computer type from the host.
func computerFrom(t Type, from Sender) bool {
	switch {
	case t == TypeCUAction:
		return from == FromDevice
	case knownComputer(t):
		return from == FromHost
	default:
		return false
	}
}

// NewAction returns a cu_action frame; errors wrap ErrMalformed.
func NewAction(a Action) (Frame, error) { return jsonFrame(TypeCUAction, a) }

// NewResult returns a cu_result frame; errors wrap ErrMalformed.
func NewResult(r Result) (Frame, error) { return jsonFrame(TypeCUResult, r) }

// NewImageBegin returns a cu_image_begin frame; errors wrap ErrMalformed.
func NewImageBegin(b ImageBegin) (Frame, error) { return jsonFrame(TypeCUImageBegin, b) }

// NewState returns a cu_state frame; errors wrap ErrMalformed.
func NewState(s State) (Frame, error) { return jsonFrame(TypeCUState, s) }

// ImageData returns a cu_image_data frame; b must hold 1 to MaxImageChunk bytes and is not copied.
func ImageData(b []byte) Frame { return Frame{Type: TypeCUImageData, Data: b} }

// ImageEnd returns the cu_image_end frame.
func ImageEnd() Frame { return Frame{Type: TypeCUImageEnd} }

// jsonFrame marshals v as the body of t and checks it with the decode rules.
func jsonFrame(t Type, v any) (Frame, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return Frame{}, fmt.Errorf("frame: encode type 0x%02x: %w", byte(t), ErrMalformed)
	}
	if _, err := decodeComputer(t, b); err != nil {
		return Frame{}, err
	}
	return Frame{Type: t, Data: b}, nil
}

// encodeComputer returns the bytes of a computer frame after the body rules of its type.
func encodeComputer(f Frame) ([]byte, error) {
	if f.Cols != 0 || f.Rows != 0 || f.ExitCode != 0 {
		return nil, encodeMalformed(f.Type)
	}
	if len(f.Data) >= MaxLen {
		return nil, fmt.Errorf("frame: encode type 0x%02x: %d bytes, limit %d: %w", byte(f.Type), 1+len(f.Data), MaxLen, ErrTooLong)
	}
	if _, err := decodeComputer(f.Type, f.Data); err != nil {
		return nil, err
	}
	out := make([]byte, 1+len(f.Data))
	out[0] = byte(f.Type)
	copy(out[1:], f.Data)
	return out, nil
}

// decodeComputer applies the body rules of a computer type (27 6.2 and 6.3): image data carries
// at least one byte, image end none, every other type one JSON object with exactly its members.
func decodeComputer(t Type, body []byte) (Frame, error) {
	//exhaustive:ignore // Every other computer type carries a JSON body (checkBody).
	switch t {
	case TypeCUImageData:
		if len(body) == 0 {
			return Frame{}, decodeMalformed(t, 0)
		}
		return Frame{Type: t, Data: body}, nil
	case TypeCUImageEnd:
		if len(body) != 0 {
			return Frame{}, decodeMalformed(t, len(body))
		}
		return Frame{Type: t}, nil
	}
	if err := checkBody(t, body); err != nil {
		return Frame{}, err
	}
	return Frame{Type: t, Data: body}, nil
}

// checkBody decodes body strictly into the body type of t and applies its value rules.
func checkBody(t Type, body []byte) error {
	bad := func() error { return decodeMalformed(t, len(body)) }
	m, err := object(body)
	if err != nil {
		return bad()
	}
	rule := members[t]
	for _, k := range rule.required {
		if _, ok := m[k]; !ok {
			return bad()
		}
	}
	for k := range m {
		if !slices.Contains(rule.required, k) && !slices.Contains(rule.optional, k) {
			return bad()
		}
	}
	if !valid(t, body) {
		return bad()
	}
	return nil
}

// valid decodes body into the body type of t and checks the values.
func valid(t Type, body []byte) bool {
	//exhaustive:ignore // Only the types with a JSON body reach this check; any other is invalid.
	switch t {
	case TypeCUAction:
		var a Action
		return strict(body, &a) == nil && a.Seq >= 1 && a.Action != "" && isObject(a.Args)
	case TypeCUResult:
		var r Result
		return strict(body, &r) == nil && r.Seq >= 0 && r.Code != "" && r.Width >= 0 && r.Height >= 0 && r.ScaleID >= 0
	case TypeCUImageBegin:
		var b ImageBegin
		return strict(body, &b) == nil && (b.Format == FormatJPEG || b.Format == FormatPNG) && b.Width > 0 && b.Height > 0 &&
			b.ScaleID >= 0 && b.Bytes > 0
	case TypeCUState:
		var s State
		return strict(body, &s) == nil && (s.State == StateActive || s.State == StatePaused || s.State == StateStopped)
	default:
		return false
	}
}

// object decodes b as exactly one JSON object without null members and without trailing data.
func object(b []byte) (map[string]json.RawMessage, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	var m map[string]json.RawMessage
	if err := dec.Decode(&m); err != nil || m == nil {
		return nil, ErrMalformed
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, ErrMalformed
	}
	for _, v := range m {
		if bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
			return nil, ErrMalformed
		}
	}
	return m, nil
}

// strict decodes b into v and refuses unknown members, also in nested objects; a number with a
// fraction fails to decode into an integer field.
func strict(b []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// isObject reports whether raw is a JSON object.
func isObject(raw json.RawMessage) bool {
	t := bytes.TrimSpace(raw)
	return len(t) >= 2 && t[0] == '{'
}

// ActionBody returns the body of a cu_action frame; errors wrap ErrMalformed.
func (f Frame) ActionBody() (Action, error) {
	var a Action
	return a, f.body(TypeCUAction, &a)
}

// ResultBody returns the body of a cu_result frame; errors wrap ErrMalformed.
func (f Frame) ResultBody() (Result, error) {
	var r Result
	return r, f.body(TypeCUResult, &r)
}

// ImageBeginBody returns the body of a cu_image_begin frame; errors wrap ErrMalformed.
func (f Frame) ImageBeginBody() (ImageBegin, error) {
	var b ImageBegin
	return b, f.body(TypeCUImageBegin, &b)
}

// StateBody returns the body of a cu_state frame; errors wrap ErrMalformed.
func (f Frame) StateBody() (State, error) {
	var s State
	return s, f.body(TypeCUState, &s)
}

func (f Frame) body(t Type, v any) error {
	if f.Type != t {
		return fmt.Errorf("frame: body of type 0x%02x read as 0x%02x: %w", byte(f.Type), byte(t), ErrMalformed)
	}
	if err := checkBody(t, f.Data); err != nil {
		return err
	}
	if err := strict(f.Data, v); err != nil {
		return decodeMalformed(t, len(f.Data))
	}
	return nil
}
