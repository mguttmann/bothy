// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/runs"
)

// CreateInteraction implements rest.StrictServerInterface (POST
// /components/{component_id}/interactions, createInteraction, M2-15): 201 with the interaction,
// 200 with Idempotent-Replay for a repetition of client_interaction_id (04 5.1; no
// Idempotency-Key, x-idempotency-exempt client_id).
func (h *Runs) CreateInteraction(ctx context.Context, req rest.CreateInteractionRequestObject) (rest.CreateInteractionResponseObject, error) {
	if req.Body == nil {
		return nil, missingBody()
	}
	cmd := runs.CreateInteraction{ComponentID: ids.ID(req.ComponentId), ClientInteractionID: ids.ID(req.Body.ClientInteractionId)}
	if req.Body.Values != nil {
		v, err := json.Marshal(*req.Body.Values)
		if err != nil {
			return nil, errs.Wrap(errs.CodeInternalError, err, "runs: encoding the values of an interaction")
		}
		cmd.Values = v
	}
	res, err := h.interactions.Create(ctx, cmd)
	if err != nil {
		return nil, err
	}
	body := InteractionREST(res.Interaction)
	if res.Replay {
		return rest.CreateInteraction200JSONResponse{Body: body, Headers: rest.CreateInteraction200ResponseHeaders{IdempotentReplay: replay(true)}}, nil
	}
	return rest.CreateInteraction201JSONResponse{Body: body}, nil
}

// RespondToInteraction implements rest.StrictServerInterface (POST
// /interactions/{interaction_id}/respond, respondToInteraction, M2-15): 200 with the interaction
// and its response; the Idempotency-Key stage of the chain answers a repetition.
func (h *Runs) RespondToInteraction(ctx context.Context, req rest.RespondToInteractionRequestObject) (rest.RespondToInteractionResponseObject, error) {
	if req.Body == nil {
		return nil, missingBody()
	}
	cmd := runs.RespondInteraction{InteractionID: ids.ID(req.InteractionId), State: string(req.Body.State)}
	if req.Body.Response != nil {
		cmd.HasResponse, cmd.ResponseText = true, req.Body.Response.Text
	}
	ix, err := h.interactions.Respond(ctx, cmd)
	if err != nil {
		return nil, err
	}
	return rest.RespondToInteraction200JSONResponse{Body: InteractionREST(ix)}, nil
}

// UpdateComponent implements rest.StrictServerInterface (PATCH /components/{component_id},
// updateComponent, M2-15): 200 with the component and its ETag; 428 for a missing If-Match is
// the chain's; the Idempotency-Key stage answers a repetition.
func (h *Runs) UpdateComponent(ctx context.Context, req rest.UpdateComponentRequestObject) (rest.UpdateComponentResponseObject, error) {
	if req.Body == nil {
		return nil, missingBody()
	}
	cmd := runs.UpdateComponent{ComponentID: ids.ID(req.ComponentId), IfMatch: ifMatch(req.Params.IfMatch)}
	if req.Body.State != nil {
		st := string(*req.Body.State)
		cmd.State = &st
	}
	if req.Body.Definition != nil {
		d, err := json.Marshal(*req.Body.Definition)
		if err != nil {
			return nil, errs.Wrap(errs.CodeInternalError, err, "runs: encoding a component definition")
		}
		cmd.Definition = d
	}
	c, err := h.interactions.UpdateComponent(ctx, cmd)
	if err != nil {
		return nil, err
	}
	tag := httpserver.FormatETag(c.Version)
	return rest.UpdateComponent200JSONResponse{Body: ComponentREST(c), Headers: rest.UpdateComponent200ResponseHeaders{ETag: &tag}}, nil
}

// ifMatch returns the version of an If-Match value; a value that is not a version gives 0, which
// matches no component, so the use case answers 412 (the chain refused a missing one with 428).
func ifMatch(v string) int64 {
	h := http.Header{}
	h.Set("If-Match", v)
	im := httpserver.ParseIfMatch(h)
	if !im.Valid {
		return 0
	}
	return im.Version
}

// InteractionREST converts an interaction (schema Interaction): empty values become absent
// members, never null (ADR 0053); values is always an object.
func InteractionREST(v runs.Interaction) rest.Interaction {
	values := rest.JsonObject{}
	if len(v.Values) > 0 {
		_ = json.Unmarshal(v.Values, &values)
	}
	out := rest.Interaction{
		Id: rest.Uuid(v.ID), ComponentId: rest.Uuid(v.ComponentID), MessageId: rest.Uuid(v.MessageID), PrincipalId: rest.Uuid(v.PrincipalID),
		ClientInteractionId: rest.Uuid(v.ClientInteractionID), Values: values, State: rest.InteractionState(v.State),
		ThreadId: uuidOrNil(v.ThreadID), RunId: uuidOrNil(v.RunID), CreatedAt: v.CreatedAt.UTC().Truncate(time.Millisecond),
	}
	if v.CustomID != "" {
		c := v.CustomID
		out.CustomId = &c
	}
	if v.HasResponse {
		out.Response = &rest.InteractionResponse{Text: v.ResponseText}
	}
	return out
}

// ComponentREST converts a component (schema Component); a definition that does not decode as
// an object becomes the empty object, which never happens for a stored row.
func ComponentREST(c runs.Component) rest.Component {
	def := rest.JsonObject{}
	if len(c.Definition) > 0 {
		_ = json.Unmarshal(c.Definition, &def)
	}
	out := rest.Component{
		Id: rest.Uuid(c.ID), CustomId: c.CustomID, Type: rest.ComponentType(c.Type), Definition: def, Position: c.Position,
		SingleUse: c.SingleUse, RestrictedTo: rest.ComponentRestriction(c.RestrictedTo), RunId: uuidOrNil(c.RunID),
		State: rest.ComponentState(c.State), ExpiresAt: timeOrNil(c.ExpiresAt), UsedBy: uuidOrNil(c.UsedBy), UsedAt: timeOrNil(c.UsedAt),
		Version: c.Version,
	}
	if c.RequiredCapability != "" {
		rc := rest.Capability(c.RequiredCapability)
		out.RequiredCapability = &rc
	}
	return out
}

// uuidOrNil converts an optional id; the zero ID is absent.
func uuidOrNil(id ids.ID) *rest.Uuid {
	if id.IsZero() {
		return nil
	}
	u := rest.Uuid(id)
	return &u
}

// timeOrNil converts an optional time in milliseconds; the zero time is absent.
func timeOrNil(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	v := t.UTC().Truncate(time.Millisecond)
	return &v
}
