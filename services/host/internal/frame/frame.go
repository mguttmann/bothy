// SPDX-License-Identifier: Apache-2.0

package frame

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// MaxLen is the largest encoded frame: the largest plaintext of a Noise transport message,
// 65535 minus the 16-byte tag (docs/spec/24-terminal.md 5.2, 5.4). A protocol constant, not a
// configuration key: changing it breaks the protocol.
const MaxLen = 65519

// sizeBodyLen is the body of start and resize: columns and rows, 2 bytes each
// (docs/spec/24-terminal.md 5.4).
const sizeBodyLen = 4

// exitBodyLen is the body of exit: a signed 32-bit exit code (docs/spec/24-terminal.md 5.4).
const exitBodyLen = 4

// Type is the first byte of a frame (docs/spec/24-terminal.md 5.4).
type Type byte

// The frame types of docs/spec/24-terminal.md 5.4. The values are protocol constants.
const (
	TypeStart       Type = 0x01 // device: columns and rows; once, as the first transport message
	TypeData        Type = 0x02 // both: bytes of the PTY or input
	TypeResize      Type = 0x03 // device: columns and rows
	TypeClose       Type = 0x04 // device: end the session
	TypeExit        Type = 0x05 // host: exit code of the process
	TypeReplayBegin Type = 0x06 // host: start of the scrollback replay on attach
	TypeReplayEnd   Type = 0x07 // host: end of the scrollback replay
)

// Sender is the side that sent a frame; Decode checks the direction against it.
type Sender uint8

// The two ends of a terminal stream.
const (
	FromDevice Sender = iota + 1
	FromHost
)

// String returns "device" or "host", the values of the field from in the recordings.
func (s Sender) String() string {
	switch s {
	case FromDevice:
		return "device"
	case FromHost:
		return "host"
	default:
		return fmt.Sprintf("sender(%d)", uint8(s))
	}
}

// Errors of Encode and Decode. Each error returned wraps exactly one of them; every one of
// them ends the session with close code 4104 (docs/DECISIONS.md D-M2-38-10).
var (
	ErrEmpty          = errors.New("frame: empty frame")
	ErrUnknownType    = errors.New("frame: unknown frame type")
	ErrWrongDirection = errors.New("frame: frame type not allowed from this sender")
	ErrMalformed      = errors.New("frame: malformed frame body")
	ErrTooLong        = errors.New("frame: frame too long")
)

// Frame is one decoded frame. Only the fields of its type are set: Cols and Rows for start and
// resize, Data for data, ExitCode for exit.
type Frame struct {
	Type       Type
	Cols, Rows uint16
	Data       []byte
	ExitCode   int32
}

// Start returns a start frame with the terminal size; both values must be 1 to 65535.
func Start(cols, rows uint16) Frame { return Frame{Type: TypeStart, Cols: cols, Rows: rows} }

// Data returns a data frame; b must hold 1 to MaxLen-1 bytes and is not copied.
func Data(b []byte) Frame { return Frame{Type: TypeData, Data: b} }

// Resize returns a resize frame with the new terminal size; both values must be 1 to 65535.
func Resize(cols, rows uint16) Frame { return Frame{Type: TypeResize, Cols: cols, Rows: rows} }

// Close returns the close frame of the device.
func Close() Frame { return Frame{Type: TypeClose} }

// Exit returns the exit frame of the host with the exit code of the process.
func Exit(code int32) Frame { return Frame{Type: TypeExit, ExitCode: code} }

// ReplayBegin returns the frame that opens the scrollback replay.
func ReplayBegin() Frame { return Frame{Type: TypeReplayBegin} }

// ReplayEnd returns the frame that closes the scrollback replay.
func ReplayEnd() Frame { return Frame{Type: TypeReplayEnd} }

// IsInput reports whether the frame counts as input of the device for the idle timeout: true
// exactly for data and resize (docs/spec/24-terminal.md 6.3 "Leerlauf"). A data frame always
// carries bytes, so an empty frame never counts.
func (f Frame) IsInput() bool {
	return f.Type == TypeData || f.Type == TypeResize
}

// allowedFrom reports whether sender may send frames of type t (docs/spec/24-terminal.md 5.4).
func allowedFrom(t Type, from Sender) bool {
	switch t {
	case TypeStart, TypeResize, TypeClose:
		return from == FromDevice
	case TypeExit, TypeReplayBegin, TypeReplayEnd:
		return from == FromHost
	case TypeData:
		return from == FromDevice || from == FromHost
	default:
		return false
	}
}

func known(t Type) bool { return t >= TypeStart && t <= TypeReplayEnd }

