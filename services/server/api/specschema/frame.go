// SPDX-License-Identifier: Apache-2.0

package specschema

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math/big"
	"slices"
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// ptrType is the JSON pointer of the event type of a dispatch frame; ptrOp that of the op.
const (
	ptrType = "/d/type"
	ptrOp   = "/op"
)

// maxSafeInteger is 2^53 - 1, the largest integer every client decodes exactly (06 "Basic rules").
const maxSafeInteger = 1<<53 - 1

// FrameError is one violation of a frame: the JSON pointer of the offending value and the rule.
type FrameError struct {
	// Op is the op of the frame, "" when the frame has none.
	Op string
	// Pointer is the JSON pointer of the value inside the frame, "" for the whole frame.
	Pointer string
	// Reason says which rule failed.
	Reason string
}

// Error returns op, pointer and reason.
func (e *FrameError) Error() string {
	return fmt.Sprintf("frame op %q at %q: %s", e.Op, e.Pointer, e.Reason)
}

// ValidateServerFrame checks one frame the server sent: size up to sync.ws_max_frame_bytes, one
// JSON object {op, d}, no null at any depth, integers at most 2^53 - 1, message lookup by op (and
// by d.type for dispatch; the op must be a server-to-client one), the strict schema, the
// durable/ephemeral envelope rule and the presence of resource.version against the catalogue.
// Errors: a *FrameError, or a schema error wrapped with the op.
func (c *Contract) ValidateServerFrame(raw []byte) error {
	op, frame, err := c.decode(raw)
	if err != nil {
		return err
	}
	d, _ := frame["d"].(map[string]any)
	if op == opDispatch {
		if d == nil {
			return &FrameError{Op: op, Pointer: "/d", Reason: "dispatch without an object d"}
		}
		t, _ := d["type"].(string)
		ev, ok := c.events[t]
		if !ok {
			return &FrameError{Op: op, Pointer: ptrType, Reason: fmt.Sprintf("event type %q has no message event.%s in asyncapi.yaml", t, t)}
		}
		if err := c.envelopeRule(t, ev.Kind, d); err != nil {
			return err
		}
		return c.validate(op, "event."+t, frame)
	}
	key, ok := c.serverOps[op]
	if !ok {
		return &FrameError{Op: op, Pointer: ptrOp, Reason: fmt.Sprintf("op %q is no server-to-client message of the channel gateway", op)}
	}
	return c.validate(op, key, frame)
}

// ValidateClientFrame checks one frame a test client sent with the same rules: size, no null,
// integer bound, a client-to-server op of the channel gateway and its strict schema.
func (c *Contract) ValidateClientFrame(raw []byte) error {
	op, frame, err := c.decode(raw)
	if err != nil {
		return err
	}
	key, ok := c.clientOps[op]
	if !ok {
		return &FrameError{Op: op, Pointer: ptrOp, Reason: fmt.Sprintf("op %q is no client-to-server message of the channel gateway", op)}
	}
	return c.validate(op, key, frame)
}

// ValidateChannelFrame checks one text frame of a terminal channel (M2-43): channel is hostControl,
// hostData or terminalStream (M2-44), fromServer the direction (true: sent by the server). The rules of every frame
// apply (size up to sync.ws_max_frame_bytes, one JSON object {op, d}, no null, integers at most
// 2^53 - 1), then the op must be a message of the channel in that direction and the frame must
// pass its strict schema. Errors: a *FrameError, or a schema error wrapped with the op.
func (c *Contract) ValidateChannelFrame(channel string, fromServer bool, raw []byte) error {
	dirs, ok := c.channels[channel]
	if !ok {
		return &FrameError{Reason: fmt.Sprintf("channel %q is no terminal channel of asyncapi.yaml", channel)}
	}
	op, frame, err := c.decode(raw)
	if err != nil {
		return err
	}
	key, ok := dirs[fromServer][op]
	if !ok {
		dir := "host-to-server"
		if fromServer {
			dir = "server-to-host"
		}
		return &FrameError{Op: op, Pointer: ptrOp, Reason: fmt.Sprintf("op %q is no %s message of the channel %s", op, dir, channel)}
	}
	return c.validate(op, key, frame)
}

// decode applies the rules every frame shares and returns its op and the decoded frame.
func (c *Contract) decode(raw []byte) (string, map[string]any, error) {
	if len(raw) > c.maxFrame {
		return "", nil, &FrameError{Reason: fmt.Sprintf("%d bytes, above sync.ws_max_frame_bytes %d", len(raw), c.maxFrame)}
	}
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return "", nil, &FrameError{Reason: "no single JSON value: " + err.Error()}
	}
	frame, ok := v.(map[string]any)
	if !ok {
		return "", nil, &FrameError{Reason: "not a JSON object"}
	}
	op, _ := frame["op"].(string)
	if err := plainValues(v, ""); err != nil {
		err.Op = op
		return "", nil, err
	}
	return op, frame, nil
}

