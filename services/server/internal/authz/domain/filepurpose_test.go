// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
)

// TestFileReadRequirement holds the entry of file.read (brief M3-10 design note 6): the 05 text
// stays "read", the expression chooses by file purpose (attachment read on the path, avatar and
// branding Auth, gdpr_export read but refused by the visibility), and an event row without facts
// takes read on its path or, without a path, Self. Contains reaches into every choice.
func TestFileReadRequirement(t *testing.T) {
	t.Parallel()
	r, ok := domain.RequirementOf("file.read")
	if !ok || r.Source != "read" {
		t.Fatalf("file.read: %+v %v", r, ok)
	}
	want := "ByFilePurpose(attachment:Level(read),avatar:Auth,branding:Auth,event_path:Level(read),event_self:Self,gdpr_export:Level(read))"
	if got := r.Expr.String(); got != want {
		t.Fatalf("file.read: %s, want %s", got, want)
	}
	for _, a := range []domain.Expr{domain.Auth, domain.Self, domain.AtLeast(domain.LevelRead)} {
		if !r.Expr.Contains(a) {
			t.Errorf("file.read does not contain %s", a)
		}
	}
	if r.Expr.Contains(domain.Owner) || r.Expr.IsReadLevel() {
		t.Errorf("file.read: Owner or plain read level reported in %s", r.Expr)
	}
}
