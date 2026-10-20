// SPDX-License-Identifier: Apache-2.0

package restart

import (
	"testing"

	"github.com/OWNER/bothy/services/server/test/terminal/suite"
)

// scenarioFiles are the scenario files of this package (Design note 2).
var scenarioFiles = []string{"tm-t16"}

// TestScenarios runs one subtest <scenario>/<case> per case of every scenario file of the package.
func TestScenarios(t *testing.T) {
	for _, id := range scenarioFiles {
		suite.RunFile(t, id)
	}
}
