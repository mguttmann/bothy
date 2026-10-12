// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// register is the typed form of architecture/modules.yaml (docs/spec/02-architecture.md,
// "Module register as the source"). A yaml tag without omitempty marks a required key: the node walk
// reports it as missing, so that a forgotten section never silently means "empty".
type register struct {
	Version     int                      `yaml:"version"`
	Exclude     []string                 `yaml:"exclude"`
	Kernel      kernelEntry              `yaml:"kernel"`
	Platform    map[string]platformEntry `yaml:"platform"`
	Wire        vendorsEntry             `yaml:"wire"`
	Cmd         vendorsEntry             `yaml:"cmd"`
	API         *vendorsEntry            `yaml:"api,omitempty"`
	TestSupport testSupportEntry         `yaml:"testsupport"`
	Modules     map[string]moduleEntry   `yaml:"modules"`
	Vendors     map[string][]string      `yaml:"vendors"`

	// moduleOrder is the order of the modules in the file: the layers of 02 section 4.4, which
	// the generated config and cycle messages keep.
	moduleOrder []string
	// platformOrder is the order of the platform packages in the file; only cycle messages use
	// it, the generated config sorts platform packages by name (docs/DECISIONS.md M1-05/6).
	platformOrder []string
	// pos is the position of every YAML path, for problems found after parsing (checkDirectories).
	pos map[string]position
}

// kernelEntry lists the packages of internal/kernel: core packages every package may use, and
// ports that module code reaches through kernel-ports.
type kernelEntry struct {
	Core  []string `yaml:"core"`
	Ports []string `yaml:"ports"`
}

// platformEntry is one package below internal/platform. Both keys are optional because most
// platform packages neither depend on another one nor use a vendor library.
type platformEntry struct {
	DependsOn []string `yaml:"depends_on,omitempty"`
	Vendors   []string `yaml:"vendors,omitempty"`
}

// vendorsEntry names the vendor groups a fixed component (wire, cmd, api) may import.
type vendorsEntry struct {
	Vendors []string `yaml:"vendors"`
}

// testSupportEntry lists the directories of test code across modules (test, perf).
type testSupportEntry struct {
	Paths []string `yaml:"paths"`
}

// moduleEntry is one module of 02 section 4.2 with the data the gates of 02 section 7.3 compare.
type moduleEntry struct {
	Tables          []string `yaml:"tables"`
	Actions         []string `yaml:"actions"`
	Events          []string `yaml:"events"`
	EphemeralEvents []string `yaml:"ephemeral_events"`
	WSOps           []string `yaml:"ws_ops"`
	DependsOn       []string `yaml:"depends_on"`
	AdapterVendors  []string `yaml:"adapter_vendors"`
}

// rootPath names the document itself in a problem, where no YAML path exists.
const rootPath = "(root)"

// problem is one finding in the register, with the position of the node it is about.
type problem struct {
	line, column int
	path, msg    string
}

// format renders the problem as "<file>:<line>:<col>: <yaml path>: <message>", the form editors
// and CI annotations link to. A problem without position (an empty file) omits line and column;
// one with a line but no column (a YAML syntax error) omits the column.
func (p problem) format(file string) string {
	switch {
	case p.line == 0:
		return fmt.Sprintf("%s: %s: %s", file, p.path, p.msg)
	case p.column == 0:
		return fmt.Sprintf("%s:%d: %s: %s", file, p.line, p.path, p.msg)
	default:
		return fmt.Sprintf("%s:%d:%d: %s: %s", file, p.line, p.column, p.path, p.msg)
	}
}

// readRegister reads and validates the register at path.
//
// The error is either an I/O error or lists every problem of the register, one line each in the
// form of problem.format, so that a run shows all mistakes at once.
func readRegister(path string) (*register, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("read register: %w", err)
	}
	r, problems := parseRegister(data)
	if len(problems) > 0 {
		lines := make([]string, 0, len(problems))
		for _, p := range problems {
			lines = append(lines, p.format(path))
		}
		return nil, errors.New(strings.Join(lines, "\n"))
	}
	return r, nil
}

