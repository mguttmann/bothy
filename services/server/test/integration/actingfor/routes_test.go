// SPDX-License-Identifier: Apache-2.0

package actingfor_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/OWNER/bothy/services/server/api/routes"
)

// exceptionRoutes are the operations of the SR9 exceptions and of the bound run context whose
// end-to-end cases belong to the tickets that enable them (brief M2-14 "Out of scope"):
// getRunContext (M2-13). patchRun (M2-10) and reportAgentSession (M2-12) are enabled; their cases
// run in TestAZP11_SR9RunLifecycle (own run after the revocation, foreign run 404).
// respondToInteraction is enabled by M2-15; its cases run in TestSR9_RespondToInteraction.
var exceptionRoutes = []string{"getRunContext"}

// TestSR9_ExceptionRoutesEnabled (brief "TestSR9_ExceptionRoutesEnabled"): on this base the routes
// of exceptionRoutes are not enabled, so their end-to-end cases (own run with T after Alex lost
// every grant succeeds, the foreign run answers 404; getRunContext with T answers 404) cannot run
// here. The test asserts that each still stands under later in api/operations.yaml and is not
// registered; it fails as soon as one is enabled, so that ticket adds its end-to-end case.
func TestSR9_ExceptionRoutesEnabled(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "api", "operations.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var list struct {
		Enabled []string `yaml:"enabled"`
		Later   []struct {
			ID     string `yaml:"operation_id"`
			Ticket string `yaml:"ticket"`
		} `yaml:"later"`
	}
	if err := yaml.Unmarshal(raw, &list); err != nil {
		t.Fatal(err)
	}
	for _, id := range exceptionRoutes {
		if slices.Contains(list.Enabled, id) {
			t.Errorf("%s is enabled: add its end-to-end SR9 case (own run with T after the loss of every grant, foreign run 404)", id)
			continue
		}
		if _, ok := routes.ByID(id); ok {
			t.Errorf("%s is registered although api/operations.yaml does not enable it", id)
		}
		stage := ""
		for _, l := range list.Later {
			if l.ID == id {
				stage = "later, ticket " + l.Ticket
			}
		}
		if stage == "" {
			t.Errorf("%s is neither enabled nor under later", id)
			continue
		}
		t.Logf("%s is not enabled on this base (%s); its end-to-end SR9 case comes with that ticket", id, stage)
	}
}
