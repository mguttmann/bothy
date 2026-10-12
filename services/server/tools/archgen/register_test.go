// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// validRegister is the smallest register that uses every section; each rejection case changes
// one detail of it so that exactly one rule is broken.
const validRegister = `# SPDX-License-Identifier: Apache-2.0
version: 1
exclude: [tools]
kernel:
  core: [ids, clock]
  ports: [txn]
platform:
  pg: {vendors: [pgx]}
  jobs: {depends_on: [pg], vendors: [river]}
wire: {vendors: []}
cmd: {vendors: []}
api: {vendors: []}
testsupport: {paths: [test]}
modules:
  chat:
    tables: [channels]
    actions: [channel.read]
    events: [channel.created]
    ephemeral_events: [channel.typing]
    ws_ops: [typing.start]
    depends_on: []
    adapter_vendors: [pgx]
  agents:
    tables: [agents]
    actions: [agent.read]
    events: [agent.created]
    ephemeral_events: []
    ws_ops: []
    depends_on: [chat]
    adapter_vendors: []
vendors:
  pgx: [github.com/jackc/pgx/v5, github.com/jackc/pgx/v5/**]
  river: [github.com/riverqueue/river]
`

// edit replaces old by new in validRegister; each old must occur exactly once so that a case
// cannot silently change nothing.
func edit(t *testing.T, pairs ...string) string {
	t.Helper()
	text := validRegister
	for i := 0; i+1 < len(pairs); i += 2 {
		if n := strings.Count(text, pairs[i]); n != 1 {
			t.Fatalf("edit: %q occurs %d times in the register, want exactly once", pairs[i], n)
		}
		text = strings.Replace(text, pairs[i], pairs[i+1], 1)
	}
	return text
}

func formatProblems(problems []problem) string {
	lines := make([]string, 0, len(problems))
	for _, p := range problems {
		lines = append(lines, p.format("modules.yaml"))
	}
	return strings.Join(lines, "\n")
}

func TestParseRegisterAcceptsValidRegister(t *testing.T) {
	t.Parallel()
	r, problems := parseRegister([]byte(validRegister))
	if len(problems) != 0 {
		t.Fatalf("unexpected problems:\n%s", formatProblems(problems))
	}
	if r == nil {
		t.Fatal("parseRegister returned neither a register nor a problem")
	}
	if want := []string{"chat", "agents"}; !slices.Equal(r.moduleOrder, want) {
		t.Errorf("moduleOrder = %v, want %v", r.moduleOrder, want)
	}
	if got := r.Modules["agents"].DependsOn; !slices.Equal(got, []string{"chat"}) {
		t.Errorf("agents.depends_on = %v, want [chat]", got)
	}
	if r.API == nil || r.Platform["jobs"].DependsOn[0] != "pg" {
		t.Errorf("api or platform not decoded: %+v", r)
	}
}

func TestProblemFormat(t *testing.T) {
	t.Parallel()
	p := problem{line: 7, column: 5, path: "modules.chat.depends_on[1]", msg: `unknown module "x"`}
	if got, want := p.format("a/modules.yaml"), `a/modules.yaml:7:5: modules.chat.depends_on[1]: unknown module "x"`; got != want {
		t.Errorf("format = %q, want %q", got, want)
	}
	p = problem{path: rootPath, msg: "empty register"}
	if got, want := p.format("m.yaml"), "m.yaml: (root): empty register"; got != want {
		t.Errorf("format without position = %q, want %q", got, want)
	}
}

type rejectCase struct {
	name  string
	text  string
	path  string
	msg   string
	noPos bool // the problem has no line, for example in an empty file
}

