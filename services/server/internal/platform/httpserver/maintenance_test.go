// SPDX-License-Identifier: Apache-2.0

package httpserver_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	authzdomain "github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// fakeRuns answers RunningRuns from a map of agent to runs; err fails every call.
type fakeRuns struct {
	runs map[ids.ID][]httpserver.RunningRun
	err  error
}

func (f fakeRuns) Running(_ context.Context, agent ids.ID) ([]httpserver.RunningRun, error) {
	return f.runs[agent], f.err
}

// catalogueWrites is the effect of the catalogue as internal/wire passes it.
func catalogueWrites(action string) bool {
	spec, ok := authzdomain.LookupAction(action)
	return !ok || spec.Effect == authzdomain.EffectWrite
}

// maintenanceStage builds the stage over runs with the meter provider of reader.
func maintenanceStage(t *testing.T, runs httpserver.RunningRuns, writes func(string) bool, reader sdkmetric.Reader) func(http.Handler) http.Handler {
	t.Helper()
	stage, err := httpserver.NewMaintenanceStage(httpserver.MaintenanceConfig{
		RetryAfter: 42 * time.Second, Writes: writes, Runs: runs, Actions: append(authzdomain.ActionIDs(), "thing.create", "thing.read"),
		MeterProvider: sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)),
	})
	if err != nil {
		t.Fatal(err)
	}
	return stage
}

// stageResult is what one request through the stage alone did.
type stageResult struct {
	status     int
	retryAfter string
	code       string
	reached    bool
	exempt     bool
}

// throughStage sends a request of action through stage with actor a and the path values.
func throughStage(stage func(http.Handler) http.Handler, action string, a actor.Actor, path map[string]string) stageResult {
	var res stageResult
	h := stage(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res.reached, res.exempt = true, txn.WritesInMaintenance(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}))
	ctx := httpserver.WithOperation(context.Background(), httpserver.Operation{ID: "op", Action: action})
	r := httptest.NewRequestWithContext(actor.WithActor(ctx, a), http.MethodPost, "/api/v1/x", nil)
	for k, v := range path {
		r.SetPathValue(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	res.status, res.retryAfter = w.Code, w.Header().Get("Retry-After")
	if w.Code == http.StatusServiceUnavailable {
		res.code = string(errs.CodeMaintenance)
	}
	return res
}

// TestMaintenance_TableOverCatalogue (ADR 0065 "Confirmation"): over every action of
// docs/spec/actions.yaml, an action with effect write without an exception gets 503 maintenance
// with Retry-After and never reaches the handler, an action with effect read passes without the
// write exemption, and every exception passes with it when its condition holds: the fixed ones
// for a person, the agent ones for an agent with a running run in the run, thread and channel of
// the path. The rejections are counted with the action and nothing else.
func TestMaintenance_TableOverCatalogue(t *testing.T) {
	t.Parallel()
	agent, run, thread, channel := ids.New(), ids.New(), ids.New(), ids.New()
	runs := fakeRuns{runs: map[ids.ID][]httpserver.RunningRun{agent: {{RunID: run, ThreadID: thread, ChannelID: channel}}}}
	reader := sdkmetric.NewManualReader()
	stage := maintenanceStage(t, runs, catalogueWrites, reader)
	person := actor.Actor{Kind: actor.KindUser, ID: ids.New()}
	bot := actor.Actor{Kind: actor.KindAgent, ID: agent}
	paths := map[string]string{"run_id": run.String(), "thread_id": thread.String()}
	exceptions := httpserver.MaintenanceExceptions()
	rejected := map[string]bool{}
	for _, id := range authzdomain.ActionIDs() {
		spec, _ := authzdomain.LookupAction(id)
		exc, isException := exceptions[id]
		who := person
		if isException && exc != httpserver.ExceptionAlways {
			who = bot
		}
		got := throughStage(stage, id, who, paths)
		switch {
		case spec.Effect == authzdomain.EffectRead:
			if !got.reached || got.exempt {
				t.Errorf("%s (read): reached %v, exempt %v; want the handler without exemption", id, got.reached, got.exempt)
			}
		case isException:
			if !got.reached || !got.exempt {
				t.Errorf("%s (exception): status %d, exempt %v; want the handler with the exemption", id, got.status, got.exempt)
			}
		default:
			rejected[id] = true
			if got.reached || got.status != http.StatusServiceUnavailable || got.retryAfter != "42" {
				t.Errorf("%s (write): status %d, Retry-After %q, reached %v; want 503 maintenance with Retry-After 42", id, got.status, got.retryAfter, got.reached)
			}
		}
	}
	if len(rejected) == 0 {
		t.Fatal("no action was rejected")
	}
	if got := rejectedByAction(t, reader); len(got) != len(rejected) {
		t.Errorf("bothy.maintenance.rejected has %d actions, want %d", len(got), len(rejected))
	} else {
		for id, n := range got {
			if !rejected[id] || n != 1 {
				t.Errorf("bothy.maintenance.rejected{%s} = %d", id, n)
			}
		}
	}
}

// rejectedByAction returns bothy.maintenance.rejected by action; it fails on any other attribute.
func rejectedByAction(t *testing.T, reader sdkmetric.Reader) map[string]int64 {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatal(err)
	}
	out := map[string]int64{}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			sum, ok := m.Data.(metricdata.Sum[int64])
			if m.Name != "bothy.maintenance.rejected" || !ok {
				continue
			}
			for _, dp := range sum.DataPoints {
				if dp.Attributes.Len() != 1 {
					t.Errorf("attributes %v, want only bothy.action", dp.Attributes.ToSlice())
				}
				v, _ := dp.Attributes.Value(attribute.Key("bothy.action"))
				out[v.AsString()] += dp.Value
			}
		}
	}
	return out
}

