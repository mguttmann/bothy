// SPDX-License-Identifier: Apache-2.0

package main

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// fixtureAsyncAPI is a gateway channel with two client ops (ping with a closed body, pong), one
// server op (hello) and a client op of another channel, which the gateway list must ignore.
const fixtureAsyncAPI = `asyncapi: 3.0.0
info:
  title: x
  version: 0.0.1
  x-protocol-versions: [1]
channels:
  gateway:
    address: /gateway
    messages:
      op.ping:
        $ref: '#/components/messages/op.ping'
      op.pong:
        $ref: '#/components/messages/op.pong'
      op.hello:
        $ref: '#/components/messages/op.hello'
  other:
    address: /other
    messages:
      op.side:
        $ref: '#/components/messages/op.side'
operations:
  receive.ping:
    action: receive
    channel:
      $ref: '#/channels/gateway'
    messages:
    - $ref: '#/channels/gateway/messages/op.ping'
    x-authz-action: thing.read
  receive.pong:
    action: receive
    channel:
      $ref: '#/channels/gateway'
    messages:
    - $ref: '#/channels/gateway/messages/op.pong'
    x-authz-action: thing.update
  send.hello:
    action: send
    channel:
      $ref: '#/channels/gateway'
    messages:
    - $ref: '#/channels/gateway/messages/op.hello'
    x-authz-action: thing.read
  receive.side:
    action: receive
    channel:
      $ref: '#/channels/other'
    messages:
    - $ref: '#/channels/other/messages/op.side'
    x-authz-action: thing.read
components:
  messages:
    op.ping:
      name: ping
      payload:
        type: object
        properties:
          d:
            $ref: ./openapi.yaml#/components/schemas/ThingCreate
    op.pong:
      name: pong
      payload:
        type: object
        properties:
          d:
            $ref: ./openapi.yaml#/components/schemas/ThingUpdate
    op.hello:
      name: hello
      payload:
        type: object
        properties:
          d:
            $ref: ./openapi.yaml#/components/schemas/Thing
    op.side:
      name: side
      payload:
        type: object
        properties:
          d:
            $ref: ./openapi.yaml#/components/schemas/Thing
`

// fixtureGatewaySection is the gateway_ops section that matches fixtureAsyncAPI.
const fixtureGatewaySection = `gateway_ops:
  enabled:
    - ping
  later:
    - op: pong
      etappe: M2
`

// gatewayFixture writes the fixture with the asyncapi document and the list extended by section
// and returns the paths of spec, catalogue, list and asyncapi.
func gatewayFixture(t *testing.T, async, section string) (spec, catalog, list, asyncPath string) {
	t.Helper()
	spec, catalog, list = fixturePaths(t, "")
	writeFile(t, list, readText(t, list)+section)
	asyncPath = filepath.Join(filepath.Dir(spec), "asyncapi.yaml")
	writeFile(t, asyncPath, async)
	return spec, catalog, list, asyncPath
}

// gatewayProblems runs the gateway checks of the fixture and returns the problems.
func gatewayProblems(t *testing.T, async, section string) ([]gatewayOp, []string) {
	t.Helper()
	spec, catalog, list, asyncPath := gatewayFixture(t, async, section)
	in, err := loadInputs(spec, catalog, list)
	if err != nil {
		t.Fatal(err)
	}
	ops, err := loadGatewayOps(in, spec, list, asyncPath)
	if err == nil {
		return ops, nil
	}
	return nil, problemsOf(t, err)
}

func TestGatewayOps_FixtureEnabled(t *testing.T) {
	ops, problems := gatewayProblems(t, fixtureAsyncAPI, fixtureGatewaySection)
	if problems != nil {
		t.Fatalf("problems: %v", problems)
	}
	want := []gatewayOp{{Op: "ping", Action: "thing.read", Schema: "ThingCreate"}}
	if !reflect.DeepEqual(ops, want) {
		t.Errorf("ops = %+v, want %+v", ops, want)
	}
}

