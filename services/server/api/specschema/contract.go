// SPDX-License-Identifier: Apache-2.0

// Package specschema is the contract side of gate G9 (docs/spec/06-api.md "Contract tests and
// gate"): it loads docs/api/openapi.yaml, docs/api/asyncapi.yaml and docs/spec/actions.yaml,
// builds one JSON Schema 2020-12 bundle of every body schema and every message of the WebSocket
// channel gateway, and validates the frames the gateway test harness records. It also compares the
// runtime register of gateway ops and event types with the catalogue and AsyncAPI (Compare).
// Only tests import it: G9 validates on the client side of the harness, never in production code
// (docs/DECISIONS.md M1-67).
//
// Example:
//
//	c, err := specschema.Default()
//	if err == nil {
//		err = c.ValidateServerFrame(raw)
//	}
package specschema

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	gosync "sync"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"go.yaml.in/yaml/v3"
)

// DefaultMaxFrameBytes is the default of sync.ws_max_frame_bytes (docs/operations/configuration.md);
// WithMaxFrameBytes sets the value a test server runs with.
const DefaultMaxFrameBytes = 1048576

// The contract files below the repository root.
const (
	openAPIFile  = "docs/api/openapi.yaml"
	asyncAPIFile = "docs/api/asyncapi.yaml"
	actionsFile  = "docs/spec/actions.yaml"
)

// bundleURL is the base URL of the compiled bundle; it never leaves the process.
const bundleURL = "urn:bothy:specschema"

// The two $ref forms the bundle knows (design note 3 of M1-67).
const (
	openAPIRefPrefix  = "#/components/schemas/"
	asyncAPIRefPrefix = "./openapi.yaml#/components/schemas/"
	defsRefPrefix     = "#/$defs/"
)

// gatewayChannel is the $ref of the channel gateway in asyncapi.yaml.
const gatewayChannel = "#/channels/gateway"

// opDispatch is the op of every event frame.
const opDispatch = "dispatch"

// Operation is one operation of the channel gateway in asyncapi.yaml.
type Operation struct {
	// ID is the operation key, for example receive.identify or send.hello.
	ID string
	// Send is true for a server-to-client operation (action send).
	Send bool
	// Op is the frame op of its message, for example identify; dispatch for send.dispatch.
	Op string
	// AuthzAction and RateLimitClass are x-authz-action and x-rate-limit-class, "" when absent.
	AuthzAction, RateLimitClass string
}

// EventMessage is one event message (event.<type>) of the channel gateway.
type EventMessage struct {
	// Type is the event type, for example message.created.
	Type string
	// Kind is x-event-kind: durable or ephemeral.
	Kind string
	// ReceiveAction is x-receive-action, for example message.read or self.
	ReceiveAction string
	// Schema is the openapi schema of the body d, for example MessageCreatedEvent.
	Schema string
}

// Contract is the loaded contract: the compiled bundle, the gateway register of asyncapi.yaml and
// the event catalogue of actions.yaml. It is immutable and safe for concurrent use.
type Contract struct {
	schemas    map[string]*jsonschema.Schema // message key -> compiled payload schema
	serverOps  map[string]string             // op of a send operation -> message key
	clientOps  map[string]string             // op of a receive operation -> message key
	events     map[string]EventMessage       // event type of send.dispatch -> message
	ops        []Operation
	closeCodes map[int]string
	catalogue  Catalogue
	defs       map[string]bool // openapi schema names
	maxFrame   int
	// channels are the message keys of the terminal channels hostControl and hostData by
	// channel, direction (true: server to host) and op (M2-43).
	channels map[string]map[bool]map[string]string
}

// RepoRoot walks up from the working directory to the directory that contains
// docs/api/openapi.yaml. Errors: no such directory up to the file system root.
func RepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, openAPIFile)); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("specschema: no directory with " + openAPIFile + " above the working directory")
		}
		dir = parent
	}
}

// Default returns the contract of the repository that contains the working directory, loaded once
// per process (the compile takes a moment and every harness connection needs it).
var Default = gosync.OnceValues(func() (*Contract, error) {
	root, err := RepoRoot()
	if err != nil {
		return nil, err
	}
	return Load(root)
})

