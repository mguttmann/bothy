// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/agents/domain"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// The hosted agents of docs/spec/26-builtin-agents.md 3 (A1-03): which Claude Code runner hosts
// which built-in agent, the moves between runners, the list a runner reads and the tokens only
// the hosting runner receives.

// Compile-time checks: the incoming port of hosting and the port identity asks for built-in agents.
var (
	_ agents.Hosting        = (*Service)(nil)
	_ identity.HostedAgents = (*Service)(nil)
)

// capableHost reads the runner host, with lock under a row lock until the end of the unit of
// work, and checks that it can host the agent self (domain.CheckHost): the record and the status
// of its principal. Errors: validation_failed with host_not_capable; store errors.
func (s *Service) capableHost(ctx context.Context, self, host ids.ID, lock bool) (domain.Agent, error) {
	read := s.d.Store.Agent
	if lock {
		read = s.d.Store.LockAgent
	}
	rec, found, err := read(ctx, host)
	if err != nil {
		return domain.Agent{}, err
	}
	active := false
	if found {
		p, err := s.d.Directory.Principal(ctx, host)
		if err != nil {
			return domain.Agent{}, err
		}
		active = p.Status == identity.StatusActive
	}
	return rec, domain.CheckHost(self, rec, found, active)
}

// hostingPayload is AgentHostingChangedPayload: only the id of the built-in agent (26 9).
type hostingPayload struct {
	AgentID string `json:"agent_id"`
}

// emitHostingChanged appends the durable personal event agent.hosting_changed about the built-in
// agent to each runner of hosts that is not zero, once per runner (26 9: the old and the new
// host). The resource is the receiving runner itself, so its receive action agent.hosted.list
// (agent-self) admits exactly that runner; the payload names the agent.
func (s *Service) emitHostingChanged(ctx context.Context, agent ids.ID, hosts ...ids.ID) error {
	payload, err := json.Marshal(hostingPayload{AgentID: agent.String()})
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "agents: agent.hosting_changed payload")
	}
	kind, by := eventActor(ctx)
	var done []ids.ID
	for _, h := range hosts {
		if h.IsZero() || containsID(done, h) {
			continue
		}
		done = append(done, h)
		if err := s.d.Events.Append(ctx, eventlog.Event{
			Type: eventHostingChanged, SchemaVersion: 1, ActorKind: kind, ActorPrincipalID: by,
			ResourceKind: resourceAgent, ResourceID: h, AudiencePrincipalID: h, ReceiveAction: receiveHostedList, Payload: payload,
		}); err != nil {
			return err
		}
	}
	return nil
}

// containsID reports whether list holds id.
func containsID(list []ids.ID, id ids.ID) bool {
	for _, x := range list {
		if x == id {
			return true
		}
	}
	return false
}

// checkCeiling refuses the tool class class for the built-in agent rec above the ceiling its
// runner reports (26 3.1 item 3); other agents have none. It reads the runner's stored heartbeat.
func (s *Service) checkCeiling(ctx context.Context, rec domain.Agent, class domain.ToolClass) error {
	if rec.HostedBy.IsZero() {
		return nil
	}
	host, _, err := s.d.Store.Agent(ctx, rec.HostedBy)
	if err != nil {
		return err
	}
	return domain.CheckCeiling(class, host.Reported.HostsAgents)
}

// checkHostedBilling holds 10 section 10.3 item 2 for the stored rows of list inside the unit of
// work that changed them, after the change (BR-14): a personal built-in agent keeps its runner's
// holder, which locks the runner row, so a concurrent change of the runner's own mode waits and
// then reads this one; and a runner's own mode and holder keep those of its personal hosted
// agents. One function for agent.billing_mode.set, its set_all form and the guided switch,
// agent.credential_holder.set and agent.host.set (design note 6 of the brief A1-03). Errors:
// conflict with hosted_holder_mismatch; store errors.
func (s *Service) checkHostedBilling(ctx context.Context, list ...ids.ID) error {
	for _, id := range list {
		rec, found, err := s.d.Store.Agent(ctx, id)
		if err != nil || !found {
			return err
		}
		if err := s.checkHostedSide(ctx, rec); err != nil {
			return err
		}
		if err := s.checkRunnerSide(ctx, rec); err != nil {
			return err
		}
	}
	return nil
}

