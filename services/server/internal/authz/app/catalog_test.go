// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/OWNER/bothy/services/server/api/routes"
	"github.com/OWNER/bothy/services/server/internal/authz/adapters/telemetry"
	"github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// record is one captured log record.
type record struct {
	level slog.Level
	msg   string
	attrs map[string]string
}

// capture is a slog.Handler that keeps every record.
type capture struct {
	mu      sync.Mutex
	records []record
}

func (h *capture) Enabled(context.Context, slog.Level) bool { return true }
func (h *capture) WithAttrs([]slog.Attr) slog.Handler       { return h }
func (h *capture) WithGroup(string) slog.Handler            { return h }
func (h *capture) Handle(_ context.Context, r slog.Record) error {
	rec := record{level: r.Level, msg: r.Message, attrs: map[string]string{}}
	r.Attrs(func(a slog.Attr) bool {
		rec.attrs[a.Key] = a.Value.String()
		return true
	})
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, rec)
	return nil
}

func (h *capture) all() []record {
	h.mu.Lock()
	defer h.mu.Unlock()
	return slices.Clone(h.records)
}

func newCatalog() (*app.Catalog, *capture) {
	logs := &capture{}
	return app.NewCatalog(slog.New(logs)), logs
}

// binding returns a valid route binding for action with its catalogue class.
func binding(t *testing.T, id, action string) authorize.RouteBinding {
	t.Helper()
	spec, ok := domain.LookupAction(action)
	if !ok {
		t.Fatalf("no action %s", action)
	}
	return authorize.RouteBinding{OperationID: id, Method: "GET", Pattern: "GET /api/v1/x/" + id, Action: action, RateLimitClass: spec.RateLimitClass}
}

func TestAZP1_RoutesNeedKnownActionAndClass(t *testing.T) {
	cat, _ := newCatalog()
	if err := cat.VerifyRegistrations(wire.RouteBindings(routes.Enabled()), authorize.NewRegistry()); err != nil {
		t.Fatalf("real route table: %v", err)
	}
	// Every action a route may carry passes with its class; deferred actions wait for their ticket
	// (TestVerifyRegistrations_DeferredActions).
	var all []authorize.RouteBinding
	for _, id := range domain.ActionIDs() {
		if s, _ := domain.LookupAction(id); !s.System && s.RateLimitClass != "" && !slices.Contains(domain.DeferredActions(), id) {
			all = append(all, binding(t, "op_"+id, id))
		}
	}
	if err := cat.VerifyRegistrations(all, nil); err != nil {
		t.Fatalf("every routable action: %v", err)
	}

	wrongClass := binding(t, "listThings", "message.read")
	wrongClass.RateLimitClass = "rl.write"
	noClass := binding(t, "deliverPush", "push.deliver")
	noClass.RateLimitClass = ""
	cases := []struct {
		name string
		b    authorize.RouteBinding
		want []string
	}{
		{"unknown action", authorize.RouteBinding{OperationID: "getThing", Method: "GET", Pattern: "GET /api/v1/things", Action: "thing.read", RateLimitClass: "rl.read"}, []string{"getThing", "thing.read", "not in the catalogue"}},
		{"system action", authorize.RouteBinding{OperationID: "bootstrap", Method: "POST", Pattern: "POST /api/v1/bootstrap", Action: "instance.bootstrap"}, []string{"bootstrap", "instance.bootstrap", "system"}},
		{"wrong class", wrongClass, []string{"listThings", "rl.write", "rl.read"}},
		{"action without class", noClass, []string{"deliverPush", "push.deliver", "no rate-limit class"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := cat.VerifyRegistrations([]authorize.RouteBinding{binding(t, "getMessage", "message.read"), tc.b}, nil)
			if err == nil {
				t.Fatal("accepted")
			}
			for _, w := range tc.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("error %q does not contain %q", err, w)
				}
			}
			if strings.Contains(err.Error(), "getMessage") {
				t.Errorf("the valid route is named: %v", err)
			}
		})
	}
}

