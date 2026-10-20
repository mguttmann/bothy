// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/sync"
	"github.com/OWNER/bothy/services/server/internal/sync/domain"
)

// List implements sync.Settings (brief M1-61 note 17): settings.read on the caller, then every
// stored row; defaults are neither stored nor returned.
func (s Settings) List(ctx context.Context) ([]sync.UserSetting, error) {
	a := caller(ctx)
	if err := s.check(ctx, a, authorize.ActionSettingsRead, self(a.ID)); err != nil {
		return nil, err
	}
	return s.d.Store.Settings(ctx, a.ID)
}

// Put implements sync.Settings (brief M1-61 note 15): settings.update on the caller, the key and
// value rules of the registry, then the upsert. The stored value answers 200 without event and
// without a new version; a change appends settings.updated. A value whose event would not stay
// below sync.event_payload_max_bytes is 400 on /value, because a setting has no reduced payload.
func (s Settings) Put(ctx context.Context, key string, value json.RawMessage) (sync.UserSetting, error) {
	a := caller(ctx)
	var out sync.UserSetting
	err := s.d.UoW.Do(ctx, func(ctx context.Context) error {
		if err := s.check(ctx, a, authorize.ActionSettingsUpdate, self(a.ID)); err != nil {
			return err
		}
		if err := domain.ValidateSetting(key, value); err != nil {
			return err
		}
		row, changed, err := s.d.Store.PutSetting(ctx, a.ID, sync.UserSetting{Key: key, Value: value, UpdatedByDeviceID: a.DeviceID, UpdatedAt: s.now()})
		if err != nil {
			return err
		}
		if !changed {
			var found bool
			if out, found, err = s.d.Store.Setting(ctx, a.ID, key); err != nil {
				return err
			}
			if !found {
				return errs.New(errs.CodeInternalError, "sync: a setting vanished inside its unit of work")
			}
			return nil
		}
		out = row
		return s.appendSetting(ctx, a, row, false)
	})
	return out, err
}

// Delete implements sync.Settings (brief M1-61 note 16): an unknown key is 404, a known key that
// is not stored answers without event; a stored key is deleted and settings.updated carries the
// last value with the last version plus one and deleted. A later Put of the key starts again at
// version 1 (docs/DECISIONS.md M1-61).
func (s Settings) Delete(ctx context.Context, key string) error {
	a := caller(ctx)
	return s.d.UoW.Do(ctx, func(ctx context.Context) error {
		if err := s.check(ctx, a, authorize.ActionSettingsUpdate, self(a.ID)); err != nil {
			return err
		}
		if !domain.KnownSetting(key) {
			return errs.New(errs.CodeNotFound, "")
		}
		old, found, err := s.d.Store.DeleteSetting(ctx, a.ID, key)
		if err != nil || !found {
			return err
		}
		old.Version++
		old.UpdatedByDeviceID, old.UpdatedAt = a.DeviceID, s.now()
		return s.appendSetting(ctx, a, old, true)
	})
}

// userSettingJSON is the schema UserSetting; absent members are omitted, never null.
type userSettingJSON struct {
	Key               string          `json:"key"`
	Value             json.RawMessage `json:"value"`
	Version           int64           `json:"version"`
	UpdatedByDeviceID string          `json:"updated_by_device_id,omitempty"`
	UpdatedAt         string          `json:"updated_at,omitempty"`
}

// appendSetting appends settings.updated (brief M1-61 note 19): scope empty, resource the user
// without a version, payload SettingsPayload with deleted for a removed key.
func (s *Personal) appendSetting(ctx context.Context, a actor.Actor, st sync.UserSetting, deleted bool) error {
	p, err := marshal(struct {
		Setting userSettingJSON `json:"setting"`
		Deleted bool            `json:"deleted,omitempty"`
	}{userSettingJSON{Key: st.Key, Value: st.Value, Version: st.Version, UpdatedByDeviceID: optionalID(st.UpdatedByDeviceID),
		UpdatedAt: stamp(st.UpdatedAt)}, deleted})
	if err != nil {
		return err
	}
	if len(p) >= s.cfg.EventPayloadMaxBytes {
		return errs.New(errs.CodeValidationFailed, "the value is too large").WithFields(errs.FieldError{
			Pointer: "/value", Code: domain.FieldTooLarge, Detail: "the value is larger than an event payload may be",
		})
	}
	e := personalEvent(a, authorize.EventSettingsUpdated, p)
	e.ResourceKind, e.ResourceID = resourceUser, a.ID
	return s.d.Events.Append(ctx, e)
}

// ValuesFor implements sync.Settings (M3-18): one read of the stored values, without authorize
// and without defaults. No people or no keys read nothing.
func (s Settings) ValuesFor(ctx context.Context, userIDs []ids.ID, keys []string) (map[ids.ID]map[string]json.RawMessage, error) {
	if len(userIDs) == 0 || len(keys) == 0 {
		return map[ids.ID]map[string]json.RawMessage{}, nil
	}
	return s.d.Store.SettingValues(ctx, userIDs, keys)
}
