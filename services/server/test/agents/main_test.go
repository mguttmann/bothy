// SPDX-License-Identifier: Apache-2.0

// Package agents_test is the conformance suite of the agent contract (M2-31): the scenarios AG-01
// to AG-32 of docs/spec/09-agents.md 16.2, one file each under scenarios/, run data driven against
// the real server (harness, pgtest, fake clock) with the Fake-Agent of the Python SDK on the other
// side. See README.md.
package agents_test

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// TestMain migrates the pgtest templates like bothy migrate up. Without a Docker-compatible
// runtime the server tests fail; they never skip (plan P-09).
func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }

// operation is one operation of docs/api/openapi.yaml.
type operation struct {
	method, path string
	// params are the names of the path parameters in order.
	params []string
	// pattern matches a request path (without the /api/v1 prefix) of the operation.
	pattern *regexp.Regexp
	// body reports whether the operation has a request body.
	body bool
}

// paramPattern finds the path parameters of a template.
var paramPattern = regexp.MustCompile(`\{([^}]+)\}`)

// operations returns every operation of the contract by operationId, read once.
var operations = sync.OnceValues(func() (map[string]operation, error) {
	var doc struct {
		Paths map[string]map[string]yaml.Node `yaml:"paths"`
	}
	if err := yaml.Unmarshal(rest.Spec, &doc); err != nil {
		return nil, err
	}
	out := map[string]operation{}
	for path, methods := range doc.Paths {
		for method, node := range methods {
			var op struct {
				OperationID string    `yaml:"operationId"`
				RequestBody yaml.Node `yaml:"requestBody"`
			}
			if node.Kind != yaml.MappingNode || node.Decode(&op) != nil || op.OperationID == "" {
				continue
			}
			var params []string
			for _, m := range paramPattern.FindAllStringSubmatch(path, -1) {
				params = append(params, m[1])
			}
			out[op.OperationID] = operation{method: strings.ToUpper(method), path: path, params: params, pattern: regexp.MustCompile(templateRegexp(path)),
				body: op.RequestBody.Kind != 0}
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no operations in the contract")
	}
	return out, nil
})

// templateRegexp turns a path template into an anchored regular expression.
func templateRegexp(path string) string {
	var b strings.Builder
	b.WriteString("^")
	rest := path
	for {
		i := strings.IndexByte(rest, '{')
		if i < 0 {
			b.WriteString(regexp.QuoteMeta(rest))
			break
		}
		j := strings.IndexByte(rest[i:], '}')
		b.WriteString(regexp.QuoteMeta(rest[:i]))
		b.WriteString(`[^/]+`)
		rest = rest[i+j+1:]
	}
	b.WriteString("$")
	return b.String()
}

// repoRoot returns the directory above the test that holds sdk/python/pyproject.toml.
var repoRoot = sync.OnceValues(func() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for d := dir; ; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "sdk", "python", "pyproject.toml")); err == nil {
			return d, nil
		}
		if filepath.Dir(d) == d {
			return "", fmt.Errorf("no sdk/python/pyproject.toml above %s", dir)
		}
	}
})

// configKeyPattern finds the key of a row of docs/operations/configuration.md.
var configKeyPattern = regexp.MustCompile("^\\| `([a-z0-9_.]+)` \\|")

// loadVocabulary reads the operationIds of the contract and the keys of the configuration
// reference.
var loadVocabulary = sync.OnceValues(func() (vocabulary, error) {
	ops, err := operations()
	if err != nil {
		return vocabulary{}, err
	}
	v := vocabulary{operations: map[string]bool{}, configKeys: map[string]bool{}}
	for id := range ops {
		v.operations[id] = true
	}
	root, err := repoRoot()
	if err != nil {
		return vocabulary{}, err
	}
	b, err := os.ReadFile(filepath.Join(root, "docs", "operations", "configuration.md")) // #nosec G304 -- a fixed file of the repository
	if err != nil {
		return vocabulary{}, err
	}
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		if m := configKeyPattern.FindStringSubmatch(sc.Text()); m != nil {
			v.configKeys[m[1]] = true
		}
	}
	return v, sc.Err()
})
