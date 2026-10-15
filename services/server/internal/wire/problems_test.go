// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/api/routes"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// TestProblemCatalogueMatchesErrs holds the two code forms of the one problem list x-problem-codes
// equal: routes.ProblemTypes (generated, served by getProblemType, M1-46) and the codes, titles
// and statuses of kernel/errs that the problem writer of the chain uses (M1-18). A code added to
// the contract without a constant in errs, or with another title or status, fails here.
func TestProblemCatalogueMatchesErrs(t *testing.T) {
	t.Parallel()
	byCode := map[string]routes.ProblemType{}
	for _, p := range routes.ProblemTypes() {
		byCode[p.Code] = p
	}
	codes := errs.AllCodes()
	if len(codes) != len(byCode) {
		t.Errorf("errs has %d codes, x-problem-codes %d", len(codes), len(byCode))
	}
	for _, c := range codes {
		p, ok := byCode[string(c)]
		if !ok {
			t.Errorf("code %s of errs is not in x-problem-codes", c)
			continue
		}
		if p.Title != c.Title() || p.Status != c.Kind().Status() {
			t.Errorf("code %s: catalogue %q %d, errs %q %d", c, p.Title, p.Status, c.Title(), c.Kind().Status())
		}
		if p.Description == "" {
			t.Errorf("code %s has no description", c)
		}
	}
}