func rejectCases(t *testing.T) []rejectCase {
	t.Helper()
	cases := []rejectCase{
		// Structure, found by the node walk.
		{name: "unknown field top level", text: validRegister + "owner: alex\n", path: "owner", msg: "unknown field"},
		{name: "unknown field module", text: edit(t, "    adapter_vendors: [pgx]\n", "    adapter_vendors: [pgx]\n    owner: alex\n"), path: "modules.chat.owner", msg: "unknown field"},
		{name: "unknown field kernel", text: edit(t, "  ports: [txn]\n", "  ports: [txn]\n  extra: [x]\n"), path: "kernel.extra", msg: "unknown field"},
		{name: "unknown field platform", text: edit(t, "pg: {vendors: [pgx]}", "pg: {vendors: [pgx], owner: alex}"), path: "platform.pg.owner", msg: "unknown field"},
		{name: "missing required field module", text: edit(t, "    ws_ops: []\n", ""), path: "modules.agents.ws_ops", msg: "missing required field"},
		{name: "missing required field top level", text: edit(t, "testsupport: {paths: [test]}\n", ""), path: "testsupport", msg: "missing required field"},
		{name: "wrong version", text: edit(t, "version: 1", "version: 2"), path: "version", msg: "unsupported version 2"},
		{name: "version not an integer", text: edit(t, "version: 1", "version: one"), path: "version", msg: "expected an integer"},
		// Built at run time: a 20-digit literal matches the white-label rule for long numeric IDs.
		{name: "version out of range", text: edit(t, "version: 1", "version: "+strconv.FormatUint(math.MaxUint64, 10)), path: "version", msg: "expected an integer"},
		{name: "duplicate module", text: edit(t, "vendors:\n  pgx", "  chat: {}\nvendors:\n  pgx"), path: "modules.chat", msg: "duplicate key"},
		{name: "duplicate field", text: edit(t, "    tables: [channels]\n", "    tables: [channels]\n    tables: [rooms]\n"), path: "modules.chat.tables", msg: "duplicate key"},
		{name: "empty file", text: "", path: rootPath, msg: "empty register", noPos: true},
		{name: "only comments", text: "# SPDX-License-Identifier: Apache-2.0\n", path: rootPath, msg: "empty register", noPos: true},
		{name: "top level not a mapping", text: "- chat\n- agents\n", path: rootPath, msg: "expected a mapping, found a list"},
		{name: "top level scalar", text: "chat\n", path: rootPath, msg: "expected a mapping, found a scalar"},
		{name: "scalar instead of list", text: edit(t, "    tables: [channels]", "    tables: channels"), path: "modules.chat.tables", msg: "expected a list, found a scalar"},
		{name: "null instead of list", text: edit(t, "    depends_on: []\n", "    depends_on:\n"), path: "modules.chat.depends_on", msg: "expected a list, found null"},
		{name: "list instead of mapping", text: edit(t, "wire: {vendors: []}", "wire: [otel]"), path: "wire", msg: "expected a mapping, found a list"},
		{name: "mapping instead of scalar", text: edit(t, "core: [ids, clock]", "core: [ids, {a: b}]"), path: "kernel.core[1]", msg: "expected a scalar, found a mapping"},
		{name: "null list entry", text: edit(t, "core: [ids, clock]", "core: [ids, ~]"), path: "kernel.core[1]", msg: "expected a scalar, found null"},
		{name: "null module key", text: edit(t, "  agents:\n", "  null:\n"), path: "modules", msg: "key must be a string, found null"},
		{name: "null platform key", text: edit(t, "  jobs: {", "  null: {"), path: "platform", msg: "key must be a string, found null"},
		{name: "tilde module key", text: edit(t, "  agents:\n", "  ~:\n"), path: "modules", msg: "key must be a string, found null"},
		{name: "key not a scalar", text: edit(t, "modules:\n", "modules:\n  [a, b]: {}\n"), path: "modules", msg: "key must be a scalar"},
		// A tagged key is decoded differently from its text (base64 for !!binary), so the node walk
		// and the typed decode would see different entries; only plain strings are keys.
		{name: "binary module key", text: edit(t, "  agents:\n", "  !!binary YWdlbnRz:\n"), path: "modules", msg: "key must be a string, found !!binary"},
		{name: "binary vendor group key", text: edit(t, "  river: [", "  !!binary cml2ZXI=: ["), path: "vendors", msg: "key must be a string, found !!binary"},
		{name: "merge key", text: validRegister + "<<: {version: 1}\n", path: rootPath, msg: "key must be a string, found !!merge"},
		{name: "timestamp module key", text: edit(t, "  agents:\n", "  2001-12-14:\n"), path: "modules", msg: "key must be a string, found !!timestamp"},
		{name: "integer module key", text: edit(t, "  agents:\n", "  42:\n"), path: "modules", msg: "key must be a string, found !!int"},
		{name: "boolean platform key", text: edit(t, "  jobs: {", "  true: {"), path: "platform", msg: "key must be a string, found !!bool"},
		{name: "binary list entry", text: edit(t, "    depends_on: [chat]", "    depends_on: [!!binary Y2hhdA==]"), path: "modules.agents.depends_on[0]", msg: "expected a string, found !!binary"},
		{name: "binary import path with invalid UTF-8", text: edit(t, "river: [github.com/riverqueue/river]", "river: [!!binary /w==]"), path: "vendors.river[0]", msg: "expected a string, found !!binary"},
		{name: "integer list entry", text: edit(t, "tables: [channels]", "tables: [42]"), path: "modules.chat.tables[0]", msg: "expected a string, found !!int"},
		{name: "alias", text: edit(t, "    tables: [channels]\n", "    tables: &t [channels]\n", "    tables: [agents]\n", "    tables: *t\n"), path: "modules.agents.tables", msg: "aliases are not allowed"},
		{name: "second document", text: validRegister + "---\nversion: 1\n", path: rootPath, msg: "more than one YAML document"},
		{name: "invalid YAML", text: "version: [1\n", path: rootPath, msg: "invalid YAML"},

		// Names.
		{name: "bad module name", text: edit(t, "  agents:\n", "  Agents:\n"), path: "modules.Agents", msg: "invalid module name"},
		{name: "bad platform name", text: edit(t, "  jobs: {", "  job2: {"), path: "platform.job2", msg: "invalid platform name"},
		{name: "bad vendor group name", text: edit(t, "  river: [", "  river-x: [", "vendors: [river]", "vendors: [river-x]"), path: "vendors.river-x", msg: "invalid vendor group name"},
		{name: "bad kernel package name", text: edit(t, "core: [ids, clock]", "core: [ids, Clock]"), path: "kernel.core[1]", msg: "invalid kernel package name"},
		{name: "bad table name", text: edit(t, "tables: [channels]", "tables: [Channels]"), path: "modules.chat.tables[0]", msg: "invalid table name"},
		{name: "bad action name", text: edit(t, "actions: [channel.read]", "actions: [read]"), path: "modules.chat.actions[0]", msg: "invalid action name"},
		{name: "bad event type", text: edit(t, "    events: [channel.created]", "    events: [channel..created]"), path: "modules.chat.events[0]", msg: "invalid event type"},
		{name: "bad ephemeral event type", text: edit(t, "ephemeral_events: [channel.typing]", "ephemeral_events: [Typing]"), path: "modules.chat.ephemeral_events[0]", msg: "invalid event type"},
		{name: "bad op name", text: edit(t, "ws_ops: [typing.start]", "ws_ops: [typing-start]"), path: "modules.chat.ws_ops[0]", msg: "invalid op name"},
		{name: "duplicate entry in one list", text: edit(t, "tables: [channels]", "tables: [channels, channels]"), path: "modules.chat.tables[1]", msg: `duplicate entry "channels"`},

		// Module graph.
		{name: "depends_on unknown", text: edit(t, "    depends_on: [chat]", "    depends_on: [chat, search]"), path: "modules.agents.depends_on[1]", msg: `unknown module "search"`},
		{name: "depends_on itself", text: edit(t, "    depends_on: []\n", "    depends_on: [chat]\n"), path: "modules.chat.depends_on[0]", msg: "depends on itself"},
		{name: "cycle", text: edit(t, "    depends_on: []\n", "    depends_on: [agents]\n"), path: "modules.chat.depends_on", msg: "dependency cycle chat -> agents -> chat"},
		{name: "cycle rotation starts at first module", text: edit(t,
			"modules:\n", "modules:\n  runs:\n    tables: []\n    actions: []\n    events: []\n    ephemeral_events: []\n    ws_ops: []\n    depends_on: [agents]\n    adapter_vendors: []\n",
			"    depends_on: []\n", "    depends_on: [agents]\n"), path: "modules.chat.depends_on", msg: "dependency cycle chat -> agents -> chat"},
		{name: "cycle of three modules", text: edit(t,
			"modules:\n", "modules:\n  runs:\n    tables: []\n    actions: []\n    events: []\n    ephemeral_events: []\n    ws_ops: []\n    depends_on: [agents]\n    adapter_vendors: []\n",
			"    depends_on: []\n", "    depends_on: [runs]\n"), path: "modules.runs.depends_on", msg: "dependency cycle runs -> agents -> chat -> runs"},

		// Ownership across modules.
		{name: "table in two modules", text: edit(t, "    tables: [agents]", "    tables: [channels]"), path: "modules.agents.tables[0]", msg: `table "channels" is already owned by module chat`},
		{name: "action in two modules", text: edit(t, "actions: [agent.read]", "actions: [channel.read]"), path: "modules.agents.actions[0]", msg: `action "channel.read" is already owned by module chat`},
		{name: "event in two modules", text: edit(t, "    events: [agent.created]", "    events: [channel.created]"), path: "modules.agents.events[0]", msg: `event type "channel.created" is already owned by module chat`},
		{name: "ephemeral event in two modules", text: edit(t, "    ephemeral_events: []\n", "    ephemeral_events: [channel.typing]\n"), path: "modules.agents.ephemeral_events[0]", msg: `event type "channel.typing" is already owned by module chat`},
		{name: "event durable and ephemeral", text: edit(t, "ephemeral_events: [channel.typing]", "ephemeral_events: [channel.created]"), path: "modules.chat.ephemeral_events[0]", msg: `event type "channel.created" is already owned by module chat`},
		{name: "op in two modules", text: edit(t, "    ws_ops: []\n", "    ws_ops: [typing.start]\n"), path: "modules.agents.ws_ops[0]", msg: `op "typing.start" is already owned by module chat`},

		// Vendor groups.
		{name: "undefined vendor group adapter_vendors", text: edit(t, "adapter_vendors: [pgx]", "adapter_vendors: [pgx, redis]"), path: "modules.chat.adapter_vendors[1]", msg: `undefined vendor group "redis"`},
		{name: "undefined vendor group platform", text: edit(t, "vendors: [river]}", "vendors: [redis]}"), path: "platform.jobs.vendors[0]", msg: `undefined vendor group "redis"`},
		{name: "undefined vendor group wire", text: edit(t, "wire: {vendors: []}", "wire: {vendors: [redis]}"), path: "wire.vendors[0]", msg: `undefined vendor group "redis"`},
		{name: "undefined vendor group cmd", text: edit(t, "cmd: {vendors: []}", "cmd: {vendors: [redis]}"), path: "cmd.vendors[0]", msg: `undefined vendor group "redis"`},
		{name: "undefined vendor group api", text: edit(t, "api: {vendors: []}", "api: {vendors: [redis]}"), path: "api.vendors[0]", msg: `undefined vendor group "redis"`},
		{name: "empty vendor group", text: edit(t, "river: [github.com/riverqueue/river]", "river: []"), path: "vendors.river", msg: "must not be empty"},
		{name: "import path with space", text: edit(t, "[github.com/riverqueue/river]", "[github.com/river queue]"), path: "vendors.river[0]", msg: "invalid import path"},
		{name: "import path with inner glob", text: edit(t, "[github.com/riverqueue/river]", "[github.com/**/river]"), path: "vendors.river[0]", msg: "invalid import path"},
		{name: "import path empty", text: edit(t, "[github.com/riverqueue/river]", `[""]`), path: "vendors.river[0]", msg: "invalid import path"},
		// go-arch-lint turns a vendor pattern into a regular expression and escapes only "." and
		// "/", so any other character outside a Go import path could widen the pattern.
		{name: "import path with regex alternation", text: edit(t, "[github.com/riverqueue/river]", `["github.com/jackc/pgx/v5|"]`), path: "vendors.river[0]", msg: "invalid import path"},
		{name: "import path with regex class", text: edit(t, "[github.com/riverqueue/river]", `["[^/]+/**"]`), path: "vendors.river[0]", msg: "invalid import path"},
		{name: "import path with regex group", text: edit(t, "[github.com/riverqueue/river]", `["github.com/(riverqueue)/river"]`), path: "vendors.river[0]", msg: "invalid import path"},
		{name: "import path with regex quantifier", text: edit(t, "[github.com/riverqueue/river]", `["github.com/riverqueue+/river"]`), path: "vendors.river[0]", msg: "invalid import path"},
		{name: "import path without domain", text: edit(t, "[github.com/riverqueue/river]", "[riverqueue/river]"), path: "vendors.river[0]", msg: "invalid import path"},
		{name: "import path with single star", text: edit(t, "[github.com/riverqueue/river]", `["github.com/riverqueue/*"]`), path: "vendors.river[0]", msg: "invalid import path"},
		{name: "vendor glob over a whole hosting domain", text: edit(t, "[github.com/riverqueue/river]", `["github.com/**"]`), path: "vendors.river[0]", msg: "invalid import path"},

		// Platform graph.
		{name: "platform depends_on unknown", text: edit(t, "depends_on: [pg]", "depends_on: [db]"), path: "platform.jobs.depends_on[0]", msg: `unknown platform package "db"`},
		{name: "platform depends_on itself", text: edit(t, "depends_on: [pg]", "depends_on: [jobs]"), path: "platform.jobs.depends_on[0]", msg: "depends on itself"},
		{name: "platform cycle", text: edit(t, "pg: {vendors: [pgx]}", "pg: {depends_on: [jobs], vendors: [pgx]}"), path: "platform.pg.depends_on", msg: "platform dependency cycle pg -> jobs -> pg"},

		// Kernel.
		{name: "kernel core empty", text: edit(t, "core: [ids, clock]", "core: []"), path: "kernel.core", msg: "must not be empty"},
		{name: "kernel ports empty", text: edit(t, "ports: [txn]", "ports: []"), path: "kernel.ports", msg: "must not be empty"},
		{name: "kernel lists overlap", text: edit(t, "ports: [txn]", "ports: [txn, ids]"), path: "kernel.ports[1]", msg: `"ids" is also listed in kernel.core`},

		// Paths.
		{name: "exclude absolute", text: edit(t, "exclude: [tools]", "exclude: [/tools]"), path: "exclude[0]", msg: "invalid path"},
		{name: "exclude parent", text: edit(t, "exclude: [tools]", "exclude: [../tools]"), path: "exclude[0]", msg: "invalid path"},
		{name: "exclude dot dot", text: edit(t, "exclude: [tools]", "exclude: [..]"), path: "exclude[0]", msg: "invalid path"},
		{name: "exclude empty", text: edit(t, "exclude: [tools]", `exclude: [""]`), path: "exclude[0]", msg: "invalid path"},
		{name: "exclude upper case", text: edit(t, "exclude: [tools]", "exclude: [Tools]"), path: "exclude[0]", msg: "invalid path"},
		{name: "testsupport path absolute", text: edit(t, "paths: [test]", "paths: [/test]"), path: "testsupport.paths[0]", msg: "invalid path"},
		{name: "testsupport path parent", text: edit(t, "paths: [test]", "paths: [test/../x]"), path: "testsupport.paths[0]", msg: "invalid path"},
		{name: "testsupport path empty", text: edit(t, "paths: [test]", `paths: [""]`), path: "testsupport.paths[0]", msg: "invalid path"},
		{name: "testsupport paths empty", text: edit(t, "paths: [test]", "paths: []"), path: "testsupport.paths", msg: "must not be empty"},

		// Entries that would take product code out of the architecture gate.
		{name: "exclude internal", text: edit(t, "exclude: [tools]", "exclude: [tools, internal]"), path: "exclude[1]", msg: "takes product code under internal/ out of the architecture gate"},
		{name: "exclude below cmd", text: edit(t, "exclude: [tools]", "exclude: [cmd/bothy]"), path: "exclude[0]", msg: "takes product code under cmd/ out of the architecture gate"},
		{name: "exclude api", text: edit(t, "exclude: [tools]", "exclude: [api]"), path: "exclude[0]", msg: "takes product code under api/ out of the architecture gate"},
		{name: "testsupport below internal", text: edit(t, "paths: [test]", "paths: [test, internal/chat]"), path: "testsupport.paths[1]", msg: "turns product code under internal/ into test support"},
		{name: "testsupport overlaps exclude", text: edit(t, "exclude: [tools]", "exclude: [tools, test/load]"), path: "testsupport.paths[0]", msg: `overlaps exclude entry "test/load"`},
		// go-arch-lint excludes every path that starts with an entry as a string, so "in" takes all
		// of internal/ out of the gate as soon as a directory in/ exists.
		{name: "exclude prefix of internal", text: edit(t, "exclude: [tools]", "exclude: [tools, in]"), path: "exclude[1]", msg: "takes product code under internal/ out of the architecture gate"},
		{name: "exclude prefix of cmd", text: edit(t, "exclude: [tools]", "exclude: [c]"), path: "exclude[0]", msg: "takes product code under cmd/ out of the architecture gate"},
		{name: "exclude prefix of api", text: edit(t, "exclude: [tools]", "exclude: [tools, ap]"), path: "exclude[1]", msg: "takes product code under api/ out of the architecture gate"},
		{name: "testsupport under an exclude prefix", text: edit(t, "exclude: [tools]", "exclude: [tools, te]"), path: "testsupport.paths[0]", msg: `overlaps exclude entry "te"`},
		{name: "vendor pattern without a fixed first element", text: edit(t, "river: [github.com/riverqueue/river]", "river: [\"**\"]"), path: "vendors.river[0]", msg: "invalid import path"},
	}
	for _, name := range []string{"kernel", "platform", "wire", "cmd", "api", "testsupport"} {
		cases = append(cases, rejectCase{
			name: "reserved module name " + name,
			text: edit(t, "  agents:\n", "  "+name+":\n"),
			path: "modules." + name,
			msg:  "reserved",
		})
	}
	return cases
}

