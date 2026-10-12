// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
)

func TestUnknownFileKeyNamesKeyAndFile(t *testing.T) {
	t.Parallel()
	path := writeFile(t, "server:\n  rol: api\n")
	got := problems(t, allKinds(t), config.Sources{File: path})
	wantProblem(t, got, `config: unknown key "server.rol" (source: file `+path+")")

	// An unknown section is reported once, not once per key below it.
	path = writeFile(t, "serverx:\n  role: api\n  listen_addr: 127.0.0.1:1\n")
	got = problems(t, allKinds(t), config.Sources{File: path})
	wantProblem(t, got, `config: unknown key "serverx" (source: file `+path+")")
}

func TestYAMLDuplicateKeyRejected(t *testing.T) {
	t.Parallel()
	path := writeFile(t, "server:\n  role: api\n  role: worker\n")
	got := problems(t, allKinds(t), config.Sources{File: path})
	wantProblem(t, got, `config: duplicate key "server.role" at lines 2 and 3 (source: file `+path+")")

	path = writeFile(t, "server:\n  role: api\nserver:\n  listen_addr: 127.0.0.1:1\n")
	got = problems(t, allKinds(t), config.Sources{File: path})
	wantProblem(t, got, `config: duplicate key "server" at lines 1 and 3 (source: file `+path+")")
}

func TestYAMLDottedKeyRejected(t *testing.T) {
	t.Parallel()
	path := writeFile(t, "sync.event_retention: 30d\n")
	got := problems(t, allKinds(t), config.Sources{File: path})
	wantProblem(t, got, `config: dotted key "sync.event_retention"; nest each segment as its own mapping level (source: file `+path+")")

	path = writeFile(t, "backup:\n  pg.full_schedule: 0 2 * * 0\n")
	got = problems(t, allKinds(t), config.Sources{File: path})
	wantProblem(t, got, `config: dotted key "backup.pg.full_schedule"; nest each segment as its own mapping level (source: file `+path+")")
}

func TestYAMLMappingWhereScalarExpected(t *testing.T) {
	t.Parallel()
	path := writeFile(t, "server:\n  role:\n    name: api\n")
	got := problems(t, allKinds(t), config.Sources{File: path})
	wantProblem(t, got, "config: server.role: expected a value, found a mapping (source: file "+path+")")

	path = writeFile(t, "server:\n  role: [api]\n")
	got = problems(t, allKinds(t), config.Sources{File: path})
	wantProblem(t, got, "config: server.role: expected a single value, found a list (source: file "+path+")")
}

func TestYAMLScalarWhereMappingExpected(t *testing.T) {
	t.Parallel()
	for _, doc := range []string{"server: api\n", "server:\n", "server: [a]\n", "server: ~\n"} {
		path := writeFile(t, doc)
		got := problems(t, allKinds(t), config.Sources{File: path})
		if len(got) != 1 || !strings.HasPrefix(got[0].String(), `config: section "server" must be a mapping, found `) {
			t.Errorf("%q: problems = %v", doc, got)
		}
	}
	for _, doc := range []string{"api\n", "- a\n- b\n"} {
		path := writeFile(t, doc)
		got := problems(t, allKinds(t), config.Sources{File: path})
		if len(got) != 1 || !strings.HasPrefix(got[0].String(), "config: the configuration file must be a mapping, found ") {
			t.Errorf("%q: problems = %v", doc, got)
		}
	}
}

func TestYAMLEmptyFileHasNoValues(t *testing.T) {
	t.Parallel()
	for _, doc := range []string{"", "# only a comment\n", "---\n", "{}\n"} {
		v := mustLoad(t, allKinds(t), config.Sources{File: writeFile(t, doc)})
		if v.Source("server.role").Kind != config.SourceDefault {
			t.Errorf("%q: server.role from %s", doc, v.Source("server.role"))
		}
	}
}

