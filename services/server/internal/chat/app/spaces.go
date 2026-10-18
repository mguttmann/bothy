// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/chat/domain/fracindex"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// ListSpaces implements chat.Structure (07 section 7.4): Scope gives the visible spaces (all for
// owners and instance-wide readers, built per request, so a new space needs no new rights
// version), the query keeps the ones that are not archived in the global order, and authorize
// checks each row in filter mode; a row authorize refuses is left out.
func (s *Service) ListSpaces(ctx context.Context, a actor.Actor) ([]chat.SpaceView, error) {
	sc, err := s.d.Authorizer.Scope(ctx, a, authorize.ActionSpaceRead)
	if err != nil {
		return nil, err
	}
	rows, err := s.d.Store.VisibleSpaces(ctx, sc.InstanceWide, sc.SpaceIDs)
	if err != nil {
		return nil, err
	}
	fctx := authorize.Filter(ctx)
	out := make([]chat.SpaceView, 0, len(rows))
	for _, sp := range rows {
		d, err := s.d.Authorizer.Authorize(fctx, a, authorize.ActionSpaceRead, spaceResource(sp.ID))
		if err != nil {
			return nil, err
		}
		if !d.Allowed() {
			continue
		}
		v, err := s.view(ctx, a, sp)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// GetSpace implements chat.Structure: space.read, then 404 for an unknown id; an archived space
// stays readable.
func (s *Service) GetSpace(ctx context.Context, a actor.Actor, id ids.ID) (chat.SpaceView, error) {
	sp, err := s.load(ctx, a, authorize.ActionSpaceRead, id)
	if err != nil {
		return chat.SpaceView{}, err
	}
	return s.view(ctx, a, sp)
}

// CreateSpace implements chat.Structure in one unit of work: space.create on the instance, the
// checks of name, icon and color, then the order lock (every space that is not archived, ascending
// id) and the key after after_space_id or at the end, the insert, the audit entry and
// space.created.
func (s *Service) CreateSpace(ctx context.Context, a actor.Actor, cmd chat.CreateSpaceCmd) (chat.Space, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return chat.Space{}, err
	}
	var out domain.Space
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.create(ctx, a, cmd)
		return err
	})
	return spaceView(out), err
}

// create is CreateSpace inside its unit of work.
func (s *Service) create(ctx context.Context, a actor.Actor, cmd chat.CreateSpaceCmd) (domain.Space, error) {
	if err := s.check(ctx, a, authorize.ActionSpaceCreate, instanceResource); err != nil {
		return domain.Space{}, err
	}
	name, err := s.checkBody(&cmd.Name, cmd.Icon, cmd.Color)
	if err != nil {
		return domain.Space{}, err
	}
	order, err := s.lockOrder(ctx)
	if err != nil {
		return domain.Space{}, err
	}
	after := len(order) - 1
	if !cmd.AfterSpaceID.IsZero() {
		if after = indexOf(order, cmd.AfterSpaceID); after < 0 {
			return domain.Space{}, fieldError("/after_space_id", "the space to insert after is unknown or archived")
		}
	}
	key, err := between(order, after)
	if err != nil {
		return domain.Space{}, err
	}
	out := domain.NewSpace(ids.New(), name, deref(cmd.Icon), deref(cmd.Color), key, a.ID)
	out.CreatedAt = s.d.Clock.Now().UTC().Truncate(time.Microsecond)
	if err := s.d.Store.InsertSpace(ctx, out); err != nil {
		return domain.Space{}, err
	}
	if err := s.record(ctx, authorize.ActionSpaceCreate, out.ID, nil); err != nil {
		return domain.Space{}, err
	}
	return out, s.emitSpace(ctx, authorize.EventSpaceCreated, out)
}

// deref returns *s, "" for nil.
func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// checkBody checks the given name, icon and color and returns the trimmed name ("" without one).
func (s *Service) checkBody(name, icon, color *string) (string, error) {
	var out string
	if name != nil {
		n, err := domain.SpaceName(*name, s.cfg.SpaceNameMaxChars)
		if err != nil {
			return "", err
		}
		out = n
	}
	if icon != nil {
		if err := domain.CheckIcon(*icon); err != nil {
			return "", err
		}
	}
	if color != nil {
		if err := domain.CheckColor(*color); err != nil {
			return "", err
		}
	}
	return out, nil
}

// lockOrder locks the order of the spaces and returns it (docs/DECISIONS.md M1-52).
func (s *Service) lockOrder(ctx context.Context) ([]SpaceKey, error) {
	if err := s.d.Store.LockActiveSpaces(ctx); err != nil {
		return nil, err
	}
	return s.d.Store.ActiveOrder(ctx)
}