func TestRegisterRejects(t *testing.T) {
	t.Parallel()
	for _, tc := range rejectCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r, problems := parseRegister([]byte(tc.text))
			if r != nil {
				t.Errorf("parseRegister returned a register despite problems")
			}
			found := false
			for _, p := range problems {
				if p.path != tc.path || !strings.Contains(p.msg, tc.msg) {
					continue
				}
				found = true
				if !tc.noPos && p.line <= 0 {
					t.Errorf("problem %q has no position", p.format("modules.yaml"))
				}
			}
			if !found {
				t.Fatalf("want a problem at %s containing %q, got:\n%s", tc.path, tc.msg, formatProblems(problems))
			}
		})
	}
}

func TestRegisterRejectsReportsLineAndColumn(t *testing.T) {
	t.Parallel()
	text := edit(t, "    depends_on: [chat]", "    depends_on: [chat, search]")
	_, problems := parseRegister([]byte(text))
	want := `29:24: modules.agents.depends_on[1]: unknown module "search"`
	got := formatProblems(problems)
	if !strings.Contains(got, fmt.Sprintf("modules.yaml:%s", want)) {
		t.Errorf("problems = %q, want one ending in %q", got, want)
	}
}

func TestParseRegisterNeverPanics(t *testing.T) {
	t.Parallel()
	inputs := []string{
		"", "~", "null", "[]", "{}", "version:", "modules:", "modules: {chat: ~}", "modules: {chat: []}",
		"platform: {pg: ~}", "vendors: {pgx: ~}", "api: ~", "kernel: {core: ~}", "a: &a [*a]",
		"<<: {version: 1}", "? [a]\n: b", "version: !!binary aGVsbG8=", "\t", "%YAML 1.2\n---\n",
	}
	for _, in := range inputs {
		_, problems := parseRegister([]byte(in))
		if len(problems) == 0 {
			t.Errorf("parseRegister(%q) found no problem", in)
		}
	}
}

