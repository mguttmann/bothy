// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestAccessReadsRefuse (M1-49): who-has-access, preview and view-as refuse a caller other than
// the principal of the request, a target outside their scope, an unknown person and a preview
// inside a unit of work, before they read anything.
func TestAccessReadsRefuse(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	owner, sam := w.user(w.owner), w.user(w.sam)
	ctx := ctxOf(owner)
	cmd := authz.CreateGrantCmd{SubjectKind: "user", SubjectID: w.sam, TargetKind: "channel", TargetID: w.kampagnen, Level: "trigger", Reason: "preview"}

	if _, err := w.admin.ExplainAccess(ctxOf(sam), owner, authz.AccessTargetRef{Kind: "channel", ID: w.kampagnen}); err == nil {
		t.Error("explain with another caller than the request's principal passed")
	}
	if _, err := w.admin.ExplainAccess(ctx, owner, authz.AccessTargetRef{Kind: "user", ID: w.sam}); errs.KindOf(err) != errs.KindInternal {
		t.Errorf("explain of a person = %v, want internal", err)
	}
	if _, err := w.admin.ViewAs(ctxOf(sam), owner, w.alex); err == nil {
		t.Error("view-as with another caller than the request's principal passed")
	}
	if _, err := w.admin.ViewAs(ctx, owner, ids.New()); errs.KindOf(err) != errs.KindNotFound {
		t.Errorf("view-as of an unknown person = %v, want not_found", err)
	}
	if _, err := w.admin.ViewAs(ctxOf(sam), sam, w.alex); err == nil {
		t.Error("view-as without owner role passed")
	}
	if _, err := w.admin.PreviewGrant(ctxOf(sam), owner, cmd); err == nil {
		t.Error("preview with another caller than the request's principal passed")
	}
	bad := cmd
	bad.Level = "no-such-level"
	if _, err := w.admin.PreviewGrant(ctx, owner, bad); errs.KindOf(err) != errs.KindBadRequest {
		t.Errorf("preview of an invalid grant = %v, want validation_failed", err)
	}
	err := w.uow.Do(ctx, func(ctx context.Context) error {
		_, err := w.admin.PreviewGrant(ctx, owner, cmd)
		return err
	})
	if errs.KindOf(err) != errs.KindInternal {
		t.Errorf("preview inside a unit of work = %v, want internal", err)
	}
}
