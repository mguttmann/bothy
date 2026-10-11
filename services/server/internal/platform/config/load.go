// SPDX-License-Identifier: Apache-2.0

package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Sources are the inputs of Load besides the built-in defaults.
type Sources struct {
	// Env holds "NAME=value" entries as os.Environ returns them. Only cmd/bothy reads the
	// process environment and passes it here; modules and tests never read it.
	Env []string
	// File is the path from --config. "" means the path in BOTHY_CONFIG_FILE from Env; when
	// that is unset or empty, no file is read.
	File string
	// Flags maps a key to the raw text of its command-line flag. Only "server.role" (--role)
	// has a flag (02 "Konfigurationsladen" item 2).
	Flags map[string]string
}

// flagNames maps the keys the command line may set to their flag.
var flagNames = map[string]string{"server.role": "--role"}

// Load reads the configuration of schema s from the defaults and src, with the precedence
// default < file < environment < flag, and returns the values or a *LoadError that lists every
// problem sorted by key (02 "Konfigurationsladen", 16-betrieb.md 5.1, ADR 0011).
//
// Problems are: a variable with prefix BOTHY_ that is neither the variable of a key, nor
// BOTHY_CONFIG_FILE, nor named by an env: reference in the effective value of a secret_ref key;
// a variable set twice; an unreadable file, invalid YAML, an unknown, duplicate or dotted file
// key, a mapping or list where a value is expected and a scalar where a section is expected;
// anchors, aliases, merge keys and explicit tags in the file; a value that does not parse or
// violates a bound or the enum, in any source, also when a higher source overrides it; a
// required key without a non-empty value; a failed cross-check. Variables without the prefix
// BOTHY_ are ignored (OTEL_* belong to the telemetry SDK). No problem contains a value.
//
// Placeholders <other.key> are expanded in secret_ref defaults with the effective value of the
// other key. In a secret_ref value from the file or the environment only <secrets.dir> is
// expanded, with the effective value of secrets.dir when the schema has that string key, so a
// configured reference reads like the defaults (ruling M1-09/4); a problem about the expanded text
// says "after expanding <secrets.dir>". A placeholder whose key has a problem adds no second
// problem. Load does not resolve secret references and never reads the process environment.
//
// Example:
//
//	values, err := config.Load(schema, config.Sources{Env: os.Environ(), File: configFlag})
//	if err != nil {
//		fmt.Fprintln(os.Stderr, err) // one line per problem
//		return 1
//	}
//	role := values.String("server.role")
func Load(s *Schema, src Sources) (*Values, error) {
	l := &loader{s: s, provided: map[string][]provided{}, failed: map[string]bool{}, rejected: map[string]bool{}, vals: map[string]value{}}
	env := l.readEnv(src.Env)

	file := src.File
	if file == "" {
		file = env[configFileVariable]
	}
	if file != "" {
		l.readFile(file)
	}
	for _, name := range slices.Sorted(maps.Keys(env)) {
		if key, ok := KeyFromEnv(name); ok && s.entries[key] != nil {
			l.provide(key, provided{src: Source{Kind: SourceEnv, Name: name}, text: env[name]})
		}
	}
	l.readFlags(src.Flags)

	l.resolve()
	l.checkUnknownVariables(env)
	l.runChecks()

	if len(l.problems) > 0 {
		slices.SortStableFunc(l.problems, cmpProblems)
		return nil, &LoadError{Problems: l.problems}
	}
	return &Values{schema: s, vals: l.vals, file: file}, nil
}

func cmpProblems(a, b Problem) int {
	if c := strings.Compare(a.Key, b.Key); c != 0 {
		return c
	}
	if c := strings.Compare(a.Source, b.Source); c != 0 {
		return c
	}
	return strings.Compare(a.Message, b.Message)
}

// provided is one text a source gives for a key: a scalar text, or the elements of a YAML
// sequence.
type provided struct {
	src   Source
	text  string
	elems []string
	seq   bool
}

// loader holds the state of one Load call.
type loader struct {
	s        *Schema
	provided map[string][]provided // per key in rising precedence: file, env, flag
	failed   map[string]bool       // keys with a problem in any source
	rejected map[string]bool       // file entries rejected as a whole, "" for the whole file
	vals     map[string]value      // effective values
	problems []Problem
}

func (l *loader) problem(key, source, msg string) {
	l.problems = append(l.problems, Problem{Key: key, Source: source, Message: msg})
}

func (l *loader) keyProblem(key, source, msg string) {
	l.failed[key] = true
	l.problem(key, source, msg)
}

func (l *loader) provide(key string, p provided) {
	l.provided[key] = append(l.provided[key], p)
}

