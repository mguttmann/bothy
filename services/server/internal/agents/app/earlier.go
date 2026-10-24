// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/agents/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// Compile-time check: SessionService is the port of the importer (M8-06).
var _ agents.ImportedSessions = (*SessionService)(nil)

// CreateEarlier implements agents.ImportedSessions (M8-06 design note 2) in the caller's unit of
// work: the thread and its operator, the checks of domain.CheckEarlier, the outcome of
// domain.EarlierOutcome over the rows of the thread, then the insert of the earlier row. The row
// is history and no change of the current session, so no audit entry and no
// agent_session.changed follow; the binding of the first trigger (ForInvocation) audits.
func (s *SessionService) CreateEarlier(ctx context.Context, e agents.EarlierSession) (string, error) {
	if !txn.InUnit(ctx) {
		return "", errs.New(errs.CodeInternalError, "agents: CreateEarlier outside a unit of work")
	}
	info, err := s.thread(ctx, e.ThreadID)
	if err != nil {
		return "", err
	}
	in := domain.Earlier{ThreadID: info.ID, AgentID: e.AgentID, ExternalSessionRef: e.ExternalSessionRef,
		CostBaselineMicros: e.CostBaselineMicros, Currency: e.Currency}
	if err := domain.CheckEarlier(in, info.OperatorAgentID, s.cfg.SessionRefMaxBytes); err != nil {
		return "", err
	}
	rows, err := s.d.Store.ThreadSessions(ctx, info.ID)
	if err != nil {
		return "", err
	}
	history := make([]domain.Session, 0, len(rows))
	for _, r := range rows {
		history = append(history, toDomain(r))
	}
	switch domain.EarlierOutcome(history, e.AgentID, e.ExternalSessionRef) {
	case agents.EarlierDroppedLate:
		return agents.EarlierDroppedLate, nil
	case agents.EarlierAlreadyPresent:
		return agents.EarlierAlreadyPresent, nil
	}
	now := s.now()
	_, inserted, err := s.d.Store.InsertSession(ctx, NewSession{
		ID: ids.New(), ThreadID: info.ID, AgentID: e.AgentID, ExternalSessionRef: e.ExternalSessionRef, Started: true,
		CostBaselineMicros: e.CostBaselineMicros, Currency: e.Currency, CreatedAt: now, EndedAt: now, EndReason: domain.EndImport,
	})
	if err != nil {
		return "", err
	}
	if !inserted {
		// An ended row conflicts with neither partial unique index, so a refusal is a defect.
		return "", errs.New(errs.CodeInternalError, "agents: the earlier session row was refused")
	}
	return agents.EarlierTakenOver, nil
}