// parseRegister parses and validates register text. It returns the register, or nil and the
// problems sorted by position. It never panics, whatever the input.
//
// The text is read twice: a walk over the yaml.Node tree checks the structure against the Go
// types (unknown and missing keys, duplicate keys, node kinds) with exact positions, which a
// plain decode reports only in part and without YAML paths; then a decode with KnownFields fills
// the typed register, and the semantic rules of validate run on it.
func parseRegister(data []byte) (*register, []problem) {
	root, p := parseDocument(data)
	if p != nil {
		return nil, []problem{*p}
	}
	w := walker{pos: map[string]position{}, order: map[string][]string{}}
	w.walk(root, reflect.TypeFor[register](), "")
	if len(w.problems) > 0 {
		return nil, sortProblems(w.problems)
	}

	r := &register{}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(r); err != nil {
		// The walk accepts only what the decoder accepts; this is a guard, not a rule.
		return nil, []problem{{path: rootPath, msg: "invalid register: " + err.Error()}}
	}
	r.moduleOrder = w.order["modules"]
	r.platformOrder = w.order["platform"]
	r.pos = w.pos

	v := validator{r: r, pos: w.pos}
	v.validate()
	if len(v.problems) > 0 {
		return nil, sortProblems(v.problems)
	}
	return r, nil
}

func sortProblems(problems []problem) []problem {
	slices.SortStableFunc(problems, func(a, b problem) int {
		if a.line != b.line {
			return a.line - b.line
		}
		return a.column - b.column
	})
	return problems
}

var yamlErrorLine = regexp.MustCompile(`line (\d+)`)

// parseDocument returns the top node of the single YAML document in data.
func parseDocument(data []byte) (*yaml.Node, *problem) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var doc yaml.Node
	err := dec.Decode(&doc)
	if errors.Is(err, io.EOF) || (err == nil && len(doc.Content) == 0) {
		return nil, &problem{path: rootPath, msg: "empty register: expected a mapping with version, kernel, modules and the other sections"}
	}
	if err != nil {
		return nil, syntaxProblem(err)
	}
	var extra yaml.Node
	switch err := dec.Decode(&extra); {
	case err == nil:
		return nil, &problem{line: extra.Line, column: extra.Column, path: rootPath, msg: "more than one YAML document; the register is a single document"}
	case !errors.Is(err, io.EOF):
		return nil, syntaxProblem(err)
	}
	return doc.Content[0], nil
}

func syntaxProblem(err error) *problem {
	p := &problem{path: rootPath, msg: "invalid YAML: " + strings.TrimPrefix(err.Error(), "yaml: ")}
	if m := yamlErrorLine.FindStringSubmatch(err.Error()); m != nil {
		p.line, _ = strconv.Atoi(m[1])
	}
	return p
}

// position is a line and column in the register file, both counted from 1.
type position struct{ line, column int }

// walker checks a yaml.Node tree against the Go type it decodes into. It records the position of
// every YAML path (a key's position for mapping entries, the item's position for list entries) so
// that the semantic checks can point at the exact node, and the key order of every mapping.
type walker struct {
	problems []problem
	pos      map[string]position
	order    map[string][]string
}

func (w *walker) add(n *yaml.Node, path, msg string) {
	if path == "" {
		path = rootPath
	}
	w.problems = append(w.problems, problem{line: n.Line, column: n.Column, path: path, msg: msg})
}

func (w *walker) record(n *yaml.Node, path string) {
	if _, ok := w.pos[path]; !ok {
		w.pos[path] = position{line: n.Line, column: n.Column}
	}
}

func (w *walker) walk(n *yaml.Node, t reflect.Type, path string) {
	w.record(n, path)
	if n.Kind == yaml.AliasNode {
		w.add(n, path, "aliases are not allowed in the register; write the value out")
		return
	}
	//exhaustive:ignore // The register types use only these kinds; every other field is a string.
	switch t.Kind() {
	case reflect.Pointer:
		w.walk(n, t.Elem(), path)
	case reflect.Struct:
		if w.expect(n, yaml.MappingNode, path) {
			w.walkStruct(n, t, path)
		}
	case reflect.Map:
		if w.expect(n, yaml.MappingNode, path) {
			w.walkMap(n, t.Elem(), path)
		}
	case reflect.Slice:
		if w.expect(n, yaml.SequenceNode, path) {
			for i, item := range n.Content {
				w.walk(item, t.Elem(), fmt.Sprintf("%s[%d]", path, i))
			}
		}
	case reflect.Int:
		w.walkInt(n, path)
	default: // string, the only other kind the register types use
		if w.expect(n, yaml.ScalarNode, path) && n.ShortTag() != strTag {
			w.add(n, path, "expected a string, found "+n.ShortTag())
		}
	}
}

// strTag is the tag of a plain or quoted string. Any other tag (!!int, !!bool, !!binary, a custom
// tag) makes the decoded text differ from the written one, or the value no string at all; other
// readers of the register (the checks of M1-06) would then see other names than this generator.
const strTag = "!!str"

