// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/instance/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
)

// The audit attribute names of feature_flag.update (brief M1-47 "putFeatureFlag"); the audit
// values hold no nested object, so the rollout is two ID lists.
const (
	attrEnabled         = "enabled"
	attrRolloutUserIDs  = "rollout_user_ids"
	attrRolloutGroupIDs = "rollout_group_ids"
	attrFlagReason      = "reason"
)

// FlagRow is one row of feature_flags as the use cases read and write it.
type FlagRow struct {
	// Key is the flag key.
	Key string
	// Enabled is the global value.
	Enabled bool
	// Rollout is the stored rollout, normalized.
	Rollout domain.Rollout
	// Version is the row version, 1 after the first write.
	Version int64
	// UpdatedBy is the principal of the last write; zero for NULL.
	UpdatedBy ids.ID
	// UpdatedAt is the time of the last write.
	UpdatedAt time.Time
}

// FlagRepository is the outgoing port to feature_flags; adapters/postgres implements it.
// Statements run in the ambient transaction of ctx when there is one.
type FlagRepository interface {
	// GetFlag reads the row of key by its primary key; found is false without a row.
	GetFlag(ctx context.Context, key string) (row FlagRow, found bool, err error)
	// ListFlags returns every stored row, registered or orphaned, in key order.
	ListFlags(ctx context.Context) ([]FlagRow, error)
	// LockFlag reads the row of key under its row lock until the commit (SELECT ... FOR UPDATE);
	// found is false without a row. It requires a unit of work.
	LockFlag(ctx context.Context, key string) (row FlagRow, found bool, err error)
	// InsertFlag inserts the first row of row.Key with version 1 and returns it. inserted is false
	// when a concurrent writer inserted the key first (ON CONFLICT DO NOTHING).
	InsertFlag(ctx context.Context, row FlagRow) (out FlagRow, inserted bool, err error)
	// UpdateFlag writes enabled, rollout, updated_by and updated_at of a locked row with
	// version + 1 and returns the new row.
	UpdateFlag(ctx context.Context, row FlagRow) (FlagRow, error)
}

// Compile-time checks: Admin is the read port and the incoming port of the flags.
var (
	_ instance.Flags     = (*Admin)(nil)
	_ instance.FlagAdmin = (*Admin)(nil)
)

// flagDefinition returns the registration of key; ok is false for a key that is not registered.
func flagDefinition(key string) (instance.FlagDefinition, bool) {
	defs := instance.RegisteredFlags()
	i := slices.IndexFunc(defs, func(d instance.FlagDefinition) bool { return d.Key == key })
	if i < 0 {
		return instance.FlagDefinition{}, false
	}
	return defs[i], true
}

// FlagSpecs converts the registry into its domain form for domain.CheckRegistry.
func FlagSpecs(defs []instance.FlagDefinition) []domain.FlagSpec {
	out := make([]domain.FlagSpec, 0, len(defs))
	for _, d := range defs {
		out = append(out, domain.FlagSpec{Key: d.Key, Kind: string(d.Kind), Description: d.Description, Module: d.Module, RemoveBy: d.RemoveBy, Default: d.Default})
	}
	return out
}

// subjectGroups returns the valid groups of s at the time of the clock, read only when s is a
// person and some rollout of rows names a group; nil otherwise.
func (s *Admin) subjectGroups(ctx context.Context, subj instance.FlagSubject, rows ...FlagRow) ([]ids.ID, error) {
	if subj.Kind != domain.SubjectUser {
		return nil, nil
	}
	if !slices.ContainsFunc(rows, func(r FlagRow) bool { return len(r.Rollout.GroupIDs) > 0 }) {
		return nil, nil
	}
	return s.d.FlagGroups.ValidGroupIDs(ctx, subj.PrincipalID, s.d.Clock.Now())
}

