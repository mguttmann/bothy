// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io/fs"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// Register paths and names that several checks share.
const (
	testSupportPaths = "testsupport.paths"
	kernelPortsPath  = "kernel.ports"
	dependsOnKey     = ".depends_on"
	eventTypeWhat    = "event type"
)

// rule is a naming rule for one kind of register entry.
type rule struct {
	what  string
	valid func(string) bool
	want  string
}

func patternRule(what, expr string) rule {
	re := regexp.MustCompile(expr)
	return rule{what: what, valid: re.MatchString, want: expr}
}

// The patterns keep every name usable as a component name, a Go package path or a key of the
// gates in 02 section 7.3.
var (
	moduleRule      = patternRule("module name", `^[a-z]+$`)
	platformRule    = patternRule("platform name", `^[a-z]+$`)
	vendorGroupRule = patternRule("vendor group name", `^[a-z0-9]+$`)
	kernelRule      = patternRule("kernel package name", `^[a-z]+$`)
	tableRule       = patternRule("table name", `^[a-z][a-z0-9_]*$`)
	actionRule      = patternRule("action name", `^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`)
	eventRule       = patternRule(eventTypeWhat, `^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`)
	opRule          = patternRule("op name", `^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*$`)
	pathRule        = patternRule("path", `^[a-z0-9_-]+(/[a-z0-9_-]+)*$`)
	importPathRule  = rule{
		what:  "import path",
		valid: validImportPath,
		want: "a Go import path: a domain with a dot, then elements of letters, digits and - . _ ~; " +
			"** only as the last element, after at least two fixed elements",
	}
)

// reservedModules are the names of the fixed components of the generated config; a module with one
// of these names would produce a second component of the same name (the generated config would be
// ambiguous).
var reservedModules = []string{"kernel", "platform", componentWire, componentCmd, componentAPI, componentTestSupport}

