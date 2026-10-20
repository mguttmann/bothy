// SPDX-License-Identifier: Apache-2.0

package wsrelay

import (
	"bytes"
	"encoding/json"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/terminal/app"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// The ops of the channel hostControl (asyncapi.yaml).
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

// hostOps are the ops a host service sends, with the schema of their body d.
var hostOps = map[string]string{
	opIdentify: "HostIdentify", opHeartbeat: "HostHeartbeat", opReport: "HostReport",
	opSessionStarted: "HostSessionStarted", opSessionEnded: "HostSessionEnded",
}

// SchemaNames returns the schemas of the host ops, for the validator of internal/wire.
func SchemaNames() []string {
	return []string{"HostIdentify", "HostHeartbeat", "HostReport", "HostSessionStarted", "HostSessionEnded"}
}

// SchemaChecker checks a JSON value against a named schema of the contract
// (httpserver.SchemaValidator over api/rest.Spec).
type SchemaChecker interface {
	Validate(name string, body []byte) error
}

// errDecode is a frame the codec cannot read: close 4002.
var errDecode = errors.New("wsrelay: decode error")

// errUnknownOp is a well-formed frame whose op the server does not accept from a host: close 4002.
var errUnknownOp = errors.New("wsrelay: unknown op")

// envelope is the outer object {op, d} of a host frame.
type envelope struct {
	Op *string         `json:"op"`
	D  json.RawMessage `json:"d"`
}

// frame is a decoded host frame: the op and its body, already of the generated type of its schema.
type frame struct {
	op string
	d  any
}

// decodeFrame reads one text frame of a host service (Design note 2 of M2-43): SEC-IN-2 first
// (valid UTF-8, one JSON value, no duplicate member, at most maxDepth levels), then exactly the
// members op (a non-empty string) and d, an op the host may send, its body checked against its
// schema and decoded strictly (no unknown member) into the generated type. Errors: errDecode for
// every malformed frame, errUnknownOp for an op the server does not take from a host.
func decodeFrame(b []byte, maxDepth int, check SchemaChecker) (frame, error) {
	if err := httpserver.CheckJSON(b, maxDepth); err != nil {
		return frame{}, errDecode
	}
	var env envelope
	if err := strict(b, &env); err != nil || env.Op == nil || *env.Op == "" {
		return frame{}, errDecode
	}
	op := *env.Op
	schema, ok := hostOps[op]
	if !ok {
		return frame{}, errUnknownOp
	}
	if len(env.D) == 0 || check.Validate(schema, env.D) != nil {
		return frame{}, errDecode
	}
	var d any
	switch op {
	case opIdentify:
		d = &rest.HostIdentify{}
	case opHeartbeat:
		d = &rest.HostHeartbeat{}
	case opReport:
		d = &rest.HostReport{}
	case opSessionStarted:
		d = &rest.HostSessionStarted{}
	case opSessionEnded:
		d = &rest.HostSessionEnded{}
	}
	if err := strict(env.D, d); err != nil {
		return frame{}, errDecode
	}
	return frame{op: op, d: d}, nil
}

// strict decodes b into v and refuses unknown members and trailing data.
func strict(b []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if dec.More() {
		return errDecode
	}
	return nil
}

// identifyOf converts the body of identify.
func identifyOf(f *rest.HostIdentify) app.Identify {
	in := app.Identify{ProtocolVersion: f.ProtocolVersion, HostID: ids.ID(f.HostId), Proof: f.Proof, HostVersion: f.HostVersion,
		HasLiveSessions: f.LiveSessions != nil, HasSessionID: f.SessionId != nil}
	if f.LiveSessions != nil {
		for _, id := range *f.LiveSessions {
			in.LiveSessions = append(in.LiveSessions, ids.ID(id))
		}
	}
	if f.SessionId != nil {
		in.SessionID = ids.ID(*f.SessionId)
	}
	return in
}

// reportOf converts the body of report.
func reportOf(f *rest.HostReport) domain.Report {
	r := domain.Report{IsServerHost: f.IsServerHost, RequireLocalConfirm: f.RequireLocalConfirm, Profiles: []domain.Profile{},
		SessionUsers: []domain.ReportedSessionUser{}, Doctor: []domain.DoctorCheck{}}
	if f.OsVersion != nil {
		r.OSVersion = *f.OsVersion
	}
	for _, p := range f.Profiles {
		r.Profiles = append(r.Profiles, domain.Profile{Name: p.Name, DisplayName: p.DisplayName, Kind: string(p.Kind)})
	}
	for _, u := range f.SessionUsers {
		dirs := u.Workdirs
		if dirs == nil {
			dirs = []string{}
		}
		r.SessionUsers = append(r.SessionUsers, domain.ReportedSessionUser{OSUser: u.OsUser, Workdirs: dirs})
	}
	for _, c := range f.Doctor {
		e := domain.DoctorCheck{Check: string(c.Check), OK: c.Ok}
		if c.Code != nil {
			e.Code = *c.Code
		}
		if c.SessionUser != nil {
			e.SessionUser = *c.SessionUser
		}
		r.Doctor = append(r.Doctor, e)
	}
	return r
}

// endOf converts the body of session.ended.
func endOf(f *rest.HostSessionEnded) domain.HostSessionEnd {
	e := domain.HostSessionEnd{Reason: domain.CloseReason(f.CloseReason), ExitCode: f.ExitCode}
	if f.CloseDetail != nil {
		e.Detail = domain.CloseDetail(*f.CloseDetail)
	}
	return e
}

// out is the outer object of a server frame.
type out struct {
	Op string `json:"op"`
	D  any    `json:"d"`
}

// stamp is the form of every time of a frame: UTC with millisecond precision (schema Timestamp).
func stamp(t time.Time) time.Time { return t.UTC().Truncate(time.Millisecond) }

// jwks converts a key set into the schema Jwks.
func jwks(keys []app.JWK) rest.Jwks {
	set := rest.Jwks{Keys: make([]rest.JsonObject, 0, len(keys))}
	for _, k := range keys {
		set.Keys = append(set.Keys, rest.JsonObject{"kty": k.Kty, "crv": k.Crv, "x": k.X, "kid": k.Kid, "alg": k.Alg, "use": k.Use})
	}
	return set
}

// encode returns the JSON text of the server frame op with body d (a generated Host* type).
func encode(op string, d any) []byte {
	b, err := json.Marshal(out{Op: op, D: d})
	if err != nil {
		// The generated types hold only strings, numbers, booleans and lists of them.
		panic("wsrelay: frame not encodable: " + err.Error())
	}
	return b
}

// helloFrame is hello (schema HostChannelHello) on the control and the data connection: the
// heartbeat keys and the server values of cfg the host service applies to a session (M2-45
// Design note 2a).
func helloFrame(challenge string, at time.Time, cfg Config, keys []app.JWK) []byte {
	return encode(opHello, rest.HostChannelHello{
		ProtocolVersions: domain.SupportedProtocolVersions(), Challenge: challenge, ServerTime: stamp(at),
		HeartbeatIntervalMs: cfg.HeartbeatInterval.Milliseconds(), HeartbeatMissLimit: cfg.HeartbeatMissLimit,
		ClockLeewayMs: cfg.ClockLeeway.Milliseconds(), IdleTimeoutMs: cfg.IdleTimeout.Milliseconds(),
		DetachGraceMs: cfg.DetachGrace.Milliseconds(), Jwks: jwks(keys),
	})
}

// readyFrame is ready (schema HostReady).
func readyFrame(r app.Ready) []byte {
	return encode(opReady, rest.HostReady{HostId: rest.Uuid(r.HostID), State: rest.HostState(r.State), LlmCredentials: rest.LlmCredentials(r.LLMCredentials)})
}

// heartbeatAckFrame is heartbeat_ack (schema HostHeartbeatAck); keys nil leaves jwks out.
func heartbeatAckFrame(at time.Time, keys []app.JWK) []byte {
	t := stamp(at)
	ack := rest.HostHeartbeatAck{ServerTime: &t}
	if keys != nil {
		set := jwks(keys)
		ack.Jwks = &set
	}
	return encode(opHeartbeatAck, ack)
}

// sessionCloseFrame is session.close (schema HostSessionClose).
func sessionCloseFrame(c domain.SessionClose) []byte {
	return encode(opSessionClose, rest.HostSessionClose{SessionId: rest.Uuid(c.SessionID), CloseReason: rest.TerminalCloseReason(c.Reason)})
}

// sessionOpenFrame is session.open (schema HostSessionOpen).
func sessionOpenFrame(c app.SessionOpen) []byte {
	return encode(opSessionOpen, rest.HostSessionOpen{SessionId: rest.Uuid(c.SessionID), DataPath: c.DataPath, Profile: c.Profile,
		ExpiresAt: stamp(c.ExpiresAt)})
}

// errorFrame is error (schema GatewayError) for op with code.
func errorFrame(op, code string) []byte {
	return encode(opError, rest.GatewayError{Op: op, Code: code})
}
