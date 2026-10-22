// SPDX-License-Identifier: Apache-2.0

package ws

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/gateway/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// SchemaChecker checks a JSON value against a named schema of the contract
// (httpserver.SchemaValidator over api/rest.Spec).
type SchemaChecker interface {
	Validate(name string, body []byte) error
}

// SnapshotEncoder converts the snapshot of ready into its schema (the converter of GET
// /api/v1/sync/snapshot, so both answers are the same object).
type SnapshotEncoder func(sync.SnapshotView) (rest.Snapshot, error)

// frameError is a frame the codec cannot read; the reason is machine data for the close.
type frameError struct{ reason string }

func (e *frameError) Error() string { return "gateway: " + e.reason }

// The reasons of 4002 the reader names.
const (
	reasonFrameTooLarge = "frame_too_large"
	reasonBinaryFrame   = "binary_frame"
	reasonDecodeError   = "decode_error"
)

// envelope is the outer object {op, d} of a client frame.
type envelope struct {
	Op *string         `json:"op"`
	D  json.RawMessage `json:"d"`
}

// decodeEnvelope reads the outer object of a client frame: SEC-IN-2 first (valid UTF-8, one JSON
// value, no duplicate member at any level, at most maxDepth levels; httpserver.CheckJSON), then
// exactly the members op (a non-empty string) and d. d may be absent; the op's schema decides.
func decodeEnvelope(b []byte, maxDepth int) (string, json.RawMessage, error) {
	if err := httpserver.CheckJSON(b, maxDepth); err != nil {
		return "", nil, &frameError{reasonDecodeError}
	}
	var env envelope
	if err := strictUnmarshal(b, &env); err != nil || env.Op == nil || *env.Op == "" {
		return "", nil, &frameError{reasonDecodeError}
	}
	return *env.Op, env.D, nil
}

// strictUnmarshal decodes b into v and refuses unknown members (the decoder of M1-18 for
// request bodies).
func strictUnmarshal(b []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if dec.More() {
		return errors.New("gateway: trailing data")
	}
	return nil
}

// Decoder implements app.FrameDecoder over the contract: the body d of an op is checked against
// the schema of its gateway op (routes.GatewayOps), then decoded strictly into the generated
// Gateway* type and converted into the frame of the app package.
type Decoder struct {
	schemas map[string]string
	check   SchemaChecker
}

// NewDecoder returns the decoder for the ops of schemas (op to schema name) checked by check.
func NewDecoder(schemas map[string]string, check SchemaChecker) *Decoder {
	m := make(map[string]string, len(schemas))
	for op, s := range schemas {
		m[op] = s
	}
	return &Decoder{schemas: m, check: check}
}

// Decode implements app.FrameDecoder. Errors: an op without a schema, a missing body, a body that
// fails its schema or the strict decoder.
func (d *Decoder) Decode(op string, raw json.RawMessage) (any, error) {
	schema, ok := d.schemas[op]
	if !ok {
		return nil, fmt.Errorf("gateway: no schema for the op %q", op)
	}
	if len(raw) == 0 {
		return nil, &frameError{reasonDecodeError}
	}
	if err := d.check.Validate(schema, raw); err != nil {
		return nil, err
	}
	switch op {
	case "identify":
		var f rest.GatewayIdentify
		if err := strictUnmarshal(raw, &f); err != nil {
			return nil, err
		}
		return identifyFrame(f), nil
	case "resume":
		var f rest.GatewayResume
		if err := strictUnmarshal(raw, &f); err != nil {
			return nil, err
		}
		return app.Resume{SessionID: ids.ID(f.GatewaySessionId), AccessToken: deref(f.AccessToken), APIToken: deref(f.ApiToken),
			DPoP: deref(f.Dpop), Cursor: f.Cursor, SyncEpoch: optionalID(f.SyncEpoch)}, nil
	case "heartbeat":
		return decodeHeartbeat(raw)
	case "ack":
		return decodeAck(raw)
	case "reauth":
		var f rest.GatewayReauth
		if err := strictUnmarshal(raw, &f); err != nil {
			return nil, err
		}
		return app.Reauth{AccessToken: f.AccessToken, DPoP: f.Dpop}, nil
	case "focus":
		return decodeFocus(raw)
	case "command.autocomplete_result":
		return decodeAutocompleteResult(raw)
	case "typing.start", "typing.stop":
		var f rest.GatewayTyping
		if err := strictUnmarshal(raw, &f); err != nil {
			return nil, err
		}
		return app.Typing{ThreadID: ids.ID(f.ThreadId)}, nil
	}
	return nil, fmt.Errorf("gateway: no frame type for the op %q", op)
}

// decodeHeartbeat decodes the body of heartbeat strictly into its frame. The member agent goes on
// raw: agents.Presence is the only mapping of the heartbeat body (brief M2-20 note 1); the schema
// check of Decode already covered it.
func decodeHeartbeat(raw json.RawMessage) (app.Heartbeat, error) {
	var f rest.GatewayHeartbeat
	if err := strictUnmarshal(raw, &f); err != nil {
		return app.Heartbeat{}, err
	}
	var member struct {
		Agent json.RawMessage `json:"agent"`
	}
	if err := json.Unmarshal(raw, &member); err != nil {
		return app.Heartbeat{}, err
	}
	return app.Heartbeat{LastSeq: f.LastSeq, Agent: member.Agent}, nil
}