// TestVerifyRegistrations_CollectsAll: every violation is listed, not only the first.
func TestVerifyRegistrations_CollectsAll(t *testing.T) {
	cat, _ := newCatalog()
	reg := authorize.NewRegistry()
	reg.Op("gateway", authorize.Op("no_such_op"))
	reg.Event("messaging", authorize.EventType("no.such.event"))
	err := cat.VerifyRegistrations([]authorize.RouteBinding{{OperationID: "a", Action: "x.y"}, {OperationID: "b", Action: "instance.bootstrap"}}, reg)
	var re *app.RegistrationError
	if !errors.As(err, &re) || len(re.Violations) != 4 {
		t.Fatalf("err = %v", err)
	}
	for _, w := range []string{`"a"`, `"b"`, "no_such_op", "no.such.event"} {
		if !strings.Contains(err.Error(), w) {
			t.Errorf("error does not name %s: %v", w, err)
		}
	}
}

func TestAZP2_OpsAndEventTypesNeedCatalogueEntry(t *testing.T) {
	cat, _ := newCatalog()
	if err := cat.VerifyRegistrations(nil, wire.BuildRegistry(nil)); err != nil {
		t.Fatalf("real registry: %v", err)
	}
	good := authorize.NewRegistry()
	good.Event("messaging", authorize.EventMessageCreated)
	good.Event("authz", authorize.EventAccessChanged)
	good.Op("gateway", authorize.OpHeartbeat)
	good.Op("gateway", authorize.OpTypingStart)
	good.Op("agents", authorize.OpCommandAutocompleteResult)
	// Every terminal channel op names a deferred action of M2 (host.connect, terminal.stream), so
	// none is valid in M1 (07 section 4.2, docs/briefs/M1-28-authz-core.md D3); see
	// TestVerifyRegistrations_DeferredActions.
	if err := cat.VerifyRegistrations(nil, good); err != nil {
		t.Fatalf("valid registrations: %v", err)
	}

	cases := []struct {
		name     string
		register func(r *authorize.Registry)
		want     []string
	}{
		{"unknown event type", func(r *authorize.Registry) { r.Event("messaging", authorize.EventType("no.such.event")) }, []string{"no.such.event", "messaging"}},
		{"unknown op", func(r *authorize.Registry) { r.Op("gateway", authorize.Op("no_such_op")) }, []string{"no_such_op"}},
		{"server-to-client op", func(r *authorize.Registry) { r.Op("gateway", authorize.OpHello) }, []string{`"hello"`, "server_to_client"}},
		{"event type of two modules", func(r *authorize.Registry) {
			r.Event("authz", authorize.EventAccessChanged)
			r.Event("identity", authorize.EventAccessChanged)
		}, []string{"access.changed", "authz", "identity"}},
		{"unknown channel op", func(r *authorize.Registry) { r.ChannelOp("terminal", authorize.ChannelOp("host_control/no_such")) }, []string{"host_control/no_such"}},
		{"gateway op as channel op", func(r *authorize.Registry) { r.ChannelOp("terminal", authorize.ChannelOp("hello")) }, []string{`"hello"`}},
		{"channel op as gateway op", func(r *authorize.Registry) { r.Op("gateway", authorize.Op("host_control/hello")) }, []string{"host_control/hello"}},
		{"no module", func(r *authorize.Registry) { r.Event("", authorize.EventMessageCreated) }, []string{"message.created", "module"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := authorize.NewRegistry()
			tc.register(r)
			err := cat.VerifyRegistrations(nil, r)
			if err == nil {
				t.Fatal("accepted")
			}
			for _, w := range tc.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("error %q does not contain %q", err, w)
				}
			}
		})
	}
}

// TestAZP2_SameModuleTwiceIsOne: rule 4, a duplicate registration of one module is idempotent.
func TestAZP2_SameModuleTwiceIsOne(t *testing.T) {
	cat, _ := newCatalog()
	r := authorize.NewRegistry()
	r.Event("authz", authorize.EventAccessChanged)
	r.Event("authz", authorize.EventAccessChanged)
	r.Op("gateway", authorize.OpTypingStart)
	r.Op("gateway", authorize.OpTypingStart)
	if err := cat.VerifyRegistrations(nil, r); err != nil {
		t.Fatal(err)
	}
}

