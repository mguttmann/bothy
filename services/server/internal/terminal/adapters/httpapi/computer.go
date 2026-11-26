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

// Computers implements listComputerDesktops, openComputerSession, listComputerSessions,
// stopComputerSession and disableComputerDevice (CU-04, docs/spec/27-computer-use.md 12.2) as part
// of rest.StrictServerInterface. The use cases of terminal.Computers authorize; the handlers only
// convert. The attach token leaves the server only in the answer of an opening (Reveal, once);
// openComputerSession is exempt from Idempotency-Key (x-idempotency-exempt auth_flow), so no copy
// of it is stored.
type Computers struct {
	computers terminal.Computers
}

// NewComputers returns the handlers over the use cases.
func NewComputers(c terminal.Computers) *Computers { return &Computers{computers: c} }

// ListComputerDesktops implements rest.StrictServerInterface (GET /computer/desktops).
func (h *Computers) ListComputerDesktops(ctx context.Context, _ rest.ListComputerDesktopsRequestObject) (rest.ListComputerDesktopsResponseObject, error) {
	list, err := h.computers.Desktops(ctx, principal(ctx))
	if err != nil {
		return nil, err
	}
	out := rest.ComputerDesktopList{Items: make([]rest.ComputerDesktop, 0, len(list))}
	for _, d := range list {
		out.Items = append(out.Items, rest.ComputerDesktop{HostId: rest.Uuid(d.HostID), Name: d.Name, Platform: rest.HostPlatform(d.Platform),
			Online: d.Online, Locked: d.Locked, Busy: d.Busy})
	}
	return rest.ListComputerDesktops200JSONResponse{Body: out}, nil
}

// OpenComputerSession implements rest.StrictServerInterface (POST /computer/sessions).
func (h *Computers) OpenComputerSession(ctx context.Context, req rest.OpenComputerSessionRequestObject) (rest.OpenComputerSessionResponseObject, error) {
	var cmd terminal.OpenComputer
	if b := req.Body; b != nil {
		cmd.HostID, cmd.CEK = ids.ID(b.HostId), b.ClientEphemeralKeyHash
		cmd.PinChanged = b.PinChanged != nil && *b.PinChanged
	}
	g, err := h.computers.Open(ctx, principal(ctx), cmd)
	if err != nil {
		return nil, err
	}
	return rest.OpenComputerSession201JSONResponse{Body: rest.ComputerSessionGrant{
		SessionId: rest.Uuid(g.SessionID), Token: g.Token.Reveal(), AttachPath: g.AttachPath,
		NoisePublicKey: base64.RawURLEncoding.EncodeToString(g.NoisePublicKey), KeyFingerprint: g.KeyFingerprint,
		KeyGeneration: int(g.KeyGeneration), ExpiresAt: g.ExpiresAt.UTC().Truncate(time.Millisecond),
	}}, nil
}

// ListComputerSessions implements rest.StrictServerInterface (GET /computer/sessions).
func (h *Computers) ListComputerSessions(ctx context.Context, req rest.ListComputerSessionsRequestObject) (rest.ListComputerSessionsResponseObject, error) {
	state := ""
	if req.Params.State != nil {
		state = string(*req.Params.State)
	}
	list, err := h.computers.List(ctx, principal(ctx), state)
	if err != nil {
		return nil, err
	}
	out := rest.ComputerSessionList{Items: make([]rest.ComputerSession, 0, len(list))}
	for _, v := range list {
		out.Items = append(out.Items, computerREST(v))
	}
	return rest.ListComputerSessions200JSONResponse{Body: out}, nil
}

// StopComputerSession implements rest.StrictServerInterface (POST /computer/sessions/{session_id}/stop).
func (h *Computers) StopComputerSession(ctx context.Context, req rest.StopComputerSessionRequestObject) (rest.StopComputerSessionResponseObject, error) {
	v, err := h.computers.Stop(ctx, principal(ctx), ids.ID(req.SessionId))
	if err != nil {
		return nil, err
	}
	return rest.StopComputerSession200JSONResponse{Body: computerREST(v)}, nil
}

// DisableComputerDevice implements rest.StrictServerInterface (POST /hosts/{host_id}/computer/disable).
func (h *Computers) DisableComputerDevice(ctx context.Context, req rest.DisableComputerDeviceRequestObject) (rest.DisableComputerDeviceResponseObject, error) {
	v, err := h.computers.DisableDevice(ctx, principal(ctx), ids.ID(req.HostId))
	if err != nil {
		return nil, err
	}
	return rest.DisableComputerDevice200JSONResponse{Body: toREST(v)}, nil
}

// computerREST converts a session (schema ComputerSession); empty values are absent members.
func computerREST(v terminal.ComputerSessionView) rest.ComputerSession {
	out := rest.ComputerSession{Id: rest.Uuid(v.ID), HostId: rest.Uuid(v.HostID), Desktop: v.Desktop, UserId: rest.Uuid(v.UserID),
		AgentId: rest.Uuid(v.AgentID), RunId: rest.Uuid(v.RunID), State: rest.ComputerSessionState(v.State), Generation: int(v.Generation),
		KeyGeneration: int(v.KeyGeneration), AttachedAt: optionalTime(v.AttachedAt), OpenedAt: v.OpenedAt.UTC().Truncate(time.Millisecond),
		StartedAt: optionalTime(v.StartedAt), PausedAt: optionalTime(v.PausedAt), PausedMs: v.PausedMS, ClosedAt: optionalTime(v.ClosedAt),
		StoppedBy: optionalID(v.StoppedBy), ActionCount: int(v.ActionCount), FrontApp: optional(v.FrontApp), Version: v.Version}
	if v.PauseReason != "" {
		r := rest.ComputerSessionPauseReason(v.PauseReason)
		out.PauseReason = &r
	}
	if v.CloseReason != "" {
		r := rest.ComputerCloseReason(v.CloseReason)
		out.CloseReason = &r
	}
	if v.CloseDetail != "" {
		d := rest.ComputerCloseDetail(v.CloseDetail)
		out.CloseDetail = &d
	}
	return out
}
