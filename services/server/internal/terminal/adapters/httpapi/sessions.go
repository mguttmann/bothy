// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"encoding/base64"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal"
)

// TerminalSessions implements openTerminalSession, attachTerminalSession, listTerminalSessions,
// getTerminalSession and closeTerminalSession (M2-44) as part of rest.StrictServerInterface. The
// use cases of terminal.Sessions authorize; the handlers only convert. The session token leaves
// the server only in the answer of an opening or an attach (Reveal, once); both operations are
// exempt from Idempotency-Key (x-idempotency-exempt auth_flow), so no copy of it is stored.
type TerminalSessions struct {
	sessions terminal.Sessions
}

// NewSessions returns the handlers over the use cases.
func NewSessions(s terminal.Sessions) *TerminalSessions { return &TerminalSessions{sessions: s} }

// OpenTerminalSession implements rest.StrictServerInterface (POST /hosts/{host_id}/terminal/sessions).
func (h *TerminalSessions) OpenTerminalSession(ctx context.Context, req rest.OpenTerminalSessionRequestObject) (rest.OpenTerminalSessionResponseObject, error) {
	cmd := terminal.OpenSession{HostID: ids.ID(req.HostId)}
	if b := req.Body; b != nil {
		cmd.Profile, cmd.WorkdirIndex, cmd.Cols, cmd.Rows, cmd.CEK = b.Profile, b.WorkdirIndex, b.Cols, b.Rows, b.ClientEphemeralKeyHash
		cmd.PinChanged = b.PinChanged != nil && *b.PinChanged
	}
	g, err := h.sessions.Open(ctx, principal(ctx), cmd)
	if err != nil {
		return nil, err
	}
	return rest.OpenTerminalSession201JSONResponse{Body: grantREST(g)}, nil
}

// AttachTerminalSession implements rest.StrictServerInterface (POST
// /terminal/sessions/{session_id}/attach).
func (h *TerminalSessions) AttachTerminalSession(ctx context.Context, req rest.AttachTerminalSessionRequestObject) (rest.AttachTerminalSessionResponseObject, error) {
	cmd := terminal.AttachSession{SessionID: ids.ID(req.SessionId)}
	if b := req.Body; b != nil {
		cmd.Cols, cmd.Rows, cmd.CEK = b.Cols, b.Rows, b.ClientEphemeralKeyHash
		cmd.PinChanged = b.PinChanged != nil && *b.PinChanged
	}
	g, err := h.sessions.Attach(ctx, principal(ctx), cmd)
	if err != nil {
		return nil, err
	}
	return rest.AttachTerminalSession200JSONResponse{Body: grantREST(g)}, nil
}

// ListTerminalSessions implements rest.StrictServerInterface (GET /terminal/sessions).
func (h *TerminalSessions) ListTerminalSessions(ctx context.Context, req rest.ListTerminalSessionsRequestObject) (rest.ListTerminalSessionsResponseObject, error) {
	var q terminal.ListSessions
	p := req.Params
	if p.Cursor != nil {
		q.Cursor = *p.Cursor
	}
	if p.Limit != nil {
		q.Limit = *p.Limit
	}
	if p.HostId != nil {
		q.HostID = ids.ID(*p.HostId)
	}
	if p.State != nil {
		q.State = string(*p.State)
	}
	page, err := h.sessions.List(ctx, principal(ctx), q)
	if err != nil {
		return nil, err
	}
	out := rest.TerminalSessionPage{Items: make([]rest.TerminalSession, 0, len(page.Items))}
	for _, v := range page.Items {
		out.Items = append(out.Items, sessionREST(v))
	}
	if page.NextCursor != "" {
		out.NextCursor = &page.NextCursor
	}
	return rest.ListTerminalSessions200JSONResponse{Body: out}, nil
}

// GetTerminalSession implements rest.StrictServerInterface (GET /terminal/sessions/{session_id}).
func (h *TerminalSessions) GetTerminalSession(ctx context.Context, req rest.GetTerminalSessionRequestObject) (rest.GetTerminalSessionResponseObject, error) {
	v, err := h.sessions.Get(ctx, principal(ctx), ids.ID(req.SessionId))
	if err != nil {
		return nil, err
	}
	return rest.GetTerminalSession200JSONResponse{Body: sessionREST(v)}, nil
}

// CloseTerminalSession implements rest.StrictServerInterface (POST
// /terminal/sessions/{session_id}/close); a replay under the same Idempotency-Key answers the
// stored response (M1-20).
func (h *TerminalSessions) CloseTerminalSession(ctx context.Context, req rest.CloseTerminalSessionRequestObject) (rest.CloseTerminalSessionResponseObject, error) {
	cmd := terminal.CloseSession{SessionID: ids.ID(req.SessionId)}
	if b := req.Body; b != nil {
		cmd.Reason = b.Reason
	}
	v, err := h.sessions.Close(ctx, principal(ctx), cmd)
	if err != nil {
		return nil, err
	}
	return rest.CloseTerminalSession200JSONResponse{Body: sessionREST(v)}, nil
}

// grantREST converts a grant (schema TerminalSessionGrant); the token is revealed here only.
func grantREST(g terminal.Grant) rest.TerminalSessionGrant {
	return rest.TerminalSessionGrant{
		Session: sessionREST(g.Session), SessionToken: g.Token.Reveal(), SessionTokenExpiresAt: g.TokenExpiresAt.UTC().Truncate(time.Millisecond),
		StreamPath: g.StreamPath, NoisePublicKey: base64.RawURLEncoding.EncodeToString(g.NoisePublicKey), KeyFingerprint: g.KeyFingerprint,
		KeyGeneration: int(g.KeyGeneration), NoiseProtocol: rest.TerminalSessionGrantNoiseProtocol(g.NoiseProtocol),
		PrologueVersion: rest.TerminalSessionGrantPrologueVersion(g.PrologueVersion),
	}
}

// sessionREST converts a session (schema TerminalSession); empty values are absent members.
func sessionREST(v terminal.SessionView) rest.TerminalSession {
	out := rest.TerminalSession{
		Id: rest.Uuid(v.ID), HostId: rest.Uuid(v.HostID), UserId: rest.Uuid(v.UserID), DeviceId: rest.Uuid(v.DeviceID), Profile: v.Profile,
		State: rest.TerminalSessionState(v.State), OpenedAt: v.OpenedAt.UTC().Truncate(time.Millisecond), StartedAt: optionalTime(v.StartedAt),
		DetachedAt: optionalTime(v.DetachedAt), ClosedAt: optionalTime(v.ClosedAt), ExitCode: v.ExitCode, BytesIn: v.BytesIn,
		BytesOut: v.BytesOut, Version: v.Version,
	}
	if v.CloseReason != "" {
		r := rest.TerminalCloseReason(v.CloseReason)
		out.CloseReason = &r
	}
	if v.CloseDetail != "" {
		d := rest.TerminalCloseDetail(v.CloseDetail)
		out.CloseDetail = &d
	}
	return out
}