// Load builds the validator from the two contract files and the catalogue below repoRoot. It
// fails on any $ref form it does not know, so a new reference style cannot silently skip
// validation, and on a message of the channel gateway that does not compile.
func Load(repoRoot string) (*Contract, error) {
	var openapi, asyncapi, actions map[string]any
	for _, f := range []struct {
		path string
		dst  *map[string]any
	}{{openAPIFile, &openapi}, {asyncAPIFile, &asyncapi}, {actionsFile, &actions}} {
		v, err := readYAML(filepath.Join(repoRoot, f.path))
		if err != nil {
			return nil, err
		}
		*f.dst = v
	}
	schemas := mapAt(openapi, "components", "schemas")
	if schemas == nil {
		return nil, errors.New("specschema: openapi.yaml has no components.schemas")
	}
	messages := mapAt(asyncapi, "components", "messages")
	c := &Contract{schemas: map[string]*jsonschema.Schema{}, serverOps: map[string]string{}, clientOps: map[string]string{},
		events: map[string]EventMessage{}, defs: map[string]bool{}, maxFrame: DefaultMaxFrameBytes}
	// The gateway register first: rewriteRefs below changes the message payloads in place.
	if err := c.readGateway(asyncapi, messages); err != nil {
		return nil, err
	}
	if err := c.readTerminalChannels(asyncapi, messages); err != nil {
		return nil, err
	}
	defs := map[string]any{}
	for name, s := range schemas {
		c.defs[name] = true
		defs[name] = s
	}
	for key, m := range messages {
		mm, _ := m.(map[string]any)
		defs["msg."+key] = mm["payload"]
	}
	if err := rewriteRefs(defs); err != nil {
		return nil, err
	}
	bases := allOfRefTargets(defs)
	for name, s := range defs {
		defs[name] = normalize(s, !bases[name], false)
	}
	cat, err := readCatalogue(actions)
	if err != nil {
		return nil, err
	}
	c.catalogue = cat
	if err := c.compile(defs); err != nil {
		return nil, err
	}
	return c, nil
}

// compile compiles the payload of every message of the channel gateway.
func (c *Contract) compile(defs map[string]any) error {
	doc := map[string]any{"$schema": "https://json-schema.org/draft/2020-12/schema", "$defs": defs}
	// Through JSON so the compiler sees json.Number and plain maps only.
	b, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("specschema: bundle: %w", err)
	}
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		return err
	}
	comp := jsonschema.NewCompiler()
	comp.DefaultDraft(jsonschema.Draft2020)
	comp.AssertFormat()
	if err := comp.AddResource(bundleURL, v); err != nil {
		return err
	}
	keys := slices.Sorted(maps.Keys(c.allMessages()))
	for _, key := range keys {
		s, err := comp.Compile(bundleURL + "#/$defs/" + jsonPointerEscape("msg."+key))
		if err != nil {
			return fmt.Errorf("specschema: message %s: %w", key, err)
		}
		c.schemas[key] = s
	}
	return nil
}

// allMessages returns every message key of the channel gateway the contract uses.
func (c *Contract) allMessages() map[string]bool {
	out := map[string]bool{}
	for _, k := range c.serverOps {
		out[k] = true
	}
	for _, k := range c.clientOps {
		out[k] = true
	}
	for t := range c.events {
		out["event."+t] = true
	}
	for _, dirs := range c.channels {
		for _, ops := range dirs {
			for _, k := range ops {
				out[k] = true
			}
		}
	}
	return out
}

// terminalChannels are the channels of the host service whose frames ValidateChannelFrame checks.
var terminalChannels = []string{"hostControl", "hostData"}

// readTerminalChannels reads, per terminal channel, the op of every message of its send and
// receive operations: the message name is the op (asyncapi.yaml components.messages host.*).
// Binary messages (noise.frame) have no op and are left out.
func (c *Contract) readTerminalChannels(asyncapi, messages map[string]any) error {
	c.channels = map[string]map[bool]map[string]string{}
	ops := mapAt(asyncapi, "operations")
	for _, ch := range terminalChannels {
		channel := mapAt(asyncapi, "channels", ch)
		if channel == nil {
			return errors.New("specschema: asyncapi.yaml has no channel " + ch)
		}
		g := gatewayDoc{chMsgs: mapAt(channel, "messages"), messages: messages}
		c.channels[ch] = map[bool]map[string]string{true: {}, false: {}}
		for _, id := range slices.Sorted(maps.Keys(ops)) {
			op, _ := ops[id].(map[string]any)
			if ref, _ := mapAt(op, "channel")["$ref"].(string); ref != "#/channels/"+ch {
				continue
			}
			if err := c.readChannelOp(g, ch, id, op); err != nil {
				return err
			}
		}
	}
	return nil
}