// evaluate returns the value of def for subj with the stored row (found false: the default).
func evaluate(def instance.FlagDefinition, row FlagRow, found bool, subj instance.FlagSubject, groups []ids.ID) bool {
	if !found {
		return def.Default
	}
	return domain.Evaluate(row.Enabled, domain.InRollout(row.Rollout, subj.Kind, subj.PrincipalID, groups))
}

// Enabled implements instance.Flags: one primary-key read, the valid groups at most once; an
// unknown key is off without a read. No cache, so a change in another process applies at once.
func (s *Admin) Enabled(ctx context.Context, subj instance.FlagSubject, key string) (bool, error) {
	def, ok := flagDefinition(key)
	if !ok {
		return false, nil
	}
	row, found, err := s.d.Flags.GetFlag(ctx, key)
	if err != nil {
		return false, err
	}
	groups, err := s.subjectGroups(ctx, subj, row)
	if err != nil {
		return false, err
	}
	return evaluate(def, row, found, subj, groups), nil
}

// EvaluateAll implements instance.Flags: every registered flag in key order from one read of all
// rows and at most one read of the valid groups; orphaned rows never appear.
func (s *Admin) EvaluateAll(ctx context.Context, subj instance.FlagSubject) ([]instance.EvaluatedFlag, error) {
	rows, err := s.d.Flags.ListFlags(ctx)
	if err != nil {
		return nil, err
	}
	groups, err := s.subjectGroups(ctx, subj, rows...)
	if err != nil {
		return nil, err
	}
	byKey := map[string]FlagRow{}
	for _, r := range rows {
		byKey[r.Key] = r
	}
	defs := instance.RegisteredFlags()
	out := make([]instance.EvaluatedFlag, 0, len(defs))
	for _, d := range defs {
		r, found := byKey[d.Key]
		out = append(out, instance.EvaluatedFlag{Key: d.Key, Enabled: evaluate(d, r, found, subj, groups)})
	}
	return out, nil
}

// toFeatureFlag returns the API view of a registered flag with its row (found false: the default
// with version 0).
func toFeatureFlag(def instance.FlagDefinition, row FlagRow, found bool) instance.FeatureFlag {
	f := instance.FeatureFlag{
		Key: def.Key, Kind: def.Kind, Description: def.Description, Default: def.Default, Module: def.Module, RemoveBy: def.RemoveBy,
		Enabled: def.Default,
	}
	if found {
		f.Enabled, f.Version, f.UpdatedBy, f.UpdatedAt = row.Enabled, row.Version, row.UpdatedBy, row.UpdatedAt
		f.Rollout = instance.FlagRollout{UserIDs: slices.Clone(row.Rollout.UserIDs), GroupIDs: slices.Clone(row.Rollout.GroupIDs)}
	}
	return f
}

// ListFlags implements instance.FlagAdmin (listFeatureFlags, owner): every registered flag (the
// row or its default), then every orphaned row, both in key order.
func (s *Admin) ListFlags(ctx context.Context, a actor.Actor) ([]instance.FeatureFlag, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return nil, err
	}
	if err := s.check(ctx, a, authorize.ActionFeatureFlagRead); err != nil {
		return nil, err
	}
	rows, err := s.d.Flags.ListFlags(ctx)
	if err != nil {
		return nil, err
	}
	byKey := map[string]FlagRow{}
	for _, r := range rows {
		byKey[r.Key] = r
	}
	out := []instance.FeatureFlag{}
	for _, d := range instance.RegisteredFlags() {
		r, found := byKey[d.Key]
		out = append(out, toFeatureFlag(d, r, found))
		delete(byKey, d.Key)
	}
	for _, r := range rows {
		if _, orphan := byKey[r.Key]; orphan {
			out = append(out, instance.FeatureFlag{
				Key: r.Key, Orphaned: true, Enabled: r.Enabled, Version: r.Version, UpdatedBy: r.UpdatedBy, UpdatedAt: r.UpdatedAt,
				Rollout: instance.FlagRollout{UserIDs: slices.Clone(r.Rollout.UserIDs), GroupIDs: slices.Clone(r.Rollout.GroupIDs)},
			})
		}
	}
	return out, nil
}

