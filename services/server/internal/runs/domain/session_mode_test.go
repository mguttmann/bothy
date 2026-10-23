// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"encoding/json"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// TestSessionMode is the mode table of 09 9.1 (M2-12 design note 9): a guest run gives no
// session fields; no row gives new without agent_session; an unstarted row gives new with the
// row; a started row some run used gives resume; a started row no run used gives bind.
func TestSessionMode(t *testing.T) {
	t.Parallel()
	row := json.RawMessage(`{"id":"r"}`)
	for _, c := range []struct {
		name     string
		operator bool
		row      *domain.SessionRow
		mode     domain.SessionMode
		withRow  bool
	}{
		{"guest run", false, nil, domain.SessionGuest, false},
		{"guest run with a row of the operator", false, &domain.SessionRow{JSON: row, Started: true, Used: true}, domain.SessionGuest, false},
		{"no row", true, nil, domain.SessionNew, false},
		{"unstarted row", true, &domain.SessionRow{JSON: row}, domain.SessionNew, true},
		{"unstarted row a run used", true, &domain.SessionRow{JSON: row, Used: true}, domain.SessionNew, true},
		{"started and used", true, &domain.SessionRow{JSON: row, Started: true, Used: true}, domain.SessionResume, true},
		{"started and unused", true, &domain.SessionRow{JSON: row, Started: true}, domain.SessionBind, true},
	} {
		d := domain.DecideSession(c.operator, c.row)
		if d.Mode != c.mode || (d.Row != nil) != c.withRow {
			t.Errorf("%s: %+v, want mode %s with row %v", c.name, d, c.mode, c.withRow)
		}
		if c.withRow && string(d.Row) != string(row) {
			t.Errorf("%s: row %s", c.name, d.Row)
		}
	}
}