func (w *walker) walkInt(n *yaml.Node, path string) {
	if n.Kind != yaml.ScalarNode || n.ShortTag() != "!!int" {
		w.add(n, path, "expected an integer, found "+describe(n))
		return
	}
	var v int
	if err := n.Decode(&v); err != nil {
		w.add(n, path, "expected an integer that fits into int, found "+n.Value)
	}
}

// expect reports a problem unless n has the wanted kind; null counts as a kind of its own,
// because an empty value where a list belongs is a mistake, not an empty list.
func (w *walker) expect(n *yaml.Node, kind yaml.Kind, path string) bool {
	if n.Kind == kind && !isNull(n) {
		return true
	}
	w.add(n, path, fmt.Sprintf("expected a %s, found %s", kindName(kind), describe(n)))
	return false
}

func isNull(n *yaml.Node) bool {
	return n.Kind == yaml.ScalarNode && n.ShortTag() == "!!null"
}

var kindNames = map[yaml.Kind]string{
	yaml.DocumentNode: "document",
	yaml.SequenceNode: "list",
	yaml.MappingNode:  "mapping",
	yaml.ScalarNode:   "scalar",
	yaml.AliasNode:    "alias",
}

func kindName(k yaml.Kind) string {
	if name, ok := kindNames[k]; ok {
		return name
	}
	return "node"
}

func describe(n *yaml.Node) string {
	if isNull(n) {
		return "null"
	}
	return "a " + kindName(n.Kind)
}

// yamlField is a struct field as the register file names it.
type yamlField struct {
	typ      reflect.Type
	required bool
}

// yamlFields returns the fields of a struct type by YAML key, in declaration order. Fields
// without a yaml tag (unexported bookkeeping) are not part of the file.
func yamlFields(t reflect.Type) (map[string]yamlField, []string) {
	fields := map[string]yamlField{}
	var names []string
	for f := range t.Fields() {
		tag, ok := f.Tag.Lookup("yaml")
		if !ok {
			continue
		}
		name, opts, _ := strings.Cut(tag, ",")
		fields[name] = yamlField{typ: f.Type, required: !slices.Contains(strings.Split(opts, ","), "omitempty")}
		names = append(names, name)
	}
	return fields, names
}

func join(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

// entries yields the key and value nodes of a mapping, skipping and reporting keys that are no
// scalars, keys that are no plain strings and keys that occur twice. A node parse keeps duplicate
// keys silently, so a second module entry would vanish without a word. A key with another tag
// (null, !!binary, !!int, !!timestamp, the merge key <<) is a scalar whose text may even pass a
// name check, but the typed decode stores it under another name ("" for null, the decoded bytes
// for !!binary) or merges it, so the walker and the decoded register would disagree about which
// entries exist.
func (w *walker) entries(n *yaml.Node, path string, yield func(k, v *yaml.Node, child string)) {
	first := map[string]*yaml.Node{}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		if k.Kind != yaml.ScalarNode {
			w.add(k, path, "key must be a scalar, found "+describe(k))
			continue
		}
		if k.ShortTag() != strTag {
			found := k.ShortTag()
			if isNull(k) {
				found = "null"
			}
			w.add(k, path, "key must be a string, found "+found)
			continue
		}
		child := join(path, k.Value)
		if f, dup := first[k.Value]; dup {
			w.add(k, child, fmt.Sprintf("duplicate key, first defined at %d:%d", f.Line, f.Column))
			continue
		}
		first[k.Value] = k
		w.record(k, child)
		yield(k, v, child)
	}
}

func (w *walker) walkStruct(n *yaml.Node, t reflect.Type, path string) {
	fields, names := yamlFields(t)
	seen := map[string]bool{}
	w.entries(n, path, func(k, v *yaml.Node, child string) {
		seen[k.Value] = true
		f, ok := fields[k.Value]
		if !ok {
			w.add(k, child, "unknown field; allowed: "+strings.Join(names, ", "))
			return
		}
		w.walk(v, f.typ, child)
	})
	at := w.pos[path]
	for _, name := range names {
		if fields[name].required && !seen[name] {
			w.problems = append(w.problems, problem{line: at.line, column: at.column, path: join(path, name), msg: "missing required field"})
		}
	}
}

func (w *walker) walkMap(n *yaml.Node, elem reflect.Type, path string) {
	w.order[path] = []string{}
	w.entries(n, path, func(k, v *yaml.Node, child string) {
		w.order[path] = append(w.order[path], k.Value)
		w.walk(v, elem, child)
	})
}
