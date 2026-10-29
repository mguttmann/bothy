// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"slices"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ProjectFolders implements agents.Presence (listAgentProjectFolders, M4-44): it authorizes
// agent.project_folders.read on the agent, reads agents.reported_project_folders and leaves out
// every name a binding names, which chat answers through the readiness port, so this module never
// reads project_bindings (D-M4-44-3, D-M4-44-4).
func (t *Tracker) ProjectFolders(ctx context.Context, a actor.Actor, agentID ids.ID) (agents.ProjectFolders, error) {
	d, err := t.d.Authorizer.Authorize(ctx, a, authorize.ActionAgentProjectFoldersRead, agentResource(agentID))
	if err != nil {
		return agents.ProjectFolders{}, err
	}
	if err := d.Err(); err != nil {
		return agents.ProjectFolders{}, err
	}
	names, reported, found, err := t.d.Store.GetProjectFolders(ctx, agentID)
	if err != nil {
		return agents.ProjectFolders{}, internal(err, "agents: reading the project folders")
	}
	if !found {
		return agents.ProjectFolders{}, errs.New(errs.CodeNotFound, "")
	}
	out := agents.ProjectFolders{Names: []string{}, Reported: reported}
	if len(names) == 0 {
		return out, nil
	}
	_, port := t.bound()
	if port == nil {
		return agents.ProjectFolders{}, errs.New(errs.CodeInternalError, "agents: no readiness port bound")
	}
	bound, err := port.BoundProjectFolders(ctx, names)
	if err != nil {
		return agents.ProjectFolders{}, internal(err, "agents: reading the bound project folders")
	}
	for _, n := range names {
		if !slices.ContainsFunc(bound, func(b string) bool { return strings.EqualFold(b, n) }) {
			out.Names = append(out.Names, n)
		}
	}
	return out, nil
}
