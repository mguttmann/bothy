// SPDX-License-Identifier: Apache-2.0

package agents_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// Scenario is one file under scenarios/: one test ID of docs/spec/09-agents.md 16.2 with its
// cases. The Go types and Validate are the format; there is no separate schema file (README
// "Scenario format").
type Scenario struct {
	// ID is the test ID, exactly the name of the file in upper case (AG-14 in ag-14.yaml).
	ID string `yaml:"id"`
	// Title is the English row text of 09 16.2.
	Title string `yaml:"title"`
	// Sources are the chapter references the expectations come from ("09 16.2", "04 15.5").
	Sources []string `yaml:"sources"`
	// World is the world the cases start from; only "matrix" (BuildMatrixWorld plus world_test.go).
	World string `yaml:"world"`
	// Extras are world parts only some scenarios need: "tracker" (the app Tracker with G-6),
	// "webhook" (an incoming webhook of format native on Support/Eskalation).
	Extras []string `yaml:"extras"`
	// Config holds dotted keys of docs/operations/configuration.md with test values; every key a
	// step advances the clock by is set here.
	Config map[string]string `yaml:"config"`
	// SharedWorld runs all cases one after the other on one server and world.
	SharedWorld bool `yaml:"shared_world"`
	// Setup runs before the first step of every case (once with SharedWorld).
	Setup []Step `yaml:"setup"`
	// Agents are the Fake-Agent processes of a case by name.
	Agents map[string]AgentEntry `yaml:"agents"`
	// Cases run in order; each on its own server unless SharedWorld.
	Cases []Case `yaml:"cases"`
	// Deferred are the parts that need a later ticket (testdata/deferred.golden).
	Deferred []Deferred `yaml:"deferred"`
}

// AgentEntry is one Fake-Agent process of a case.
type AgentEntry struct {
	// Agent is the world name of the agent (Lotse, Texter, Runner, Assistent).
	Agent string `yaml:"agent"`
	// Consumer is the consumer name; empty means "default".
	Consumer string `yaml:"consumer"`
	// Transport is "rest" (default) or "gateway".
	Transport string `yaml:"transport"`
	// Launch is "start" (default: at the start of the case) or "later" (a step do: launch).
	Launch string `yaml:"launch"`
	// Runtime is the actual state of the runtime: billing_mode, billing_evidence, tool_class,
	// capabilities, model. Absent members take the configured values of the agent.
	Runtime map[string]any `yaml:"runtime"`
	// Settings are SDK keys (adapter.invocation_token_leeway, adapter.jwks_refresh_min_interval,
	// sdk.*) handed to the Fake-Agent; the conductor adds small retry and poll waits.
	Settings map[string]string `yaml:"settings"`
}

// Case is one named list of steps; Rows repeats the steps once per row with {{name}} replaced by
// the row's values (the matrix of AG-02 and AG-12): a scalar that is exactly {{name}} takes the
// row's value with its type, {{name}} inside a longer text is replaced by the value's text. The
// decoder expands the rows, so Steps holds every row's steps in row order.
type Case struct {
	Name  string           `yaml:"name"`
	Rows  []map[string]any `yaml:"rows"`
	Steps []Step           `yaml:"steps"`
}

// UnmarshalYAML decodes a case strictly and expands its rows.
func (c *Case) UnmarshalYAML(n *yaml.Node) error {
	var raw struct {
		Name  string           `yaml:"name"`
		Rows  []map[string]any `yaml:"rows"`
		Steps yaml.Node        `yaml:"steps"`
	}
	if err := strictNode(n, &raw); err != nil {
		return err
	}
	c.Name, c.Rows = raw.Name, raw.Rows
	if len(raw.Rows) == 0 {
		return strictNode(&raw.Steps, &c.Steps)
	}
	for i, row := range raw.Rows {
		node, err := substitute(&raw.Steps, row)
		if err != nil {
			return fmt.Errorf("row %d: %w", i+1, err)
		}
		var steps []Step
		if err := strictNode(node, &steps); err != nil {
			return fmt.Errorf("row %d: %w", i+1, err)
		}
		c.Steps = append(c.Steps, steps...)
	}
	return nil
}