// PutFlag implements instance.FlagAdmin (putFeatureFlag, owner with step-up): authorize, 404 for
// a key that is not registered, 400 for the body, then under the row lock 412 for a precondition
// mismatch, no write for the same value and rollout, else the upsert with instance.updated and
// the allow entry.
func (s *Admin) PutFlag(ctx context.Context, a actor.Actor, cmd instance.PutFlagCmd) (instance.FeatureFlag, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return instance.FeatureFlag{}, err
	}
	var out instance.FeatureFlag
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		if err := s.check(ctx, a, authorize.ActionFeatureFlagUpdate); err != nil {
			return err
		}
		def, ok := flagDefinition(cmd.Key)
		if !ok {
			return errs.New(errs.CodeNotFound, "")
		}
		if err := s.validateFlagPut(ctx, cmd); err != nil {
			return err
		}
		rollout := domain.NormalizeRollout(domain.Rollout{UserIDs: cmd.Rollout.UserIDs, GroupIDs: cmd.Rollout.GroupIDs})
		out, err = s.putFlagLocked(ctx, a, def, cmd, rollout)
		return err
	})
	return out, err
}

// errConcurrentFlagInsert reports that a concurrent writer inserted the flag first.
var errConcurrentFlagInsert = errors.New("instance: a concurrent writer inserted the flag first")

// putFlagLocked is PutFlag under the row lock of the key. A lost first insert means a concurrent
// writer committed a newer version: the second round reads it, so with If-Match the caller's
// version is outdated (412), without it the round writes over the new row.
func (s *Admin) putFlagLocked(ctx context.Context, a actor.Actor, def instance.FlagDefinition, cmd instance.PutFlagCmd, rollout domain.Rollout) (instance.FeatureFlag, error) {
	for range 2 {
		out, err := s.putFlagRound(ctx, a, def, cmd, rollout)
		if !errors.Is(err, errConcurrentFlagInsert) {
			return out, err
		}
	}
	return instance.FeatureFlag{}, errs.New(errs.CodeInternalError, "instance: the flag row is neither insertable nor lockable")
}

// putFlagRound is one round of putFlagLocked. Errors: errConcurrentFlagInsert when a concurrent
// writer inserted the key first.
func (s *Admin) putFlagRound(ctx context.Context, a actor.Actor, def instance.FlagDefinition, cmd instance.PutFlagCmd, rollout domain.Rollout) (instance.FeatureFlag, error) {
	row, found, err := s.d.Flags.LockFlag(ctx, def.Key)
	if err != nil {
		return instance.FeatureFlag{}, err
	}
	cur := toFeatureFlag(def, row, found)
	if cmd.IfMatch.Present && !matches(cmd.IfMatch, cur.Version) {
		return instance.FeatureFlag{}, errs.VersionConflict(cur)
	}
	if cur.Enabled == cmd.Enabled && (domain.Rollout{UserIDs: cur.Rollout.UserIDs, GroupIDs: cur.Rollout.GroupIDs}).Equal(rollout) {
		return cur, s.recordFlag(ctx, def.Key, cmd, rollout, true)
	}
	written, err := s.writeFlag(ctx, found, FlagRow{Key: def.Key, Enabled: cmd.Enabled, Rollout: rollout, UpdatedBy: a.ID, UpdatedAt: s.d.Clock.Now()})
	if err != nil {
		return instance.FeatureFlag{}, err
	}
	if err := s.emit(ctx, []string{def.Key}); err != nil {
		return instance.FeatureFlag{}, err
	}
	if err := s.recordFlag(ctx, def.Key, cmd, rollout, false); err != nil {
		return instance.FeatureFlag{}, err
	}
	return toFeatureFlag(def, written, true), nil
}

