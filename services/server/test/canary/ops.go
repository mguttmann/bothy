// SPDX-License-Identifier: Apache-2.0

package canary

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"go.yaml.in/yaml/v3"

	"github.com/OWNER/bothy/services/server/api/rest"
)

// opRoutes maps "METHOD /path-template" of every operation of the contract (rest.Spec, byte-equal
// to docs/api/openapi.yaml) to its operation ID; matching uses a ServeMux, so templates match like
// the server routes.
var opRoutes = sync.OnceValues(func() (*http.ServeMux, error) {
	var doc struct {
		Paths map[string]map[string]yaml.Node `yaml:"paths"`
	}
	if err := yaml.Unmarshal(rest.Spec, &doc); err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	for path, item := range doc.Paths {
		for method, node := range item {
			var op struct {
				ID string `yaml:"operationId"`
			}
			if node.Decode(&op) != nil || op.ID == "" {
				continue
			}
			id := op.ID
			mux.Handle(strings.ToUpper(method)+" "+path, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Operation", id)
			}))
		}
	}
	return mux, nil
})

// OperationOf returns the operation ID of method and path (relative to /api/v1, with or without
// query), or "" when no operation of the contract matches.
func OperationOf(method, path string) string {
	mux, err := opRoutes()
	if err != nil {
		return ""
	}
	p, _, _ := strings.Cut(path, "?")
	req := &http.Request{Method: method, URL: &url.URL{Path: p}, Host: "x"}
	h, pattern := mux.Handler(req)
	if pattern == "" {
		return ""
	}
	rec := headerRecorder{h: http.Header{}}
	h.ServeHTTP(rec, req)
	return rec.h.Get("Operation")
}

// routeOf returns the operation ID of method and path, or the method and the path without query
// when the contract has none; it names error responses without their raw path.
func routeOf(method, path string) string {
	if id := OperationOf(method, path); id != "" {
		return id
	}
	p, _, _ := strings.Cut(path, "?")
	return method + " " + p
}

// headerRecorder is the minimal ResponseWriter OperationOf needs.
type headerRecorder struct{ h http.Header }

func (r headerRecorder) Header() http.Header       { return r.h }
func (headerRecorder) Write(b []byte) (int, error) { return len(b), nil }
func (headerRecorder) WriteHeader(int)             {}

// EnabledOperations reads the enabled operation IDs of services/server/api/operations.yaml
// (plan decision P-04), sorted. Errors: the file cannot be read or has another format version.
func EnabledOperations() ([]string, error) {
	_, file, _, _ := runtime.Caller(0)
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "api", "operations.yaml")) // #nosec G304 -- a fixed path of the module
	if err != nil {
		return nil, err
	}
	var list struct {
		Version int      `yaml:"version"`
		Enabled []string `yaml:"enabled"`
	}
	if err := yaml.Unmarshal(raw, &list); err != nil {
		return nil, err
	}
	if list.Version != 1 {
		return nil, fmt.Errorf("operations.yaml: version %d, want 1", list.Version)
	}
	sort.Strings(list.Enabled)
	return list.Enabled, nil
}

// Coverage records which operations a scenario called. It is safe for concurrent use.
type Coverage struct {
	mu     sync.Mutex
	called map[string]int
}

// NewCoverage returns an empty record.
func NewCoverage() *Coverage { return &Coverage{called: map[string]int{}} }

// Observe records the operation of r when the server answered it (status other than 0).
func (c *Coverage) Observe(method, path string, status int) {
	if status == 0 {
		return
	}
	if id := OperationOf(method, path); id != "" {
		c.mu.Lock()
		c.called[id]++
		c.mu.Unlock()
	}
}

// Missing returns the operations of enabled that were never called and are not in exempt.
func (c *Coverage) Missing(enabled []string, exempt map[string]string) []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []string
	for _, id := range enabled {
		if c.called[id] == 0 && exempt[id] == "" {
			out = append(out, id)
		}
	}
	return out
}

// known reports whether the contract has an operation with id (a check of the enabled list).
func (c *Coverage) known(id string) bool {
	ops, err := contractOperations()
	return err == nil && ops[id]
}

// contractOperations returns the operation IDs of the contract.
var contractOperations = sync.OnceValues(func() (map[string]bool, error) {
	var doc struct {
		Paths map[string]map[string]struct {
			ID string `yaml:"operationId"`
		} `yaml:"paths"`
	}
	out := map[string]bool{}
	if err := yaml.Unmarshal(rest.Spec, &doc); err != nil {
		return nil, err
	}
	for _, item := range doc.Paths {
		for _, op := range item {
			if op.ID != "" {
				out[op.ID] = true
			}
		}
	}
	return out, nil
})
