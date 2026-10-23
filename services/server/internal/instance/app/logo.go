// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/instance/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// attrFileID is the audit attribute of the uploaded logo file (brief M3-10 design note 9).
const attrFileID = "file_id"

// Compile-time check: the incoming port of the logo operations.
var _ instance.Logos = (*Admin)(nil)

// logoKeys are the keys the logo operations lock and write: only branding.logo_file_id.
var logoKeys = []string{domain.KeyLogoFileID}

// BindLogoFiles binds the source port through which the logo operations reach the module files
// (docs/decisions.d/M3-10.md D-M3-10-1). internal/wire calls it once at start, because it builds files
// after instance; until then uploadBrandingLogo answers 503 unavailable and getBrandingLogo 404.
// A nil f unbinds.
//
// Example:
//
//	admin.BindLogoFiles(profileimages.Logo{Images: filesService})
func (s *Admin) BindLogoFiles(f instance.LogoFiles) {
	if f == nil {
		s.logos.Store(nil)
		return
	}
	s.logos.Store(&f)
}

// logoFiles returns the bound source port, nil before BindLogoFiles.
func (s *Admin) logoFiles() instance.LogoFiles {
	if p := s.logos.Load(); p != nil {
		return *p
	}
	return nil
}

// errLogoUnavailable is the answer of a logo write before the source port is bound.
func errLogoUnavailable() error {
	return errs.New(errs.CodeUnavailable, "")
}

// errLogoNotFound is the one answer of getBrandingLogo without a deliverable logo: absent, empty,
// unbound and not deliverable look the same (05 rule 2, brief M3-10 design note 8).
func errLogoNotFound() error {
	return errs.New(errs.CodeNotFound, "")
}

// UploadLogo implements instance.Logos (uploadBrandingLogo, owner with step-up, audit). It
// authorizes before open reads one byte of the body (brief M3-10 design note 5; authz audits a
// denial itself), stages the image outside any unit of work (no object store call inside a
// transaction, 02 8.5), then links it in one unit of work. No If-Match (design note 10): the row
// lock serializes concurrent uploads, the last one wins. Errors: those of authorize, open and
// LogoFiles; 503 unavailable before BindLogoFiles.
func (s *Admin) UploadLogo(ctx context.Context, a actor.Actor, open instance.LogoOpener) (instance.Branding, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return instance.Branding{}, err
	}
	if err := s.check(ctx, a, authorize.ActionInstanceBrandingUpdate); err != nil {
		return instance.Branding{}, err
	}
	files := s.logoFiles()
	if files == nil {
		return instance.Branding{}, errLogoUnavailable()
	}
	if open == nil {
		return instance.Branding{}, errs.New(errs.CodeInternalError, "instance: UploadLogo without an opener")
	}
	img, err := open()
	if err != nil {
		return instance.Branding{}, err
	}
	fileID, err := files.Stage(ctx, a.ID, img)
	if err != nil {
		return instance.Branding{}, err
	}
	var out instance.Branding
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		out, err = s.uploadLocked(ctx, a, files, fileID)
		return err
	})
	return out, err
}

// uploadLocked is the unit of work of UploadLogo: write the new id under the row lock, link the
// staged file, release the previous logo, append instance.updated and audit.
func (s *Admin) uploadLocked(ctx context.Context, a actor.Actor, files instance.LogoFiles, fileID ids.ID) (instance.Branding, error) {
	prev, err := s.writeLogo(ctx, a, fileID.String())
	if err != nil {
		return instance.Branding{}, err
	}
	if err := files.Attach(ctx, fileID, a.ID); err != nil {
		return instance.Branding{}, err
	}
	if !prev.IsZero() {
		if err := files.Release(ctx, prev); err != nil {
			return instance.Branding{}, err
		}
	}
	if err := s.emit(ctx, logoKeys); err != nil {
		return instance.Branding{}, err
	}
	if err := s.recordLogo(ctx, logoKeys, fileID); err != nil {
		return instance.Branding{}, err
	}
	// The lock covers only the logo row; the answer needs the whole branding, read in the same
	// transaction after the write.
	return s.Branding(ctx)
}

// writeLogo locks branding.logo_file_id and writes value with version + 1 (version 1 for the
// first row). It returns the logo it replaced (zero for none). The
// write runs before Attach, so the second round after a concurrent first insert (as in
// PutSetting) sees the winner's row and never links the staged file twice; both run in the same
// unit of work, so the order is not visible outside it.
func (s *Admin) writeLogo(ctx context.Context, a actor.Actor, value string) (ids.ID, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return ids.ID{}, errs.Wrap(errs.CodeInternalError, err, "instance: logo value")
	}
	for range 2 {
		v, prev, err := s.lockLogo(ctx)
		if err != nil {
			return ids.ID{}, err
		}
		_, err = s.write(ctx, v, SettingRow{Key: domain.KeyLogoFileID, Value: raw, UpdatedBy: a.ID, UpdatedAt: s.d.Clock.Now()})
		if errors.Is(err, errConcurrentInsert) {
			continue
		}
		return prev, err
	}
	return ids.ID{}, errs.New(errs.CodeInternalError, "instance: the logo setting row is neither insertable nor lockable")
}