// environmentSource names the environment as a whole, for problems about variables.
const environmentSource = "environment"

// readEnv returns the variables with prefix BOTHY_ by name. A variable that occurs twice is a
// problem of its key: which entry wins would depend on the caller, and a typo must not hide
// behind it.
func (l *loader) readEnv(entries []string) map[string]string {
	env := map[string]string{}
	reported := map[string]bool{}
	for _, entry := range entries {
		name, val, _ := strings.Cut(entry, "=")
		if !strings.HasPrefix(name, envPrefix) {
			continue
		}
		if _, dup := env[name]; dup {
			if !reported[name] {
				reported[name] = true
				l.problem("", environmentSource, "variable "+oneLine(name)+" is set more than once")
			}
			if key, ok := KeyFromEnv(name); ok {
				l.failed[key] = true
			}
			continue
		}
		env[name] = val
	}
	return env
}

func (l *loader) readFlags(flags map[string]string) {
	for _, key := range slices.Sorted(maps.Keys(flags)) {
		flag, ok := flagNames[key]
		switch {
		case !ok:
			l.keyProblem(key, "command line", "no command-line flag sets this key")
		case l.s.entries[key] == nil:
			l.keyProblem(key, "flag "+flag, "unknown key")
		default:
			l.provide(key, provided{src: Source{Kind: SourceFlag, Name: flag}, text: flags[key]})
		}
	}
}

// secretsDirKey is the one key a configured secret_ref value may name as a placeholder (ruling
// M1-09/4): the default references are file:<secrets.dir>/<name> (16-betrieb.md 5.2), and an
// operator who sets one of them should be able to write it the same way. Any other <text> in a
// configured value stays as written; more would be a template language without a user.
const (
	secretsDirKey         = "secrets.dir"
	secretsDirPlaceholder = "<" + secretsDirKey + ">"
)

// resolve parses every provided text of every key, picks the effective value and fills in
// defaults. secret_ref keys come in a second pass, when the string keys their placeholders name
// have their effective values: <other.key> in a default, <secrets.dir> in a configured value.
// NewSchema lets a placeholder name only a string key, so no secret_ref key waits for another.
func (l *loader) resolve() {
	var refs []*keyEntry
	for _, name := range l.s.names {
		e := l.s.entries[name]
		if e.Kind == KindSecretRef {
			refs = append(refs, e)
			continue
		}
		l.resolveKey(e)
	}
	for _, e := range refs {
		if len(l.provided[e.Name]) == 0 && len(e.placeholders) > 0 {
			l.resolvePlaceholders(e)
			continue
		}
		l.resolveKey(e)
	}
}

// resolveKey parses every text the sources provide for e and keeps the last one as the
// effective value, or sets the default when no source sets e. When the text of the highest
// source does not parse, e has no value; every text that does not parse is a problem.
func (l *loader) resolveKey(e *keyEntry) {
	name := e.Name
	list := l.provided[name]
	if len(list) == 0 {
		if e.Required {
			l.keyProblem(name, "none", "is required; set it in the configuration file or with "+EnvName(name))
			return
		}
		l.setDefault(e, e.Default)
		return
	}
	var eff *value
	for _, p := range list {
		p, prefix, ok := l.expandSecretsDir(e, p)
		if !ok {
			// secrets.dir has its own problem already; a second one about an expansion that
			// never happened would point the operator at the wrong key.
			l.failed[name] = true
			eff = nil
			continue
		}
		v, err := l.parse(e, p)
		if err != nil {
			l.keyProblem(name, p.src.String(), prefix+err.Error())
			eff = nil
			continue
		}
		eff = &v
	}
	if eff == nil {
		return
	}
	if e.Required && eff.text == "" {
		l.keyProblem(name, eff.src.String(), "is required and must not be empty")
		return
	}
	l.vals[name] = *eff
}

// expandSecretsDir replaces every <secrets.dir> in a configured text of the secret_ref key e
// with the effective value of secrets.dir and returns the text, the prefix a problem about it
// gets, and false when the text needs secrets.dir while that key has a problem. Texts of other
// kinds, texts without the placeholder and schemas without the string key secrets.dir pass
// unchanged. The inserted text is not scanned again, as in a default.
func (l *loader) expandSecretsDir(e *keyEntry, p provided) (provided, string, bool) {
	if e.Kind != KindSecretRef || p.seq || !strings.Contains(p.text, secretsDirPlaceholder) {
		return p, "", true
	}
	if dir := l.s.entries[secretsDirKey]; dir == nil || dir.Kind != KindString {
		return p, "", true
	}
	dir, ok := l.vals[secretsDirKey]
	if !ok || l.hasProblem(secretsDirKey) {
		return p, "", false
	}
	p.text = strings.ReplaceAll(p.text, secretsDirPlaceholder, dir.v.(string))
	return p, "after expanding " + secretsDirPlaceholder + ": ", true
}

