// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// The audit details of the declaration of the LLM login (24 section 10.1, Design note 10 of M2-40).
const (
	opLLMCredentials       = "llm_credentials"
	attrFromLLMCredentials = "from_llm_credentials" //nolint:gosec // G101: an audit attribute name, no secret
	attrToLLMCredentials   = "to_llm_credentials"   //nolint:gosec // G101: an audit attribute name, no secret
	attrFromHolder         = "from_credential_holder_id"
	attrToHolder           = "to_credential_holder_id"
)

// declarationOf returns the stored declaration of h.
func declarationOf(h domain.Host) domain.Declaration {
	d := domain.Declaration{Credentials: h.LLMCredentials}
	if h.LLMCredentials == domain.LLMPersonal {
		d.HolderID = h.CredentialHolderID
	}
	return d
}

// SetLlmCredentials implements terminal.Hosts (24 sections 7.2 and 7.3, Design note 10 of M2-40).
// A first unit authorizes host.manage on the host (gate, 404, owner_only, step-up), checks the
// reason, the declaration and its holder (a person that is not deleted), If-Match, refuses a
// retired host with 409 conflict and answers the declaration the host has as a call without
// effect (no_change, no version, no event, no rights version). Otherwise a second unit, declared as
// the rights change host.manage with the old and the new holder, locks the host after
// permission_version (03 section 6.8), checks the version again, applies the declaration
// (ApplyLlmDeclaration), records the allow entry and appends host.updated; phase 3.1 then runs I6,
// whose violation rolls everything back, the ended sessions included, with 409 i6_violation.
func (s *Service) SetLlmCredentials(ctx context.Context, a actor.Actor, cmd terminal.SetLlmCredentials) (terminal.HostView, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return terminal.HostView{}, err
	}
	d := domain.Declaration{Credentials: cmd.LLMCredentials, HolderID: cmd.CredentialHolderID}
	var (
		out    terminal.HostView
		reason string
		before domain.Declaration
		change bool
	)
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		reason, before, change, out, err = s.llmCheck(ctx, a, cmd, d)
		return err
	})
	if err != nil || !change {
		return out, err
	}
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.llmWrite(ctx, a, cmd, d, reason)
		return err
	}, rightsChange(cmd.HostID, before.HolderID, d.HolderID))
	if err == nil {
		// The host service treats every ready as the current declaration (24 section 7.4).
		s.control().SendReady(cmd.HostID)
	}
	return out, err
}

// llmCheck is the first unit of SetLlmCredentials: authorize, the body, If-Match, the retired
// host and the call without effect. It returns the reason, the stored declaration and whether the
// call changes the host.
func (s *Service) llmCheck(ctx context.Context, a actor.Actor, cmd terminal.SetLlmCredentials, d domain.Declaration) (string, domain.Declaration, bool, terminal.HostView, error) {
	if err := s.check(ctx, a, authorize.ActionHostManage, hostResource(cmd.HostID)); err != nil {
		return "", domain.Declaration{}, false, terminal.HostView{}, err
	}
	reason, err := domain.CheckReason(cmd.Reason, s.cfg.ReasonMaxChars)
	if err != nil {
		return "", domain.Declaration{}, false, terminal.HostView{}, err
	}
	if err := domain.ValidateDeclaration(d); err != nil {
		return "", domain.Declaration{}, false, terminal.HostView{}, err
	}
	if !d.HolderID.IsZero() {
		if err := s.person(ctx, d.HolderID, "/credential_holder_id"); err != nil {
			return "", domain.Declaration{}, false, terminal.HostView{}, err
		}
	}
	st, err := s.loadFor(ctx, cmd.HostID, cmd.IfMatch)
	if err != nil {
		return "", domain.Declaration{}, false, terminal.HostView{}, err
	}
	if st.host.State == domain.StateRetired {
		return "", domain.Declaration{}, false, terminal.HostView{}, errRetired()
	}
	before := declarationOf(st.host)
	if before.Same(d) {
		return reason, before, false, s.view(ctx, st), s.noChange(ctx, st.host.ID, opLLMCredentials, reason)
	}
	return reason, before, true, terminal.HostView{}, nil
}

// errRetired is the 409 conflict of a write on a retired host (24 section 3.1: retired is final).
func errRetired() error { return errs.New(errs.CodeConflict, "The host is retired.") }

// llmWrite is the second unit of SetLlmCredentials, declared as the rights change.
func (s *Service) llmWrite(ctx context.Context, a actor.Actor, cmd terminal.SetLlmCredentials, d domain.Declaration, reason string) (terminal.HostView, error) {
	h, err := s.lockFor(ctx, cmd.HostID, cmd.IfMatch)
	if err != nil {
		return terminal.HostView{}, err
	}
	if h.State == domain.StateRetired {
		return terminal.HostView{}, errRetired()
	}
	before := declarationOf(h)
	next, _, err := s.ApplyLlmDeclaration(ctx, h, d, a.ID, s.d.Clock.Now())
	if err != nil {
		return terminal.HostView{}, err
	}
	if err := s.record(ctx, h.ID, llmAttrs(before, d, reason)); err != nil {
		return terminal.HostView{}, err
	}
	st, _, err := s.withUsers(ctx, next)
	if err != nil {
		return terminal.HostView{}, err
	}
	v := s.view(ctx, st)
	return v, s.emitUpdated(ctx, v)
}

// ApplyLlmDeclaration writes the declaration d on the host h, whose row the caller locked inside a
// rights change, as declared by by at t with the next version, and when EndsForeignSessions holds
// ends every session on the host that is not closed and not of the new holder with
// close_reason i6_violation in the same transaction (24 section 7.3 point 3, 03 section 6.8). It
// returns the host after the write and the number of ended sessions. The caller records its audit
// entry and appends host.updated; the I6 check of the rights change runs in phase 3.1. M2-41 calls
// it for confirmHost. Errors: those of the store and of EndSessions.
func (s *Service) ApplyLlmDeclaration(ctx context.Context, h domain.Host, d domain.Declaration, by ids.ID, t time.Time) (domain.Host, int, error) {
	before := declarationOf(h)
	next := h
	next.LLMCredentials, next.CredentialHolderID = d.Credentials, ids.ID{}
	if d.Credentials == domain.LLMPersonal {
		next.CredentialHolderID = d.HolderID
	}
	next.Version++
	if err := s.d.Store.UpdateLlmCredentials(ctx, h.ID, d, by, t, next.Version); err != nil {
		return domain.Host{}, 0, err
	}
	if !domain.EndsForeignSessions(before, d) {
		return next, 0, nil
	}
	n, err := s.EndSessions(ctx, SessionSelector{HostID: h.ID, ExceptUserID: d.HolderID}, domain.CloseI6Violation)
	if err != nil {
		return domain.Host{}, 0, err
	}
	return next, n, nil
}

// llmAttrs are the flat details of the allow entry of a declaration (24 section 10.1): the
// operation, the old and the new value as codes, the holders (absent for none) and the reason.
func llmAttrs(before, after domain.Declaration, reason string) auditing.Attrs {
	attrs := auditing.Attrs{
		attrOperation: auditing.Code(opLLMCredentials), attrReason: auditing.ReasonText(reason),
		attrFromLLMCredentials: auditing.Code(before.Credentials), attrToLLMCredentials: auditing.Code(after.Credentials),
	}
	if !before.HolderID.IsZero() {
		attrs[attrFromHolder] = auditing.ID(before.HolderID)
	}
	if !after.HolderID.IsZero() {
		attrs[attrToHolder] = auditing.ID(after.HolderID)
	}
	return attrs
}