// readChannelOp records the message keys of the operation id of the terminal channel ch by op.
func (c *Contract) readChannelOp(g gatewayDoc, ch, id string, op map[string]any) error {
	send := op["action"] == "send"
	list, _ := op["messages"].([]any)
	for _, e := range list {
		ref, _ := e.(map[string]any)["$ref"].(string)
		chKey, ok := strings.CutPrefix(ref, "#/channels/"+ch+"/messages/")
		if !ok {
			return fmt.Errorf("specschema: operation %s: message reference %q", id, ref)
		}
		target, _ := mapAt(g.chMsgs, chKey)["$ref"].(string)
		key, ok := strings.CutPrefix(target, "#/components/messages/")
		msg, _ := g.messages[key].(map[string]any)
		if !ok || msg == nil {
			return fmt.Errorf("specschema: channel %s message %s: reference %q", ch, chKey, target)
		}
		if msg["contentType"] == "application/octet-stream" {
			continue
		}
		name, _ := msg["name"].(string)
		c.channels[ch][send][name] = key
	}
	return nil
}

// readGateway reads the operations, the message keys and x-close-codes of the channel gateway.
func (c *Contract) readGateway(asyncapi, messages map[string]any) error {
	channel := mapAt(asyncapi, "channels", "gateway")
	if channel == nil {
		return errors.New("specschema: asyncapi.yaml has no channel gateway")
	}
	if err := c.readCloseCodes(channel); err != nil {
		return err
	}
	g := gatewayDoc{chMsgs: mapAt(channel, "messages"), messages: messages}
	ops := mapAt(asyncapi, "operations")
	for _, id := range slices.Sorted(maps.Keys(ops)) {
		op, _ := ops[id].(map[string]any)
		if ref, _ := mapAt(op, "channel")["$ref"].(string); ref != gatewayChannel {
			continue
		}
		list, _ := op["messages"].([]any)
		for _, e := range list {
			ref, _ := e.(map[string]any)["$ref"].(string)
			if err := c.readMessage(g, id, op, ref); err != nil {
				return err
			}
		}
		if id == "send."+opDispatch {
			c.ops = append(c.ops, Operation{ID: id, Send: true, Op: opDispatch})
		}
	}
	return nil
}

// gatewayDoc holds the message maps of the channel gateway and of components.messages.
type gatewayDoc struct {
	chMsgs, messages map[string]any
}

// readCloseCodes reads x-close-codes of the channel.
func (c *Contract) readCloseCodes(channel map[string]any) error {
	c.closeCodes = map[int]string{}
	codes, _ := channel["x-close-codes"].([]any)
	for _, e := range codes {
		m, _ := e.(map[string]any)
		code, ok := m["code"].(int)
		name, _ := m["name"].(string)
		if !ok || name == "" {
			return fmt.Errorf("specschema: x-close-codes entry %v", e)
		}
		c.closeCodes[code] = name
	}
	return nil
}

// readMessage records the message ref of the operation id: an event of send.dispatch or the op
// of a send or receive operation.
func (c *Contract) readMessage(g gatewayDoc, id string, op map[string]any, ref string) error {
	chKey, ok := strings.CutPrefix(ref, gatewayChannel+"/messages/")
	if !ok {
		return fmt.Errorf("specschema: operation %s: message reference %q", id, ref)
	}
	target, _ := mapAt(g.chMsgs, chKey)["$ref"].(string)
	key, ok := strings.CutPrefix(target, "#/components/messages/")
	if !ok || g.messages[key] == nil {
		return fmt.Errorf("specschema: channel message %s: reference %q", chKey, target)
	}
	send := op["action"] == "send"
	if t, isEvent := strings.CutPrefix(key, "event."); isEvent && send {
		msg, _ := g.messages[key].(map[string]any)
		kind, _ := msg["x-event-kind"].(string)
		recv, _ := msg["x-receive-action"].(string)
		body, _ := mapAt(msg, "payload", "properties", "d")["$ref"].(string)
		c.events[t] = EventMessage{Type: t, Kind: kind, ReceiveAction: recv, Schema: strings.TrimPrefix(body, asyncAPIRefPrefix)}
		return nil
	}
	name, ok := strings.CutPrefix(key, "op.")
	if !ok {
		return fmt.Errorf("specschema: operation %s: message %s is neither op.* nor event.*", id, key)
	}
	if send {
		c.serverOps[name] = key
	} else {
		c.clientOps[name] = key
	}
	aa, _ := op["x-authz-action"].(string)
	rl, _ := op["x-rate-limit-class"].(string)
	c.ops = append(c.ops, Operation{ID: id, Send: send, Op: name, AuthzAction: aa, RateLimitClass: rl})
	return nil
}

