// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// GetAgentTriggerers implements rest.StrictServerInterface (GET /agents/me/triggerers): the
// allow list of the calling agent's adapter (09 17.7) from agents.Triggerers, which authorizes
// agent.triggerers.read. Errors pass to the problem writer.
func (h *Agents) GetAgentTriggerers(ctx context.Context, _ rest.GetAgentTriggerersRequestObject) (rest.GetAgentTriggerersResponseObject, error) {
	r, ok := h.agents.(agents.Triggerers)
	if !ok {
		return nil, errs.New(errs.CodeInternalError, "agents: no trigger set bound")
	}
	set, err := r.Triggerers(ctx, principal(ctx))
	if err != nil {
		return nil, err
	}
	out := rest.AgentTriggerers{PrincipalIds: make([]rest.Uuid, 0, len(set.PrincipalIDs)), PermissionVersion: set.PermissionVersion}
	for _, id := range set.PrincipalIDs {
		out.PrincipalIds = append(out.PrincipalIds, rest.Uuid(id))
	}
	return rest.GetAgentTriggerers200JSONResponse{Body: out}, nil
}
