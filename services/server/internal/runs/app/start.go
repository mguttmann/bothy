// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// startResult is what one attempt of the unit of work of a start returns: the run and the replay
// flag, or a refusal that commits with the failed run (an I3 mismatch, design note 3 d).
type startResult struct {
	run       runs.Run
	replay    bool
	committed error
	handle    string
}

// Start implements runs.Runs (startRun, run.start): queued -> running with the current invocation
// token (M2-09 design note 3).
func (s *Service) Start(ctx context.Context, cmd runs.StartRun) (runs.StartResult, error) {
	return s.start(ctx, cmd, domain.StartOpStart)
}

// Resume implements runs.Runs (resumeRun, run.start): waiting_input -> running with the token of
// the answer (M2-09 design note 3).
func (s *Service) Resume(ctx context.Context, cmd runs.StartRun) (runs.StartResult, error) {
	return s.start(ctx, cmd, domain.StartOpResume)
}

// start runs one start or resume in one unit of work with the run row locked first: the token is
// verified (signature, kid, typ, alg, iss, aud, run) without its expiry, StartVerdict decides
// currency, expiry and endpoint, authorize decides run.start on the verdict and the mode facts, and
// an allowed transition stores started_jti, the reported mode, the evidence and the model; a start
// (not a resume) then attaches its agent session (M2-12 design note 10). An I3 mismatch fails the
// run in the same unit, which commits, and then answers 409.
func (s *Service) start(ctx context.Context, cmd runs.StartRun, op domain.StartOp) (runs.StartResult, error) {
	v, err := s.verifier()
	if err != nil {
		return runs.StartResult{}, err
	}
	a := caller(ctx)
	var res startResult
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		res, err = s.startTx(ctx, a, cmd, op, v)
		return err
	})
	if err != nil {
		return runs.StartResult{}, err
	}
	if res.committed != nil {
		s.d.Mismatches.BillingModeMismatch(context.WithoutCancel(ctx), res.handle)
		return runs.StartResult{}, res.committed
	}
	return runs.StartResult{Run: res.run, Replay: res.replay}, nil
}

// startTx is start inside its unit of work.
func (s *Service) startTx(ctx context.Context, a actor.Actor, cmd runs.StartRun, op domain.StartOp, v TokenVerifier) (startResult, error) {
	row, found, err := s.d.Repo.LockRun(ctx, cmd.RunID)
	if err != nil {
		return startResult{}, err
	}
	check, claims, err := s.startCheck(ctx, a, cmd, op, row, found, v)
	if err != nil {
		return startResult{}, err
	}
	facts := &authorize.TokenFacts{Verdict: authzVerdict(check.Verdict)}
	if check.ModeFacts {
		facts.BillingModeUsed, facts.BillingModeExpected = cmd.BillingModeUsed, row.BillingModeExpected
		facts.ToolClassUsed, facts.ToolClassClaim = cmd.ToolClassUsed, claims.toolClass
		if ev := cmd.BillingEvidence; ev != nil {
			facts.BillingKeyType, facts.BillingProvider, facts.BillingMethod = ev.KeyType, ev.Provider, ev.Method
		}
	}
	d, err := s.authorizeAgent(ctx, a, authorize.ActionRunStart, row, found, cmd.RunID, facts)
	if err != nil {
		return startResult{}, err
	}
	if !d.Allowed() {
		return s.refuseStart(ctx, row, d)
	}
	//exhaustive:ignore // Every other next step after an allow is an internal error (default, fail closed).
	switch check.Next {
	case domain.NextReplay:
		doc, err := s.documentOf(ctx, row)
		return startResult{run: doc, replay: true}, err
	case domain.NextAlreadyStarted:
		return startResult{}, errs.New(errs.CodeAlreadyStarted, "the run has already started or ended")
	case domain.NextInvalidTransition:
		return startResult{}, errs.New(errs.CodeInvalidStateTransition, "this endpoint does not perform the transition of the run's state")
	case domain.NextTransition:
	default:
		// authorize allowed a verdict that is not valid: never start (fail closed).
		return startResult{}, errs.New(errs.CodeInternalError, "runs: run.start allowed a token that is not valid")
	}
	evidence, err := s.startValues(cmd)
	if err != nil {
		return startResult{}, err
	}
	t, err := s.Transition(ctx, TransitionCmd{
		RunID: row.ID, From: row.State, Version: row.Version, To: domain.StateRunning, SetBy: domain.SetterAdapter,
		Store: TransitionStore{StartedJTI: claims.jti, BillingModeUsed: cmd.BillingModeUsed, Model: cmd.Model, BillingEvidence: evidence},
	})
	if err != nil {
		return startResult{}, err
	}
	if op == domain.StartOpStart {
		// resumeRun keeps the row the run already has (09 9.1).
		if err := s.attachOnStart(ctx, row, cmd); err != nil {
			return startResult{}, err
		}
	}
	doc, err := s.documentByID(ctx, t.Run.ID)
	return startResult{run: doc}, err
}