// checkHostedSide holds BR-14 for rec as a personal built-in agent, under the runner's row lock.
func (s *Service) checkHostedSide(ctx context.Context, rec domain.Agent) error {
	if rec.HostedBy.IsZero() || rec.Policy.Mode != domain.Personal {
		return nil
	}
	host, _, err := s.d.Store.LockAgent(ctx, rec.HostedBy)
	if err != nil {
		return err
	}
	return domain.CheckHostedHolder(rec.Policy, host)
}

// checkRunnerSide holds BR-14 for rec as a runner with personal hosted agents.
func (s *Service) checkRunnerSide(ctx context.Context, rec domain.Agent) error {
	if rec.AdapterKind != domain.AdapterCodeRunner {
		return nil
	}
	hosted, err := s.d.Store.HostedAgentsOf(ctx, rec.ID)
	if err != nil {
		return err
	}
	return domain.CheckRunnerHolders(rec.Policy, hosted)
}

// SetHost implements agents.Hosting (26 3.1 item 4, agent.host.set): the frame of the single
// changes with If-Match. The check refuses an agent that is not built in, takes the same runner as
// a call without effect, and checks the runner (host_not_capable) and its ceiling; the write
// unit, declared as a rights change because tokens end in it (docs/decisions.d/A1-03.md
// D-A1-03-3), locks the new runner, checks again, moves the agent, revokes every token of the
// agent that the old runner holds, holds the billing rule and writes agent.updated and
// agent.hosting_changed to the old and the new runner.
func (s *Service) SetHost(ctx context.Context, a actor.Actor, id ids.ID, cmd agents.HostCmd) (agents.Agent, error) {
	c := change{
		act: authorize.ActionAgentHostSet, reason: cmd.Reason, match: cmd.IfMatch, limitReason: true,
		decide: func(ctx context.Context, rec domain.Agent) (bool, error) {
			if rec.AdapterKind != domain.AdapterClaudeAgent {
				return false, domain.NotABuiltinAgent()
			}
			if cmd.HostedBy == rec.HostedBy {
				return false, nil
			}
			host, err := s.capableHost(ctx, rec.ID, cmd.HostedBy, false)
			if err != nil {
				return false, err
			}
			return true, domain.CheckCeiling(rec.ToolClass, host.Reported.HostsAgents)
		},
	}
	c.write = func(ctx context.Context, rec domain.Agent) (agents.Agent, error) {
		return s.moveHost(ctx, a, rec, cmd)
	}
	return s.run(ctx, a, id, c)
}

// moveHost is the write of SetHost under the agent's row lock: the new runner under its row lock
// and its ceiling again, the move, the revocation of the old runner's tokens of the agent, the
// allow entry and agent.updated, the billing rule and agent.hosting_changed to both runners.
func (s *Service) moveHost(ctx context.Context, a actor.Actor, rec domain.Agent, cmd agents.HostCmd) (agents.Agent, error) {
	host, err := s.capableHost(ctx, rec.ID, cmd.HostedBy, true)
	if err != nil {
		return agents.Agent{}, err
	}
	if err := domain.CheckCeiling(rec.ToolClass, host.Reported.HostsAgents); err != nil {
		return agents.Agent{}, err
	}
	from := rec.HostedBy
	rec.SetHost(cmd.HostedBy)
	revoked, err := s.d.Hosted.RevokeHosted(ctx, rec.ID, from, a.ID)
	if err != nil {
		return agents.Agent{}, err
	}
	out, err := s.changedAndEmit(ctx, authorize.ActionAgentHostSet, rec, auditing.Attrs{
		attrFromHostedBy: auditing.ID(from), attrToHostedBy: auditing.ID(cmd.HostedBy),
		attrRevokedTokens: auditing.Int(int64(revoked)), attrReason: auditing.ReasonText(cmd.Reason),
	})
	if err != nil {
		return agents.Agent{}, err
	}
	if err := s.checkHostedBilling(ctx, rec.ID); err != nil {
		return agents.Agent{}, err
	}
	return out, s.emitHostingChanged(ctx, rec.ID, from, cmd.HostedBy)
}

// hostedList is the list name of the cursors of listHostedAgents.
const hostedList = "hosted_agents"