func TestYAMLImplicitTypingKeepsText(t *testing.T) {
	t.Parallel()
	s := mustSchema(t,
		key("a.flag", config.KindString, ""),
		key("a.octal", config.KindString, ""),
		key("a.tilde", config.KindString, ""),
		key("a.version", config.KindString, ""),
		key("a.quoted", config.KindInt, "0"),
		key("a.hex", config.KindInt, "0"),
		key("a.yes", config.KindBool, "false"),
	)
	path := writeFile(t, "a:\n  flag: yes\n  octal: 0o17\n  tilde: ~\n  version: 1.10\n  quoted: \"42\"\n")
	v := mustLoad(t, s, config.Sources{File: path})
	for k, want := range map[string]string{"a.flag": "yes", "a.octal": "0o17", "a.tilde": "~", "a.version": "1.10"} {
		if got := v.String(k); got != want {
			t.Errorf("%s = %q, want the text %q", k, got, want)
		}
	}
	if v.Int("a.quoted") != 42 {
		t.Errorf("a.quoted = %d", v.Int("a.quoted"))
	}
	// YAML would read these as numbers or booleans; the parsers of the kinds do not.
	path = writeFile(t, "a:\n  hex: 0x10\n  yes: on\n")
	got := problems(t, s, config.Sources{File: path})
	if len(got) != 2 || got[0].Key != "a.hex" || got[1].Key != "a.yes" {
		t.Fatalf("problems = %v", got)
	}
}

func TestListFromYAMLSequence(t *testing.T) {
	t.Parallel()
	path := writeFile(t, "server:\n  trusted_proxies:\n    - 192.0.2.0/24\n    - \"2001:db8::/32\"\negress:\n  allowed_hosts: [api.example.com:443, \"a,b\"]\n")
	v := mustLoad(t, allKinds(t), config.Sources{File: path})
	want := []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24"), netip.MustParsePrefix("2001:db8::/32")}
	if got := v.Prefixes("server.trusted_proxies"); !reflect.DeepEqual(got, want) {
		t.Errorf("Prefixes = %v, want %v", got, want)
	}
	if got, want := v.Strings("egress.allowed_hosts"), []string{"api.example.com:443", "a,b"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Strings = %q, want %q", got, want)
	}
	if got := v.Text("egress.allowed_hosts"); got != `["api.example.com:443","a,b"]` {
		t.Errorf("Text = %q", got)
	}

	// A scalar is parsed like the environment form; an empty sequence is the empty list.
	path = writeFile(t, "server:\n  trusted_proxies: 192.0.2.0/24, 198.51.100.0/24\negress:\n  allowed_hosts: []\n")
	v = mustLoad(t, allKinds(t), config.Sources{File: path})
	if got := v.Text("server.trusted_proxies"); got != "192.0.2.0/24, 198.51.100.0/24" {
		t.Errorf("Text = %q", got)
	}
	if got := v.Strings("egress.allowed_hosts"); got == nil || len(got) != 0 {
		t.Errorf("Strings = %#v, want an empty list", got)
	}

	// Elements must be non-empty scalars.
	for _, doc := range []string{
		"egress:\n  allowed_hosts:\n    - a\n    - [b]\n",
		"egress:\n  allowed_hosts:\n    - a\n    - {b: c}\n",
		"egress:\n  allowed_hosts:\n    - a\n    - \"\"\n",
		"server:\n  trusted_proxies:\n    - 192.0.2.1/24\n",
	} {
		got := problems(t, allKinds(t), config.Sources{File: writeFile(t, doc)})
		if len(got) != 1 || (got[0].Key != "egress.allowed_hosts" && got[0].Key != "server.trusted_proxies") {
			t.Errorf("%q: problems = %v", doc, got)
		}
	}
}

