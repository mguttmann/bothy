// SPDX-License-Identifier: Apache-2.0

package authz_test

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// sortedIDs returns n new ids in ascending order.
func sortedIDs(n int) []ids.ID {
	out := make([]ids.ID, n)
	for i := range out {
		out[i] = ids.New()
	}
	slices.SortFunc(out, func(a, b ids.ID) int { return strings.Compare(a.String(), b.String()) })
	return out
}

// TestInvariantError_I1: kind invariant_violation, code i1_violation, the hint, and the conflicts
// sorted by agent with sorted principal ids, IDs only (10 section 5.4).
func TestInvariantError_I1(t *testing.T) {
	t.Parallel()
	a := sortedIDs(5)
	agent1, agent2, holder, p1, p2 := a[0], a[1], a[2], a[3], a[4]
	err := authz.NewI1Violation([]authz.I1Conflict{
		{AgentID: agent2, PrincipalIDs: []ids.ID{p2, p1}},
		{AgentID: agent1, CredentialHolderID: holder, PrincipalIDs: []ids.ID{p1}},
	})
	var wrapped error = err
	e, ok := errs.As(wrapped)
	if !ok || e.Code != errs.CodeI1Violation || e.Kind() != errs.KindInvariantViolation || e.Hint != "Switch the agent to organization first." {
		t.Fatalf("errs view %+v ok %v", e, ok)
	}
	var ie *authz.InvariantError
	if !errors.As(wrapped, &ie) || len(ie.Conflicts) != 2 || ie.Conflicts[0].AgentID != agent1 ||
		!slices.Equal(ie.Conflicts[1].PrincipalIDs, []ids.ID{p1, p2}) {
		t.Fatalf("conflicts %+v", ie.Conflicts)
	}
	var ext errs.ProblemExtender
	if !errors.As(wrapped, &ext) {
		t.Fatal("no ProblemExtender")
	}
	body, mErr := json.Marshal(ext.ProblemExtensions())
	if mErr != nil {
		t.Fatal(mErr)
	}
	want := `{"conflicts":[{"code":"i1_violation","agent_id":"` + agent1.String() + `","credential_holder_id":"` + holder.String() +
		`","principal_ids":["` + p1.String() + `"]},{"code":"i1_violation","agent_id":"` + agent2.String() +
		`","principal_ids":["` + p1.String() + `","` + p2.String() + `"]}]}`
	if string(body) != want {
		t.Fatalf("extensions\n%s\nwant\n%s", body, want)
	}
	if testkit.HasJSONNull(body) {
		t.Fatal("null in the extensions")
	}
}

// TestInvariantError_I1EmptyPrincipals: a conflict without extra principals (a missing holder)
// still carries principal_ids as an empty array, never null.
func TestInvariantError_I1EmptyPrincipals(t *testing.T) {
	t.Parallel()
	body, err := json.Marshal(authz.NewI1Violation([]authz.I1Conflict{{AgentID: ids.New()}}).ProblemExtensions())
	if err != nil || !strings.Contains(string(body), `"principal_ids":[]`) {
		t.Fatalf("%s %v", body, err)
	}
}

// TestInvariantError_LastOwner: code last_owner with its hint and no extensions.
func TestInvariantError_LastOwner(t *testing.T) {
	t.Parallel()
	err := authz.NewLastOwnerViolation()
	e, ok := errs.As(err)
	if !ok || e.Code != errs.CodeLastOwner || e.Kind() != errs.KindInvariantViolation || e.Hint != "Appoint another owner first." {
		t.Fatalf("%+v", e)
	}
	if len(err.ProblemExtensions()) != 0 || err.Error() == "" {
		t.Fatalf("extensions %v", err.ProblemExtensions())
	}
}

// TestInvariantError_WithHint: WithHint returns a copy with the new hint and the same code and
// conflicts, and leaves the original unchanged (M1-42).
func TestInvariantError_WithHint(t *testing.T) {
	t.Parallel()
	agent := ids.New()
	orig := authz.NewI1Violation([]authz.I1Conflict{{AgentID: agent}})
	got := orig.WithHint(authz.HintI1ClaimSync)
	e, _ := errs.As(got)
	if e.Hint != authz.HintI1ClaimSync || got.Code != errs.CodeI1Violation || len(got.Conflicts) != 1 || got.Conflicts[0].AgentID != agent {
		t.Fatalf("copy %+v %+v", got, e)
	}
	if o, _ := errs.As(orig); o.Hint != authz.HintI1 {
		t.Fatalf("the original changed: %q", o.Hint)
	}
}

// TestInvariantError_I6 (M2-40, Design note 9): kind invariant_violation, code i6_violation, the
// hint, and the conflicts sorted by host with host_id, credential_holder_id, sorted principal_ids
// and the hint per entry; an empty principal list stays an array.
func TestInvariantError_I6(t *testing.T) {
	t.Parallel()
	a := sortedIDs(5)
	host1, host2, holder, p1, p2 := a[0], a[1], a[2], a[3], a[4]
	err := authz.NewI6Violation([]authz.I6Conflict{
		{HostID: host2, CredentialHolderID: holder},
		{HostID: host1, CredentialHolderID: holder, PrincipalIDs: []ids.ID{p2, p1}},
	})
	e, ok := errs.As(error(err))
	if !ok || e.Code != errs.CodeI6Violation || e.Kind() != errs.KindInvariantViolation || e.Hint != authz.HintI6 || e.Reason != "i6_violation" {
		t.Fatalf("errs view %+v ok %v", e, ok)
	}
	raw, mErr := json.Marshal(err.ProblemExtensions())
	if mErr != nil {
		t.Fatal(mErr)
	}
	want := `{"conflicts":[{"code":"i6_violation","host_id":"` + host1.String() + `","credential_holder_id":"` + holder.String() +
		`","principal_ids":["` + p1.String() + `","` + p2.String() + `"],"hint":"` + authz.HintI6 + `"},{"code":"i6_violation","host_id":"` +
		host2.String() + `","credential_holder_id":"` + holder.String() + `","principal_ids":[],"hint":"` + authz.HintI6 + `"}]}`
	if string(raw) != want {
		t.Fatalf("conflicts\n got %s\nwant %s", raw, want)
	}
}
