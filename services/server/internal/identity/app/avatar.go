// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: People serves the avatar operations (M3-10).
var _ identity.Avatars = (*People)(nil)

// errAvatarFilesUnbound is the internal cause of 503 unavailable while internal/wire has not
// bound the files of the avatars; the client sees no detail.
var errAvatarFilesUnbound = errors.New("identity: no AvatarFiles bound")

// BindAvatarFiles hands the use cases the source port of the module files (02 4.3). internal/wire
// calls it once at start, because files is built after identity. A nil f is ignored.
//
// Example:
//
//	people.BindAvatarFiles(profileimages.Avatar{Images: filesService})
func (s *People) BindAvatarFiles(f identity.AvatarFiles) {
	if f == nil {
		return
	}
	s.avatarMu.Lock()
	defer s.avatarMu.Unlock()
	s.avatarFiles = f
}

// boundAvatarFiles returns the bound port or 503 unavailable before BindAvatarFiles ran.
func (s *People) boundAvatarFiles() (identity.AvatarFiles, error) {
	s.avatarMu.RLock()
	defer s.avatarMu.RUnlock()
	if s.avatarFiles == nil {
		return nil, errs.Wrap(errs.CodeUnavailable, errAvatarFilesUnbound, "")
	}
	return s.avatarFiles, nil
}

// SetMyAvatar implements identity.Avatars (setMyAvatar, brief M3-10 design notes 3 and 5):
// self.profile.update on the caller's person without step-up and without audit. The decision and
// the kind check come before open, so a refused caller costs no byte of the body; Stage writes the
// object outside any unit of work (02 8.5); one unit then locks the person, attaches the file,
// links it, releases the previous avatar and appends user.updated. Errors: 401, 403
// subject_kind_not_allowed for an agent or app, 503 unavailable without bound files, the errors of
// open, Stage and Attach unchanged.
func (s *People) SetMyAvatar(ctx context.Context, a actor.Actor, open identity.AvatarOpener) (identity.UserView, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return identity.UserView{}, err
	}
	if err := s.checkMyAvatar(ctx, a); err != nil {
		return identity.UserView{}, err
	}
	files, err := s.boundAvatarFiles()
	if err != nil {
		return identity.UserView{}, err
	}
	img, err := open()
	if err != nil {
		return identity.UserView{}, err
	}
	fileID, err := files.Stage(ctx, a.ID, img)
	if err != nil {
		return identity.UserView{}, err
	}
	var out identity.UserView
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.linkAvatar(ctx, files, a.ID, fileID)
		return err
	})
	return out, err
}

// DeleteMyAvatar implements identity.Avatars (deleteMyAvatar): the same check as SetMyAvatar, then
// one unit that locks the person and, when it has an avatar, clears it, releases the file and
// appends user.updated. Without an avatar it writes nothing and emits nothing (06 7: DELETE is
// idempotent). Errors: as SetMyAvatar; 503 unavailable only when there is an avatar to release.
func (s *People) DeleteMyAvatar(ctx context.Context, a actor.Actor) error {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return err
	}
	if err := s.checkMyAvatar(ctx, a); err != nil {
		return err
	}
	return s.d.UoW.Do(ctx, func(ctx context.Context) error { return s.unlinkAvatar(ctx, a.ID) })
}

// checkMyAvatar authorizes self.profile.update on the caller's person once, outside the unit of
// work: the check log of the request records it for the proof stage of the chain. The avatar is a
// column of a person (03 4.2; agents have no operation for it), so an agent or app gets the 403 of
// updateMyProfile.
func (s *People) checkMyAvatar(ctx context.Context, a actor.Actor) error {
	if err := s.check(ctx, a, authorize.ActionSelfProfileUpdate, userResource(a.ID)); err != nil {
		return err
	}
	if a.Kind != actor.KindUser {
		return errs.New(errs.CodeForbidden, "").WithReason(reasonKindNotUser)
	}
	return nil
}

// linkAvatar is the unit of SetMyAvatar. The person row is locked first and the files rows after
// it (03 6.3), so two concurrent uploads of one person serialize here and each releases the
// avatar the other linked.
func (s *People) linkAvatar(ctx context.Context, files identity.AvatarFiles, id, fileID ids.ID) (identity.UserView, error) {
	rec, found, err := s.d.Store.LockUser(ctx, id)
	if err != nil {
		return identity.UserView{}, err
	}
	if !found {
		return identity.UserView{}, invisible()
	}
	if err := files.Attach(ctx, fileID, id); err != nil {
		return identity.UserView{}, err
	}
	if err := s.d.Store.SetAvatar(ctx, id, fileID); err != nil {
		return identity.UserView{}, err
	}
	if prev := rec.User.Principal.AvatarFileID; !prev.IsZero() && prev != fileID {
		if err := files.Release(ctx, prev, id); err != nil {
			return identity.UserView{}, err
		}
	}
	rec.User.Principal.AvatarFileID = fileID
	out := userView(rec)
	return out, s.emitUpdated(ctx, out)
}

// unlinkAvatar is the unit of DeleteMyAvatar.
func (s *People) unlinkAvatar(ctx context.Context, id ids.ID) error {
	rec, found, err := s.d.Store.LockUser(ctx, id)
	if err != nil {
		return err
	}
	if !found {
		return invisible()
	}
	prev := rec.User.Principal.AvatarFileID
	if prev.IsZero() {
		return nil
	}
	files, err := s.boundAvatarFiles()
	if err != nil {
		return err
	}
	if err := s.d.Store.SetAvatar(ctx, id, ids.ID{}); err != nil {
		return err
	}
	if err := files.Release(ctx, prev, id); err != nil {
		return err
	}
	rec.User.Principal.AvatarFileID = ids.ID{}
	return s.emitUpdated(ctx, userView(rec))
}