func TestImportPathRule(t *testing.T) {
	t.Parallel()
	for _, s := range []string{
		"github.com/jackc/pgx/v5", "github.com/jackc/pgx/v5/**", "go.yaml.in/yaml/v3", "gopkg.in/yaml.v3",
		"go.opentelemetry.io/otel/**", "github.com/BurntSushi/toml", "golang.org/x/crypto/**",
		"k8s.io/client-go/kubernetes", "github.com/riverqueue/river", "example.com/a_b/c~d/e-f",
	} {
		if !validImportPath(s) {
			t.Errorf("validImportPath(%q) = false, want true", s)
		}
	}
	for _, s := range []string{
		"", "**", "github.com/**", "example.com/**", "github.com/jackc/pgx/v5|", "[^/]+/**", "x/**",
		"github.com/a|b/**", "github.com/(a)/b", "github.com/a+/b", "github.com/a?/b", "github.com/a$",
		"github.com/a^b", "github.com/a{1}/b", `github.com/a\b`, "github.com/*/b", "github.com/a/*",
		"github.com/**/b", "github.com//b", "github.com/.a/b", "github.com/a./b", "github.com/a..b/c",
		"GitHub.com/a/b", "localhost/a/b", "-x.com/a/b", "github.com/a b/c", "github.com/a/b/",
		"github.com/\u00e4/b",
	} {
		if validImportPath(s) {
			t.Errorf("validImportPath(%q) = true, want false", s)
		}
	}
}