// strictNode decodes a node with known fields only (node.Decode alone does not check them).
func strictNode(n *yaml.Node, v any) error {
	b, err := yaml.Marshal(n)
	if err != nil {
		return err
	}
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	return dec.Decode(v)
}

// placeholder finds {{name}} in a scalar.
var placeholder = regexp.MustCompile(`\{\{([a-z_0-9]+)\}\}`)

// substitute returns a deep copy of n with the placeholders of row replaced.
func substitute(n *yaml.Node, row map[string]any) (*yaml.Node, error) {
	out := *n
	if n.Kind == yaml.ScalarNode {
		if m := placeholder.FindStringSubmatch(n.Value); m != nil && m[0] == n.Value {
			v, ok := row[m[1]]
			if !ok {
				return nil, fmt.Errorf("no value %s in the row", m[1])
			}
			var vn yaml.Node
			if err := vn.Encode(v); err != nil {
				return nil, err
			}
			return &vn, nil
		}
		var missing error
		out.Value = placeholder.ReplaceAllStringFunc(n.Value, func(s string) string {
			name := placeholder.FindStringSubmatch(s)[1]
			v, ok := row[name]
			if !ok {
				missing = fmt.Errorf("no value %s in the row", name)
				return s
			}
			return fmt.Sprint(v)
		})
		return &out, missing
	}
	out.Content = nil
	for _, c := range n.Content {
		sub, err := substitute(c, row)
		if err != nil {
			return nil, err
		}
		out.Content = append(out.Content, sub)
	}
	return &out, nil
}

// Deferred is one part of a scenario that a later ticket brings.
type Deferred struct {
	Part   string `yaml:"part"`
	Ticket string `yaml:"ticket"`
	Reason string `yaml:"reason"`
}

// Step is one step of a case; exactly one of As (with Call), Agent (with Do), Server, Check,
// Parallel, Repeat and Await is set.
type Step struct {
	// As is the subject of a human call: owner, alex, sam, robin, tracker, webhook.
	As string `yaml:"as"`
	// Call is the operationId of docs/api/openapi.yaml.
	Call string `yaml:"call"`
	// With are the arguments: path parameters by name, query (map), if_match, and the body
	// members; for an agent op its arguments.
	With map[string]any `yaml:"with"`
	// Async starts the call in the background under this name; a later Await takes its answer.
	Async string `yaml:"async"`

	// Agent is the name of a Fake-Agent of the scenario; Do is its op.
	Agent string `yaml:"agent"`
	Do    string `yaml:"do"`

	// Server is a conductor action: advance_clock, run_jobs, snapshot_cursors, restart, proxy,
	// dropped.
	Server string `yaml:"server"`
	// By are the configuration keys advance_clock moves the clock by (their sum plus one tick).
	By StringList `yaml:"by"`
	// Kind is the job kind of run_jobs.
	Kind string `yaml:"kind"`
	// Name names a cursor snapshot (snapshot_cursors).
	Name string `yaml:"name"`
	// RestoreCursors names the snapshot restart writes back.
	RestoreCursors string `yaml:"restore_cursors"`
	// DropNext is the operationId the proxy drops the answer of next.
	DropNext string `yaml:"drop_next"`

	// Check is a state check through AdminDB: run, runs, message, messages, followup, component,
	// interaction, session, thread, events, audit, consumer.
	Check string `yaml:"check"`
	// ID names the row of a single check (run, message, thread).
	ID string `yaml:"id"`
	// Where filters a set check by column (dotted for a JSON member of payload or details).
	Where map[string]any `yaml:"where"`
	// Eventually polls the check until it holds or the test deadline passes (asynchronous
	// effects only).
	Eventually bool `yaml:"eventually"`

	// Parallel are steps released together behind a barrier; Tally counts their outcomes.
	Parallel []Step  `yaml:"parallel"`
	Tally    []Tally `yaml:"tally"`

	// Repeat runs its steps Times times.
	Repeat *Repeat `yaml:"repeat"`

	// Await takes the answer of an Async call.
	Await string `yaml:"await"`

	// Expect is the literal expectation of the step.
	Expect *Expect `yaml:"expect"`
	// Keep binds names to members of the answer or row by dotted path ({r1: run_id}).
	Keep map[string]string `yaml:"keep"`
	// Note is a free comment.
	Note string `yaml:"note"`
}

