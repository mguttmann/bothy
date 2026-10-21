// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"encoding/json"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/runs"
)

// PatchRun implements rest.StrictServerInterface (PATCH /runs/{run_id}, patchRun, M2-10): the
// live-box patch of the run's own agent through runs.Runs.Patch, the same use case as the gateway
// op run.update. The ops take their values from the request body as received
// (httpserver.RawJSONBody), because the generated type decodes a JSON null and an absent value
// both to nil and numbers to float64; the checks of runs need the difference. A 409
// base_version_conflict carries the run document in current exactly as getRun answers it.
func (h *Runs) PatchRun(ctx context.Context, req rest.PatchRunRequestObject) (rest.PatchRunResponseObject, error) {
	if req.Body == nil {
		return nil, missingBody()
	}
	ops, err := patchOps(ctx, req.Body)
	if err != nil {
		return nil, err
	}
	res, err := h.runs.Patch(ctx, runs.PatchRun{RunID: ids.ID(req.RunId), BaseVersion: req.Body.BaseVersion, Ops: ops})
	if err != nil {
		return nil, conflictDocument(err)
	}
	return rest.PatchRun200JSONResponse{Body: rest.RunPatchResult{Version: res.Version}}, nil
}

// rawPatch is the body RunPatch with the values of the ops as raw JSON.
type rawPatch struct {
	Ops []struct {
		Op    string          `json:"op"`
		Path  string          `json:"path"`
		Value json.RawMessage `json:"value"`
	} `json:"ops"`
}

// patchOps returns the ops of the body with their raw values; outside the chain (no raw body) it
// encodes the decoded values again.
func patchOps(ctx context.Context, body *rest.RunPatch) ([]runs.PatchOp, error) {
	if raw, ok := httpserver.RawJSONBody(ctx); ok {
		var p rawPatch
		if err := json.Unmarshal(raw, &p); err == nil && len(p.Ops) == len(body.Ops) {
			out := make([]runs.PatchOp, 0, len(p.Ops))
			for _, o := range p.Ops {
				out = append(out, runs.PatchOp{Op: o.Op, Path: o.Path, Value: o.Value})
			}
			return out, nil
		}
	}
	return PatchOps(body.Ops)
}

// PatchOps converts the generated operations of RunPatch and GatewayRunUpdate into the port's
// ops, the value encoded again (absent stays absent). Errors: internal when a value cannot be
// encoded, which a decoded JSON value always can.
func PatchOps(in []rest.JsonPatchOperation) ([]runs.PatchOp, error) {
	out := make([]runs.PatchOp, 0, len(in))
	for _, o := range in {
		op := runs.PatchOp{Op: string(o.Op), Path: o.Path}
		if o.Value != nil {
			b, err := json.Marshal(*o.Value)
			if err != nil {
				return nil, errs.Wrap(errs.CodeInternalError, err, "runs: encoding a patch value")
			}
			op.Value = b
		}
		out = append(out, op)
	}
	return out, nil
}

// conflictDocument replaces the run document in current of a base_version_conflict with its
// REST form, the body getRun answers (04 section 11.2; 06 "Errors" point 3). Every other error
// passes unchanged.
func conflictDocument(err error) error {
	e, ok := errs.As(err)
	if !ok || e.Code != errs.CodeBaseVersionConflict {
		return err
	}
	doc, ok := e.Current.(runs.Run)
	if !ok {
		return err
	}
	body, cerr := RunREST(doc)
	if cerr != nil {
		return cerr
	}
	e.Current = body
	return e
}
