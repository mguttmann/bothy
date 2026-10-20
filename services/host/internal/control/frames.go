// SPDX-License-Identifier: Apache-2.0

package control

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// The ops of the channel hostControl.
const (
	opHello          = "hello"
	opIdentify       = "identify"
	opReady          = "ready"
	opHeartbeat      = "heartbeat"
	opHeartbeatAck   = "heartbeat_ack"
	opReport         = "report"
	opSessionOpen    = "session.open"
	opSessionClose   = "session.close"
	opSessionStarted = "session.started"
	opSessionEnded   = "session.ended"
	opError          = "error"
)

// JWKS is a key set of purpose terminal (schema Jwks): JSON objects as the server sends them.
type JWKS struct {
	Keys []map[string]any `json:"keys"`
}

// Hello is the first frame of the server (schema HostChannelHello).
type Hello struct {
	ProtocolVersions    []int  `json:"protocol_versions"`
	Challenge           string `json:"challenge"`
	ServerTime          string `json:"server_time"`
	HeartbeatIntervalMs int64  `json:"heartbeat_interval_ms"`
	HeartbeatMissLimit  int    `json:"heartbeat_miss_limit"`
	JWKS                JWKS   `json:"jwks"`
	// ClockLeewayMs, IdleTimeoutMs and DetachGraceMs are auth.clock_leeway, terminal.idle_timeout
	// and terminal.detach_grace; the host service applies those of the data connection that
	// carries a handshake to that session (M2-45 Design note 2a).
	ClockLeewayMs int64 `json:"clock_leeway_ms"`
	IdleTimeoutMs int64 `json:"idle_timeout_ms"`
	DetachGraceMs int64 `json:"detach_grace_ms"`
}

// ReadyFrame is ready (schema HostReady).
type ReadyFrame struct {
	HostID         string `json:"host_id"`
	State          string `json:"state"`
	LLMCredentials string `json:"llm_credentials"`
}

// HeartbeatAck is heartbeat_ack (schema HostHeartbeatAck).
type HeartbeatAck struct {
	ServerTime string `json:"server_time,omitempty"`
	JWKS       *JWKS  `json:"jwks,omitempty"`
}

// SessionOpen is session.open (schema HostSessionOpen).
type SessionOpen struct {
	SessionID string `json:"session_id"`
	DataPath  string `json:"data_path"`
	Profile   string `json:"profile"`
	ExpiresAt string `json:"expires_at"`
}

// SessionClose is session.close (schema HostSessionClose).
type SessionClose struct {
	SessionID   string `json:"session_id"`
	CloseReason string `json:"close_reason"`
}

// ErrorFrame is error (schema GatewayError).
type ErrorFrame struct {
	Op     string `json:"op"`
	Code   string `json:"code"`
	Detail string `json:"detail,omitempty"`
	Ref    string `json:"ref,omitempty"`
}

// Identify is identify (schema HostIdentify) on the control connection: live_sessions always
// present, no session_id.
type Identify struct {
	ProtocolVersion int      `json:"protocol_version"`
	HostID          string   `json:"host_id"`
	Proof           string   `json:"proof"`
	HostVersion     string   `json:"host_version"`
	LiveSessions    []string `json:"live_sessions"`
}

// Heartbeat is heartbeat (schema HostHeartbeat).
type Heartbeat struct {
	ActiveSessions int `json:"active_sessions"`
}

// SessionStarted is session.started (schema HostSessionStarted).
type SessionStarted struct {
	SessionID string `json:"session_id"`
}

// SessionEnded is session.ended (schema HostSessionEnded).
type SessionEnded struct {
	SessionID   string `json:"session_id"`
	CloseReason string `json:"close_reason"`
	CloseDetail string `json:"close_detail,omitempty"`
	ExitCode    *int   `json:"exit_code,omitempty"`
}

// required are the members each server op must carry (asyncapi messages and their schemas).
var required = map[string][]string{
	opHello:        {"protocol_versions", "challenge", "server_time", "heartbeat_interval_ms", "heartbeat_miss_limit", "jwks"},
	opReady:        {"host_id", "state", "llm_credentials"},
	opHeartbeatAck: {},
	opSessionOpen:  {"session_id", "data_path", "profile", "expires_at"},
	opSessionClose: {"session_id", "close_reason"},
	opError:        {"op", "code"},
}

// ErrFrame is a server frame the client cannot read: close 4002.
var ErrFrame = errors.New("control: malformed frame of the server")

// decodeServerFrame decodes one text frame of the server strictly: exactly the members op and d,
// an op the server sends, every required member present, no unknown member, no null. It returns
// the op and the body as its type (Hello, ReadyFrame, HeartbeatAck, SessionOpen, SessionClose or
// ErrorFrame). Errors wrap ErrFrame.
func decodeServerFrame(b []byte) (string, any, error) {
	var env struct {
		Op *string         `json:"op"`
		D  json.RawMessage `json:"d"`
	}
	if err := strictDecode(b, &env); err != nil || env.Op == nil || len(env.D) == 0 {
		return "", nil, fmt.Errorf("%w: envelope", ErrFrame)
	}
	op := *env.Op
	keys, ok := required[op]
	if !ok {
		return "", nil, fmt.Errorf("%w: op is no server op", ErrFrame)
	}
	var members map[string]json.RawMessage
	if err := json.Unmarshal(env.D, &members); err != nil || members == nil {
		return "", nil, fmt.Errorf("%w: body is no object", ErrFrame)
	}
	for _, k := range keys {
		if _, present := members[k]; !present {
			return "", nil, fmt.Errorf("%w: %s lacks %s", ErrFrame, op, k)
		}
	}
	for _, v := range members {
		if bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
			return "", nil, fmt.Errorf("%w: null", ErrFrame)
		}
	}
	var v any
	switch op {
	case opHello:
		v = &Hello{}
	case opReady:
		v = &ReadyFrame{}
	case opHeartbeatAck:
		v = &HeartbeatAck{}
	case opSessionOpen:
		v = &SessionOpen{}
	case opSessionClose:
		v = &SessionClose{}
	case opError:
		v = &ErrorFrame{}
	}
	if err := strictDecode(env.D, v); err != nil {
		return "", nil, fmt.Errorf("%w: %s body", ErrFrame, op)
	}
	return op, v, nil
}

// strictDecode decodes b into v, refusing unknown members and trailing data.
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

// encodeFrame returns the frame {op, d}.
func encodeFrame(op string, d any) ([]byte, error) {
	return json.Marshal(struct {
		Op string `json:"op"`
		D  any    `json:"d"`
	}{op, d})
}