// Tally requires that exactly Count steps of a parallel group matched Match.
type Tally struct {
	Match Expect `yaml:"match"`
	Count int    `yaml:"count"`
}

// Repeat repeats steps.
type Repeat struct {
	Times int    `yaml:"times"`
	Steps []Step `yaml:"steps"`
}

// Expect is a literal expectation: the problem members, none, alternatives and any other member
// of the answer or row by dotted path.
type Expect struct {
	Status int        `yaml:"status"`
	Code   string     `yaml:"code"`
	Reason string     `yaml:"reason"`
	Errors []FieldErr `yaml:"errors"`
	// None requires that nothing arrived (receive) or that no row matched (count 0).
	None bool `yaml:"none"`
	// OneOf passes when one alternative passes.
	OneOf []Expect `yaml:"one_of"`
	// Count is the number of rows of a set check.
	Count *int `yaml:"count"`
	// Rows are partial matches of the rows of a set check, in order.
	Rows []map[string]any `yaml:"rows"`
	// Fields are other members by dotted path; the value <absent> requires the member to be
	// missing (or NULL in a row), <present> to be there.
	Fields map[string]any `yaml:",inline"`
}

// FieldErr is one entry of errors[] of a problem.
type FieldErr struct {
	Code    string `yaml:"code"`
	Pointer string `yaml:"pointer"`
}

// StringList accepts a single string or a list.
type StringList []string

// UnmarshalYAML reads a scalar or a sequence.
func (l *StringList) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		*l = []string{n.Value}
		return nil
	}
	var s []string
	if err := n.Decode(&s); err != nil {
		return err
	}
	*l = s
	return nil
}

// The vocabularies of the format.
var (
	// fakeAgentOps are the ops of the Fake-Agent (brief Design note 5) plus launch.
	fakeAgentOps = []string{
		"launch", "connect", "receive", "verify", "start", "resume", "renew", "context", "patch", "await_input", "finish",
		"heartbeat", "post", "respond", "update_component", "set_title", "report_session", "register_commands",
		"autocomplete_mode", "export_token", "import_token", "crash",
	}
	serverActions = []string{"advance_clock", "run_jobs", "snapshot_cursors", "restart", "proxy", "dropped", "fixture"}
	checkKinds    = []string{"run", "runs", "message", "messages", "followup", "component", "interaction", "session", "thread", "events", "audit", "consumer", "commands", "signing_keys"}
	subjects      = []string{"owner", "alex", "sam", "robin", "tracker", "webhook"}
	// jobKinds are the kinds run_jobs accepts (brief Design note 7).
	jobKinds = []string{"runs.cancel_timeout", "runs.stall_check", "runs.queued_expiry", "instance.key_rotation"}
	// singleChecks name one row by ID.
	singleChecks = []string{"run", "message", "thread"}
)

// idPattern is the form of a test ID.
var idPattern = regexp.MustCompile(`^AG-\d\d$`)

// decodeScenario decodes one document strictly: unknown keys and trailing documents fail.
func decodeScenario(b []byte) (*Scenario, error) {
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	var s Scenario
	if err := dec.Decode(&s); err != nil {
		return nil, err
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, errors.New("more than one YAML document")
	}
	return &s, nil
}

// vocabulary is what Validate checks names against: the operationIds of the contract and the
// configuration keys of the reference.
type vocabulary struct {
	operations map[string]bool
	configKeys map[string]bool
}