// WithMaxFrameBytes returns a copy of c that allows frames of at most n bytes, the value of
// sync.ws_max_frame_bytes the server under test runs with.
func (c *Contract) WithMaxFrameBytes(n int) *Contract {
	cp := *c
	cp.maxFrame = n
	return &cp
}

// MaxFrameBytes returns the frame limit of c.
func (c *Contract) MaxFrameBytes() int { return c.maxFrame }

// CloseCodes returns x-close-codes of the channel gateway (code -> name) in a new map.
func (c *Contract) CloseCodes() map[int]string { return maps.Clone(c.closeCodes) }

// Operations returns the operations of the channel gateway, sorted by ID; send.dispatch appears
// once with op dispatch.
func (c *Contract) Operations() []Operation { return slices.Clone(c.ops) }

// EventMessages returns the event messages of send.dispatch by event type, in a new map.
func (c *Contract) EventMessages() map[string]EventMessage { return maps.Clone(c.events) }

// Messages returns every message key of the channel gateway, sorted.
func (c *Contract) Messages() []string { return slices.Sorted(maps.Keys(c.schemas)) }

// Compiles reports whether the payload of the message key compiled; an error for an unknown key.
func (c *Contract) Compiles(key string) error {
	if c.schemas[key] == nil {
		return fmt.Errorf("specschema: no compiled schema for message %s", key)
	}
	return nil
}

// AsyncAPI returns the gateway register of asyncapi.yaml for Compare.
func (c *Contract) AsyncAPI() AsyncAPI {
	return AsyncAPI{Operations: c.Operations(), Events: c.EventMessages(), Schemas: maps.Clone(c.defs)}
}

// Catalogue returns the event types and WebSocket ops of docs/spec/actions.yaml.
func (c *Contract) Catalogue() Catalogue {
	return Catalogue{Events: maps.Clone(c.catalogue.Events), Ops: maps.Clone(c.catalogue.Ops)}
}

// readYAML parses a YAML file into JSON-compatible values.
func readYAML(path string) (map[string]any, error) {
	b, err := os.ReadFile(path) // #nosec G304 -- a contract file below the repository root
	if err != nil {
		return nil, err
	}
	var v any
	if err := yaml.Unmarshal(b, &v); err != nil {
		return nil, fmt.Errorf("specschema: %s: %w", filepath.Base(path), err)
	}
	m, ok := toJSON(v).(map[string]any)
	if !ok {
		return nil, fmt.Errorf("specschema: %s is not a mapping", filepath.Base(path))
	}
	return m, nil
}

// toJSON turns YAML values into JSON-compatible ones: map keys become strings and timestamps
// RFC 3339 text (examples hold such values; their keys are annotations and change nothing).
func toJSON(v any) any {
	switch x := v.(type) {
	case map[string]any:
		for k, e := range x {
			x[k] = toJSON(e)
		}
		return x
	case map[any]any:
		out := make(map[string]any, len(x))
		for k, e := range x {
			out[fmt.Sprint(k)] = toJSON(e)
		}
		return out
	case []any:
		for i, e := range x {
			x[i] = toJSON(e)
		}
		return x
	case time.Time:
		return x.Format(time.RFC3339Nano)
	default:
		return v
	}
}

// mapAt walks the mapping keys of path from m; nil when a step is missing.
func mapAt(m map[string]any, path ...string) map[string]any {
	for _, p := range path {
		next, ok := m[p].(map[string]any)
		if !ok {
			return nil
		}
		m = next
	}
	return m
}

// jsonPointerEscape escapes one JSON pointer token (RFC 6901).
func jsonPointerEscape(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1")
}