// indexOf returns the position of id in order, -1 when absent.
func indexOf(order []SpaceKey, id ids.ID) int {
	return slices.IndexFunc(order, func(k SpaceKey) bool { return k.ID == id })
}

// between returns a key after position i of order and before position i+1; i = -1 is the start.
func between(order []SpaceKey, i int) (string, error) {
	lo, hi := "", ""
	if i >= 0 {
		lo = order[i].SortKey
	}
	if i+1 < len(order) {
		hi = order[i+1].SortKey
	}
	k, err := fracindex.KeyBetween(lo, hi)
	if err != nil {
		return "", errs.Wrap(errs.CodeInternalError, err, "chat: the global order of the spaces is broken")
	}
	return k, nil
}

// fieldError returns validation_failed with one field error of code invalid.
func fieldError(pointer, detail string) error {
	return errs.New(errs.CodeValidationFailed, detail).WithFields(errs.FieldError{Pointer: pointer, Code: domain.FieldInvalid, Detail: detail})
}

// UpdateSpace implements chat.Structure in one unit of work (space.update is no rights change):
// it authorizes before the row lock, refuses an archived space with 409, checks If-Match against
// the locked version and the body, and bumps the version once. A call without change records
// no_change and appends nothing.
func (s *Service) UpdateSpace(ctx context.Context, a actor.Actor, id ids.ID, cmd chat.UpdateSpaceCmd) (chat.Space, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return chat.Space{}, err
	}
	var out domain.Space
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.update(ctx, a, id, cmd)
		return err
	})
	return spaceView(out), err
}

// update is UpdateSpace inside its unit of work.
func (s *Service) update(ctx context.Context, a actor.Actor, id ids.ID, cmd chat.UpdateSpaceCmd) (domain.Space, error) {
	if _, err := s.load(ctx, a, authorize.ActionSpaceUpdate, id); err != nil {
		return domain.Space{}, err
	}
	sp, err := s.lock(ctx, id)
	if err != nil {
		return domain.Space{}, err
	}
	if sp.Archived() {
		return domain.Space{}, archivedConflict()
	}
	if err := precondition(cmd.IfMatch, sp); err != nil {
		return domain.Space{}, err
	}
	name, err := s.checkBody(cmd.Name, cmd.Icon, cmd.Color)
	if err != nil {
		return domain.Space{}, err
	}
	if cmd.Name != nil {
		cmd.Name = &name
	}
	changed := sp.Update(cmd.Name, cmd.Icon, cmd.Color)
	if len(changed) == 0 {
		return sp, s.noChange(ctx, authorize.ActionSpaceUpdate, id, "")
	}
	if err := s.d.Store.UpdateSpace(ctx, sp); err != nil {
		return domain.Space{}, err
	}
	if err := s.record(ctx, authorize.ActionSpaceUpdate, id, auditing.Attrs{attrChangedFields: codes(changed)}); err != nil {
		return domain.Space{}, err
	}
	return sp, s.emitSpace(ctx, authorize.EventSpaceUpdated, sp)
}

// precondition checks the If-Match of a request against the current space (06 section 8): no
// header is 428 precondition_required, a stale or malformed one 412 version_conflict with the
// current space.
func precondition(p chat.Precondition, sp domain.Space) error {
	if !p.Present {
		return errs.PreconditionRequired()
	}
	if p.Valid && p.Version == sp.Version {
		return nil
	}
	return errs.VersionConflict(spaceBody(sp))
}

// MoveSpace implements chat.Structure in one unit of work: space.reorder on the instance, the
// position check (at most one of after_id and before_id, never the space itself, only spaces
// that are not archived), the order lock, a new key between the neighbours and one written row
// (sort_key and version), the audit entry and space.updated.
func (s *Service) MoveSpace(ctx context.Context, a actor.Actor, id ids.ID, cmd chat.MoveSpaceCmd) (chat.Space, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return chat.Space{}, err
	}
	var out domain.Space
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.move(ctx, a, id, cmd)
		return err
	})
	return spaceView(out), err
}