// writeFlag updates the locked row when found, else inserts the first row. Errors:
// errConcurrentFlagInsert when a concurrent writer inserted the key first.
func (s *Admin) writeFlag(ctx context.Context, found bool, next FlagRow) (FlagRow, error) {
	if found {
		return s.d.Flags.UpdateFlag(ctx, next)
	}
	written, inserted, err := s.d.Flags.InsertFlag(ctx, next)
	if err == nil && !inserted {
		err = errConcurrentFlagInsert
	}
	return written, err
}

// validateFlagPut checks the body: a reason that is not blank and has no forbidden character,
// and every rollout ID a person (FlagUserSource) or a group that exists (FlagGroupSource). The
// pointers name the index in the request as sent.
func (s *Admin) validateFlagPut(ctx context.Context, cmd instance.PutFlagCmd) error {
	fields := reasonFields(cmd.Reason)
	if len(cmd.Rollout.UserIDs) > 0 {
		known, err := s.d.FlagUsers.ExistingUserIDs(ctx, cmd.Rollout.UserIDs)
		if err != nil {
			return err
		}
		fields = append(fields, unknownIDs("/rollout/user_ids/", "unknown_user", "No person has this ID.", cmd.Rollout.UserIDs, known)...)
	}
	if len(cmd.Rollout.GroupIDs) > 0 {
		known, err := s.d.FlagGroups.ExistingGroupIDs(ctx, cmd.Rollout.GroupIDs)
		if err != nil {
			return err
		}
		fields = append(fields, unknownIDs("/rollout/group_ids/", "unknown_group", "No group has this ID.", cmd.Rollout.GroupIDs, known)...)
	}
	if len(fields) > 0 {
		return errs.New(errs.CodeValidationFailed, "").WithFields(fields...)
	}
	return nil
}

// pointerReason is the field of the reason in the bodies of putFeatureFlag and rotateSigningKey.
const pointerReason = "/reason"

// reasonFields returns the field error of a management reason: required when blank,
// invalid_format with a character the display text check refuses; none for a valid reason.
func reasonFields(reason string) []errs.FieldError {
	switch {
	case strings.TrimSpace(reason) == "":
		return []errs.FieldError{{Pointer: pointerReason, Code: "required", Detail: "A reason is required."}}
	case textcheck.DisplayText(reason) != nil:
		return []errs.FieldError{{Pointer: pointerReason, Code: "invalid_format", Detail: "The reason contains a forbidden character."}}
	}
	return nil
}

// unknownIDs returns one field error per entry of sent that is not in known.
func unknownIDs(prefix, code, detail string, sent, known []ids.ID) []errs.FieldError {
	var out []errs.FieldError
	for i, id := range sent {
		if !slices.Contains(known, id) {
			out = append(out, errs.FieldError{Pointer: prefix + strconv.Itoa(i), Code: code, Detail: detail})
		}
	}
	return out
}

// recordFlag writes the allow entry of feature_flag.update with key, value, rollout (IDs only)
// and reason; no_change when nothing was written.
func (s *Admin) recordFlag(ctx context.Context, key string, cmd instance.PutFlagCmd, rollout domain.Rollout, noChange bool) error {
	attrs := auditing.Attrs{
		attrSettingKey: auditing.Code(key), attrEnabled: auditing.Bool(cmd.Enabled), attrFlagReason: auditing.ReasonText(cmd.Reason),
		attrRolloutUserIDs: idList(rollout.UserIDs), attrRolloutGroupIDs: idList(rollout.GroupIDs),
	}
	if noChange {
		attrs[attrNoChange] = auditing.Bool(true)
	}
	return s.d.Audit.Record(ctx, auditing.Entry{
		Action: string(authorize.ActionFeatureFlagUpdate), Decision: auditing.DecisionAllow,
		ResourceKind: string(authorize.ResourceInstance), Attrs: attrs,
	})
}

// idList converts IDs into an audit list.
func idList(list []ids.ID) auditing.Value {
	vs := make([]auditing.Value, 0, len(list))
	for _, id := range list {
		vs = append(vs, auditing.ID(id))
	}
	return auditing.List(vs...)
}