// startValues validates the model of a start before the transition stores it (agents
// Sessions.CheckStartModel, M2-12 design note 13; without Sessions it is not checked) and returns
// the JSON of the billing evidence.
func (s *Service) startValues(cmd runs.StartRun) ([]byte, error) {
	if s.d.Sessions != nil {
		if err := s.d.Sessions.CheckStartModel(cmd.Model); err != nil {
			return nil, err
		}
	}
	return evidenceJSON(cmd.BillingEvidence)
}

// startCheck verifies the token of a start and returns the verdict of StartVerdict; a token that
// fails verification is VerdictInvalid. An unknown run gets VerdictInvalid, so authorize answers
// not_found for it like for an invisible one.
func (s *Service) startCheck(ctx context.Context, a actor.Actor, cmd runs.StartRun, op domain.StartOp, row RunRow, found bool,
	v TokenVerifier) (domain.StartCheck, tokenClaims, error) {
	invalid := domain.StartCheck{Verdict: domain.VerdictInvalid, Next: domain.NextDeny}
	if !found {
		return invalid, tokenClaims{}, nil
	}
	c, ok, err := s.verifyToken(ctx, a, cmd.Token, row.ID, v)
	if err != nil || !ok {
		return invalid, tokenClaims{}, err
	}
	check := domain.StartVerdict(domain.TokenClaims{JTI: c.jti, ExpiresAt: c.expiresAt}, tokenState(row), op, s.d.Clock.Now(), s.cfg.ClockLeeway)
	return check, c, nil
}

// tokenClaims are the members of a verified token the lifecycle needs.
type tokenClaims struct {
	jti       ids.ID
	expiresAt time.Time
	toolClass string
	claims    domain.InvocationClaims
}

// verifyToken verifies token for the agent a and the run without its expiry (09 4.5, 4.5.1 step 1).
// ok is false for every failed check (verdict invalid); the error is only a failure of the key
// store or the instance row.
func (s *Service) verifyToken(ctx context.Context, a actor.Actor, token string, runID ids.ID, v TokenVerifier) (tokenClaims, bool, error) {
	if token == "" || a.ID.IsZero() {
		return tokenClaims{}, false, nil
	}
	c, err := v.Verify(ctx, token, VerifyInvocation{Audience: a.ID, RunID: runID, IgnoreExpiry: true})
	if fe, ok := errs.As(err); ok && fe.Code == errs.CodeInvocationTokenInvalid {
		return tokenClaims{}, false, nil
	}
	if err != nil {
		return tokenClaims{}, false, err
	}
	jti, err := ids.Parse(c.Jti)
	if err != nil {
		return tokenClaims{}, false, nil
	}
	return tokenClaims{jti: jti, expiresAt: time.Unix(c.Exp, 0).UTC(), toolClass: c.ToolClass, claims: c}, true, nil
}

// tokenState is the token columns of row for the verdicts.
func tokenState(row RunRow) domain.RunTokenState {
	return domain.RunTokenState{State: row.State, InvocationJTI: row.InvocationJTI, StartedJTI: row.StartedJTI, RenewedFromJTI: row.RenewedFromJTI}
}