// TestMaintenance_AgentConditions: the agent exceptions depend on a running run at request time:
// a person, an agent without a running run, another run, thread or channel give 503; the channel
// of createUpload and the file_id routes pass for the agent with a running run; an error of the
// lookup is the answer.
func TestMaintenance_AgentConditions(t *testing.T) {
	t.Parallel()
	agent, idle, run, thread, channel := ids.New(), ids.New(), ids.New(), ids.New(), ids.New()
	runs := fakeRuns{runs: map[ids.ID][]httpserver.RunningRun{agent: {{RunID: run, ThreadID: thread, ChannelID: channel}}}}
	stage := maintenanceStage(t, runs, catalogueWrites, sdkmetric.NewManualReader())
	bot := actor.Actor{Kind: actor.KindAgent, ID: agent}
	cases := []struct {
		name, action string
		who          actor.Actor
		path         map[string]string
		pass         bool
	}{
		{"person posts", "message.create", actor.Actor{Kind: actor.KindUser, ID: ids.New()}, map[string]string{"thread_id": thread.String()}, false},
		{"idle agent posts", "message.create", actor.Actor{Kind: actor.KindAgent, ID: idle}, map[string]string{"thread_id": thread.String()}, false},
		{"other thread", "message.create", bot, map[string]string{"thread_id": ids.New().String()}, false},
		{"run thread", "message.create", bot, map[string]string{"thread_id": thread.String()}, true},
		{"run channel upload", "file.upload", bot, map[string]string{"channel_id": channel.String()}, true},
		{"other channel upload", "file.upload", bot, map[string]string{"channel_id": ids.New().String()}, false},
		{"upload part", "file.upload", bot, map[string]string{"file_id": ids.New().String()}, true},
		{"idle agent upload part", "file.upload", actor.Actor{Kind: actor.KindAgent, ID: idle}, map[string]string{"file_id": ids.New().String()}, false},
		{"own run", "run.finish", bot, map[string]string{"run_id": run.String()}, true},
		{"other run", "run.update", bot, map[string]string{"run_id": ids.New().String()}, false},
		{"bad run id", "run.update", bot, map[string]string{"run_id": "x"}, false},
		{"heartbeat", "agent.heartbeat", bot, nil, true},
		{"idle heartbeat", "agent.heartbeat", actor.Actor{Kind: actor.KindAgent, ID: idle}, nil, false},
		{"start", "run.start", bot, map[string]string{"run_id": run.String()}, false},
		{"anonymous refresh", "auth.token.refresh", actor.Anonymous(), nil, true},
	}
	for _, c := range cases {
		got := throughStage(stage, c.action, c.who, c.path)
		if got.reached != c.pass || (!c.pass && got.status != http.StatusServiceUnavailable) {
			t.Errorf("%s: status %d, reached %v; want pass %v", c.name, got.status, got.reached, c.pass)
		}
	}
	failing := maintenanceStage(t, fakeRuns{err: errs.New(errs.CodeUnavailable, "")}, catalogueWrites, sdkmetric.NewManualReader())
	if got := throughStage(failing, "agent.heartbeat", bot, nil); got.reached || got.status != http.StatusServiceUnavailable || got.code == "" {
		t.Errorf("lookup error: status %d, reached %v", got.status, got.reached)
	}
}