func (l *loader) parse(e *keyEntry, p provided) (value, error) {
	var (
		v     any
		canon string
		err   error
	)
	switch {
	case p.seq && !e.Kind.isList():
		return value{}, errors.New("expected a single value, found a list")
	case p.seq:
		v, canon, err = parseElements(e.Kind, p.elems)
	default:
		v, canon, err = parseValue(e.Kind, e.Enum, p.text)
	}
	if err == nil {
		err = e.checkBounds(v)
	}
	if err == nil {
		err = e.checkKeyRule(v)
	}
	if err != nil {
		return value{}, err
	}
	return value{v: v, text: canon, src: p.src}, nil
}

// checkKeyRule applies the rule of a key that no kind expresses. Only secrets.dir has one: it is
// what <secrets.dir> stands for in every file: reference, so it must turn file:<secrets.dir>/<name>
// into a clean file: reference and must not bring a placeholder of its own (docs/DECISIONS.md
// M1-09/15). Checked here, a bad value is one problem of secrets.dir, and the references that use
// it add none (their expansion waits for a key without problem).
func (e *keyEntry) checkKeyRule(v any) error {
	if e.Name != secretsDirKey || e.Kind != KindString {
		return nil
	}
	return checkSecretsDir(v.(string))
}

// setDefault stores the default of e; NewSchema has already checked that it parses.
func (l *loader) setDefault(e *keyEntry, text string) {
	v, canon, err := parseValue(e.Kind, e.Enum, text)
	if err == nil {
		err = e.checkKeyRule(v)
	}
	if err != nil {
		l.keyProblem(e.Name, "default", err.Error())
		return
	}
	l.vals[e.Name] = value{v: v, text: canon, src: Source{Kind: SourceDefault}}
}

// hasProblem reports whether key has a problem in any source or lies at or below a file entry
// that was rejected as a whole. A cross-check or a placeholder that read its value would only add
// a follow-up problem about a value the operator meant to replace (docs/DECISIONS.md M1-07/8).
func (l *loader) hasProblem(key string) bool {
	if l.failed[key] {
		return true
	}
	name := key
	for {
		if l.rejected[name] {
			return true
		}
		i := strings.LastIndex(name, ".")
		if i < 0 {
			return l.rejected[""]
		}
		name = name[:i]
	}
}

// resolvePlaceholders expands the default of e with the effective values of the keys it names.
// When one of them has a problem in any source, that problem is reported already, and e stays
// without value instead of adding a second problem about an expansion the operator never wrote.
func (l *loader) resolvePlaceholders(e *keyEntry) {
	var names []string
	for _, p := range e.placeholders {
		if _, ok := l.vals[p]; !ok || l.hasProblem(p) {
			l.failed[e.Name] = true
			return
		}
		names = append(names, "<"+p+">")
	}
	text := l.s.expandDefault(e, func(name string) string { return l.vals[name].v.(string) })
	v, canon, err := parseValue(e.Kind, e.Enum, text)
	if err != nil {
		l.keyProblem(e.Name, "default", fmt.Sprintf("default after expanding %s: %v", strings.Join(slices.Compact(names), ", "), err))
		return
	}
	l.vals[e.Name] = value{v: v, text: canon, src: Source{Kind: SourceDefault}}
}

// checkUnknownVariables reports every BOTHY_ variable that is no key variable, not
// BOTHY_CONFIG_FILE and not named by a valid env: reference in an effective secret_ref value
// (16-betrieb.md 5.1 item 3). Only the effective value counts: a reference that a
// higher source overrides exempts nothing.
func (l *loader) checkUnknownVariables(env map[string]string) {
	exempt := map[string]bool{}
	for name, v := range l.vals {
		if l.s.entries[name].Kind == KindSecretRef {
			if variable, ok := envReference(v.text); ok {
				exempt[variable] = true
			}
		}
	}
	for _, name := range slices.Sorted(maps.Keys(env)) {
		if name == configFileVariable || exempt[name] {
			continue
		}
		if key, ok := KeyFromEnv(name); ok && l.s.entries[key] != nil {
			continue
		}
		l.problem("", environmentSource, "unknown variable "+oneLine(name))
	}
}