func TestYAMLAliasesTagsAndMergeKeysRejected(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"base: &b api\nserver:\n  role: *b\n":     `config: unknown key "base"`,
		"server: &s\n  role: api\n":               "anchors and aliases are not allowed",
		"server:\n  role: !!str api\n":            "explicit YAML tags are not allowed",
		"server:\n  <<: {role: api}\n":            "merge keys are not allowed",
		"server:\n  ? [role]\n  : api\n":          "keys must be plain scalars",
		"server:\n  role: api\nother: *missing\n": "anchors and aliases are not allowed",
	}
	for doc, want := range cases {
		got := problems(t, allKinds(t), config.Sources{File: writeFile(t, doc)})
		all := (&config.LoadError{Problems: got}).Error()
		if !strings.Contains(all, want) {
			t.Errorf("%q: problems\n%s\nwant one containing %q", doc, all, want)
		}
	}
}

func TestYAMLSyntaxErrorNeverEchoesValue(t *testing.T) {
	t.Parallel()
	for _, doc := range []string{
		"server:\n  role: " + canary + "\n bad indent: " + canary + "\n",
		"server: {role: " + canary + "\n",
		"server:\n  role: \"" + canary + "\n",
		"server:\n\trole: " + canary + "\n",
		"server: *" + canary + "\n",
		"server:\n  role: " + canary + "\n---\nmore: " + canary + "\n",
		"&" + canary + " [a]: b\n",
		"server: !<" + canary + "> x\n",
	} {
		path := writeFile(t, doc)
		got := problems(t, allKinds(t), config.Sources{File: path})
		all := (&config.LoadError{Problems: got}).Error()
		if strings.Contains(all, canary) {
			t.Errorf("%q: problem echoes the value:\n%s", doc, all)
		}
		if !strings.Contains(all, "(source: file "+path+")") {
			t.Errorf("%q: problem does not name the file:\n%s", doc, all)
		}
	}
}

func TestYAMLMultipleDocumentsRejected(t *testing.T) {
	t.Parallel()
	path := writeFile(t, "server:\n  role: api\n---\nserver:\n  role: worker\n")
	got := problems(t, allKinds(t), config.Sources{File: path})
	wantProblem(t, got, "config: the configuration file must hold one YAML document (source: file "+path+")")
}

func TestConfigFileUnreadable(t *testing.T) {
	t.Parallel()
	path := writeFile(t, "server:\n  role: api\n")
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatal(err)
	}
	if f, err := os.Open(filepath.Clean(path)); err == nil {
		_ = f.Close()
		t.Skip("the file stays readable (running as root); permission check not possible here")
	}
	got := problems(t, allKinds(t), config.Sources{File: path})
	wantProblem(t, got, "config: cannot read the configuration file: permission denied (source: file "+path+")")
}

func TestYAMLRootAndSecondDocumentProblems(t *testing.T) {
	t.Parallel()
	path := writeFile(t, "&root\nserver:\n  role: api\n")
	got := problems(t, allKinds(t), config.Sources{File: path})
	wantProblem(t, got, "config: anchors and aliases are not allowed (source: file "+path+")")

	path = writeFile(t, "server:\n  role: api\n---\nserver: {role: [\n")
	got = problems(t, allKinds(t), config.Sources{File: path})
	if len(got) != 1 || !strings.HasPrefix(got[0].Message, "invalid YAML: ") {
		t.Fatalf("problems = %v, want one invalid YAML problem", got)
	}
}

func TestFileKeyTypoNeverEchoesValue(t *testing.T) {
	t.Parallel()
	// A YAML typo can turn a value into part of a key. Only text in the form of a key name is
	// quoted; everything else is reported by line.
	cases := map[string]string{
		"server: {role:" + canary + "}\n":                                 `config: unknown key at line 1 below "server"`,
		"server:\n  role " + canary + ": x\n":                             `config: unknown key at line 2 below "server"`,
		canary + ": x\n":                                                  "config: unknown key at line 1 (source",
		"server:\n  \"x " + canary + "\": 1\n  \"x " + canary + "\": 2\n": `config: duplicate key at lines 2 and 3 below "server"`,
		"server:\n  role." + canary + ": x\n":                             `config: unknown key at line 2 below "server"`,
	}
	for doc, want := range cases {
		path := writeFile(t, doc)
		got := problems(t, allKinds(t), config.Sources{File: path})
		all := (&config.LoadError{Problems: got}).Error()
		if strings.Contains(all, canary) || !strings.Contains(all, want) {
			t.Errorf("%q: problems\n%s\nwant %q and no value", doc, all, want)
		}
	}
}

