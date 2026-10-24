// SPDX-License-Identifier: Apache-2.0

package httpserver_test

import (
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/measure"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// TestChain_MeasuresAPIAvailability shows sli.api_availability of the own measurement (M6-13): one
// event per REST response that http.server.request.duration records, good without 5xx, bad with
// 5xx, an unmatched path counted like the metric counts it, a panicking request not at all (the
// metric does not record it either), and the gateway route never.
func TestChain_MeasuresAPIAvailability(t *testing.T) {
	t.Parallel()
	rec := &measure.Recording{}
	op := getThing()
	// status is the answer of the handler; 0 panics. The requests run one after the other.
	var status atomic.Int32
	status.Store(http.StatusOK)
	h := httpservertest.New(t, httpservertest.Options{Measure: rec, Routes: []httpservertest.Route{{Op: op,
		Handler: httpservertest.Authorized(op.Action, func(w http.ResponseWriter, _ *http.Request) {
			code := int(status.Load())
			if code == 0 {
				panic("boom")
			}
			httpservertest.JSON(w, code, map[string]any{"ok": true})
		})}}})
	path := "/api/v1/things/" + thingID
	h.Do(h.NewRequest(http.MethodGet, path, "", "X-Mode", "fast"))
	status.Store(http.StatusServiceUnavailable)
	h.Do(h.NewRequest(http.MethodGet, path, "", "X-Mode", "fast"))
	h.Do(h.NewRequest(http.MethodGet, "/api/v1/nothing", ""))
	status.Store(0)
	if resp := h.Do(h.NewRequest(http.MethodGet, path, "", "X-Mode", "fast")); resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("panic answered %d", resp.StatusCode)
	}
	calls := rec.Calls(measure.SLIAPIAvailability)
	var good int
	for _, c := range calls {
		if c.Value.OK {
			good++
		}
	}
	if len(calls) != 3 || good != 2 || len(rec.Invalid()) != 0 {
		t.Fatalf("sli.api_availability: %d events, %d good, invalid %v; want 3 and 2 (200, 404 good, 503 bad, panic not counted)",
			len(calls), good, rec.Invalid())
	}

	v, err := httpserver.NewValidator(httpservertest.Contract, nil)
	if err != nil {
		t.Fatal(err)
	}
	gw := &measure.Recording{}
	c, err := httpserver.NewChain(httpserver.DefaultChainConfig(httpservertest.PublicURL),
		httpserver.Deps{Authenticator: actor.AnonymousOnly{}, Validator: v, Measure: gw})
	if err != nil {
		t.Fatal(err)
	}
	c.MeasureResponse("/api/v1/gateway", http.StatusInternalServerError)
	c.MeasureResponse("/api/v1/threads/{thread_id}", http.StatusInternalServerError)
	if got := gw.Calls(""); len(got) != 1 || got[0].Value.OK {
		t.Fatalf("calls %+v, want one bad event and none for the gateway route", got)
	}
}