// ListHosted implements agents.Hosting (26 3.2, agent.hosted.list): agent-self on the caller,
// then the agents whose hosted_by is the caller in ascending id, the inactive ones left out. For
// any agent that hosts nothing the list is empty; it does not ask for hosts_agents, so a runner
// may read it before its first heartbeat is stored (docs/decisions.d/A1-03.md D-A1-03-7).
func (s *Service) ListHosted(ctx context.Context, a actor.Actor, p agents.Page) (agents.AgentPage, error) {
	if err := s.check(ctx, a, authorize.ActionAgentHostedList, agentResource(a.ID)); err != nil {
		return agents.AgentPage{}, err
	}
	binding := []byte(a.ID.String() + "\x00" + hostedList)
	var after ids.ID
	if p.Cursor != "" {
		var k agentsCursor
		if err := s.d.Cursors.Decode(hostedList, binding, p.Cursor, &k); err != nil {
			return agents.AgentPage{}, err
		}
		if k.ID.IsZero() {
			return agents.AgentPage{}, errs.New(errs.CodeCursorInvalid, "")
		}
		after = k.ID
	}
	limit := p.Limit
	if limit < 1 {
		limit = s.cfg.PageSizeDefault
	}
	limit = min(limit, s.cfg.PageSizeMax)
	active, err := s.activeHosted(ctx, a.ID, after, limit+1)
	if err != nil {
		return agents.AgentPage{}, err
	}
	out, err := s.page(ctx, active, limit)
	if err != nil || len(active) <= limit {
		return out, err
	}
	if out.NextCursor, err = s.d.Cursors.Encode(hostedList, binding, agentsCursor{ID: active[limit-1].ID}); err != nil {
		return agents.AgentPage{}, errs.Wrap(errs.CodeInternalError, err, "agents: hosted list cursor")
	}
	return out, nil
}

// activeHosted reads the agents host hosts after the keyset position in batches of want and keeps
// the active ones, until want are found or the list ends.
func (s *Service) activeHosted(ctx context.Context, host, after ids.ID, want int) ([]domain.Agent, error) {
	var out []domain.Agent
	for len(out) < want {
		batch, err := s.d.Store.ListHostedAgents(ctx, host, after, want)
		if err != nil {
			return nil, err
		}
		list := make([]ids.ID, 0, len(batch))
		for _, rec := range batch {
			list = append(list, rec.ID)
		}
		principals, err := s.d.Directory.Principals(ctx, list)
		if err != nil {
			return nil, err
		}
		for _, rec := range batch {
			if p, ok := principals[rec.ID]; ok && p.Status == identity.StatusActive && len(out) < want {
				out = append(out, rec)
			}
		}
		if len(batch) < want {
			break
		}
		after = batch[len(batch)-1].ID
	}
	return out, nil
}

// IssueHostedToken implements agents.Hosting (26 3.3 item 1, agent.hosted_token.issue). A first
// unit reads the target and authorizes the caller on it as a resource owned by the target's
// hosted_by, so AgentSelf admits only that runner (and the target itself, which the checks below
// refuse); then the target must be a built-in agent hosted by the caller and active, and the
// caller an active runner whose stored heartbeat reports hosts_agents. Every refusal is the 404
// of an unknown agent, a refusal after authorize audited as a denial (07 2.13 item 3). A second
// unit, declared as a rights change because older tokens end in it (D-A1-03-3), locks the target,
// checks the host again, lets identity revoke the runner's older tokens of the agent and insert
// the new one bound to the key of the calling token, and audits the issue with the token id,
// never the value.
func (s *Service) IssueHostedToken(ctx context.Context, a actor.Actor, id ids.ID) (agents.HostedToken, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return agents.HostedToken{}, err
	}
	if s.cfg.HostedTokenTTL <= 0 {
		return agents.HostedToken{}, errs.New(errs.CodeInternalError, "agents: agents.hosted_token_ttl is not bound")
	}
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		rec, found, err := s.d.Store.Agent(ctx, id)
		if err != nil {
			return err
		}
		res := authorize.Resource{Kind: authorize.ResourceAgent, ID: id, OwnerID: rec.HostedBy}
		if err := s.check(ctx, a, authorize.ActionAgentHostedTokenIssue, res); err != nil {
			if errs.KindOf(err) == errs.KindForbidden {
				// authz audited the denial; the answer names nothing about the agent.
				return invisible()
			}
			return err
		}
		return s.hostedIssuer(ctx, a, rec, found)
	})
	if err != nil {
		return agents.HostedToken{}, err
	}
	var out identity.IssuedToken
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		rec, found, err := s.d.Store.LockAgent(ctx, id)
		if err != nil {
			return err
		}
		if err := s.hostedIssuer(ctx, a, rec, found); err != nil {
			return err
		}
		out, err = s.issueHosted(ctx, a, id)
		return err
	}, txn.DeclareRightsChange(txn.RightsChangeSpec{
		Action: string(authorize.ActionAgentHostedTokenIssue), Resource: agentRef(id), Principals: []ids.ID{id},
	}))
	if err != nil {
		return agents.HostedToken{}, err
	}
	return hostedToken(out), nil
}