func TestProblemWithControlCharactersInPathStaysOneLine(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "a\nconfig: server.role: forged")
	got := problems(t, allKinds(t), config.Sources{File: path})
	if len(got) != 1 || strings.Contains(got[0].String(), "\n") || !strings.Contains(got[0].Source, strconv.Quote(path)) {
		t.Fatalf("problems = %q", got)
	}
}

func TestRejectedFileEntrySkipsCrossChecks(t *testing.T) {
	t.Parallel()
	s := mustSchema(t, enumKey(),
		key("server.listen_addr", config.KindListenAddr, "127.0.0.1:8080"),
		key("server.internal_listen_addr", config.KindListenAddr, "127.0.0.1:8081"))
	ran := 0
	if err := s.AddCheck("distinct", []string{"server.listen_addr", "server.internal_listen_addr"}, func(v *config.Values) error {
		ran++
		if v.String("server.listen_addr") == v.String("server.internal_listen_addr") {
			return errors.New("the listeners must differ")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// The environment moves the internal listener onto the default of the public one, and the
	// file would move the public listener away. When the file entry is rejected, its problem is
	// the only one: the check would compare a value the operator replaced (docs/DECISIONS.md
	// M1-07/8).
	env := []string{"BOTHY_SERVER__INTERNAL_LISTEN_ADDR=127.0.0.1:8080"}
	got := problems(t, s, config.Sources{Env: env})
	wantProblem(t, got, "config: server.listen_addr, server.internal_listen_addr: distinct: the listeners must differ "+
		"(source: server.listen_addr from default, server.internal_listen_addr from env BOTHY_SERVER__INTERNAL_LISTEN_ADDR)")
	cases := map[string]string{
		"server: !!map\n  listen_addr: 127.0.0.1:9000\n":                          "explicit YAML tags are not allowed",
		"server: &s\n  listen_addr: 127.0.0.1:9000\n":                             "anchors and aliases are not allowed",
		"server:\n  role: api\nserver:\n  listen_addr: 127.0.0.1:9000\n":          `duplicate key "server"`,
		"server: [listen_addr]\n":                                                 `section "server" must be a mapping`,
		"server.listen_addr: 127.0.0.1:9000\n":                                    `dotted key "server.listen_addr"`,
		"!!str server:\n  listen_addr: 127.0.0.1:9000\n":                          "keys must be plain scalars",
		"server:\n  <<: {listen_addr: 127.0.0.1:9000}\n":                          "merge keys are not allowed",
		"<<: {server: {listen_addr: 127.0.0.1:9000}}\n":                           "merge keys are not allowed",
		"server:\n  listen_addr: !!str 127.0.0.1:9000\n":                          "explicit YAML tags are not allowed",
		"server:\n  listen_addr: 127.0.0.1:9000\n  listen_addr: 127.0.0.1:9001\n": `duplicate key "server.listen_addr"`,
		"server:\n  !!merge <<: {listen_addr: 127.0.0.1:9000}\n":                  "keys must be plain scalars",
		// The whole file is rejected: every key it might have set counts as failed.
		"server: [\n": "invalid YAML",
		"server:\n  role: api\n---\nserver: {}\n": "must hold one YAML document",
		"- server\n": "must be a mapping, found a list",
		"&root\nserver:\n  listen_addr: 127.0.0.1:9000\n": "anchors and aliases are not allowed",
		"!!map\nserver:\n  listen_addr: 127.0.0.1:9000\n": "explicit YAML tags are not allowed",
	}
	for doc, want := range cases {
		ran = 0
		got := problems(t, s, config.Sources{File: writeFile(t, doc), Env: env})
		if len(got) != 1 || !strings.Contains(got[0].String(), want) || ran != 0 {
			t.Errorf("%q: check ran %d times, problems %v; want only one containing %q", doc, ran, got, want)
		}
	}
	ran = 0
	got = problems(t, s, config.Sources{File: filepath.Join(t.TempDir(), "missing.yaml"), Env: env})
	if len(got) != 1 || !strings.Contains(got[0].String(), "it does not exist") || ran != 0 {
		t.Errorf("missing file: check ran %d times, problems %v", ran, got)
	}

	// An entry that cannot set a key rejects nothing: the check still runs and reports the
	// conflict next to the problem of the entry.
	ran = 0
	got = problems(t, s, config.Sources{File: writeFile(t, "!!str \"\": x\n"), Env: env})
	if len(got) != 2 || ran != 1 {
		t.Errorf("tagged empty key: check ran %d times, problems %v; want the key problem and the conflict", ran, got)
	}

	// A variable set twice: the check runs on neither entry.
	ran = 0
	got = problems(t, s, config.Sources{Env: []string{env[0], "BOTHY_SERVER__INTERNAL_LISTEN_ADDR=127.0.0.1:9000"}})
	if len(got) != 1 || ran != 0 {
		t.Errorf("duplicate variable: check ran %d times, problems %v", ran, got)
	}
}

func TestRejectedFileEntrySkipsPlaceholders(t *testing.T) {
	t.Parallel()
	s := mustSchema(t, key("store.dir", config.KindString, "/run/store"),
		key("database.password_ref", config.KindSecretRef, "file:<store.dir>/db"))
	// The environment sets a directory that the expanded reference rejects, so expanding the
	// placeholder reports a problem of its own.
	env := []string{"BOTHY_STORE__DIR=relative"}
	got := problems(t, s, config.Sources{Env: env})
	if len(got) != 1 || !strings.Contains(got[0].String(), "default after expanding <store.dir>") {
		t.Fatalf("problems %v, want only the expansion problem", got)
	}
	// When a file entry at or above store.dir is rejected, store.dir counts as failed and the
	// placeholder is not expanded: the problem of the entry is the only one (docs/DECISIONS.md
	// M1-07/25).
	cases := map[string]string{
		"store: !!map\n  dir: /run/other\n":              "explicit YAML tags are not allowed",
		"store: &s\n  dir: /run/other\n":                 "anchors and aliases are not allowed",
		"store:\n  dir: /run/other\nstore:\n  dir: /x\n": `duplicate key "store"`,
		"store: [dir]\n":                                 `section "store" must be a mapping`,
		"store.dir: /run/other\n":                        `dotted key "store.dir"`,
		"store:\n  dir: !!str /run/other\n":              "explicit YAML tags are not allowed",
		"store:\n  dir: /run/other\n  dir: /run/third\n": `duplicate key "store.dir"`,
		"store:\n  <<: {dir: /run/other}\n":              "merge keys are not allowed",
		"store: [\n":                                     "invalid YAML",
		"!!map\nstore:\n  dir: /run/other\n":             "explicit YAML tags are not allowed",
		"store:\n  dir: /run/other\n---\nstore: {}\n":    "must hold one YAML document",
		"&root\nstore:\n  dir: /run/other\n":             "anchors and aliases are not allowed",
		"- store\n":                                      "must be a mapping, found a list",
	}
	for doc, want := range cases {
		got := problems(t, s, config.Sources{File: writeFile(t, doc), Env: env})
		if len(got) != 1 || !strings.Contains(got[0].String(), want) {
			t.Errorf("%q: problems %v; want only one containing %q", doc, got, want)
		}
	}
}