// Validate checks the rules of the format (TestScenarioFiles_Load); file is the base name.
func (s *Scenario) Validate(file string, v vocabulary) error {
	var errs []error
	fail := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }
	want := strings.ToUpper(strings.TrimSuffix(file, filepath.Ext(file)))
	if s.ID != want || !idPattern.MatchString(s.ID) {
		fail("id %q does not match the file %s", s.ID, file)
	}
	if s.Title == "" || len(s.Sources) == 0 {
		fail("title and sources are required")
	}
	if s.World != "matrix" {
		fail("world %q (want matrix)", s.World)
	}
	for _, e := range s.Extras {
		if e != "tracker" && e != "webhook" {
			fail("unknown extra %q", e)
		}
	}
	for k := range s.Config {
		if !v.configKeys[k] {
			fail("config key %s is not in docs/operations/configuration.md", k)
		}
	}
	for name, a := range s.Agents {
		if a.Agent == "" {
			fail("agent %s: agent is required", name)
		}
		if a.Transport != "" && a.Transport != "rest" && a.Transport != "gateway" {
			fail("agent %s: transport %q", name, a.Transport)
		}
		if a.Launch != "" && a.Launch != "start" && a.Launch != "later" {
			fail("agent %s: launch %q", name, a.Launch)
		}
		for k := range a.Settings {
			if !v.configKeys[k] {
				fail("agent %s: setting %s is not in docs/operations/configuration.md", name, k)
			}
		}
	}
	if len(s.Cases) == 0 {
		if len(s.Deferred) == 0 {
			fail("a file without cases needs every part in deferred")
		}
	}
	for _, d := range s.Deferred {
		if d.Part == "" || d.Ticket == "" || d.Reason == "" {
			fail("deferred entries need part, ticket and reason")
		}
	}
	seen := map[string]bool{}
	for _, c := range s.Cases {
		if c.Name == "" || seen[c.Name] {
			fail("case name %q empty or duplicate", c.Name)
		}
		seen[c.Name] = true
		if len(c.Steps) == 0 || !endsWithCheck(c.Steps) {
			fail("case %s: must end with at least one check", c.Name)
		}
		s.validateSteps("case "+c.Name, c.Steps, v, fail)
	}
	s.validateSteps("setup", s.Setup, v, fail)
	return errors.Join(errs...)
}

// endsWithCheck reports whether the last step is a check.
func endsWithCheck(steps []Step) bool { return steps[len(steps)-1].Check != "" }

// validateSteps checks every step of a list and its nested steps.
func (s *Scenario) validateSteps(where string, steps []Step, v vocabulary, fail func(string, ...any)) {
	s.validateStepList(where, steps, v, false, fail)
}

// validateStepList checks steps; tallied marks the steps of a parallel group with a tally, whose
// outcomes the tally checks, so they need no expect of their own.
func (s *Scenario) validateStepList(where string, steps []Step, v vocabulary, tallied bool, fail func(string, ...any)) {
	for i := range steps {
		st := &steps[i]
		at := fmt.Sprintf("%s step %d", where, i+1)
		if n := st.kinds(); n != 1 {
			fail("%s: %d step kinds (want exactly one)", at, n)
			continue
		}
		if (st.As == "") != (st.Call == "") || (st.Agent == "") != (st.Do == "") {
			fail("%s: as and call, agent and do come in pairs", at)
			continue
		}
		switch {
		case st.As != "":
			if !slices.Contains(subjects, st.As) {
				fail("%s: unknown subject %q", at, st.As)
			}
			if !v.operations[st.Call] {
				fail("%s: call %q is no operationId of docs/api/openapi.yaml", at, st.Call)
			}
			if st.Expect == nil && !tallied && st.Async == "" {
				fail("%s: a call needs an expect", at)
			}
		case st.Agent != "":
			if _, ok := s.Agents[st.Agent]; !ok {
				fail("%s: unknown agent %q", at, st.Agent)
			}
			if !slices.Contains(fakeAgentOps, st.Do) {
				fail("%s: unknown op %q", at, st.Do)
			}
			if st.Expect == nil && !tallied && st.Do != "crash" && st.Do != "launch" {
				fail("%s: an op needs an expect", at)
			}
		case st.Server != "":
			s.validateServer(at, st, v, fail)
		case st.Check != "":
			if !slices.Contains(checkKinds, st.Check) {
				fail("%s: unknown check %q", at, st.Check)
			}
			if slices.Contains(singleChecks, st.Check) && st.ID == "" {
				fail("%s: check %s needs an id", at, st.Check)
			}
			if st.Expect == nil {
				fail("%s: a check needs an expect", at)
			}
		case st.Parallel != nil:
			if len(st.Parallel) < 2 {
				fail("%s: parallel needs at least two steps", at)
			}
			s.validateStepList(at+" parallel", st.Parallel, v, len(st.Tally) > 0, fail)
		case st.Repeat != nil:
			if st.Repeat.Times < 1 || len(st.Repeat.Steps) == 0 {
				fail("%s: repeat needs times and steps", at)
			}
			s.validateSteps(at+" repeat", st.Repeat.Steps, v, fail)
		case st.Await != "":
			if st.Expect == nil {
				fail("%s: await needs an expect", at)
			}
		}
	}
}