// runChecks runs every cross-check whose keys all loaded without a problem (see hasProblem). A
// check sees only its own keys, so it cannot depend on a key that was not checked.
func (l *loader) runChecks() {
	for _, c := range l.s.checks {
		view := &Values{schema: l.s, vals: map[string]value{}}
		ready := true
		for _, k := range c.keys {
			v, ok := l.vals[k]
			if !ok || l.hasProblem(k) {
				ready = false
				break
			}
			view.vals[k] = v
		}
		if !ready {
			continue
		}
		if err := c.fn(view); err != nil {
			sources := make([]string, 0, len(c.keys))
			for _, k := range c.keys {
				sources = append(sources, k+" from "+l.vals[k].src.String())
			}
			l.problem(strings.Join(c.keys, ", "), strings.Join(sources, ", "), c.name+": "+err.Error())
		}
	}
}

// readFile reads the configuration file and records the text of every key it sets.
func (l *loader) readFile(path string) {
	src := Source{Kind: SourceFile, Name: path}
	where := src.String()
	data, err := os.ReadFile(path) // #nosec G304 -- the operator names the file with --config or BOTHY_CONFIG_FILE.
	if err != nil {
		l.fileProblem(where, "cannot read the configuration file: "+readReason(err))
		return
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		if !errors.Is(err, io.EOF) {
			l.fileProblem(where, yamlSyntaxMessage(err))
		}
		return
	}
	var extra yaml.Node
	switch err := dec.Decode(&extra); {
	case err == nil:
		l.fileProblem(where, "the configuration file must hold one YAML document")
		return
	case !errors.Is(err, io.EOF):
		l.fileProblem(where, yamlSyntaxMessage(err))
		return
	}
	if len(doc.Content) == 0 {
		return
	}
	root := doc.Content[0]
	if msg := nodeProblem(root); msg != "" {
		l.fileProblem(where, msg)
		return
	}
	switch {
	case isNull(root):
		// An empty document ("---" or "~") sets nothing.
	case root.Kind != yaml.MappingNode:
		l.fileProblem(where, "the configuration file must be a mapping, found "+describe(root))
	default:
		l.walk(root, "", src)
	}
}

// fileProblem reports a problem that discards the whole file. Every key counts as failed then
// (see hasProblem): the file might have set it.
func (l *loader) fileProblem(where, msg string) {
	l.rejected[""] = true
	l.problem("", where, msg)
}

func readReason(err error) string {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "it does not exist"
	case errors.Is(err, fs.ErrPermission):
		return "permission denied"
	}
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Err.Error()
	}
	return err.Error()
}

// yamlSyntaxMessage turns a YAML parse error into a problem text. The scanner and parser of
// go.yaml.in/yaml/v3 use fixed texts with a line number; only the texts about anchors quote the
// input (an anchor name), so those are replaced by a fixed text.
func yamlSyntaxMessage(err error) string {
	msg := strings.TrimPrefix(err.Error(), "yaml: ")
	if strings.Contains(msg, "anchor") {
		return "invalid YAML: anchors and aliases are not allowed"
	}
	return "invalid YAML: " + msg
}

// nodeProblem returns why a node is not allowed in the file, or "". Anchors and aliases would let
// one text stand for several keys, and an explicit tag (!!binary, !!str) would make the text mean
// something other than what the parser of the kind sees.
func nodeProblem(n *yaml.Node) string {
	switch {
	case n.Kind == yaml.AliasNode || n.Anchor != "":
		return "anchors and aliases are not allowed"
	case n.Style&yaml.TaggedStyle != 0:
		return "explicit YAML tags are not allowed"
	}
	return ""
}

func isNull(n *yaml.Node) bool {
	return n.Kind == yaml.ScalarNode && n.ShortTag() == "!!null"
}

func describe(n *yaml.Node) string {
	switch {
	case isNull(n):
		return "null"
	case n.Kind == yaml.ScalarNode:
		return "a scalar"
	case n.Kind == yaml.SequenceNode:
		return "a list"
	case n.Kind == yaml.MappingNode:
		return "a mapping"
	}
	return "an alias"
}

func joinPath(path, name string) string {
	if path == "" {
		return name
	}
	return path + "." + name
}

// quotedKey returns the quoted path of a file key, and false when a segment of name does not
// have the form of a key segment. Only key-shaped text is named in a problem: a YAML typo can make
// a value part of a key ("{role:secret}" without the space after the colon is one key), and that
// text must not reach a message.
func quotedKey(full, name string) (string, bool) {
	if !hasKeyForm(name) {
		return "", false
	}
	return strconv.Quote(full), true
}

// hasKeyForm reports whether every dot-separated segment of name has the form of a key segment.
func hasKeyForm(name string) bool {
	for _, segment := range strings.Split(name, ".") {
		if !segmentPattern.MatchString(segment) {
			return false
		}
	}
	return true
}

