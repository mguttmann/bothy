// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// ComputerRunFacts are the facts of a run that the guard rule SR13 of computer use reads (07,
// docs/spec/27-computer-use.md 5.1 step 5, CU-04): the run, its thread and agent, the triggering
// person, whether it is active, the tool class and handle of its agent, the trigger with the
// origin and author of a triggering message, the triggering command, and every row of run_feeders.
type ComputerRunFacts struct {
	RunID, ThreadID, AgentID, TriggeredBy ids.ID
	Active                                bool
	ToolClass, AgentHandle                string
	TriggerKind, TriggerOrigin            string
	TriggerAuthorID                       ids.ID
	TriggeringCommand                     bool
	Feeders                               []Feeder
}

// ComputerFacts reads the facts of runID for SR13 (the read port terminal binds through
// internal/wire; CU-04 Design note 3), in the ambient transaction of ctx when there is one. found
// is false for an unknown run. A message run whose triggering message cannot be read keeps an
// empty origin, which SR13 refuses. Errors: those of the repository, the messages and the agents.
//
// Example:
//
//	f, found, err := runsSvc.ComputerFacts(ctx, runID)
func (s *Service) ComputerFacts(ctx context.Context, runID ids.ID) (ComputerRunFacts, bool, error) {
	row, found, err := s.d.Repo.Run(ctx, runID)
	if err != nil || !found {
		return ComputerRunFacts{}, found, err
	}
	f := ComputerRunFacts{RunID: row.ID, ThreadID: row.ThreadID, AgentID: row.AgentID, TriggeredBy: row.TriggeredBy, Active: row.State.Active(),
		TriggerKind: row.TriggerKind, TriggeringCommand: !row.TriggeringCommandID.IsZero()}
	if row.TriggerKind == string(domain.TriggerMessage) && !row.TriggeringMessageID.IsZero() {
		m, err := s.d.Messages.TriggerMessageOf(ctx, row.TriggeringMessageID)
		if err != nil {
			return ComputerRunFacts{}, false, err
		}
		if !m.Deleted {
			f.TriggerOrigin, f.TriggerAuthorID = m.Origin, m.Author.ID
		}
	}
	facts, err := s.d.Agents.RunFacts(ctx, []ids.ID{row.AgentID})
	if err != nil {
		return ComputerRunFacts{}, false, err
	}
	if a, ok := facts[row.AgentID]; ok {
		f.ToolClass, f.AgentHandle = a.ToolClass, a.Handle
	}
	if f.Feeders, err = s.d.Repo.Feeders(ctx, row.ID); err != nil {
		return ComputerRunFacts{}, false, err
	}
	return f, true, nil
}
