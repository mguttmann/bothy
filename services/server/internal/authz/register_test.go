// SPDX-License-Identifier: Apache-2.0

package authz_test

import (
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
)

// TestRegister_Events: authz declares every durable event it appends (M1-29, M1-30).
func TestRegister_Events(t *testing.T) {
	t.Parallel()
	r := authorize.NewRegistry()
	authz.Register(r)
	var got []string
	for _, e := range r.Events() {
		if e.Module != "authz" {
			t.Fatalf("registration %+v", e)
		}
		got = append(got, string(e.Type))
	}
	slices.Sort(got)
	want := []string{"access.changed", "access.expiring", "grant.changed", "group.changed"}
	if !slices.Equal(got, want) {
		t.Fatalf("events %v, want %v", got, want)
	}
}
