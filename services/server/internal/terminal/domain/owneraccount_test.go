// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestPlanAssignments_OwnerAccount (ADR 0073, D-OA-6): an assignment of a name the host reports as
// the owner account needs owner_account = true, and owner_account = true needs such a name; both
// are 400 at /add_session_users/<i>/owner_account before any 409, and nothing is planned.
func TestPlanAssignments_OwnerAccount(t *testing.T) {
	t.Parallel()
	reported := []ReportedSessionUser{{OSUser: "owner", OwnerAccount: true}, {OSUser: "bothy-s-sam"}}
	robin, sam := ids.New(), ids.New()
	for _, tc := range []struct {
		name string
		add  SessionUserAssign
		code string
	}{
		{"owner account without the opt-in", SessionUserAssign{UserID: robin, OSUser: "owner"}, FieldOwnerAccountOptIn},
		{"opt-in for a separate session user", SessionUserAssign{UserID: sam, OSUser: "bothy-s-sam", OwnerAccount: true}, FieldOwnerAccountNotOnHost},
	} {
		_, err := PlanAssignmentsAt("/add_session_users/", reported, nil, nil, []SessionUserAssign{tc.add})
		e, ok := errs.As(err)
		if !ok || e.Code != errs.CodeValidationFailed || len(e.Fields) != 1 || e.Fields[0].Code != tc.code || e.Fields[0].Pointer != "/add_session_users/0/owner_account" {
			t.Errorf("%s: %v", tc.name, err)
		}
	}
	p, err := PlanAssignmentsAt("/add_session_users/", reported, nil, nil, []SessionUserAssign{{UserID: robin, OSUser: "owner", OwnerAccount: true}, {UserID: sam, OSUser: "bothy-s-sam"}})
	if err != nil || len(p.Add) != 2 || !p.Add[0].OwnerAccount || p.Add[1].OwnerAccount {
		t.Fatalf("plan %+v %v", p, err)
	}
}

// TestReportsAssignment (D-OA-7): an assignment counts for SR12 only while the host reports its name
// with the same owner account flag.
func TestReportsAssignment(t *testing.T) {
	t.Parallel()
	list := []ReportedSessionUser{{OSUser: "owner", OwnerAccount: true}, {OSUser: "bothy-s-sam"}}
	for _, tc := range []struct {
		name         string
		ownerAccount bool
		want         bool
	}{
		{"owner", true, true}, {"owner", false, false}, {"bothy-s-sam", false, true}, {"bothy-s-sam", true, false}, {"bothy-s-gone", false, false},
	} {
		if got := ReportsAssignment(list, tc.name, tc.ownerAccount); got != tc.want {
			t.Errorf("%s owner_account=%v: %v", tc.name, tc.ownerAccount, got)
		}
	}
}