// hostedToken converts identity's issued token to the agents port type.
func hostedToken(it identity.IssuedToken) agents.HostedToken {
	t := it.Token
	return agents.HostedToken{
		Token: agents.HostedTokenInfo{
			ID: t.ID, PrincipalID: t.PrincipalID, Kind: string(t.Kind), Name: t.Name, TokenPrefix: t.TokenPrefix,
			DPoPJKT: t.DPoPJKT, ExpiresAt: t.ExpiresAt, CreatedBy: t.CreatedBy, CreatedAt: t.CreatedAt,
		},
		Secret: agents.NewSecret(it.Secret.Reveal()),
	}
}

// hostedIssuer checks that the caller a may receive a token of rec (found false for an unknown
// id): rec is a built-in agent hosted by a and active, and a is an active runner whose stored
// heartbeat reports hosts_agents. A refusal is recorded as a denial and answered not_found.
func (s *Service) hostedIssuer(ctx context.Context, a actor.Actor, rec domain.Agent, found bool) error {
	reason := auditing.Reason("")
	switch {
	case !found || rec.AdapterKind != domain.AdapterClaudeAgent || rec.HostedBy != a.ID:
		reason = auditing.ReasonNotSelf
	default:
		target, err := s.d.Directory.Principal(ctx, rec.ID)
		if err != nil {
			return err
		}
		if target.Status != identity.StatusActive {
			reason = auditing.ReasonPrincipalInactive
			break
		}
		if _, err := s.capableHost(ctx, rec.ID, a.ID, false); err != nil {
			if errs.KindOf(err) != errs.KindBadRequest {
				return err
			}
			reason = auditing.ReasonCapabilityMissing
		}
	}
	if reason == "" {
		return nil
	}
	if err := s.d.Audit.RecordDenial(ctx, auditing.Entry{
		Action: string(authorize.ActionAgentHostedTokenIssue), Decision: auditing.DecisionDeny, Reason: reason,
		ResourceKind: resourceAgent, ResourceID: rec.ID,
	}); err != nil {
		return err
	}
	return invisible()
}

// issueHosted is the write of IssueHostedToken inside its rights change: the token through
// identity and the allow entry with the agent, the runner, the token id, the expiry and the number
// of older tokens revoked.
func (s *Service) issueHosted(ctx context.Context, a actor.Actor, id ids.ID) (identity.IssuedToken, error) {
	runner, err := s.d.Directory.Principal(ctx, a.ID)
	if err != nil {
		return identity.IssuedToken{}, err
	}
	res, err := s.d.Hosted.IssueHosted(ctx, identity.HostedTokenCmd{
		HolderID: id, IssuerID: a.ID, IssuerTokenID: a.TokenID, Name: "hosted by " + runner.Handle,
		ExpiresAt: s.d.Clock.Now().Add(s.cfg.HostedTokenTTL),
	})
	if err != nil {
		return identity.IssuedToken{}, err
	}
	t := res.Issued.Token
	return res.Issued, s.record(ctx, authorize.ActionAgentHostedTokenIssue, id, auditing.Attrs{
		attrHostedBy: auditing.ID(a.ID), attrTokenID: auditing.ID(t.ID), attrExpiresAt: auditing.Time(t.ExpiresAt),
		attrRevokedTokens: auditing.Int(int64(res.Revoked)),
	})
}

// IsHostedAgent implements identity.HostedAgents: whether principalID is an agent with the
// adapter kind claude_agent. createToken refuses a token for it (26 3.3 item 3).
func (s *Service) IsHostedAgent(ctx context.Context, principalID ids.ID) (bool, error) {
	rec, found, err := s.d.Store.Agent(ctx, principalID)
	if err != nil {
		return false, err
	}
	return found && rec.AdapterKind == domain.AdapterClaudeAgent, nil
}