// decodeAck decodes the body of ack strictly into its frame.
func decodeAck(raw json.RawMessage) (app.Ack, error) {
	var f rest.GatewayAck
	if err := strictUnmarshal(raw, &f); err != nil {
		return app.Ack{}, err
	}
	return app.Ack{Seq: f.Seq}, nil
}

// decodeAutocompleteResult decodes the body of command.autocomplete_result strictly into its frame
// (M2-17).
func decodeAutocompleteResult(raw json.RawMessage) (app.AutocompleteResult, error) {
	var f rest.GatewayAutocompleteResult
	if err := strictUnmarshal(raw, &f); err != nil {
		return app.AutocompleteResult{}, err
	}
	out := app.AutocompleteResult{RequestID: ids.ID(f.RequestId), Ref: deref(f.Ref), Choices: make([]runs.Choice, 0, len(f.Choices))}
	for _, c := range f.Choices {
		out.Choices = append(out.Choices, runs.Choice{Name: c.Name, Value: c.Value})
	}
	return out, nil
}

// decodeFocus decodes the body of focus strictly into its frame.
func decodeFocus(raw json.RawMessage) (app.Focus, error) {
	var f rest.GatewayFocus
	if err := strictUnmarshal(raw, &f); err != nil {
		return app.Focus{}, err
	}
	out := app.Focus{ThreadIDs: make([]ids.ID, 0, len(f.ThreadIds)), Idle: f.Idle != nil && *f.Idle}
	for _, id := range f.ThreadIds {
		out.ThreadIDs = append(out.ThreadIDs, ids.ID(id))
	}
	return out, nil
}

// identifyFrame converts the generated identify body.
func identifyFrame(f rest.GatewayIdentify) app.Identify {
	out := app.Identify{ProtocolVersion: f.ProtocolVersion, AccessToken: deref(f.AccessToken), APIToken: deref(f.ApiToken),
		DPoP: deref(f.Dpop), Consumer: f.Consumer, Cursor: f.Cursor, SyncEpoch: optionalID(f.SyncEpoch)}
	if f.DeviceId != nil {
		out.DeviceID = ids.ID(*f.DeviceId)
	}
	if f.Client != nil {
		out.Client = &app.ClientInfo{Platform: string(f.Client.Platform), Version: f.Client.Version}
	}
	return out
}

// deref returns *p, or "" for nil.
func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// optionalID converts an optional UUID.
func optionalID(u *rest.Uuid) *ids.ID {
	if u == nil {
		return nil
	}
	id := ids.ID(*u)
	return &id
}

// frame is the outer object of a server frame.
type frame struct {
	Op string `json:"op"`
	D  any    `json:"d"`
}

// timestamp is the form of every time of a frame: UTC with millisecond precision (schema
// Timestamp).
func timestamp(t time.Time) time.Time { return t.UTC().Truncate(time.Millisecond) }

// encodeFrame returns the JSON text of the server frame f of the app package with the generated
// type of its schema. Errors: an unknown frame type or a snapshot that cannot be converted.
func encodeFrame(f any, snapshot SnapshotEncoder) ([]byte, error) {
	var out frame
	switch v := f.(type) {
	case app.Hello:
		out = frame{"hello", rest.GatewayHello{
			DpopNonce: v.DPoPNonce, HeartbeatIntervalMs: v.HeartbeatInterval.Milliseconds(), ProtocolVersions: v.ProtocolVersions,
			ResumeWindowMs: v.ResumeWindow.Milliseconds(), ServerTime: timestamp(v.ServerTime), SyncEpoch: rest.Uuid(v.SyncEpoch),
		}}
	case app.Ready:
		snap, err := snapshot(v.View)
		if err != nil {
			return nil, err
		}
		r := rest.GatewayReady{GatewaySessionId: rest.Uuid(v.SessionID), HeadSeq: v.View.HeadSeq, PermissionVersion: v.View.PermissionVersion,
			Principal: snap.Principal, Snapshot: snap, SyncEpoch: rest.Uuid(v.View.SyncEpoch)}
		if !v.DeviceID.IsZero() {
			d := rest.Uuid(v.DeviceID)
			r.DeviceId = &d
		}
		out = frame{"ready", r}
	case app.Resumed:
		out = frame{"resumed", rest.GatewayResumed{GatewaySessionId: rest.Uuid(v.SessionID), HeadSeq: v.HeadSeq, SyncEpoch: rest.Uuid(v.SyncEpoch)}}
	case app.HeartbeatAck:
		out = frame{"heartbeat_ack", rest.GatewayHeartbeatAck{ServerTime: timestamp(v.ServerTime)}}
	case app.ReauthRequired:
		out = frame{"reauth_required", rest.GatewayReauthRequired{DpopNonce: v.DPoPNonce, ExpiresAt: timestamp(v.ExpiresAt)}}
	case app.ResyncRequired:
		out = frame{"resync_required", rest.GatewayResyncRequired{HeadSeq: v.HeadSeq, Reason: rest.GatewayResyncRequiredReason(v.Reason),
			SyncEpoch: rest.Uuid(v.SyncEpoch)}}
	case app.ErrorFrame:
		e := rest.GatewayError{Code: v.Code, Op: v.Op}
		if v.Detail != "" {
			e.Detail = &v.Detail
		}
		if v.Ref != "" {
			e.Ref = &v.Ref
		}
		out = frame{"error", e}
	default:
		return nil, fmt.Errorf("gateway: no encoding for the frame %T", f)
	}
	return json.Marshal(out)
}