// authzVerdict converts a verdict of the domain into the one of authorize.
func authzVerdict(v domain.TokenVerdict) authorize.TokenVerdict {
	switch v {
	case domain.VerdictValid:
		return authorize.TokenValid
	case domain.VerdictExpired:
		return authorize.TokenExpired
	case domain.VerdictSuperseded:
		return authorize.TokenSuperseded
	case domain.VerdictInvalid:
		return authorize.TokenInvalid
	case domain.VerdictZero:
	}
	return authorize.TokenVerdictZero
}

// agentResource describes a run for an agent-self action (design note 1): the thread as the path
// hint and the run's agent as owner, plus the token facts of run.start and run.token.renew.
func agentResource(row RunRow, facts *authorize.TokenFacts) authorize.Resource {
	return authorize.Resource{Kind: authorize.ResourceRun, ID: row.ID, ThreadID: row.ThreadID, OwnerID: row.AgentID, Token: facts}
}

// authorizeAgent decides act for the adapter a on the run. An unknown run is decided on a path
// that cannot resolve, so it answers the same not_found as an invisible one (02 section 9.2); the
// answer about the principal comes first (AZ-P4). It returns the decision for the caller to apply
// when it is an allow or an invariant violation; every other denial is returned as its error.
func (s *Service) authorizeAgent(ctx context.Context, a actor.Actor, act authorize.Action, row RunRow, found bool, id ids.ID,
	facts *authorize.TokenFacts) (authorize.Decision, error) {
	res := agentResource(row, facts)
	if !found {
		res = authorize.Resource{Kind: authorize.ResourceRun, ID: id, ThreadID: id, Token: facts}
	}
	d, err := s.d.Authorizer.Authorize(ctx, a, act, res)
	if err != nil {
		return authorize.Decision{}, err
	}
	if d.Outcome == authorize.OutcomeNotFound || (d.Allowed() && !found) {
		return authorize.Decision{}, invisible()
	}
	if !d.Allowed() && d.Outcome != authorize.OutcomeInvariantViolation {
		return authorize.Decision{}, d.Err()
	}
	return d, nil
}

// refuseStart applies an I3 mismatch of run.start (design note 3 d, BR-06, AG-30): the run fails
// with the mismatch as its reason in this unit of work, which commits, and the 409 follows after
// the commit. Any other invariant violation is returned at once.
func (s *Service) refuseStart(ctx context.Context, row RunRow, d authorize.Decision) (startResult, error) {
	reason := string(d.Reason)
	if reason != domain.RunReasonBillingModeMismatch && reason != domain.RunReasonToolClassMismatch {
		return startResult{}, d.Err()
	}
	t, err := s.endRun(ctx, row, domain.StateFailed, reason, domain.SetterServer, TransitionStore{})
	if err != nil {
		return startResult{}, err
	}
	return startResult{committed: d.Err(), handle: t.AgentHandle}, nil
}

// documentOf returns the run document of row with its steps and the channel of its thread now.
func (s *Service) documentOf(ctx context.Context, row RunRow) (runs.Run, error) {
	info, err := s.d.Threads.Info(ctx, row.ThreadID)
	if err != nil {
		return runs.Run{}, err
	}
	steps, err := s.d.Repo.AllSteps(ctx, row.ID)
	if err != nil {
		return runs.Run{}, err
	}
	followups, err := s.fu.Document(ctx, row.ID)
	if err != nil {
		return runs.Run{}, err
	}
	doc := document(row, info.ChannelID, steps)
	doc.Followups = followups
	return doc, nil
}

// documentByID reads the run id again in the unit of work and returns its document.
func (s *Service) documentByID(ctx context.Context, id ids.ID) (runs.Run, error) {
	row, found, err := s.d.Repo.Run(ctx, id)
	if err != nil {
		return runs.Run{}, err
	}
	if !found {
		return runs.Run{}, invisible()
	}
	return s.documentOf(ctx, row)
}

// evidenceJSON encodes billing_evidence.v1 with only the reported members; nil for none.
func evidenceJSON(ev *runs.BillingEvidence) ([]byte, error) {
	if ev == nil {
		return nil, nil
	}
	type evidence struct {
		Provider string `json:"provider,omitempty"`
		KeyType  string `json:"key_type"`
		Method   string `json:"method,omitempty"`
	}
	return encode(evidence{Provider: ev.Provider, KeyType: ev.KeyType, Method: ev.Method})
}