// TestCatalogue_ConstantsMatchMetadata: every entry of the catalogue has exactly one constant and
// the constants name nothing else.
func TestCatalogue_ConstantsMatchMetadata(t *testing.T) {
	str := func(s []string) []string { return s }
	conv := func(n int, f func(int) string) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = f(i)
		}
		return out
	}
	a, e, o, c, k := authorize.AllActions(), authorize.AllEventTypes(), authorize.AllOps(), authorize.AllChannelOps(), authorize.AllResourceKinds()
	pairs := []struct {
		name      string
		ids, meta []string
	}{
		{"actions", conv(len(a), func(i int) string { return string(a[i]) }), str(domain.ActionIDs())},
		{"events", conv(len(e), func(i int) string { return string(e[i]) }), str(domain.EventTypes())},
		{"ops", conv(len(o), func(i int) string { return string(o[i]) }), str(domain.OpNames())},
		{"channel ops", conv(len(c), func(i int) string { return string(c[i]) }), str(domain.ChannelOpIDs())},
		{"resource kinds", conv(len(k), func(i int) string { return string(k[i]) }), str(domain.ResourceKinds())},
	}
	for _, p := range pairs {
		if !slices.Equal(p.ids, p.meta) {
			t.Errorf("%s: constants %v, metadata %v", p.name, p.ids, p.meta)
		}
	}
}

func TestSpec_Known(t *testing.T) {
	cat, logs := newCatalog()
	s, err := cat.Spec(authorize.ActionMessageRead)
	if err != nil || s.ID != "message.read" || s.RateLimitClass != "rl.read" {
		t.Fatalf("Spec = %+v, %v", s, err)
	}
	if len(logs.all()) != 0 {
		t.Fatal("a known action was logged")
	}
}

// counterValue reads bothy.authz.errors{bothy.reason=reason} from the manual reader.
func counterValue(t *testing.T, r *sdkmetric.ManualReader, reason string) int64 {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := r.Collect(context.Background(), &rm); err != nil {
		t.Fatal(err)
	}
	var n int64
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != "bothy.authz.errors" {
				continue
			}
			sum, ok := m.Data.(metricdata.Sum[int64])
			if !ok {
				t.Fatalf("bothy.authz.errors is %T", m.Data)
			}
			for _, dp := range sum.DataPoints {
				if v, ok := dp.Attributes.Value(attribute.Key("bothy.reason")); ok && v.AsString() == reason {
					n += dp.Value
				}
			}
		}
	}
	return n
}

func TestAZP3_UnknownActionFailsClosed(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	counter, err := telemetry.NewErrorCounter(sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)))
	if err != nil {
		t.Fatal(err)
	}
	logs := &capture{}
	cat := app.NewCatalog(slog.New(logs), app.WithErrorCounter(counter))

	spec, err := cat.Spec(authorize.Action("no.such.action"))

	if spec.ID != "" {
		t.Errorf("Spec returned %+v for an unknown action", spec)
	}
	var e *errs.Error
	if !errors.As(err, &e) || e.Kind() != errs.KindInternal || e.Code != errs.CodeInternalError || e.Cause != errs.CauseUnknownAction {
		t.Fatalf("err = %#v", err)
	}
	if strings.Contains(err.Error(), "no.such.action") || strings.Contains(e.Detail, "no.such.action") {
		t.Errorf("the public error text names the action: %q", err)
	}
	recs := logs.all()
	if len(recs) != 1 || recs[0].level != slog.LevelError || recs[0].attrs["bothy.action"] != "no.such.action" ||
		recs[0].attrs["error.type"] != "unknown_action" {
		t.Fatalf("log records = %+v", recs)
	}
	if got := counterValue(t, reader, "unknown_action"); got != 1 {
		t.Fatalf("bothy.authz.errors{unknown_action} = %d, want 1", got)
	}
}

