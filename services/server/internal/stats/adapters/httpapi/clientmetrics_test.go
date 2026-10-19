// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/api/routes"
	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/adapters/fake"
	authzapp "github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics/metricstest"
	"github.com/OWNER/bothy/services/server/internal/stats/adapters/httpapi"
	"github.com/OWNER/bothy/services/server/internal/stats/adapters/otelmetrics"
	"github.com/OWNER/bothy/services/server/internal/stats/app"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// nopRecorder drops audit entries: client_metrics.submit is never audited.
type nopRecorder struct{}

func (nopRecorder) Record(context.Context, auditing.Entry) error         { return nil }
func (nopRecorder) RecordDenial(context.Context, auditing.Entry) error   { return nil }
func (nopRecorder) RecordDetached(context.Context, auditing.Entry) error { return nil }

// liveCredentials reports every session, device and token as alive.
type liveCredentials struct{}

func (liveCredentials) Credentials(_ context.Context, ref authz.CredentialRef) (authz.CredentialFacts, error) {
	return authz.CredentialFacts{
		SessionActive: !ref.SessionID.IsZero(), SessionExpiresAt: time.Now().Add(time.Hour),
		DeviceState: authz.DeviceActive, TokenActive: !ref.TokenID.IsZero(),
	}, nil
}

// settings is the stored privacy.client_metrics per person.
type settings struct {
	mu   sync.Mutex
	vals map[ids.ID]map[string]json.RawMessage
}

func (s *settings) ValuesFor(_ context.Context, userIDs []ids.ID, keys []string) (map[ids.ID]map[string]json.RawMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[ids.ID]map[string]json.RawMessage{}
	for _, id := range userIDs {
		for _, k := range keys {
			if v, ok := s.vals[id][k]; ok {
				if out[id] == nil {
					out[id] = map[string]json.RawMessage{}
				}
				out[id][k] = v
			}
		}
	}
	return out, nil
}

type env struct {
	h        *httpservertest.Harness
	reader   *sdkmetric.ManualReader
	settings *settings
	alex     ids.ID
}

// newEnv builds the full chain with the real Authorizer over fakes, the request validator of the
// contract and the use case over the recorder on a manual reader with its own aggregated set.
func newEnv(t *testing.T) *env {
	t.Helper()
	people := fake.NewPrincipals()
	versions := &fake.Versions{}
	versions.Set(1)
	az, err := authzapp.NewAuthorizer(authzapp.AuthorizerConfig{SnapshotCacheMaxEntries: 100, StepUpMaxAge: 5 * time.Minute}, authzapp.AuthorizerDeps{
		Catalog: authzapp.NewCatalog(slog.New(slog.DiscardHandler)), Store: fake.NewStore(), Principals: people,
		Credentials: liveCredentials{}, Structure: fake.NewStructure(), Policies: fake.NewPolicies(), Versions: versions,
		Recorder: nopRecorder{}, Clock: clock.System(), Log: slog.New(slog.DiscardHandler),
	})
	if err != nil {
		t.Fatal(err)
	}
	set := metrics.NewAggregatedSet()
	reader := sdkmetric.NewManualReader(sdkmetric.WithProducer(set))
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader), sdkmetric.WithView(metrics.View))
	t.Cleanup(func() { _ = mp.Shutdown(context.Background()) })
	rec, err := otelmetrics.New(mp, set, []int{1000, 4009})
	if err != nil {
		t.Fatal(err)
	}
	e := &env{reader: reader, settings: &settings{vals: map[ids.ID]map[string]json.RawMessage{}}, alex: ids.New()}
	svc, err := app.NewClientMetrics(app.ClientMetricsDeps{Authorizer: az, Settings: e.settings, Recorder: rec, CloseCodes: []int{1000, 4009}})
	if err != nil {
		t.Fatal(err)
	}
	built, err := routes.Build(wire.Handlers{ClientMetrics: httpapi.New(svc)}, wire.RouteOptions(), routes.SpecCatalog())
	if err != nil {
		t.Fatal(err)
	}
	var rs []httpservertest.Route
	for _, r := range built {
		if r.Operation.ID == "submitClientMetrics" {
			rs = append(rs, httpservertest.Route{Op: wire.HTTPOperation(r.Operation), Handler: r.Handler.ServeHTTP})
		}
	}
	actors := httpservertest.NewActors()
	actors.Set("alex", httpservertest.InSession(e.alex, ids.New(), ids.New(), time.Time{}))
	people.Put(authz.PrincipalFacts{ID: e.alex, Kind: actor.KindUser, Status: authz.PrincipalActive})
	bot := ids.New()
	actors.Set("bot", httpservertest.WithToken(bot, actor.KindAgent, ids.New()))
	people.Put(authz.PrincipalFacts{ID: bot, Kind: actor.KindAgent, Status: authz.PrincipalActive})
	e.h = httpservertest.New(t, httpservertest.Options{Routes: rs, Contract: rest.Spec, Authenticator: actors, ValidateResponses: true})
	return e
}

