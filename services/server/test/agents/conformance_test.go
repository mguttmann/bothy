// SPDX-License-Identifier: Apache-2.0

package agents_test

import (
	"testing"
)

// conformanceGroups is the number of top-level conformance tests: the scenarios go round-robin
// in their load order into TestConformance1 to TestConformance4, so the parts of the CI test
// shards (SPLITS in tools/ci/go_shards.py, which splits by top-level test) can divide the
// suite's time; one top-level test held it all (PL/39).
const conformanceGroups = 4

// TestConformance1 to TestConformance4 run every scenario file against the real server: one
// subtest per test ID, one per case below it (brief Design note 4). Each case starts its own
// server, world and Fake-Agent processes unless the scenario shares its world across cases.
//
// Run one scenario: go test -race -count=1 -run 'TestConformance/AG-14' ./test/agents/
func TestConformance1(t *testing.T) { runConformance(t, 0) }

func TestConformance2(t *testing.T) { runConformance(t, 1) }

func TestConformance3(t *testing.T) { runConformance(t, 2) }

func TestConformance4(t *testing.T) { runConformance(t, 3) }

// runConformance runs the scenarios of group (index modulo conformanceGroups in load order).
func runConformance(t *testing.T, group int) {
	t.Helper()
	v, err := loadVocabulary()
	if err != nil {
		t.Fatal(err)
	}
	scenarios, err := loadScenarios(v)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := fakeAgentCommand(); err != nil {
		t.Fatal(err)
	}
	for i, sc := range scenarios {
		if i%conformanceGroups != group {
			continue
		}
		t.Run(sc.ID, func(t *testing.T) {
			t.Parallel()
			if len(sc.Cases) == 0 {
				// Every part of the ID is deferred (registry_test.go holds the list).
				return
			}
			if sc.SharedWorld {
				c := startCase(t, sc)
				if err := c.launchAgents(); err != nil {
					t.Fatal(err)
				}
				c.run("setup", sc.Setup)
				for _, cs := range sc.Cases {
					c.run("case "+cs.Name, cs.Steps)
				}
				return
			}
			for _, cs := range sc.Cases {
				t.Run(cs.Name, func(t *testing.T) {
					t.Parallel()
					c := startCase(t, sc)
					if err := c.launchAgents(); err != nil {
						t.Fatal(err)
					}
					c.run("setup", sc.Setup)
					c.run("case "+cs.Name, cs.Steps)
				})
			}
		})
	}
}
