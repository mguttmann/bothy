// SPDX-License-Identifier: Apache-2.0

package agents_test

import (
	"testing"
)

// TestConformance runs every scenario file against the real server: one subtest per test ID,
// one per case below it (brief Design note 4). Each case starts its own server, world and
// Fake-Agent processes unless the scenario shares its world across cases.
//
// Run one scenario: go test -race -count=1 -run 'TestConformance/AG-14' ./test/agents/
func TestConformance(t *testing.T) {
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
	for _, sc := range scenarios {
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