// plainValues fails on null at any depth and on an integer above 2^53 - 1 in magnitude (06
// "Basic rules", ADR 0053).
func plainValues(v any, ptr string) *FrameError {
	switch x := v.(type) {
	case nil:
		return &FrameError{Pointer: ptr, Reason: "null (ADR 0053: absent instead of null)"}
	case map[string]any:
		for _, k := range slices.Sorted(maps.Keys(x)) {
			if err := plainValues(x[k], ptr+"/"+jsonPointerEscape(k)); err != nil {
				return err
			}
		}
	case []any:
		for i, e := range x {
			if err := plainValues(e, ptr+"/"+strconv.Itoa(i)); err != nil {
				return err
			}
		}
	case json.Number:
		return safeInteger(x, ptr)
	}
	return nil
}

// safeInteger fails on an integer literal above 2^53 - 1 in magnitude; fractions and exponents
// are no integer literals.
func safeInteger(n json.Number, ptr string) *FrameError {
	s := n.String()
	if strings.ContainsAny(s, ".eE") {
		return nil
	}
	i, ok := new(big.Int).SetString(s, 10)
	if !ok || i.CmpAbs(big.NewInt(maxSafeInteger)) > 0 {
		return &FrameError{Pointer: ptr, Reason: fmt.Sprintf("integer %s above 2^53 - 1", s)}
	}
	return nil
}

// envelopeRule checks seq and permission_version against x-event-kind (04 "Envelope") and
// resource.version against the catalogue (04 "Delivery guarantee" item 3).
func (c *Contract) envelopeRule(t, kind string, d map[string]any) error {
	_, hasSeq := d["seq"]
	_, hasPV := d["permission_version"]
	switch kind {
	case "durable":
		if !hasSeq {
			return &FrameError{Op: opDispatch, Pointer: "/d/seq", Reason: "durable event " + t + " without seq"}
		}
		if !hasPV {
			return &FrameError{Op: opDispatch, Pointer: "/d/permission_version", Reason: "durable event " + t + " without permission_version"}
		}
	case "ephemeral":
		if hasSeq {
			return &FrameError{Op: opDispatch, Pointer: "/d/seq", Reason: "ephemeral event " + t + " with seq"}
		}
		if hasPV {
			return &FrameError{Op: opDispatch, Pointer: "/d/permission_version", Reason: "ephemeral event " + t + " with permission_version"}
		}
	default:
		return &FrameError{Op: opDispatch, Pointer: ptrType, Reason: fmt.Sprintf("event %s has x-event-kind %q", t, kind)}
	}
	spec, ok := c.catalogue.Events[t]
	if !ok {
		return &FrameError{Op: opDispatch, Pointer: ptrType, Reason: "event type " + t + " is not in actions.yaml"}
	}
	res, _ := d["resource"].(map[string]any)
	_, hasVersion := res["version"]
	if hasVersion != spec.ResourceVersion {
		return &FrameError{Op: opDispatch, Pointer: "/d/resource/version",
			Reason: fmt.Sprintf("resource.version present %t, the catalogue says %t for %s", hasVersion, spec.ResourceVersion, t)}
	}
	return nil
}

// validate checks frame against the compiled payload of the message key.
func (c *Contract) validate(op, key string, frame map[string]any) error {
	s := c.schemas[key]
	if s == nil {
		return &FrameError{Op: op, Reason: "no compiled schema for message " + key}
	}
	err := s.Validate(frame)
	if err == nil {
		return nil
	}
	var ve *jsonschema.ValidationError
	if errors.As(err, &ve) {
		leaf := deepest(ve)
		return &FrameError{Op: op, Pointer: pointer(leaf.InstanceLocation), Reason: "schema " + key + ": " + strings.TrimSpace(ve.Error())}
	}
	return fmt.Errorf("frame op %q: %w", op, err)
}

// deepest returns the cause with the longest instance location, the most precise place.
func deepest(e *jsonschema.ValidationError) *jsonschema.ValidationError {
	best := e
	for _, c := range e.Causes {
		if d := deepest(c); len(d.InstanceLocation) > len(best.InstanceLocation) {
			best = d
		}
	}
	return best
}

// pointer joins instance location tokens to a JSON pointer.
func pointer(tokens []string) string {
	var b strings.Builder
	for _, t := range tokens {
		b.WriteString("/" + jsonPointerEscape(t))
	}
	return b.String()
}