// lockLogo takes the lock of branding.logo_file_id and returns the view of the locked row and the
// current logo (zero for an absent row or domain.NoLogo).
func (s *Admin) lockLogo(ctx context.Context) (view, ids.ID, error) {
	rows, err := s.d.Settings.LockSettings(ctx, logoKeys)
	if err != nil {
		return view{}, ids.ID{}, err
	}
	v := s.newView(rows)
	cur, err := v.logo()
	return v, cur, err
}

// logo returns the logo file id of the view; zero for an absent row or domain.NoLogo. Errors: a
// stored value that is no logo file id (internal).
func (v view) logo() (ids.ID, error) {
	stored, err := v.str(domain.KeyLogoFileID)
	if err != nil {
		return ids.ID{}, err
	}
	id, err := domain.LogoFileID(stored)
	if err != nil {
		return ids.ID{}, errs.Wrap(errs.CodeInternalError, err, "instance: the stored logo file id is not a UUID")
	}
	return id, nil
}

// RemoveLogo implements instance.Logos (deleteBrandingLogo, owner with step-up, audit). It is
// idempotent (06 7): without a logo it writes nothing and appends no event, but audits the allow
// with empty changed_keys (05 2.4). With a logo it stores domain.NoLogo with version + 1 (the row
// is never deleted, so the branding version keeps rising, design note 9) and releases the file.
// Errors: those of authorize; 503 unavailable when a logo exists before BindLogoFiles.
func (s *Admin) RemoveLogo(ctx context.Context, a actor.Actor) error {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return err
	}
	if err := s.check(ctx, a, authorize.ActionInstanceBrandingUpdate); err != nil {
		return err
	}
	return s.d.UoW.Do(ctx, func(ctx context.Context) error {
		v, cur, err := s.lockLogo(ctx)
		if err != nil {
			return err
		}
		if cur.IsZero() {
			return s.recordLogo(ctx, nil, ids.ID{})
		}
		files := s.logoFiles()
		if files == nil {
			return errLogoUnavailable()
		}
		row := SettingRow{Key: domain.KeyLogoFileID, Value: jsonNoLogo, UpdatedBy: a.ID, UpdatedAt: s.d.Clock.Now()}
		if _, err := s.write(ctx, v, row); err != nil {
			return err
		}
		if err := files.Release(ctx, cur); err != nil {
			return err
		}
		if err := s.emit(ctx, logoKeys); err != nil {
			return err
		}
		return s.recordLogo(ctx, logoKeys, ids.ID{})
	})
}

// jsonNoLogo is domain.NoLogo as the stored JSON value.
var jsonNoLogo = json.RawMessage(`""`)

// recordLogo writes the allow entry of instance.branding.update of a logo operation: changed_keys
// (empty for a removal without logo) and, for an upload, file_id; only ids and keys.
func (s *Admin) recordLogo(ctx context.Context, changed []string, fileID ids.ID) error {
	vs := make([]auditing.Value, 0, len(changed))
	for _, k := range changed {
		vs = append(vs, auditing.Code(k))
	}
	attrs := auditing.Attrs{attrChangedKeys: auditing.List(vs...)}
	if !fileID.IsZero() {
		attrs[attrFileID] = auditing.ID(fileID)
	}
	return s.d.Audit.Record(ctx, auditing.Entry{
		Action: string(authorize.ActionInstanceBrandingUpdate), Decision: auditing.DecisionAllow,
		ResourceKind: string(authorize.ResourceInstance), Attrs: attrs,
	})
}

// OpenLogo implements instance.Logos (getBrandingLogo, public through instance.discovery.read,
// also for the anonymous caller): the variant medium of the current logo. Errors: those of
// authorize; one and the same not_found while no deliverable logo exists (absent, domain.NoLogo,
// port not bound, or files answers not_found).
func (s *Admin) OpenLogo(ctx context.Context, a actor.Actor) (instance.LogoVariant, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return instance.LogoVariant{}, err
	}
	if err := s.check(ctx, a, authorize.ActionInstanceDiscoveryRead); err != nil {
		return instance.LogoVariant{}, err
	}
	v, err := s.load(ctx)
	if err != nil {
		return instance.LogoVariant{}, err
	}
	logo, err := v.logo()
	if err != nil {
		return instance.LogoVariant{}, err
	}
	files := s.logoFiles()
	if logo.IsZero() || files == nil {
		return instance.LogoVariant{}, errLogoNotFound()
	}
	out, err := files.OpenVariant(ctx, logo)
	if e, ok := errs.As(err); ok && e.PublicCode() == errs.CodeNotFound {
		// The detail of files may differ per cause; the public answer must not (AZ-P4).
		return instance.LogoVariant{}, errLogoNotFound()
	}
	if err != nil {
		return instance.LogoVariant{}, err
	}
	return out, nil
}