// below names the section of a key that is reported by line, or nothing at the top level.
func below(path string) string {
	if path == "" {
		return ""
	}
	return fmt.Sprintf(" below %q", path)
}

// walk visits the mapping n at the section path ("" for the top level).
func (l *loader) walk(n *yaml.Node, path string, src Source) {
	seen := map[string]int{}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		if full, ok := l.fileKey(k, path, src.String(), seen); ok {
			l.walkValue(full, v, src)
		}
	}
}

// fileKey checks the key node k of a mapping at the section path and returns the full name of
// the key it writes, or false after reporting why k names nothing. seen holds the line of every
// key name met so far in the mapping; where names the file in problems. An entry that might have
// set keys, but is not read, is rejected as a whole (see rejectEntry).
func (l *loader) fileKey(k *yaml.Node, path, where string, seen map[string]int) (string, bool) {
	if k.Kind != yaml.ScalarNode || nodeProblem(k) != "" {
		l.problem("", where, fmt.Sprintf("keys must be plain scalars (line %d)", k.Line))
		if k.Kind == yaml.ScalarNode {
			l.rejectEntry(path, k.Value)
		}
		return "", false
	}
	name := k.Value
	full := joinPath(path, name)
	if name == "<<" {
		l.problem("", where, fmt.Sprintf("merge keys are not allowed (line %d)", k.Line))
		l.rejectEntry(path, name)
		return "", false
	}
	quoted, nameable := quotedKey(full, name)
	if first, dup := seen[name]; dup {
		if nameable {
			l.problem("", where, fmt.Sprintf("duplicate key %s at lines %d and %d", quoted, first, k.Line))
		} else {
			l.problem("", where, fmt.Sprintf("duplicate key at lines %d and %d%s", first, k.Line, below(path)))
		}
		l.rejectEntry(path, name)
		return "", false
	}
	seen[name] = k.Line
	if !nameable {
		l.problem("", where, fmt.Sprintf("unknown key at line %d%s", k.Line, below(path)))
		return "", false
	}
	if strings.Contains(name, ".") {
		l.problem("", where, fmt.Sprintf("dotted key %s; nest each segment as its own mapping level", quoted))
		l.rejectEntry(path, name)
		return "", false
	}
	return full, true
}

// rejectEntry rejects the file entry name in the section path as a whole, so that every key it
// might have set counts as failed (see hasProblem). A merge key stands for the section it is in;
// a name without the form of key segments can set no key and rejects nothing.
func (l *loader) rejectEntry(path, name string) {
	switch {
	case name == "<<":
		l.rejected[path] = true
	case hasKeyForm(name):
		l.rejected[joinPath(path, name)] = true
	}
}

// walkValue visits the value node v of the file key full: the value of a key, or the mapping of
// a section.
func (l *loader) walkValue(full string, v *yaml.Node, src Source) {
	where := src.String()
	isKey, isSection := l.s.entries[full] != nil, l.s.sections[full]
	if !isKey && !isSection {
		l.problem("", where, fmt.Sprintf("unknown key %q", full))
		return
	}
	if msg := nodeProblem(v); msg != "" {
		l.rejected[full] = true
		l.problem("", where, fmt.Sprintf("%s (key %q)", msg, full))
		return
	}
	switch {
	case isKey:
		l.fileValue(full, v, src)
	case v.Kind == yaml.MappingNode:
		l.walk(v, full, src)
	default:
		l.rejected[full] = true
		l.problem("", where, fmt.Sprintf("section %q must be a mapping, found %s", full, describe(v)))
	}
}

// fileValue records the value node of a key: a scalar, or a sequence of plain scalars.
func (l *loader) fileValue(key string, v *yaml.Node, src Source) {
	switch v.Kind {
	case yaml.ScalarNode:
		l.provide(key, provided{src: src, text: v.Value})
	case yaml.SequenceNode:
		elems := make([]string, 0, len(v.Content))
		for i, item := range v.Content {
			if item.Kind != yaml.ScalarNode || nodeProblem(item) != "" {
				l.keyProblem(key, src.String(), fmt.Sprintf("element %d must be a plain scalar", i+1))
				return
			}
			elems = append(elems, item.Value)
		}
		l.provide(key, provided{src: src, elems: elems, seq: true})
	case yaml.MappingNode, yaml.DocumentNode, yaml.AliasNode:
		// walk has rejected aliases already; a document node never appears below the root.
		l.keyProblem(key, src.String(), "expected a value, found "+describe(v))
	}
}
