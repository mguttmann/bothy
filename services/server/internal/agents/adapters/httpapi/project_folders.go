// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ListAgentProjectFolders implements rest.StrictServerInterface (GET
// /agents/{agent_id}/project-folders, M4-44): the folders the runner reported without a binding,
// from agents.Presence, which authorizes agent.project_folders.read. Errors pass to the problem
// writer.
func (h *Agents) ListAgentProjectFolders(ctx context.Context, req rest.ListAgentProjectFoldersRequestObject) (rest.ListAgentProjectFoldersResponseObject, error) {
	if h.presence == nil {
		return nil, errs.New(errs.CodeInternalError, "agents: no presence bound")
	}
	f, err := h.presence.ProjectFolders(ctx, principal(ctx), ids.ID(req.AgentId))
	if err != nil {
		return nil, err
	}
	out := rest.ProjectFolderList{Items: make([]rest.ProjectFolder, 0, len(f.Names)), Reported: f.Reported}
	for _, n := range f.Names {
		out.Items = append(out.Items, rest.ProjectFolder{Name: n})
	}
	return rest.ListAgentProjectFolders200JSONResponse{Body: out}, nil
}
