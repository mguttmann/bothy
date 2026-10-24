// SPDX-License-Identifier: Apache-2.0

package conformance

import (
	"testing"

	"github.com/OWNER/bothy/services/server/test/terminal/suite"
)

// conformanceScenarios are the scenario files of this package (Design note 2).
var conformanceScenarios = []string{"tm-t03", "tm-t04", "tm-t05", "tm-t06", "tm-t07", "tm-t08", "tm-t09", "tm-t10", "tm-t12", "tm-t17", "tm-t19", "tm-t20", "tm-t22", "tm-t23", "tm-t28", "az-m54-61", "id-t28"}

// TestScenarios runs one subtest <scenario>/<case> per case of every scenario file of the package.
func TestScenarios(t *testing.T) {
	for _, id := range conformanceScenarios {
		suite.RunFile(t, id)
	}
}
