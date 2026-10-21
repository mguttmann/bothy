// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"os"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/OWNER/bothy/services/server/api/routes"
	"github.com/OWNER/bothy/services/server/api/specschema"
	authzdomain "github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// runtimeRegister is the register of this binary as G9 compares it: the ops of the module
// registrations with the action the dispatcher requires (routes.GatewayOps) and the class the
// gateway counts (wire.GatewayOpClasses), the registered event types with the kind and receive
// action of the server's catalogue, and the enable list gateway_ops of api/operations.yaml.
func runtimeRegister(t *testing.T) specschema.Register {
	t.Helper()
	actions := map[string]string{}
	for _, op := range routes.GatewayOps() {
		actions[op.Op] = op.Action
	}
	classes := wire.GatewayOpClasses()
	reg := wire.BuildRegistry(nil)
	var out specschema.Register
	for _, o := range reg.Ops() {
		op := string(o.Op)
		out.Ops = append(out.Ops, specschema.RegisteredOp{Op: op, Action: actions[op], RateLimitClass: classes[op]})
	}
	for _, e := range reg.Events() {
		spec, ok := authzdomain.LookupEvent(string(e.Type))
		ev := specschema.RegisteredEvent{Type: string(e.Type)}
		if ok {
			ev.Kind = "ephemeral"
			if spec.Durable {
				ev.Kind = "durable"
			}
			ev.ReceiveAction = strings.Trim(spec.ReceiveAction, "`")
			if spec.AudienceSelf {
				ev.ReceiveAction = "self"
			} else if len(spec.MentionedActions) == 1 {
				ev.ReceiveAction = spec.MentionedActions[0]
			}
		}
		out.Events = append(out.Events, ev)
	}
	b, err := os.ReadFile("../../api/operations.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		GatewayOps struct {
			Enabled []string `yaml:"enabled"`
			Later   []struct {
				Op        string `yaml:"op"`
				Milestone string `yaml:"milestone"`
			} `yaml:"later"`
		} `yaml:"gateway_ops"`
	}
	if err := yaml.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	out.Enabled = doc.GatewayOps.Enabled
	for _, l := range doc.GatewayOps.Later {
		out.Later = append(out.Later, specschema.LaterOp{Op: l.Op, Milestone: l.Milestone})
	}
	return out
}

// TestRegister_MatchesActionsAndAsyncAPI is the register half of gate G9 (06 "Contract tests and
// gate", 05 "Tests that hold this catalogue" item 2, AZ-P2): every op and event type this binary
// registers matches docs/spec/actions.yaml and docs/api/asyncapi.yaml, and the enable list
// gateway_ops covers exactly the client ops of asyncapi.yaml (PLAN P-04).
func TestRegister_MatchesActionsAndAsyncAPI(t *testing.T) {
	t.Parallel()
	c, err := specschema.Default()
	if err != nil {
		t.Fatal(err)
	}
	reg := runtimeRegister(t)
	if len(reg.Ops) == 0 || len(reg.Events) == 0 || len(reg.Enabled) == 0 {
		t.Fatalf("empty register %+v", reg)
	}
	for _, p := range specschema.Compare(reg, c.Catalogue(), c.AsyncAPI()) {
		t.Error(p)
	}
}
