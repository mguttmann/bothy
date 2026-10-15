// SPDX-License-Identifier: Apache-2.0

// Command flagcheck is the gate make flag-check (ADR 0064, docs/operations/environments-and-flags.md
// 2.5): it holds the feature flag registry of internal/instance against its rules, the server
// version and the configuration reference.
//
// It reads the server VERSION (-version-file), the module keys under modules: of
// architecture/modules.yaml (-modules) and the table of section 7 of
// docs/operations/configuration.md (-config-doc), and fails with one line per finding, each
// prefixed "flag-check:", when
//
//  1. the registry breaks a rule of domain.CheckRegistry (key form, unique key, kind, description,
//     module, remove_by only and always for release);
//  2. VERSION has reached remove_by of a registered release flag (a pre-release of remove_by has
//     not): the flag must be removed with its code, its registration and its row;
//  3. the table of configuration.md 7 differs from the registry in key, kind or default
//     ("on" is true, "off" is false).
//
// Otherwise it prints "flag-check: ok (<n> flags)". Run from services/server:
//
//	go run ./tools/flagcheck -version-file VERSION -config-doc ../../docs/operations/configuration.md -modules architecture/modules.yaml
//
// Exit codes: 0 ok, 1 findings or an unreadable input, 2 usage error.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/instance/domain"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

// prefix starts every line the tool writes.
const prefix = "flag-check: "

// run checks the real registry instance.RegisteredFlags.
func run(args []string, stdout, stderr io.Writer) int {
	return runWith(args, instance.RegisteredFlags(), stdout, stderr)
}

// runWith parses args, checks defs and returns the exit code; findings go to stderr. The registry
// is an argument so that the tests run the tool on fixture registries.
func runWith(args []string, defs []instance.FlagDefinition, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("flagcheck", flag.ContinueOnError)
	fs.SetOutput(stderr)
	versionFile := fs.String("version-file", "", "server VERSION file")
	configDoc := fs.String("config-doc", "", "docs/operations/configuration.md")
	modulesFile := fs.String("modules", "", "architecture/modules.yaml")
	if err := fs.Parse(args); err != nil || *versionFile == "" || *configDoc == "" || *modulesFile == "" || fs.NArg() > 0 {
		_, _ = fmt.Fprintln(stderr, "usage: flagcheck -version-file VERSION -config-doc configuration.md -modules modules.yaml")
		return 2
	}
	findings, err := check(defs, *versionFile, *configDoc, *modulesFile)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, prefix+err.Error())
		return 1
	}
	if len(findings) > 0 {
		for _, f := range findings {
			_, _ = fmt.Fprintln(stderr, prefix+f)
		}
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "%sok (%d flags)\n", prefix, len(defs))
	return 0
}

// check reads the inputs and returns the findings; the error is an unreadable or invalid input.
func check(defs []instance.FlagDefinition, versionFile, configDoc, modulesFile string) ([]string, error) {
	raw, err := os.ReadFile(filepath.Clean(versionFile))
	if err != nil {
		return nil, err
	}
	version := strings.TrimSpace(string(raw))
	if err := domain.ValidateSemVer(version); err != nil {
		return nil, fmt.Errorf("VERSION %q: %w", version, err)
	}
	modules, err := readModules(modulesFile)
	if err != nil {
		return nil, err
	}
	rows, err := readConfigTable(configDoc)
	if err != nil {
		return nil, err
	}
	findings := domain.CheckRegistry(app.FlagSpecs(defs), modules)
	for _, d := range defs {
		if d.Kind != instance.FlagKindRelease || domain.ValidateSemVer(d.RemoveBy) != nil {
			continue
		}
		reached, err := domain.ReachedRemoveBy(version, d.RemoveBy)
		if err != nil {
			return nil, err
		}
		if reached {
			findings = append(findings, fmt.Sprintf("remove flag %s: VERSION %s reached remove_by %s", d.Key, version, d.RemoveBy))
		}
	}
	return append(findings, compareTable(defs, rows)...), nil
}

// readModules returns the keys under modules: of the module register, sorted.
func readModules(path string) ([]string, error) {
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	var reg struct {
		Modules map[string]yaml.Node `yaml:"modules"`
	}
	if err := yaml.Unmarshal(raw, &reg); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if len(reg.Modules) == 0 {
		return nil, fmt.Errorf("%s: no modules", path)
	}
	out := make([]string, 0, len(reg.Modules))
	for k := range reg.Modules {
		out = append(out, k)
	}
	slices.Sort(out)
	return out, nil
}

// configRow is one row of the table of configuration.md 7.
type configRow struct {
	kind, def string
}

// rowPattern is a table row whose first cell is a key in backticks: key, kind, default.
var rowPattern = regexp.MustCompile("^\\|\\s*`([^`]+)`\\s*\\|\\s*([^|]*?)\\s*\\|\\s*([^|]*?)\\s*\\|")

// readConfigTable returns the rows of the tables in section "## 7." of the configuration
// reference by key. Errors: an unreadable file or a missing section.
func readConfigTable(path string) (map[string]configRow, error) {
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	rows := map[string]configRow{}
	in, found := false, false
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "## ") {
			in = strings.HasPrefix(line, "## 7.")
			found = found || in
			continue
		}
		if m := rowPattern.FindStringSubmatch(line); in && m != nil {
			rows[m[1]] = configRow{kind: m[2], def: m[3]}
		}
	}
	if !found {
		return nil, errors.New(path + ": no section 7")
	}
	return rows, nil
}

// compareTable returns one finding per difference between the registry and the table rows.
func compareTable(defs []instance.FlagDefinition, rows map[string]configRow) []string {
	var out []string
	registered := map[string]bool{}
	for _, d := range defs {
		registered[d.Key] = true
		r, ok := rows[d.Key]
		if !ok {
			out = append(out, d.Key+": registered but not in configuration.md 7")
			continue
		}
		if r.kind != string(d.Kind) {
			out = append(out, fmt.Sprintf("%s: kind %s in the registry, %s in configuration.md 7", d.Key, d.Kind, r.kind))
		}
		if want := map[bool]string{true: "on", false: "off"}[d.Default]; r.def != want {
			out = append(out, fmt.Sprintf("%s: default %v in the registry, %s in configuration.md 7", d.Key, d.Default, r.def))
		}
	}
	keys := make([]string, 0, len(rows))
	for k := range rows {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		if !registered[k] {
			out = append(out, k+": in configuration.md 7 but not registered")
		}
	}
	return out
}