// TestNewCatalog_Defaults: a nil logger and no counter still fail closed.
func TestNewCatalog_Defaults(t *testing.T) {
	cat := app.NewCatalog(nil)
	if _, err := cat.Spec(authorize.Action("")); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("err = %v", err)
	}
}

// TestClass: the catalogue answers the route check of api/routes with the same rules.
func TestClass(t *testing.T) {
	cat, logs := newCatalog()
	var _ routes.Catalog = cat
	if c, ok := cat.Class("message.read"); !ok || c != "rl.read" {
		t.Fatalf("message.read = %q, %v", c, ok)
	}
	for _, a := range []string{"instance.bootstrap", "push.deliver", "no.such.action"} {
		if _, ok := cat.Class(a); ok {
			t.Errorf("%s is routable", a)
		}
	}
	if len(logs.all()) != 0 {
		t.Fatal("Class logged")
	}
}

// TestSpec_ZeroCatalogFailsClosed: a zero Catalog or a nil *Catalog still fails closed instead of
// panicking.
func TestSpec_ZeroCatalogFailsClosed(t *testing.T) {
	var nilCat *app.Catalog
	for _, cat := range []*app.Catalog{{}, nilCat} {
		if _, err := cat.Spec(authorize.Action("no.such.action")); errs.KindOf(err) != errs.KindInternal {
			t.Fatalf("err = %v", err)
		}
		if _, err := cat.Spec(authorize.ActionMessageRead); err != nil {
			t.Fatalf("known action: %v", err)
		}
	}
}

// TestAZP1_RouteMessagesAndIdentity: a route names its pattern once, an unknown action carries the
// value in Violation.Action, and a binding without operation id, method or pattern is a violation.
func TestAZP1_RouteMessagesAndIdentity(t *testing.T) {
	cat, _ := newCatalog()
	err := cat.VerifyRegistrations([]authorize.RouteBinding{
		{OperationID: "getThing", Method: "GET", Pattern: "GET /api/v1/things", Action: "thing.read", RateLimitClass: "rl.read"},
		{Action: "message.read", RateLimitClass: "rl.read"},
	}, nil)
	var re *app.RegistrationError
	if !errors.As(err, &re) || len(re.Violations) != 2 {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(err.Error(), "GET GET") {
		t.Errorf("method written twice: %v", err)
	}
	if v := re.Violations[0]; v.Kind != app.ViolationUnknownAction || v.Action != "thing.read" {
		t.Errorf("violation = %+v", v)
	}
	if v := re.Violations[1]; v.Kind != app.ViolationIncompleteRoute {
		t.Errorf("violation = %+v", v)
	}
}

// TestVerifyRegistrations_DeferredActions: a route, a channel op or an event type that names a
// deferred action fails the start (07 section 4.2, D3).
func TestVerifyRegistrations_DeferredActions(t *testing.T) {
	cat, _ := newCatalog()
	route := authorize.RouteBinding{OperationID: "startRun", Method: "POST",
		Pattern: "/api/v1/runs/{run_id}/start", Action: "run.start", RateLimitClass: "rl.agent_runtime"}
	err := cat.VerifyRegistrations([]authorize.RouteBinding{route}, nil)
	var re *app.RegistrationError
	if !errors.As(err, &re) || len(re.Violations) != 1 || re.Violations[0].Kind != app.ViolationDeferredAction ||
		re.Violations[0].Action != "run.start" {
		t.Fatalf("route: %v", err)
	}
	// Since M2-43 no channel op names a deferred action (host.connect is HostSelf), and since M3-03
	// no event type does (file.updated receives with file.read): the registry of file.updated
	// verifies.
	r := authorize.NewRegistry()
	r.Event("files", authorize.EventFileUpdated)
	if err := cat.VerifyRegistrations(nil, r); err != nil {
		t.Errorf("file.updated: %v", err)
	}
}
