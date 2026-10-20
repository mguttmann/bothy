// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// The paths of a session (docs/api/openapi.yaml stream_path, asyncapi terminalStream and
// hostData).
const (
	streamPathPrefix = "/api/v1/terminal/sessions/"
	dataPathPrefix   = "/api/v1/hosts/sessions/"
	pathStreamSuffix = "/stream"
)

// StreamPath returns the path of the device stream of a session.
func StreamPath(id ids.ID) string { return streamPathPrefix + id.String() + pathStreamSuffix }

// DataPath returns the path of the data connection of a session.
func DataPath(id ids.ID) string { return dataPathPrefix + id.String() + pathStreamSuffix }

// errStepUpConsumed marks a ConsumeFresh that found the step-up stale or consumed: the unit rolls
// back and the use case records the denial outside it.
var errStepUpConsumed = errors.New("terminal: the step-up is not fresh")

// sessionsReady checks the configuration and the ports the sessions need; a process without them
// fails closed.
func (s *Service) sessionsReady() error {
	if s.cfg.SessionTokenTTL < time.Second || s.cfg.OpenTimeout <= 0 || s.d.StepUps == nil {
		return errs.New(errs.CodeInternalError, "terminal: terminal.session_token_ttl, terminal.open_timeout and the step-ups are not bound")
	}
	return nil
}

// openFacts are the facts of terminal.open gathered before authorize (Design note 3 of M2-44).
type openFacts struct {
	host       domain.Host
	found      bool
	assignment *domain.OwnSessionUser
	connected  bool
	known      bool
}

// gatherOpen reads the host and the caller's valid assignment there and decides the profile.
func (s *Service) gatherOpen(ctx context.Context, p actor.Actor, cmd terminal.OpenSession) (openFacts, error) {
	var f openFacts
	var err error
	if f.host, f.found, err = s.d.Store.Host(ctx, cmd.HostID); err != nil || !f.found {
		return f, err
	}
	if f.assignment, err = s.ownAssignment(ctx, p.ID, f.host.ID); err != nil {
		return f, err
	}
	own := []string{}
	if f.assignment != nil {
		own = domain.OwnWorkdirs(f.host.ReportedUsers, f.assignment.OSUser)
	}
	f.known = domain.ProfileKnown(f.host.Profiles, own, cmd.Profile, cmd.WorkdirIndex)
	f.connected = s.connected(ctx, f.host.ID)
	return f, nil
}

// ownAssignment returns the caller's valid assignment on hostID, nil for none.
func (s *Service) ownAssignment(ctx context.Context, userID, hostID ids.ID) (*domain.OwnSessionUser, error) {
	list, err := s.d.Store.OwnSessionUsers(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, u := range list {
		if u.HostID == hostID {
			return &u, nil
		}
	}
	return nil, nil
}

// openResource is the descriptor of terminal.open (Design note 3 a).
func openResource(hostID ids.ID, connected, known bool) authorize.Resource {
	return authorize.Resource{Kind: authorize.ResourceHost, ID: hostID,
		Terminal: &authorize.TerminalFacts{HostConnected: connected, ProfileKnown: known}}
}

// openSession implements Sessions.Open (Design note 4 of M2-44).
func (s *Service) openSession(ctx context.Context, p actor.Actor, cmd terminal.OpenSession) (terminal.Grant, error) {
	ctx, err := withCaller(ctx, p)
	if err != nil {
		return terminal.Grant{}, err
	}
	if err := s.sessionsReady(); err != nil {
		return terminal.Grant{}, err
	}
	f, err := s.gatherOpen(ctx, p, cmd)
	if err != nil {
		return terminal.Grant{}, err
	}
	if err := s.check(ctx, p, authorize.ActionTerminalOpen, openResource(cmd.HostID, f.connected, f.known)); err != nil {
		return terminal.Grant{}, err
	}
	if !f.found || f.assignment == nil {
		return terminal.Grant{}, errs.New(errs.CodeInternalError, "terminal: an opening was allowed without its host or session user")
	}
	owner, err := s.d.People.IsActiveOwner(ctx, p.ID)
	if err != nil {
		return terminal.Grant{}, err
	}
	h, now := f.host, s.d.Clock.Now()
	ses := domain.Session{
		ID: ids.New(), HostID: h.ID, UserID: p.ID, HostSessionUserID: f.assignment.ID, OpenedByDeviceID: p.DeviceID, DeviceID: p.DeviceID,
		AuthSessionID: p.SessionID, Profile: cmd.Profile, State: domain.SessionOpening, TokenJTI: ids.New(), KeyGeneration: h.KeyGeneration,
		OpenedAt: now, Version: 1,
	}
	tg := domain.TokenGrant{HostID: h.ID, UserID: p.ID, SessionID: ses.ID, DeviceID: p.DeviceID, Profile: cmd.Profile,
		ProfileKind: domain.ProfileKind(h.Profiles, cmd.Profile), CEK: cmd.CEK, Owner: owner, OSUser: f.assignment.OSUser, WorkdirIndex: cmd.WorkdirIndex}
	var g terminal.Grant
	s.noteGrant(h, ses, false, cmd.WorkdirIndex)
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		g, err = s.openUnit(ctx, p, h, ses, tg, cmd.PinChanged, owner)
		return err
	})
	if err != nil {
		s.notes.take(ses.ID)
		return terminal.Grant{}, s.stepUpDenied(ctx, err, authorize.ActionTerminalOpen, authorize.ResourceHost, h.ID)
	}
	if err := s.recheck(ctx, p, authorize.ActionTerminalOpen, ses.ID, s.openRecheck(ctx, p, cmd), h.ID); err != nil {
		return terminal.Grant{}, err
	}
	// The relay of every node expects the session, the node of the control connection sends
	// session.open (24 section 6.1 step 4, M2-47 Design note 3).
	s.committed(ctx, sessionOpenSignal(h.ID, ses.ID))
	return g, nil
}

