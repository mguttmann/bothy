// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ReasonSessionUserTaken is the reason of 409 conflict for a session user name that belongs or
// belonged to another person on the host, or a second valid assignment of one person (24 2.2).
const ReasonSessionUserTaken = "session_user_taken"

// SessionUserRow is one row of host_session_users of a host, valid or ended.
type SessionUserRow struct {
	ID        ids.ID
	UserID    ids.ID
	OSUser    string
	CreatedAt time.Time
	// Revoked reports an ended assignment (revoked_at set); its name stays taken.
	Revoked bool
}

// SessionUserAssign is one entry of add_session_users: a person and a session user name.
type SessionUserAssign struct {
	UserID ids.ID
	OSUser string
}

// Plan is the effect of an updateHost on the session users of a host: the valid rows to end and
// the assignments to insert, in that order.
type Plan struct {
	Revoke []SessionUserRow
	Add    []SessionUserAssign
}

// Empty reports a plan without effect.
func (p Plan) Empty() bool { return len(p.Revoke) == 0 && len(p.Add) == 0 }

// errTaken is the 409 of a taken name or a second valid assignment.
func errTaken() error {
	return errs.New(errs.CodeConflict, "The session user is taken on this host.").WithReason(ReasonSessionUserTaken)
}

// PlanAssignments plans revoke and add on the rows of a host (ruling T12, 24 section 2.2) against
// the names the host reported. Revocations come first: each listed person with a valid assignment
// has it ended, a listed person without one has no effect. Then the additions, so one request can
// move a person to a new session user: each name must equal a reported name exactly, including
// case (else 400 validation_failed with session_user_not_on_host at
// /add_session_users/<i>/os_user); then a row of the host with the same name without regard to
// case, valid or ended and of any person, a remaining valid assignment of the same person, or a
// duplicate inside the request gives 409 conflict with reason session_user_taken. Every 400 check
// of every entry comes before any 409 check. The function is pure; the rows come locked from the
// caller.
func PlanAssignments(reported []string, rows []SessionUserRow, revoke []ids.ID, add []SessionUserAssign) (Plan, error) {
	for i, a := range add {
		if !slices.Contains(reported, a.OSUser) {
			return Plan{}, FieldError("/add_session_users/"+strconv.Itoa(i)+"/os_user", FieldSessionUserNotOnHost,
				"the host does not report this session user")
		}
	}
	var p Plan
	ended := map[ids.ID]bool{}
	for _, r := range rows {
		if !r.Revoked && slices.Contains(revoke, r.UserID) && !ended[r.ID] {
			p.Revoke = append(p.Revoke, r)
			ended[r.ID] = true
		}
	}
	names := map[string]bool{}
	valid := map[ids.ID]bool{}
	for _, r := range rows {
		names[strings.ToLower(r.OSUser)] = true
		if !r.Revoked && !ended[r.ID] {
			valid[r.UserID] = true
		}
	}
	for _, a := range add {
		key := strings.ToLower(a.OSUser)
		if names[key] || valid[a.UserID] {
			return Plan{}, errTaken()
		}
		names[key], valid[a.UserID] = true, true
		p.Add = append(p.Add, a)
	}
	return p, nil
}
