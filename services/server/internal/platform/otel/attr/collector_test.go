// SPDX-License-Identifier: Apache-2.0

package attr

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/genai"
)

// TestAttrKeysEqualCollectorAllowlist holds the source allowlist and the collector allowlist
// equal (docs/spec/17-observability.md "Tests, die diese Regeln halten" item 2): the span and
// log keys of this package plus the genai keys must be exactly the non-resource keys of
// redaction/allowlist.allowed_keys in infra/otel/collector.yaml. Resource keys are those of
// signals.md "Resource-Attribute" plus telemetry.sdk.*. A difference in either direction fails.
func TestAttrKeysEqualCollectorAllowlist(t *testing.T) {
	t.Parallel()
	resource := resourceKeys(t)
	var collector []string
	for _, k := range allowedKeys(t) {
		if !resource[k] && !strings.HasPrefix(k, "telemetry.sdk.") {
			collector = append(collector, k)
		}
	}
	var source []string
	for k, s := range scopes {
		if s == scopeSpanLog {
			source = append(source, k)
		}
	}
	source = append(source, genai.Keys()...)
	for _, k := range source {
		if !slices.Contains(collector, k) {
			t.Errorf("%s is allowed at the source but not in the collector allowlist", k)
		}
	}
	for _, k := range collector {
		if !slices.Contains(source, k) {
			t.Errorf("%s is in the collector allowlist but has no span or log constant in attr or genai", k)
		}
	}
}

// TestResourceAndMetricKeysOutsideSpansAndLogs checks the keys this package keeps out of spans and
// logs: bothy.server.role is a resource key of signals.md section 3 that the collector allowlist
// lists for the resource, and bothy.signal is a metric attribute the allowlist does not need.
func TestResourceAndMetricKeysOutsideSpansAndLogs(t *testing.T) {
	t.Parallel()
	resource := resourceKeys(t)
	allowed := allowedKeys(t)
	for k, s := range scopes {
		switch s {
		case scopeSpanLog: // compared with the collector in TestAttrKeysEqualCollectorAllowlist
		case scopeResource:
			if !resource[k] || !slices.Contains(allowed, k) {
				t.Errorf("%s: resource key missing from signals.md section 3 or the collector allowlist", k)
			}
		case scopeMetric:
			if slices.Contains(allowed, k) {
				t.Errorf("%s: metric-only key in the span and log allowlist of the collector", k)
			}
		}
	}
}

// TestGenAIKeysAllowedOrScrubbed: every genai constant passes the collector allowlist, and every
// forbidden gen_ai key is deleted by transform/scrub or not in the allowlist (signals.md
// "GenAI-Konventionen" items 3 and 4).
func TestGenAIKeysAllowedOrScrubbed(t *testing.T) {
	t.Parallel()
	allowed := allowedKeys(t)
	for _, k := range genai.Keys() {
		if !slices.Contains(allowed, k) {
			t.Errorf("%s is not in redaction/allowlist.allowed_keys", k)
		}
	}
	deleted := scrubbedKeys(t)
	for _, k := range genai.Forbidden {
		if slices.Contains(allowed, k) && !deleted(k) {
			t.Errorf("%s is forbidden but the collector neither deletes it nor leaves it out of the allowlist", k)
		}
		if !deleted(k) {
			t.Errorf("%s is forbidden but transform/scrub does not delete it", k)
		}
	}
}

// repoFile opens a file by its path from the repository root, found by walking up from the
// package directory; os.Root keeps the read inside that root.
func repoFile(t *testing.T, rel string) *os.File {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, rel)); err == nil {
			root, err := os.OpenRoot(dir)
			if err != nil {
				t.Fatal(err)
			}
			f, err := root.Open(rel)
			_ = root.Close()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = f.Close() })
			return f
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("%s not found above the package directory", rel)
		}
		dir = parent
	}
}

var listItem = regexp.MustCompile(`^\s+- ([A-Za-z0-9_.]+)\s*(#.*)?$`)

// allowedKeys returns redaction/allowlist.allowed_keys of infra/otel/collector.yaml in file order.
func allowedKeys(t *testing.T) []string {
	t.Helper()
	sc := bufio.NewScanner(repoFile(t, "infra/otel/collector.yaml"))
	var keys []string
	inProcessor, inList := false, false
	for sc.Scan() {
		line := sc.Text()
		trimmed := strings.TrimSpace(line)
		switch {
		case line == "  redaction/allowlist:":
			inProcessor = true
		case inProcessor && trimmed == "allowed_keys:":
			inList = true
		case inList && (trimmed == "" || strings.HasPrefix(trimmed, "#")):
		case inList:
			m := listItem.FindStringSubmatch(line)
			if m == nil {
				return keys
			}
			keys = append(keys, m[1])
		}
	}
	t.Fatal("redaction/allowlist.allowed_keys not found in infra/otel/collector.yaml")
	return nil
}

var (
	deleteKey      = regexp.MustCompile(`delete_key\(attributes, ("[^"]*")\)`)
	deleteMatching = regexp.MustCompile(`delete_matching_keys\(attributes, ("(?:[^"\\]|\\.)*")\)`)
)

// scrubbedKeys returns a predicate that reports whether transform/scrub of
// infra/otel/collector.yaml deletes a key from span, span event or log attributes.
func scrubbedKeys(t *testing.T) func(string) bool {
	t.Helper()
	sc := bufio.NewScanner(repoFile(t, "infra/otel/collector.yaml"))
	var exact []string
	var patterns []*regexp.Regexp
	in := false
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "   ") {
			in = line == "  transform/scrub:"
			continue
		}
		if !in {
			continue
		}
		if m := deleteKey.FindStringSubmatch(line); m != nil {
			exact = append(exact, unquote(t, m[1]))
		}
		if m := deleteMatching.FindStringSubmatch(line); m != nil {
			patterns = append(patterns, regexp.MustCompile(unquote(t, m[1])))
		}
	}
	if len(exact) == 0 || len(patterns) == 0 {
		t.Fatal("transform/scrub statements not found in infra/otel/collector.yaml")
	}
	return func(key string) bool {
		return slices.Contains(exact, key) || slices.ContainsFunc(patterns, func(p *regexp.Regexp) bool { return p.MatchString(key) })
	}
}

// unquote reads an OTTL string literal; its escapes are those of Go for the backslash and quote.
func unquote(t *testing.T, s string) string {
	t.Helper()
	u, err := strconv.Unquote(s)
	if err != nil {
		t.Fatalf("OTTL string %s: %v", s, err)
	}
	return u
}

// resourceKeys returns the attributes of the table in docs/observability/signals.md section 3.
func resourceKeys(t *testing.T) map[string]bool {
	t.Helper()
	sc := bufio.NewScanner(repoFile(t, "docs/observability/signals.md"))
	keys := map[string]bool{}
	in := false
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "## ") {
			in = line == "## 3. Resource-Attribute"
			continue
		}
		if in && strings.HasPrefix(line, "| `") {
			cell := strings.TrimSpace(strings.Split(line, "|")[1])
			keys[strings.Trim(cell, "`")] = true
		}
	}
	if len(keys) == 0 {
		t.Fatal(`section "3. Resource-Attribute" not found in docs/observability/signals.md`)
	}
	return keys
}
