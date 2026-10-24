// SPDX-License-Identifier: Apache-2.0

package agents_test

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// deferredGolden is the list of scenario parts that later tickets bring (brief Design note 13).
const deferredGolden = "testdata/deferred.golden"

// mandatoryIDs reads the test IDs of the table in docs/spec/09-agents.md 16.2.
func mandatoryIDs() ([]string, error) {
	root, err := repoRoot()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(filepath.Join(root, "docs", "spec", "09-agents.md")) // #nosec G304 -- a fixed file of the repository
	if err != nil {
		return nil, err
	}
	start := bytes.Index(b, []byte("### 16.2 Required scenarios"))
	if start < 0 {
		return nil, fmt.Errorf("no section 16.2 in 09-agents.md")
	}
	rest := b[start+1:]
	if end := bytes.Index(rest, []byte("\n## ")); end >= 0 {
		rest = rest[:end]
	}
	row := regexp.MustCompile(`^\| (AG-\d\d) \|`)
	var out []string
	sc := bufio.NewScanner(bytes.NewReader(rest))
	for sc.Scan() {
		if m := row.FindStringSubmatch(sc.Text()); m != nil {
			out = append(out, m[1])
		}
	}
	return out, sc.Err()
}

// TestScenarioRegistry_CoversMandatoryScenarios: the IDs of 09 16.2 equal the IDs of the
// scenario files, none missing and none extra (M2 exit criterion 1).
func TestScenarioRegistry_CoversMandatoryScenarios(t *testing.T) {
	t.Parallel()
	want, err := mandatoryIDs()
	if err != nil {
		t.Fatal(err)
	}
	if len(want) != 32 || want[0] != "AG-01" || want[31] != "AG-32" {
		t.Fatalf("the table of 09 16.2 names %d IDs: %v", len(want), want)
	}
	v, err := loadVocabulary()
	if err != nil {
		t.Fatal(err)
	}
	scenarios, err := loadScenarios(v)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, s := range scenarios {
		got = append(got, s.ID)
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Fatalf("scenario files %v, 09 16.2 %v", got, want)
	}
}

// TestScenarioRegistry_DeferredGolden: the deferred entries of all files equal
// testdata/deferred.golden line for line, and each line names a ticket of docs/PLAN.md.
func TestScenarioRegistry_DeferredGolden(t *testing.T) {
	t.Parallel()
	v, err := loadVocabulary()
	if err != nil {
		t.Fatal(err)
	}
	scenarios, err := loadScenarios(v)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, s := range scenarios {
		for _, d := range s.Deferred {
			got = append(got, s.ID+" "+d.Part+" "+d.Ticket)
		}
	}
	b, err := os.ReadFile(deferredGolden)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if !slices.Equal(got, want) {
		t.Fatalf("deferred parts of the scenario files:\n%s\nwant (testdata/deferred.golden):\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	plan, err := os.ReadFile(filepath.Join(root, "docs", "PLAN.md")) // #nosec G304 -- a fixed file of the repository
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range want {
		fields := strings.Fields(line)
		ticket := fields[len(fields)-1]
		if !bytes.Contains(plan, []byte("| "+ticket+" |")) {
			t.Errorf("the deferred line %q names %s, which is no ticket row of docs/PLAN.md", line, ticket)
		}
	}
}
