// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal"
)

// SetHostLlmCredentials implements rest.StrictServerInterface (PUT /hosts/{host_id}/llm-credentials,
// M2-40): the declaration of the LLM login of the host with the mandatory reason; the answer is the
// host with its ETag.
func (h *Hosts) SetHostLlmCredentials(ctx context.Context, req rest.SetHostLlmCredentialsRequestObject) (rest.SetHostLlmCredentialsResponseObject, error) {
	cmd := terminal.SetLlmCredentials{HostID: ids.ID(req.HostId), IfMatch: precondition(req.Params.IfMatch)}
	if b := req.Body; b != nil {
		cmd.LLMCredentials, cmd.Reason = string(b.LlmCredentials), b.Reason
		if b.CredentialHolderId != nil {
			cmd.CredentialHolderID = ids.ID(*b.CredentialHolderId)
		}
	}
	v, err := h.hosts.SetLlmCredentials(ctx, principal(ctx), cmd)
	if err != nil {
		return nil, err
	}
	return rest.SetHostLlmCredentials200JSONResponse{Body: toREST(v), Headers: rest.SetHostLlmCredentials200ResponseHeaders{ETag: etag(v)}}, nil
}