// openUnit is the unit of work of an opening (Design note 4 c to f): the step-up as the first row
// lock, the row, the signed token, the allow entry and the event.
func (s *Service) openUnit(ctx context.Context, p actor.Actor, h domain.Host, ses domain.Session, tg domain.TokenGrant,
	pinChanged, owner bool) (terminal.Grant, error) {
	if err := s.consume(ctx, p); err != nil {
		return terminal.Grant{}, err
	}
	if err := s.d.Store.InsertSession(ctx, ses); err != nil {
		return terminal.Grant{}, err
	}
	g, err := s.grant(ctx, h, ses, tg, ses.OpenedAt)
	if err != nil {
		return terminal.Grant{}, err
	}
	if err := s.recordGrant(ctx, authorize.ActionTerminalOpen, h, ses, pinChanged, owner); err != nil {
		return terminal.Grant{}, err
	}
	if err := s.emitSessionChangedBy(ctx, ses, p.ID); err != nil {
		return terminal.Grant{}, err
	}
	if err := s.stage(ctx, sessionOpenSignal(h.ID, ses.ID)); err != nil {
		return terminal.Grant{}, err
	}
	if s.d.BeforeOpenCommit != nil {
		s.d.BeforeOpenCommit()
	}
	return g, nil
}

// openRecheck returns the descriptor builder of the recheck of an opening: fresh profile facts of
// the host read again, the connection as the first decision saw it (Design note 12).
func (s *Service) openRecheck(ctx context.Context, p actor.Actor, cmd terminal.OpenSession) func(domain.Host) (authorize.Resource, error) {
	return func(h domain.Host) (authorize.Resource, error) {
		u, err := s.ownAssignment(ctx, p.ID, h.ID)
		if err != nil {
			return authorize.Resource{}, err
		}
		known := u != nil && domain.ProfileKnown(h.Profiles, domain.OwnWorkdirs(h.ReportedUsers, u.OSUser), cmd.Profile, cmd.WorkdirIndex)
		return openResource(h.ID, true, known), nil
	}
}

// consume consumes the fresh step-up of p inside the unit of work; a stale or consumed step-up
// becomes errStepUpConsumed.
func (s *Service) consume(ctx context.Context, p actor.Actor) error {
	err := s.d.StepUps.ConsumeFresh(ctx, p)
	if e, ok := errs.As(err); ok && e.Code == errs.CodeStepUpRequired {
		return errStepUpConsumed
	}
	return err
}

// stepUpDenied turns errStepUpConsumed into the denial of 08 section 6.3 point 4: the unit rolled
// back, the deny entry with terminal_step_up_not_fresh is recorded (aggregated like every denial)
// and the answer is 401 step_up_required. Any other error passes unchanged.
func (s *Service) stepUpDenied(ctx context.Context, err error, act authorize.Action, kind authorize.ResourceKind, id ids.ID) error {
	if !errors.Is(err, errStepUpConsumed) {
		return err
	}
	if rerr := s.d.Audit.RecordDenial(ctx, auditing.Entry{Action: string(act), Decision: auditing.DecisionDeny,
		Reason: auditing.ReasonTerminalStepUpNotFresh, ResourceKind: string(kind), ResourceID: id}); rerr != nil {
		return rerr
	}
	return errs.New(errs.CodeStepUpRequired, "")
}

