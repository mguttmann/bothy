// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"context"
	"os"
	"slices"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/OWNER/bothy/services/server/api/routes"
	"github.com/OWNER/bothy/services/server/internal/agents"
	authzapp "github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/ratelimit"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// TestGatewayRoute_AuthorizedInBandOnlyGateway (M1-64): the gateway route is registered with
// gateway.connect and rl.gateway_connect and passes the catalogue check of the start (AZ-P1);
// it is the only route marked AuthorizedInBand.
func TestGatewayRoute_AuthorizedInBandOnlyGateway(t *testing.T) {
	t.Parallel()
	op := wire.GatewayOperation()
	if !op.AuthorizedInBand || op.Pattern != "GET /api/v1/gateway" || op.Action != "gateway.connect" || op.RateLimitClass != "rl.gateway_connect" {
		t.Fatalf("gateway operation %+v", op)
	}
	for _, r := range routes.Enabled() {
		if wire.HTTPOperation(r).AuthorizedInBand {
			t.Errorf("%s is marked AuthorizedInBand", r.ID)
		}
	}
	bindings := append(wire.RouteBindings(routes.Enabled()), wire.GatewayRouteBinding())
	if err := authzapp.NewCatalog(nil).VerifyRegistrations(bindings, wire.BuildRegistry(nil)); err != nil {
		t.Fatalf("catalogue check: %v", err)
	}
}

// fixedInstance is an instance row with a fixed epoch.
type fixedInstance struct{}

func (fixedInstance) Load(context.Context) (instance.Record, error) { return instance.Record{}, nil }

// noConsumers is the consumer port of agents for the wiring test; it is never called.
type noConsumers struct{ agents.Consumers }

// staticNonces is a nonce source for the wiring test.
type staticNonces struct{}

func (staticNonces) CurrentNonce() string { return "n" }

// noAnswers takes no answer of command.autocomplete_result.
type noAnswers struct{}

func (noAnswers) AnswerAutocomplete(context.Context, runs.AutocompleteAnswer) error {
	return errs.New(errs.CodeNotFound, "")
}

// noThreads is a chat port that knows no thread.
type noThreads struct{}

func (noThreads) Info(context.Context, ids.ID) (chat.ThreadInfo, error) {
	return chat.ThreadInfo{}, errs.New(errs.CodeNotFound, "")
}

// TestGatewayOpsEnableList: the gateway registers exactly the ops of gateway_ops.enabled with the
// action of each (routes.GatewayOps, M1-64, M1-66 and the ops of later tickets up to
// stats.subscribe and stats.unsubscribe of M6-15), and its hub is the EphemeralPublisher of the
// process.
func TestGatewayOpsEnableList(t *testing.T) {
	t.Parallel()
	values := loadEnv(t, nil)
	cfg, err := wire.GatewayConfigFrom(values)
	if err != nil {
		t.Fatal(err)
	}
	rl, err := wire.RateLimitConfig(values)
	if err != nil {
		t.Fatal(err)
	}
	l, err := ratelimit.New(rl, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	gw, err := wire.NewGateway(cfg, wire.GatewayDeps{Authenticator: actor.AnonymousOnly{}, Authorizer: allowAll{}, Nonces: staticNonces{},
		Limiter: l, Reads: &wire.SyncReads{}, Instance: fixedInstance{}, Threads: noThreads{}, Consumers: noConsumers{},
		Autocomplete: noAnswers{}, Runs: noRuns{}})
	if err != nil {
		t.Fatal(err)
	}
	actions := map[string]authorize.Action{
		"run.update":                  authorize.ActionRunUpdate,
		"ack":                         authorize.ActionAgentEventsConsume,
		"command.autocomplete_result": authorize.ActionCommandAutocompleteRespond,
		"focus":                       authorize.ActionGatewayFocus, "heartbeat": authorize.ActionGatewayConnect, "identify": authorize.ActionGatewayConnect,
		"reauth": authorize.ActionGatewayConnect, "resume": authorize.ActionGatewayConnect, "typing.start": authorize.ActionTypingStart,
		"typing.stop": authorize.ActionTypingStart, "stats.subscribe": authorize.ActionStatsRead, "stats.unsubscribe": authorize.ActionStatsRead,
	}
	var want []string
	for _, op := range routes.GatewayOps() {
		want = append(want, op.Op)
		if op.Action != string(actions[op.Op]) {
			t.Errorf("%s has action %s, want %s", op.Op, op.Action, actions[op.Op])
		}
	}
	if got := gw.App.Dispatcher().Ops(); !slices.Equal(got, want) ||
		!slices.Equal(want, []string{"ack", "command.autocomplete_result", "focus", "heartbeat", "identify", "reauth", "resume", "run.update",
			"stats.subscribe", "stats.unsubscribe", "typing.start", "typing.stop"}) {
		t.Errorf("registered %v, enabled %v", got, want)
	}
	if cfg.HeartbeatInterval != 30*time.Second || cfg.ResumeWindow != 5*time.Minute {
		t.Errorf("defaults %+v", cfg)
	}
	if cfg.MaxFocusedThreads != 20 || cfg.TypingMinInterval != 5*time.Second || cfg.TypingTTL != 8*time.Second || cfg.PayloadMax != 65536 {
		t.Errorf("focus and typing defaults %+v", cfg)
	}
	if gw.Publisher == nil || gw.Publisher != eventlog.EphemeralPublisher(gw.Hub) {
		t.Error("the hub is not the EphemeralPublisher of the gateway")
	}
}

// TestGatewayOpRateClasses: the class of every enabled op is the x-rate-limit-class of its
// receive operation in docs/api/asyncapi.yaml.
func TestGatewayOpRateClasses(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile("../../../../docs/api/asyncapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Operations map[string]struct {
			Class string `yaml:"x-rate-limit-class"`
		} `yaml:"operations"`
	}
	if err := yaml.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	for op, class := range wire.GatewayOpClasses() {
		if got := doc.Operations["receive."+op].Class; got != class {
			t.Errorf("%s counts %s, the contract says %q", op, class, got)
		}
	}
	if n := len(wire.GatewayOpClasses()); n != len(routes.GatewayOps()) {
		t.Errorf("%d ops with a class, want every enabled op", n)
	}
}

// noRuns is the runs module of a gateway without runs: no patch succeeds and no run is active.
type noRuns struct{}

func (noRuns) Patch(context.Context, runs.PatchRun) (runs.PatchResult, error) {
	return runs.PatchResult{}, errs.New(errs.CodeNotFound, "")
}

func (noRuns) Documents(context.Context, []ids.ID) ([]runs.Run, error) { return nil, nil }

func (noRuns) Document(context.Context, ids.ID) (runs.Run, bool, error) {
	return runs.Run{}, false, nil
}
