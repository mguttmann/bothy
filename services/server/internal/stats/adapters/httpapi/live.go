// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"net/http"
	"slices"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/stats"
)

// Live implements getStatsLive (M6-15, docs/spec/25-visualization.md 5.1 last paragraph): the
// most recently built base frame of the node filtered for the caller, the same bytes the gateway
// sends as stats.live.
type Live struct {
	live stats.Live
}

// NewLive returns the handler over the port l.
func NewLive(l stats.Live) *Live { return &Live{live: l} }

// GetStatsLive implements rest.StrictServerInterface (GET /stats/live): 200 with the frame;
// errors of the use case (403 feature_disabled, 403 forbidden, 404 not_found) go to the problem
// writer unchanged.
func (h *Live) GetStatsLive(ctx context.Context, req rest.GetStatsLiveRequestObject) (rest.GetStatsLiveResponseObject, error) {
	var q stats.LiveQuery
	if req.Params.SpaceId != nil {
		q.SpaceID = ids.ID(*req.Params.SpaceId)
	}
	f, err := h.live.Current(ctx, q)
	if err != nil {
		return nil, err
	}
	return liveFrame(f.Payload), nil
}

// liveFrame is the 200 answer of getStatsLive written as the encoded frame itself, so the REST
// body and the payload of stats.live are byte for byte the same frame (R-VZ-06).
type liveFrame []byte

// VisitGetStatsLiveResponse implements rest.GetStatsLiveResponseObject like the generated 200
// answer: JSON, status 200, the body followed by a newline.
func (f liveFrame) VisitGetStatsLiveResponse(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err := w.Write(slices.Concat([]byte(f), []byte{'\n'}))
	return err
}
