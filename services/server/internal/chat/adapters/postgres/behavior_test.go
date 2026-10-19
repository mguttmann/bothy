// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestReplaceSkills_UnindexableReferenceIsAFieldError (M2-02 red team): a skill reference that
// passes ValidateBehavior but is too large for the unique index (channel_id, skill_ref) answers
// 400 validation_failed with pointer /skills and code too_long, not 500 internal_error, and the
// error never repeats the reference.
func TestReplaceSkills_UnindexableReferenceIsAFieldError(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	alex, _ := e.person(t)
	sp := domain.NewSpace(ids.New(), "Support", "", "", "a0", alex)
	ch := domain.NewChannel(ids.New(), sp.ID, domain.ChannelText, "allgemein", "", "a0", alex, 20)
	if err := e.write(t, func(ctx context.Context) error {
		if err := e.repo.InsertSpace(ctx, sp); err != nil {
			return err
		}
		return e.repo.InsertChannel(ctx, ch)
	}); err != nil {
		t.Fatal(err)
	}
	// Random-looking text does not compress below the btree limit of a third of a page.
	var b strings.Builder
	for i := 0; b.Len() < 9000; i++ {
		b.WriteString(ids.New().String())
	}
	ref := b.String()
	err := e.write(t, func(ctx context.Context) error {
		return e.repo.ReplaceSkills(ctx, ch.ID, []string{"support-triage", ref})
	})
	e2, ok := errs.As(err)
	if !ok || e2.Code != errs.CodeValidationFailed {
		t.Fatalf("ReplaceSkills with an unindexable reference = %v, want validation_failed", err)
	}
	if len(e2.Fields) != 1 || e2.Fields[0].Pointer != "/skills" || e2.Fields[0].Code != "too_long" {
		t.Fatalf("fields = %+v, want one /skills too_long", e2.Fields)
	}
	if strings.Contains(err.Error(), ref[:64]) || strings.Contains(e2.Fields[0].Detail, ref[:64]) {
		t.Fatal("the error repeats the skill reference")
	}
}