// validateServer checks a conductor action.
func (s *Scenario) validateServer(at string, st *Step, v vocabulary, fail func(string, ...any)) {
	if !slices.Contains(serverActions, st.Server) {
		fail("%s: unknown server action %q", at, st.Server)
		return
	}
	switch st.Server {
	case "advance_clock":
		if len(st.By) == 0 {
			fail("%s: advance_clock needs by", at)
		}
		for _, k := range st.By {
			if _, ok := s.Config[k]; !ok || !v.configKeys[k] {
				fail("%s: advance_clock by %s: the key must be set in config and listed in the reference", at, k)
			}
		}
	case "run_jobs":
		if !slices.Contains(jobKinds, st.Kind) {
			fail("%s: run_jobs kind %q", at, st.Kind)
		}
	case "snapshot_cursors":
		if st.Name == "" {
			fail("%s: snapshot_cursors needs a name", at)
		}
	case "proxy":
		if !v.operations[st.DropNext] {
			fail("%s: drop_next %q is no operationId", at, st.DropNext)
		}
	}
}

// kinds counts the step kinds set on st.
func (st *Step) kinds() int {
	n := 0
	for _, set := range []bool{st.As != "" || st.Call != "", st.Agent != "" || st.Do != "", st.Server != "", st.Check != "", st.Parallel != nil, st.Repeat != nil, st.Await != ""} {
		if set {
			n++
		}
	}
	return n
}

// scenarioDir is the directory of the scenario files.
const scenarioDir = "scenarios"

// loadScenarios reads and validates every file of scenarioDir in name order.
func loadScenarios(v vocabulary) ([]*Scenario, error) {
	entries, err := os.ReadDir(scenarioDir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".yaml") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	var out []*Scenario
	ids := map[string]string{}
	var errs []error
	for _, name := range names {
		b, err := os.ReadFile(filepath.Join(scenarioDir, name)) // #nosec G304 -- a file of the own test directory
		if err != nil {
			return nil, err
		}
		s, err := decodeScenario(b)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
			continue
		}
		if prev, ok := ids[s.ID]; ok {
			errs = append(errs, fmt.Errorf("%s: duplicate id %s (also in %s)", name, s.ID, prev))
		}
		ids[s.ID] = name
		if err := s.Validate(name, v); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
			continue
		}
		out = append(out, s)
	}
	return out, errors.Join(errs...)
}