// The elements of a vendor pattern. go-arch-lint turns each pattern into a regular expression and
// escapes only "." and "/", so a pattern may hold nothing but the characters of a Go import path
// (docs/DECISIONS.md M1-05/17); none of them is special in a regular expression.
var (
	// importDomain is the first element: a lower-case host name with at least one dot, as Go
	// requires for module paths outside the standard library.
	importDomain = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$`)
	// importElement is any later element: letters, digits, "-", "_", "~" and inner dots.
	importElement = regexp.MustCompile(`^[A-Za-z0-9_~-]+(\.[A-Za-z0-9_~-]+)*$`)
)

// minGlobBase is the number of fixed elements a pattern ending in ** needs, so that a pattern
// never opens a whole hosting domain such as github.com/**.
const minGlobBase = 2

// validImportPath accepts a Go import path, optionally ending in the glob ** that go-arch-lint
// expands to every package below it.
func validImportPath(s string) bool {
	fixed := strings.Split(s, "/")
	if fixed[len(fixed)-1] == "**" {
		fixed = fixed[:len(fixed)-1]
		if len(fixed) < minGlobBase {
			return false
		}
	}
	if !importDomain.MatchString(fixed[0]) {
		return false
	}
	for _, e := range fixed[1:] {
		if !importElement.MatchString(e) {
			return false
		}
	}
	return true
}

// validator applies the semantic rules of the register to a structurally valid register.
type validator struct {
	r        *register
	pos      map[string]position
	problems []problem
}

func (v *validator) add(path, format string, args ...any) {
	at := v.pos[path]
	v.problems = append(v.problems, problem{line: at.line, column: at.column, path: path, msg: fmt.Sprintf(format, args...)})
}

func index(path string, i int) string {
	return fmt.Sprintf("%s[%d]", path, i)
}

// list checks every entry of a list against a naming rule and reports repeated entries.
func (v *validator) list(path string, items []string, r rule) {
	first := map[string]int{}
	for i, s := range items {
		if !r.valid(s) {
			v.add(index(path, i), "invalid %s %q (want %s)", r.what, s, r.want)
		}
		if j, dup := first[s]; dup {
			v.add(index(path, i), "duplicate entry %q, first at %s", s, index(path, j))
			continue
		}
		first[s] = i
	}
}

func (v *validator) nonEmpty(path string, items []string) {
	if len(items) == 0 {
		v.add(path, "must not be empty")
	}
}

func (v *validator) validate() {
	if v.r.Version != 1 {
		v.add("version", "unsupported version %d, expected 1", v.r.Version)
	}
	v.list("exclude", v.r.Exclude, pathRule)
	v.validateKernel()
	v.validateVendors()
	v.validatePlatform()
	v.vendorRefs("wire.vendors", v.r.Wire.Vendors)
	v.vendorRefs("cmd.vendors", v.r.Cmd.Vendors)
	if v.r.API != nil {
		v.vendorRefs("api.vendors", v.r.API.Vendors)
	}
	v.nonEmpty(testSupportPaths, v.r.TestSupport.Paths)
	v.list(testSupportPaths, v.r.TestSupport.Paths, pathRule)
	v.validateScope()
	v.validateModules()
}

// productRoots hold the product code that the architecture gate checks; see validateScope.
var productRoots = []string{"internal", componentCmd, componentAPI}

// within reports whether dir is root or lies below it (both slash-separated relative paths).
func within(dir, root string) bool {
	return dir == root || strings.HasPrefix(dir, root+"/")
}

// excludes reports whether the exclude entry ex takes the path p out of go-arch-lint, or a part of
// it: go-arch-lint skips every path that starts with an entry as a string, not only the entry and
// the directories below it (docs/DECISIONS.md M1-05/18), and an entry inside p takes that part.
func excludes(ex, p string) bool {
	return strings.HasPrefix(p, ex) || within(ex, p)
}

// validateScope rejects exclude and testsupport entries that reach into product code: an
// excluded directory is not checked at all, and test support may import anything, so either
// would switch the gate off for that code with one register line. Test support must also not
// overlap an excluded directory, or part of it would silently go unchecked. Directories that only
// exist on disk are checked by checkDirectories.
func (v *validator) validateScope() {
	for i, dir := range v.r.Exclude {
		v.excludeScope(i, dir)
	}
	for i, dir := range v.r.TestSupport.Paths {
		v.testSupportScope(i, dir)
	}
}

// excludeScope reports an exclude entry that takes product code out of the gate.
func (v *validator) excludeScope(i int, dir string) {
	for _, root := range productRoots {
		if excludes(dir, root) {
			v.add(index("exclude", i), "%q takes product code under %s/ out of the architecture gate "+
				"(go-arch-lint excludes every path that starts with the entry)", dir, root)
		}
	}
}

// testSupportScope reports a test support entry that covers product code or an excluded directory.
func (v *validator) testSupportScope(i int, dir string) {
	for _, root := range productRoots {
		if within(dir, root) {
			v.add(index(testSupportPaths, i), "%q turns product code under %s/ into test support, which may import anything", dir, root)
		}
	}
	for _, ex := range v.r.Exclude {
		if excludes(ex, dir) {
			v.add(index(testSupportPaths, i), "%q overlaps exclude entry %q", dir, ex)
		}
	}
}

// checkDirectories reports exclude entries that are a string prefix of another directory below
// root, the module root that holds the generated config: go-arch-lint would exclude that
// directory too (next to tools/, the entry "tools" also takes toolsx/ out of the gate), and
// whatever code lands there later would go unchecked. The problems carry the position of the
// entry in the register; the error is only a failure to read the tree.
func checkDirectories(r *register, root string) ([]problem, error) {
	var problems []problem
	err := filepath.WalkDir(root, func(name string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() || name == root {
			return err
		}
		rel, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		for i, ex := range r.Exclude {
			if strings.HasPrefix(rel, ex) && !within(rel, ex) {
				at := r.pos[index("exclude", i)]
				problems = append(problems, problem{line: at.line, column: at.column, path: index("exclude", i),
					msg: fmt.Sprintf("%q also takes %s/ out of the architecture gate (go-arch-lint excludes every "+
						"path that starts with the entry); rename the directory or the entry", ex, rel)})
				return fs.SkipDir
			}
		}
		return nil
	})
	return problems, err
}

func (v *validator) validateKernel() {
	k := v.r.Kernel
	v.nonEmpty("kernel.core", k.Core)
	v.nonEmpty(kernelPortsPath, k.Ports)
	v.list("kernel.core", k.Core, kernelRule)
	v.list(kernelPortsPath, k.Ports, kernelRule)
	for i, p := range k.Ports {
		if slices.Contains(k.Core, p) {
			v.add(index(kernelPortsPath, i), "%q is also listed in kernel.core; a package is either core or port", p)
		}
	}
}

func (v *validator) validateVendors() {
	for _, g := range slices.Sorted(maps.Keys(v.r.Vendors)) {
		path := "vendors." + g
		if !vendorGroupRule.valid(g) {
			v.add(path, "invalid vendor group name %q (want %s)", g, vendorGroupRule.want)
		}
		v.nonEmpty(path, v.r.Vendors[g])
		v.list(path, v.r.Vendors[g], importPathRule)
	}
}

// vendorRefs checks a list of vendor group names against the groups the register defines.
func (v *validator) vendorRefs(path string, groups []string) {
	v.list(path, groups, vendorGroupRule)
	for i, g := range groups {
		if _, ok := v.r.Vendors[g]; !ok {
			v.add(index(path, i), "undefined vendor group %q; add it under vendors", g)
		}
	}
}

func (v *validator) validatePlatform() {
	for _, p := range v.r.platformOrder {
		path := "platform." + p
		if !platformRule.valid(p) {
			v.add(path, "invalid platform name %q (want %s)", p, platformRule.want)
		}
		e := v.r.Platform[p]
		v.list(path+dependsOnKey, e.DependsOn, platformRule)
		for i, d := range e.DependsOn {
			_, known := v.r.Platform[d]
			v.dependency(index(path+dependsOnKey, i), p, d, known, "platform package")
		}
		v.vendorRefs(path+".vendors", e.Vendors)
	}
	for _, c := range findCycles(v.r.platformOrder, func(p string) []string { return v.r.Platform[p].DependsOn }) {
		v.add("platform."+c[0]+dependsOnKey, "platform dependency cycle %s", cycleText(c))
	}
}

// dependency checks one depends_on entry of from; known says whether the target exists.
func (v *validator) dependency(path, from, to string, known bool, what string) {
	switch {
	case to == from:
		v.add(path, "%s depends on itself", from)
	case !known:
		v.add(path, "unknown %s %q", what, to)
	}
}

// owner is the module and list entry that declared a table, action, event type or op first.
type owner struct{ module, path string }

// own records the entries of one list and reports those another list declared before. Repeats
// within the same list are left to list, which reports them as duplicate entries.
func (v *validator) own(owners map[string]owner, what, path, module string, items []string) {
	for i, s := range items {
		first, taken := owners[s]
		switch {
		case !taken:
			owners[s] = owner{module: module, path: index(path, i)}
		case !strings.HasPrefix(first.path, path+"["):
			v.add(index(path, i), "%s %q is already owned by module %s (%s)", what, s, first.module, first.path)
		}
	}
}

func (v *validator) validateModules() {
	tables, actions, events, ops := map[string]owner{}, map[string]owner{}, map[string]owner{}, map[string]owner{}
	for _, m := range v.r.moduleOrder {
		path := "modules." + m
		switch {
		case slices.Contains(reservedModules, m):
			v.add(path, "module name %q is reserved for a fixed component (%s)", m, strings.Join(reservedModules, ", "))
		case !moduleRule.valid(m):
			v.add(path, "invalid module name %q (want %s)", m, moduleRule.want)
		}
		e := v.r.Modules[m]
		v.list(path+".tables", e.Tables, tableRule)
		v.list(path+".actions", e.Actions, actionRule)
		v.list(path+".events", e.Events, eventRule)
		v.list(path+".ephemeral_events", e.EphemeralEvents, eventRule)
		v.list(path+".ws_ops", e.WSOps, opRule)
		v.list(path+dependsOnKey, e.DependsOn, moduleRule)
		for i, d := range e.DependsOn {
			_, known := v.r.Modules[d]
			v.dependency(index(path+dependsOnKey, i), m, d, known, "module")
		}
		v.vendorRefs(path+".adapter_vendors", e.AdapterVendors)

		v.own(tables, "table", path+".tables", m, e.Tables)
		v.own(actions, "action", path+".actions", m, e.Actions)
		v.own(events, eventTypeWhat, path+".events", m, e.Events)
		v.own(events, eventTypeWhat, path+".ephemeral_events", m, e.EphemeralEvents)
		v.own(ops, "op", path+".ws_ops", m, e.WSOps)
	}
	for _, c := range findCycles(v.r.moduleOrder, func(m string) []string { return v.r.Modules[m].DependsOn }) {
		v.add("modules."+c[0]+dependsOnKey, "dependency cycle %s", cycleText(c))
	}
}

func cycleText(c []string) string {
	return strings.Join(append(slices.Clone(c), c[0]), " -> ")
}

// findCycles returns the dependency cycles a depth-first search over order finds, each once and
// rotated to start at the member that comes first in order, so that the message does not depend
// on where the search entered the cycle. Unknown names and self-dependencies are skipped; the
// caller reports them on their own.
func findCycles(order []string, edges func(string) []string) [][]string {
	f := cycleFinder{rank: map[string]int{}, state: map[string]int{}, seen: map[string]bool{}, edges: edges}
	for i, n := range order {
		f.rank[n] = i
	}
	for _, n := range order {
		if f.state[n] == nodeUnvisited {
			f.visit(n)
		}
	}
	return f.cycles
}

// The states of a node in the depth-first search of findCycles.
const (
	nodeUnvisited = iota
	nodeOnStack
	nodeDone
)

// cycleFinder holds the state of one findCycles search.
type cycleFinder struct {
	rank   map[string]int
	state  map[string]int
	seen   map[string]bool
	edges  func(string) []string
	stack  []string
	cycles [][]string
}

func (f *cycleFinder) visit(n string) {
	f.state[n] = nodeOnStack
	f.stack = append(f.stack, n)
	for _, d := range f.edges(n) {
		if _, known := f.rank[d]; !known || d == n {
			continue
		}
		switch f.state[d] {
		case nodeUnvisited:
			f.visit(d)
		case nodeOnStack:
			f.record(rotate(f.stack[slices.Index(f.stack, d):], f.rank))
		}
	}
	f.stack = f.stack[:len(f.stack)-1]
	f.state[n] = nodeDone
}

// record keeps a cycle the first time the search finds it.
func (f *cycleFinder) record(c []string) {
	if key := strings.Join(c, " "); !f.seen[key] {
		f.seen[key] = true
		f.cycles = append(f.cycles, c)
	}
}

// rotate returns a copy of the cycle that starts at its member with the lowest rank.
func rotate(cycle []string, rank map[string]int) []string {
	start := 0
	for i, n := range cycle {
		if rank[n] < rank[cycle[start]] {
			start = i
		}
	}
	return append(slices.Clone(cycle[start:]), cycle[:start]...)
}