// post sends body as token to submitClientMetrics.
func (e *env) post(token, body string) *httpservertest.Response {
	req := e.h.NewRequest(http.MethodPost, "/api/v1/client-metrics", body, "Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return e.h.Do(req)
}

// exampleBody is the request example of submitClientMetrics plus one entry with an unknown name.
const exampleBody = `{"platform":"ios","app_version":"1.4.0","density":"touch","metrics":[
	{"name":"client.thread_open.duration","kind":"histogram","buckets":[{"le":0.1,"count":12},{"le":0.25,"count":3}]},
	{"name":"client.reconnects","kind":"counter","close_code":4009,"value":1},
	{"name":"client.screen_views","kind":"counter","value":7}]}`

// TestSubmitClientMetrics_Accepted: 202 for a batch with an unknown entry; the known entries are
// exported under bothy.client.*, the unknown one is counted (brief M4-31 Tests).
func TestSubmitClientMetrics_Accepted(t *testing.T) {
	metricstest.Strict(t)
	e := newEnv(t)
	resp := e.post("alex", exampleBody)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status %d, body %s", resp.StatusCode, resp.Body)
	}
	got := metricstest.Collect(t, e.reader)
	if p := metricstest.Points(got["bothy.client.thread_open.duration"]); len(p) != 1 || p[0].Count != 15 {
		t.Errorf("thread open %+v", p)
	}
	if p := metricstest.Points(got["bothy.client.reconnects"]); len(p) != 1 || p[0].Attrs["bothy.close.code"] != "4009" || p[0].Value != 1 {
		t.Errorf("reconnects %+v", p)
	}
	if p := metricstest.Points(got["bothy.client_metrics.discarded"]); len(p) != 1 || p[0].Attrs["bothy.reason"] != "unknown_name" {
		t.Errorf("discarded %+v", p)
	}
}

// TestSubmitClientMetrics_FeatureDisabled: with privacy.client_metrics off the answer is 403
// feature_disabled and nothing is exported (01 section 11).
func TestSubmitClientMetrics_FeatureDisabled(t *testing.T) {
	e := newEnv(t)
	e.settings.vals[e.alex] = map[string]json.RawMessage{"privacy.client_metrics": json.RawMessage(`false`)}
	resp := e.post("alex", exampleBody)
	if resp.StatusCode != http.StatusForbidden || !strings.Contains(string(resp.Body), `"code":"feature_disabled"`) {
		t.Fatalf("status %d, body %s", resp.StatusCode, resp.Body)
	}
	if got := metricstest.Collect(t, e.reader); len(got) != 0 {
		t.Errorf("exported %v with the setting off", got)
	}
}

// TestSubmitClientMetrics_SchemaViolation: the request validator answers 400 for a body outside
// ClientMetricsBatch (an unknown member, a platform outside the enum, more than maxItems
// entries), before the use case runs.
func TestSubmitClientMetrics_SchemaViolation(t *testing.T) {
	e := newEnv(t)
	many := `{"platform":"macos","app_version":"1.0.0","metrics":[` + strings.TrimSuffix(strings.Repeat(`{"name":"client.frames","kind":"counter","value":1},`, 201), ",") + `]}`
	for name, body := range map[string]string{
		"unknown member": `{"platform":"macos","app_version":"1.0.0","metrics":[],"thread_id":"x"}`,
		"platform":       `{"platform":"beos","app_version":"1.0.0","metrics":[]}`,
		"max items":      many,
		"no metrics":     `{"platform":"macos","app_version":"1.0.0"}`,
	} {
		if resp := e.post("alex", body); resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: status %d, body %s", name, resp.StatusCode, resp.Body)
		}
	}
	if got := metricstest.Collect(t, e.reader); len(got) != 0 {
		t.Errorf("exported %v for refused bodies", got)
	}
}

// TestSubmitClientMetrics_OnlyPeople: an agent gets 403 and an anonymous caller 401
// (client_metrics.submit has the right human, 05).
func TestSubmitClientMetrics_OnlyPeople(t *testing.T) {
	e := newEnv(t)
	if resp := e.post("bot", exampleBody); resp.StatusCode != http.StatusForbidden {
		t.Errorf("agent: status %d, body %s", resp.StatusCode, resp.Body)
	}
	if resp := e.post("", exampleBody); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("anonymous: status %d, body %s", resp.StatusCode, resp.Body)
	}
}