func TestGatewayOps_Rules(t *testing.T) {
	cases := map[string]struct {
		async, section, want string
	}{
		"missing section": {fixtureAsyncAPI, "", "gateway_ops"},
		"op missing from list": {fixtureAsyncAPI, "gateway_ops:\n  enabled:\n    - ping\n  later: []\n",
			"pong"},
		"unknown op": {fixtureAsyncAPI, "gateway_ops:\n  enabled:\n    - ping\n    - zap\n  later:\n    - op: pong\n      etappe: M2\n",
			"zap"},
		"server op listed": {fixtureAsyncAPI, "gateway_ops:\n  enabled:\n    - hello\n    - ping\n  later:\n    - op: pong\n      etappe: M2\n",
			"hello"},
		"other channel op listed": {fixtureAsyncAPI, "gateway_ops:\n  enabled:\n    - ping\n    - side\n  later:\n    - op: pong\n      etappe: M2\n",
			"side"},
		"in both sections": {fixtureAsyncAPI, "gateway_ops:\n  enabled:\n    - ping\n    - pong\n  later:\n    - op: pong\n      etappe: M2\n",
			"pong"},
		"unsorted": {fixtureAsyncAPI, "gateway_ops:\n  enabled:\n    - pong\n    - ping\n  later: []\n",
			"sorted"},
		"bad etappe": {fixtureAsyncAPI, "gateway_ops:\n  enabled:\n    - ping\n  later:\n    - op: pong\n      etappe: X9\n",
			"etappe"},
		"open frame body": {strings.Replace(fixtureAsyncAPI, "schemas/ThingCreate", "schemas/Thing", 1), fixtureGatewaySection,
			"Thing"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, problems := gatewayProblems(t, c.async, c.section)
			if len(problems) == 0 {
				t.Fatal("accepted")
			}
			if !strings.Contains(strings.Join(problems, "\n"), c.want) {
				t.Errorf("problems %v do not name %q", problems, c.want)
			}
		})
	}
}

// TestRealInputs_GatewayOps is the gate check of gateway_ops (M1-64): every client-to-server op of
// the channel gateway in docs/api/asyncapi.yaml is listed exactly once, every listed op is one,
// every body of a client frame refuses unknown members (W7, SEC-IN-1), and the enabled ops are
// identify, resume, heartbeat and reauth with the action gateway.connect (M1-64) and focus,
// typing.start and typing.stop with gateway.focus and typing.start (M1-66).
func TestRealInputs_GatewayOps(t *testing.T) {
	in, err := realInputs()
	if err != nil {
		t.Fatal(err)
	}
	ops, err := loadGatewayOps(in, "../../../../../docs/api/openapi.yaml", "../../operations.yaml", realAsyncAPI)
	if err != nil {
		t.Fatalf("gateway ops: %v", err)
	}
	want := []gatewayOp{
		{Op: "focus", Action: "gateway.focus", Schema: "GatewayFocus"},
		{Op: "heartbeat", Action: "gateway.connect", Schema: "GatewayHeartbeat"},
		{Op: "identify", Action: "gateway.connect", Schema: "GatewayIdentify"},
		{Op: "reauth", Action: "gateway.connect", Schema: "GatewayReauth"},
		{Op: "resume", Action: "gateway.connect", Schema: "GatewayResume"},
		{Op: "typing.start", Action: "typing.start", Schema: "GatewayTyping"},
		{Op: "typing.stop", Action: "typing.start", Schema: "GatewayTyping"},
	}
	if !reflect.DeepEqual(ops, want) {
		t.Errorf("enabled gateway ops = %+v, want %+v", ops, want)
	}
}

func TestRenderMeta_GatewayOps(t *testing.T) {
	spec, catalog, list := fixturePaths(t, "")
	in, err := loadInputs(spec, catalog, list)
	if err != nil {
		t.Fatal(err)
	}
	in.gatewayOps = []gatewayOp{{Op: "ping", Action: "thing.read", Schema: "ThingCreate"}}
	files, err := renderRoutes(in, corePkg, fixtureRestPkg)
	if err != nil {
		t.Fatal(err)
	}
	src := string(files["meta.gen.go"])
	if !strings.Contains(src, `{Op: "ping", Action: "thing.read", Schema: "ThingCreate"}`) || !strings.Contains(src, "routes.GatewayOp") {
		t.Errorf("meta.gen.go lacks the gateway op:\n%s", src)
	}
}