// Encode returns the bytes of f. It checks the body rules of f.Type (fields of other types
// must be zero) and the size limit; errors wrap ErrUnknownType, ErrMalformed or ErrTooLong.
// Encode does not check the direction: the caller sends only its own frame types.
func Encode(f Frame) ([]byte, error) {
	if !known(f.Type) {
		return nil, fmt.Errorf("frame: encode type 0x%02x: %w", byte(f.Type), ErrUnknownType)
	}
	hasSize, hasData, hasExit := f.Cols != 0 || f.Rows != 0, len(f.Data) != 0, f.ExitCode != 0
	switch f.Type {
	case TypeStart, TypeResize:
		if f.Cols == 0 || f.Rows == 0 || hasData || hasExit {
			return nil, encodeMalformed(f.Type)
		}
		out := []byte{byte(f.Type), 0, 0, 0, 0}
		binary.BigEndian.PutUint16(out[1:], f.Cols)
		binary.BigEndian.PutUint16(out[3:], f.Rows)
		return out, nil
	case TypeData:
		return encodeData(f, hasSize || hasExit)
	case TypeExit:
		if hasSize || hasData {
			return nil, encodeMalformed(f.Type)
		}
		out := []byte{byte(f.Type), 0, 0, 0, 0}
		binary.BigEndian.PutUint32(out[1:], uint32(f.ExitCode)) //nolint:gosec // G115: two's complement is the wire format
		return out, nil
	case TypeClose, TypeReplayBegin, TypeReplayEnd:
		if hasSize || hasData || hasExit {
			return nil, encodeMalformed(f.Type)
		}
		return []byte{byte(f.Type)}, nil
	default:
		return nil, fmt.Errorf("frame: encode type 0x%02x: %w", byte(f.Type), ErrUnknownType)
	}
}

func encodeData(f Frame, otherFields bool) ([]byte, error) {
	if len(f.Data) == 0 || otherFields {
		return nil, encodeMalformed(f.Type)
	}
	if 1+len(f.Data) > MaxLen {
		return nil, fmt.Errorf("frame: encode type 0x%02x: %d bytes, limit %d: %w", byte(f.Type), 1+len(f.Data), MaxLen, ErrTooLong)
	}
	out := make([]byte, 1+len(f.Data))
	out[0] = byte(f.Type)
	copy(out[1:], f.Data)
	return out, nil
}

func encodeMalformed(t Type) error {
	return fmt.Errorf("frame: encode type 0x%02x: fields against the body rules: %w", byte(t), ErrMalformed)
}

// Decode parses one frame sent by from. The returned Data aliases b, so the caller must not
// change b while it uses the frame. Errors wrap ErrEmpty (no bytes), ErrTooLong (more than
// MaxLen bytes), ErrUnknownType (type byte outside 0x01 to 0x07), ErrWrongDirection (a known
// type from the wrong side) or ErrMalformed (a body against the rules); their text names the
// type byte and lengths, never body bytes.
func Decode(b []byte, from Sender) (Frame, error) {
	if len(b) == 0 {
		return Frame{}, fmt.Errorf("frame: decode: %w", ErrEmpty)
	}
	if len(b) > MaxLen {
		return Frame{}, fmt.Errorf("frame: decode: %d bytes, limit %d: %w", len(b), MaxLen, ErrTooLong)
	}
	t, body := Type(b[0]), b[1:]
	if !known(t) {
		return Frame{}, fmt.Errorf("frame: decode type 0x%02x: %w", byte(t), ErrUnknownType)
	}
	if !allowedFrom(t, from) {
		return Frame{}, fmt.Errorf("frame: decode type 0x%02x from %s: %w", byte(t), from, ErrWrongDirection)
	}
	return decodeBody(t, body)
}

// decodeBody applies the body rules of a known type.
func decodeBody(t Type, body []byte) (Frame, error) {
	switch t {
	case TypeStart, TypeResize:
		if len(body) != sizeBodyLen {
			return Frame{}, decodeMalformed(t, len(body))
		}
		cols, rows := binary.BigEndian.Uint16(body), binary.BigEndian.Uint16(body[2:])
		if cols == 0 || rows == 0 {
			return Frame{}, fmt.Errorf("frame: decode type 0x%02x: columns and rows must be 1 to 65535: %w", byte(t), ErrMalformed)
		}
		return Frame{Type: t, Cols: cols, Rows: rows}, nil
	case TypeData:
		if len(body) == 0 {
			return Frame{}, decodeMalformed(t, 0)
		}
		return Frame{Type: t, Data: body}, nil
	case TypeExit:
		if len(body) != exitBodyLen {
			return Frame{}, decodeMalformed(t, len(body))
		}
		return Frame{Type: t, ExitCode: int32(binary.BigEndian.Uint32(body))}, nil //nolint:gosec // G115: two's complement is the wire format
	case TypeClose, TypeReplayBegin, TypeReplayEnd:
		if len(body) != 0 {
			return Frame{}, decodeMalformed(t, len(body))
		}
		return Frame{Type: t}, nil
	default:
		return Frame{}, fmt.Errorf("frame: decode type 0x%02x: %w", byte(t), ErrUnknownType)
	}
}

func decodeMalformed(t Type, bodyLen int) error {
	return fmt.Errorf("frame: decode type 0x%02x: body of %d bytes against the rules: %w", byte(t), bodyLen, ErrMalformed)
}