// TestExceptionsAreCatalogueWrites (design note 1): every exception names an action of the
// catalogue whose effect is write; an exception of a read action or of an unknown one would be
// dead code or a typo that silently blocks the action it meant.
func TestExceptionsAreCatalogueWrites(t *testing.T) {
	t.Parallel()
	for id := range httpserver.MaintenanceExceptions() {
		spec, ok := authzdomain.LookupAction(id)
		if !ok {
			t.Errorf("exception %s is not in the catalogue", id)
			continue
		}
		if spec.Effect != authzdomain.EffectWrite {
			t.Errorf("exception %s has effect %s, want write", id, spec.Effect)
		}
	}
}

// TestMaintenance_ChainOrder: in the chain the stage runs after the rate limit (an exhausted class
// answers 429, not 503) and before the handler, where authorize runs; a read passes.
func TestMaintenance_ChainOrder(t *testing.T) {
	t.Parallel()
	limited := true
	create := httpservertest.Op("createThing", http.MethodPost, "/api/v1/things", "thing.create")
	get := httpservertest.Op("getThing", http.MethodGet, "/api/v1/things/{thing_id}", "thing.read")
	var handled int
	stage := maintenanceStage(t, fakeRuns{}, func(a string) bool { return a == "thing.create" }, sdkmetric.NewManualReader())
	h := httpservertest.New(t, httpservertest.Options{
		Stages: map[httpserver.Stage]func(http.Handler) http.Handler{
			httpserver.StageRateLimit: func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if limited {
						httpserver.WriteProblem(w, r, errs.New(errs.CodeRateLimited, "").WithRetryAfter(time.Second))
						return
					}
					next.ServeHTTP(w, r)
				})
			},
			httpserver.StageMaintenance: stage,
		},
		Routes: []httpservertest.Route{
			{Op: create, Handler: httpservertest.Authorized(create.Action, func(w http.ResponseWriter, _ *http.Request) {
				handled++
				w.WriteHeader(http.StatusCreated)
			})},
			{Op: get, Handler: httpservertest.Authorized(get.Action, func(w http.ResponseWriter, _ *http.Request) {
				handled++
				httpservertest.JSON(w, http.StatusOK, map[string]any{"id": thingID})
			})},
		},
	})
	body := `{"name":"Alex"}`
	if resp := h.Do(h.NewRequest(http.MethodPost, "/api/v1/things", body)); resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("exhausted rate limit: status %d, want 429 before maintenance", resp.StatusCode)
	}
	limited = false
	resp := h.Do(h.NewRequest(http.MethodPost, "/api/v1/things", body))
	if resp.StatusCode != http.StatusServiceUnavailable || resp.Header.Get("Retry-After") != "42" {
		t.Fatalf("write: status %d, Retry-After %q", resp.StatusCode, resp.Header.Get("Retry-After"))
	}
	if p := resp.Problem(t); p["code"] != "maintenance" {
		t.Fatalf("write: problem %v", p)
	}
	if handled != 0 {
		t.Fatal("the handler of a blocked write ran")
	}
	if resp := h.Do(h.NewRequest(http.MethodGet, "/api/v1/things/"+thingID, "", "X-Mode", "fast")); resp.StatusCode != http.StatusOK || handled != 1 {
		t.Fatalf("read: status %d, handled %d", resp.StatusCode, handled)
	}
}

// TestNewMaintenanceStage_Refuses: a missing collaborator or a Retry-After that is not positive
// is a start error.
func TestNewMaintenanceStage_Refuses(t *testing.T) {
	t.Parallel()
	for name, cfg := range map[string]httpserver.MaintenanceConfig{
		"no retry":  {Writes: catalogueWrites, Runs: fakeRuns{}},
		"no writes": {RetryAfter: time.Second, Runs: fakeRuns{}},
		"no runs":   {RetryAfter: time.Second, Writes: catalogueWrites},
	} {
		if _, err := httpserver.NewMaintenanceStage(cfg); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}