// move is MoveSpace inside its unit of work.
func (s *Service) move(ctx context.Context, a actor.Actor, id ids.ID, cmd chat.MoveSpaceCmd) (domain.Space, error) {
	if err := s.check(ctx, a, authorize.ActionSpaceReorder, instanceResource); err != nil {
		return domain.Space{}, err
	}
	if !cmd.AfterID.IsZero() && !cmd.BeforeID.IsZero() {
		return domain.Space{}, fieldError("/before_id", "after_id and before_id exclude each other")
	}
	order, err := s.lockOrder(ctx)
	if err != nil {
		return domain.Space{}, err
	}
	sp, err := s.lock(ctx, id)
	if err != nil {
		return domain.Space{}, err
	}
	if sp.Archived() {
		return domain.Space{}, archivedConflict()
	}
	order = slices.DeleteFunc(order, func(k SpaceKey) bool { return k.ID == id })
	pos, err := position(order, id, cmd)
	if err != nil {
		return domain.Space{}, err
	}
	key, err := between(order, pos)
	if err != nil {
		return domain.Space{}, err
	}
	sp.Move(key)
	if err := s.d.Store.MoveSpace(ctx, sp); err != nil {
		return domain.Space{}, err
	}
	if err := s.record(ctx, authorize.ActionSpaceReorder, id, nil); err != nil {
		return domain.Space{}, err
	}
	return sp, s.emitSpace(ctx, authorize.EventSpaceUpdated, sp)
}

// position returns the index in order (without the moved space id) after which the space goes:
// -1 for the start.
func position(order []SpaceKey, id ids.ID, cmd chat.MoveSpaceCmd) (int, error) {
	switch {
	case !cmd.AfterID.IsZero():
		i := indexOf(order, cmd.AfterID)
		if cmd.AfterID == id || i < 0 {
			return 0, fieldError("/after_id", "after_id is the space itself, unknown or archived")
		}
		return i, nil
	case !cmd.BeforeID.IsZero():
		i := indexOf(order, cmd.BeforeID)
		if cmd.BeforeID == id || i < 0 {
			return 0, fieldError("/before_id", "before_id is the space itself, unknown or archived")
		}
		return i - 1, nil
	}
	return -1, nil
}

// ArchiveSpace implements chat.Structure (07 section 4.5). A first unit loads, authorizes
// space.archive (owner, step-up) and checks the reason; an archived space is a call without
// effect that answers the space. Otherwise a second unit, declared as the rights change
// space.archive with the space as target, locks the row, sets archived_at, bumps the version,
// publishes SpaceArchived, records the audit entry with the reason and appends space.archived;
// the rights hook raises permission_version and writes access.changed for everyone whose level on
// the space was above read.
func (s *Service) ArchiveSpace(ctx context.Context, a actor.Actor, id ids.ID, reason string) (chat.Space, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return chat.Space{}, err
	}
	var (
		out    domain.Space
		change bool
	)
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, change, err = s.archiveCheck(ctx, a, id, reason)
		return err
	})
	if err != nil || !change {
		return spaceView(out), err
	}
	ref := txn.Ref{Kind: resourceSpace, ID: id}
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.archiveWrite(ctx, id, reason)
		return err
	}, txn.DeclareRightsChange(txn.RightsChangeSpec{Action: string(authorize.ActionSpaceArchive), Resource: ref, Targets: []txn.Ref{ref}}))
	return spaceView(out), err
}

// archiveCheck is the first unit of ArchiveSpace: it loads and authorizes, checks the reason and
// records an archived space as a call without effect; change reports a space to archive.
func (s *Service) archiveCheck(ctx context.Context, a actor.Actor, id ids.ID, reason string) (domain.Space, bool, error) {
	sp, err := s.load(ctx, a, authorize.ActionSpaceArchive, id)
	if err != nil {
		return domain.Space{}, false, err
	}
	if err := domain.CheckReason(reason); err != nil {
		return domain.Space{}, false, err
	}
	if sp.Archived() {
		return sp, false, s.noChange(ctx, authorize.ActionSpaceArchive, id, reason)
	}
	return sp, true, nil
}

// archiveWrite is the second unit of ArchiveSpace, declared as the rights change space.archive.
func (s *Service) archiveWrite(ctx context.Context, id ids.ID, reason string) (domain.Space, error) {
	sp, err := s.lock(ctx, id)
	if err != nil {
		return domain.Space{}, err
	}
	if sp.Archived() {
		// A concurrent call won the race: answered as without effect (06 section 2).
		return sp, s.noChange(ctx, authorize.ActionSpaceArchive, id, reason)
	}
	sp.Archive(s.d.Clock.Now().UTC().Truncate(time.Microsecond))
	if err := s.d.Store.ArchiveSpace(ctx, sp); err != nil {
		return domain.Space{}, err
	}
	if err := txn.Publish(ctx, chat.SpaceArchived{SpaceID: id}); err != nil {
		return domain.Space{}, err
	}
	if err := s.record(ctx, authorize.ActionSpaceArchive, id, auditing.Attrs{attrReason: auditing.ReasonText(reason)}); err != nil {
		return domain.Space{}, err
	}
	return sp, s.emitSpace(ctx, authorize.EventSpaceArchived, sp)
}