// grant signs the token of tg with the jti of ses at now and builds the answer (Design note 4 h).
func (s *Service) grant(ctx context.Context, h domain.Host, ses domain.Session, tg domain.TokenGrant, now time.Time) (terminal.Grant, error) {
	id, err := s.d.Instance.InstanceID(ctx)
	if err != nil {
		return terminal.Grant{}, err
	}
	tg.InstanceID = id
	tok, exp, err := s.issueToken(ctx, tg, ses.TokenJTI, now)
	if err != nil {
		return terminal.Grant{}, err
	}
	return terminal.Grant{Session: sessionView(ses), Token: tok, TokenExpiresAt: exp, StreamPath: StreamPath(ses.ID),
		NoisePublicKey: h.NoisePublicKey, KeyFingerprint: h.KeyFingerprint, KeyGeneration: h.KeyGeneration,
		NoiseProtocol: domain.NoiseProtocol, PrologueVersion: domain.PrologueVersion}, nil
}

// recordGrant writes the allow entry of terminal.open or terminal.attach (24 section 10.1):
// host_id, session_id, profile, device_id, pin_changed and foreign_assignee (an active owner on a
// host assigned to another person, 24 section 8.4); never the token or a name.
func (s *Service) recordGrant(ctx context.Context, act authorize.Action, h domain.Host, ses domain.Session, pinChanged, owner bool) error {
	foreign := owner && !h.AssignedUserID.IsZero() && h.AssignedUserID != ses.UserID
	return s.d.Audit.Record(ctx, auditing.Entry{
		Action: string(act), Decision: auditing.DecisionAllow, ResourceKind: resourceSession, ResourceID: ses.ID,
		Attrs: auditing.Attrs{attrHostID: auditing.ID(h.ID), attrSessionID: auditing.ID(ses.ID), attrProfile: auditing.Code(ses.Profile),
			attrDeviceID: auditing.ID(ses.DeviceID), attrPinChanged: auditing.Bool(pinChanged), attrForeignAssignee: auditing.Bool(foreign)},
	})
}

// recheck authorizes act again after the commit with fresh facts under authorize.Recheck (Design
// note 12 of M2-44): when it no longer allows, the session ends through the one end path with the
// mapped close reason and the answer is the recheck's error; nothing is sent to the host.
func (s *Service) recheck(ctx context.Context, p actor.Actor, act authorize.Action, sessionID ids.ID,
	resource func(domain.Host) (authorize.Resource, error), hostID ids.ID) error {
	h, found, err := s.d.Store.Host(ctx, hostID)
	if err != nil {
		return err
	}
	if !found {
		h = domain.Host{ID: hostID}
	}
	r, err := resource(h)
	if err != nil {
		return err
	}
	d, err := s.d.Authorizer.Authorize(authorize.Recheck(ctx), p, act, r)
	if err != nil {
		return err
	}
	if d.Allowed() {
		return nil
	}
	reason := domain.RecheckEnd(string(d.Reason), h.State)
	if err := s.d.UoW.Do(ctx, func(ctx context.Context) error {
		ses, found, err := s.d.Store.LockSession(ctx, sessionID)
		if err != nil || !found || ses.State == domain.SessionClosed {
			return err
		}
		return s.endSession(ctx, ses, domain.HostSessionEnd{Reason: reason}, false)
	}); err != nil {
		return err
	}
	if derr := d.Err(); derr != nil {
		return derr
	}
	return errs.New(errs.CodeForbidden, "")
}

// noteGrant remembers the working directory index of an opening of a claude profile for the
// handler of terminal.session_open on this node (ruling D-M2-44-4); other grants keep none.
func (s *Service) noteGrant(h domain.Host, ses domain.Session, started bool, wdi *int) {
	if !started && domain.ProfileKind(h.Profiles, ses.Profile) == domain.ProfileKindClaude {
		s.notes.put(ses.ID, wdi)
		return
	}
	s.notes.put(ses.ID, nil)
}

// sessionView converts a session row (schema TerminalSession).
func sessionView(ses domain.Session) terminal.SessionView {
	return terminal.SessionView{
		ID: ses.ID, HostID: ses.HostID, UserID: ses.UserID, DeviceID: ses.DeviceID, Profile: ses.Profile, State: ses.State,
		OpenedAt: ses.OpenedAt, StartedAt: ses.StartedAt, DetachedAt: ses.DetachedAt, ClosedAt: ses.ClosedAt,
		CloseReason: string(ses.CloseReason), CloseDetail: string(ses.CloseDetail), ExitCode: ses.ExitCode,
		BytesIn: ses.BytesIn, BytesOut: ses.BytesOut, Version: ses.Version,
	}
}