// TestScenarioFiles_Load: every file under scenarios/ loads strictly and passes Validate, and the
// format rejects the broken documents of the table (unknown key, missing expect, two step kinds,
// unknown operation, unknown op, a clock key that is not configured, an unknown job kind, a case
// without a closing check, a file without cases and without deferred parts, a second document).
func TestScenarioFiles_Load(t *testing.T) {
	t.Parallel()
	v, err := loadVocabulary()
	if err != nil {
		t.Fatal(err)
	}
	scenarios, err := loadScenarios(v)
	if err != nil {
		t.Fatal(err)
	}
	if len(scenarios) != 32 {
		t.Fatalf("%d scenario files", len(scenarios))
	}
	head := "id: AG-99\ntitle: x\nsources: [\"09 16.2\"]\nworld: matrix\nagents:\n  lotse: {agent: Lotse}\n"
	check := "      - check: runs\n        expect: {count: 0}\n"
	for name, doc := range map[string]string{
		"unknown key":             head + "colour: red\ncases:\n  - name: c\n    steps:\n" + check,
		"unknown step key":        head + "cases:\n  - name: c\n    steps:\n      - check: runs\n        expect: {count: 0}\n        colour: red\n",
		"missing expect":          head + "cases:\n  - name: c\n    steps:\n      - as: alex\n        call: getRun\n        with: {run_id: x}\n" + check,
		"call without subject":    head + "cases:\n  - name: c\n    steps:\n      - call: getRun\n        expect: {status: 200}\n" + check,
		"two step kinds":          head + "cases:\n  - name: c\n    steps:\n      - as: alex\n        call: getRun\n        check: runs\n        expect: {status: 200}\n" + check,
		"unknown operation":       head + "cases:\n  - name: c\n    steps:\n      - as: alex\n        call: getRuns\n        expect: {status: 200}\n" + check,
		"unknown op":              head + "cases:\n  - name: c\n    steps:\n      - agent: lotse\n        do: dance\n        expect: {status: 200}\n" + check,
		"unknown agent":           head + "cases:\n  - name: c\n    steps:\n      - agent: texter\n        do: connect\n        expect: {status: 200}\n" + check,
		"clock key not set":       head + "cases:\n  - name: c\n    steps:\n      - server: advance_clock\n        by: agent.ack_timeout\n" + check,
		"unknown job kind":        head + "cases:\n  - name: c\n    steps:\n      - server: run_jobs\n        kind: runs.everything\n" + check,
		"no closing check":        head + "cases:\n  - name: c\n    steps:\n      - server: run_jobs\n        kind: runs.stall_check\n",
		"no cases, no deferred":   head,
		"unknown config key":      head + "config: {agent.colour: red}\ncases:\n  - name: c\n    steps:\n" + check,
		"unknown subject":         head + "cases:\n  - name: c\n    steps:\n      - as: mallory\n        call: getRun\n        expect: {status: 200}\n" + check,
		"unknown check":           head + "cases:\n  - name: c\n    steps:\n      - check: everything\n        expect: {count: 0}\n",
		"single check without id": head + "cases:\n  - name: c\n    steps:\n      - check: run\n        expect: {state: queued}\n",
		"row without its value":   head + "cases:\n  - name: c\n    rows: [{a: 1}]\n    steps:\n      - check: runs\n        expect: {count: \"{{b}}\"}\n",
		"second document":         head + "cases:\n  - name: c\n    steps:\n" + check + "---\nid: AG-98\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s, err := decodeScenario([]byte(doc))
			if err == nil {
				err = s.Validate("ag-99.yaml", v)
			}
			if err == nil {
				t.Fatal("the broken document was accepted")
			}
		})
	}
	s, err := decodeScenario([]byte(head + "cases:\n  - name: c\n    steps:\n" + check))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Validate("ag-98.yaml", v); err == nil {
		t.Fatal("an id that differs from the file name was accepted")
	}
}

// TestScenarioFiles_NoSecretsAndWhiteLabel: no scenario file holds a compact JWS, an API token, a
// private JWK member or an address outside example.com, and every mention names a world handle.
func TestScenarioFiles_NoSecretsAndWhiteLabel(t *testing.T) {
	t.Parallel()
	entries, err := os.ReadDir(scenarioDir)
	if err != nil {
		t.Fatal(err)
	}
	jws := regexp.MustCompile(`[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}`)
	apiToken := regexp.MustCompile(`bothy_[a-z]+_[A-Za-z0-9]`)
	privateJWK := regexp.MustCompile(`"?\bd"?\s*:\s*"[A-Za-z0-9_-]{20,}`)
	email := regexp.MustCompile(`[A-Za-z0-9._%+-]+@([A-Za-z0-9-]+\.)+[A-Za-z]{2,}`)
	mention := regexp.MustCompile(`(?:^|[\s"(])@([a-z][a-z0-9_-]*)`)
	handles := []string{"lotse", "texter", "runner", "assistent", "alex", "sam", "robin", "owner"}
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(scenarioDir, e.Name())) // #nosec G304 -- a file of the own test directory
		if err != nil {
			t.Fatal(err)
		}
		text := string(b)
		for _, re := range []*regexp.Regexp{jws, apiToken, privateJWK} {
			if re.MatchString(text) {
				t.Errorf("%s holds something that looks like a credential (%s)", e.Name(), re)
			}
		}
		for _, m := range email.FindAllString(text, -1) {
			if !strings.HasSuffix(m, "@example.com") {
				t.Errorf("%s names the address %s outside example.com", e.Name(), m)
			}
		}
		for _, m := range mention.FindAllStringSubmatch(text, -1) {
			if !slices.Contains(handles, m[1]) {
				t.Errorf("%s mentions @%s, which is no handle of the world", e.Name(), m[1])
			}
		}
	}
}
