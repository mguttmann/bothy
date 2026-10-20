// SPDX-License-Identifier: Apache-2.0

package suite

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Repo names the files of the repository the suite reads.
type Repo struct {
	// Root is the repository root.
	Root string
}

// ErrNoRepo reports a working directory outside the repository.
var ErrNoRepo = errors.New("suite: the repository root is not above the working directory")

// FindRepo returns the repository of the working directory: the nearest folder above it that holds
// services/server/test/terminal/profiles.yaml.
//
// Errors: ErrNoRepo.
func FindRepo() (Repo, error) {
	dir, err := os.Getwd()
	if err != nil {
		return Repo{}, err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "services", "server", "test", "terminal", "profiles.yaml")); err == nil {
			return Repo{Root: dir}, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return Repo{}, ErrNoRepo
		}
		dir = parent
	}
}

// SuiteDir is services/server/test/terminal.
func (r Repo) SuiteDir() string {
	return filepath.Join(r.Root, "services", "server", "test", "terminal")
}

// ProfilesPath is profiles.yaml of the suite.
func (r Repo) ProfilesPath() string { return filepath.Join(r.SuiteDir(), "profiles.yaml") }

// CataloguePath is catalogue.yaml of the suite.
func (r Repo) CataloguePath() string { return filepath.Join(r.SuiteDir(), "catalogue.yaml") }

// ScenarioPath returns scenarios/<id>.yaml.
func (r Repo) ScenarioPath(id string) string {
	return filepath.Join(r.SuiteDir(), "scenarios", id+".yaml")
}

// configRow matches the first cell of a table row of docs/operations/configuration.md.
var configRow = regexp.MustCompile("^\\| `([a-z0-9_]+(?:\\.[a-z0-9_]+)+)` \\|")

// ConfigKeys returns the keys of the tables of docs/operations/configuration.md.
func ConfigKeys(root string) (map[string]bool, error) {
	b, err := os.ReadFile(filepath.Join(root, "docs", "operations", "configuration.md")) //nolint:gosec // G304: a fixed file of the repository
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		if m := configRow.FindStringSubmatch(sc.Text()); m != nil {
			out[m[1]] = true
		}
	}
	if len(out) == 0 {
		return nil, errors.New("suite: no key row in docs/operations/configuration.md")
	}
	return out, sc.Err()
}

// Operation is the method and path of an operation of docs/api/openapi.yaml.
type Operation struct {
	Method, Path string
}

// Operations returns the operations of docs/api/openapi.yaml by operationId.
func Operations(root string) (map[string]Operation, error) {
	b, err := os.ReadFile(filepath.Join(root, "docs", "api", "openapi.yaml")) //nolint:gosec // G304: a fixed file of the repository
	if err != nil {
		return nil, err
	}
	var doc struct {
		Paths map[string]map[string]yaml.Node `yaml:"paths"`
	}
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	out := map[string]Operation{}
	for path, methods := range doc.Paths {
		for method, node := range methods {
			var op struct {
				OperationID string `yaml:"operationId"`
			}
			if node.Kind != yaml.MappingNode || node.Decode(&op) != nil || op.OperationID == "" {
				continue
			}
			out[op.OperationID] = Operation{Method: strings.ToUpper(method), Path: path}
		}
	}
	return out, nil
}

// MultiNodeClusterMode returns BOTHY_CLUSTER__MODE that M2-47's compose.multi-node.yaml sets for
// api, api-2 and worker; the three must agree.
//
// Errors: a missing value or a disagreement.
func MultiNodeClusterMode(root string) (string, error) {
	b, err := os.ReadFile(filepath.Join(root, "infra", "compose", "compose.multi-node.yaml")) //nolint:gosec // G304: a fixed file of the repository
	if err != nil {
		return "", err
	}
	var doc struct {
		Services map[string]struct {
			Environment map[string]string `yaml:"environment"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return "", err
	}
	mode := ""
	for _, svc := range []string{"api", "api-2", "worker"} {
		v := doc.Services[svc].Environment["BOTHY_CLUSTER__MODE"]
		if v == "" || (mode != "" && v != mode) {
			return "", fmt.Errorf("suite: compose.multi-node.yaml sets BOTHY_CLUSTER__MODE %q for %s", v, svc)
		}
		mode = v
	}
	return mode, nil
}
