// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/searchlang"
)

// Directory implements identity.Directory over a Store. It is read only and does not call
// authorize: callers authorize their own actions. It is safe for concurrent use when the
// Store is.
type Directory struct {
	store Store
	// search is the language configuration of WithSearch; nil fails SearchLanguage and
	// InstanceSearchLanguage closed, so a writer that was wired without it never writes simple
	// by accident.
	search *searchLanguages
}

// searchLanguages is what SearchLanguage needs besides the Store: the validated configuration
// and the reader of locale.default.
type searchLanguages struct {
	langs    searchlang.Map
	settings instance.Settings
}

// Compile-time check: Directory is the port identity.Directory.
var _ identity.Directory = (*Directory)(nil)

// NewDirectory returns the directory over store.
func NewDirectory(store Store) *Directory {
	return &Directory{store: store}
}

// WithSearch returns a copy of d that answers SearchLanguage and InstanceSearchLanguage with the
// language configuration langs (search.languages, search.locale_map) and reads locale.default
// through settings (instance.Settings.LocaleDefault). The writers of messages and threads
// (messaging, chat) get such a directory from internal/wire.
func (d *Directory) WithSearch(langs searchlang.Map, settings instance.Settings) *Directory {
	return &Directory{store: d.store, search: &searchLanguages{langs: langs, settings: settings}}
}

// SearchLanguage implements identity.Directory (14 3.1, docs/DECISIONS.md D-M3-13-1): a person
// gets the mapping of users.locale, every other principal the mapping of locale.default. A person
// costs one read by primary key, another principal two. Errors: kind not_found for an unknown
// id; internal without WithSearch; the errors of the Store and of the settings.
func (d *Directory) SearchLanguage(ctx context.Context, id ids.ID) (string, error) {
	if err := d.searchConfigured(); err != nil {
		return "", err
	}
	rec, found, err := d.store.User(ctx, id)
	if err != nil {
		return "", err
	}
	if found {
		return d.search.langs.For(rec.User.Locale), nil
	}
	if _, found, err = d.store.Principal(ctx, id); err != nil {
		return "", err
	}
	if !found {
		return "", notFound("principal")
	}
	return d.InstanceSearchLanguage(ctx)
}

// InstanceSearchLanguage implements identity.Directory: the mapping of locale.default. Errors:
// internal without WithSearch or without settings; the error of the settings.
func (d *Directory) InstanceSearchLanguage(ctx context.Context) (string, error) {
	if err := d.searchConfigured(); err != nil {
		return "", err
	}
	if d.search.settings == nil {
		return "", errs.New(errs.CodeInternalError, "identity: the directory has no instance settings for locale.default")
	}
	locale, err := d.search.settings.LocaleDefault(ctx)
	if err != nil {
		return "", err
	}
	return d.search.langs.For(locale), nil
}

// searchConfigured fails when the directory was built without WithSearch.
func (d *Directory) searchConfigured() error {
	if d.search == nil {
		return errs.New(errs.CodeInternalError, "identity: the directory has no search language configuration")
	}
	return nil
}

// Principal implements identity.Directory. Errors: kind not_found for an unknown id; the
// errors of the Store.
func (d *Directory) Principal(ctx context.Context, id ids.ID) (identity.PrincipalView, error) {
	rec, found, err := d.store.Principal(ctx, id)
	if err != nil {
		return identity.PrincipalView{}, err
	}
	if !found {
		return identity.PrincipalView{}, notFound("principal")
	}
	return principalView(rec.Principal), nil
}

// Principals implements identity.Directory: unknown ids are absent from the map, and an empty
// list reads nothing.
func (d *Directory) Principals(ctx context.Context, list []ids.ID) (map[ids.ID]identity.PrincipalView, error) {
	out := map[ids.ID]identity.PrincipalView{}
	if len(list) == 0 {
		return out, nil
	}
	ps, err := d.store.Principals(ctx, list)
	if err != nil {
		return nil, err
	}
	for _, p := range ps {
		out[p.ID] = principalView(p)
	}
	return out, nil
}

// PrincipalByHandle implements identity.Directory: the match ignores case. Errors: kind
// not_found when no principal has the handle; the errors of the Store.
func (d *Directory) PrincipalByHandle(ctx context.Context, handle string) (identity.PrincipalView, error) {
	p, found, err := d.store.PrincipalByHandle(ctx, handle)
	if err != nil {
		return identity.PrincipalView{}, err
	}
	if !found {
		return identity.PrincipalView{}, notFound("principal")
	}
	return principalView(p), nil
}

// ActiveByHandles implements identity.Directory: the handles are lower-cased and read in one
// query; an empty list reads nothing.
func (d *Directory) ActiveByHandles(ctx context.Context, handles []string) (map[string]identity.PrincipalView, error) {
	out := map[string]identity.PrincipalView{}
	if len(handles) == 0 {
		return out, nil
	}
	lower := make([]string, 0, len(handles))
	for _, h := range handles {
		lower = append(lower, strings.ToLower(h))
	}
	ps, err := d.store.ActivePrincipalsByHandles(ctx, lower)
	if err != nil {
		return nil, err
	}
	for _, p := range ps {
		v := principalView(p)
		out[strings.ToLower(v.Handle)] = v
	}
	return out, nil
}

// User implements identity.Directory. Errors: kind not_found for an unknown id and for a
// principal that is not a person; the errors of the Store.
func (d *Directory) User(ctx context.Context, id ids.ID) (identity.UserView, error) {
	rec, found, err := d.store.User(ctx, id)
	if err != nil {
		return identity.UserView{}, err
	}
	if !found {
		return identity.UserView{}, notFound("person")
	}
	return userView(rec), nil
}

// Users implements identity.Directory: one read for the list; an empty list reads nothing.
func (d *Directory) Users(ctx context.Context, list []ids.ID) (map[ids.ID]identity.UserView, error) {
	out := map[ids.ID]identity.UserView{}
	if len(list) == 0 {
		return out, nil
	}
	recs, err := d.store.Users(ctx, list)
	if err != nil {
		return nil, err
	}
	for _, rec := range recs {
		out[rec.User.Principal.ID] = userView(rec)
	}
	return out, nil
}

// ActiveOwners implements identity.Directory: owners rows with revoked_at IS NULL, ordered by
// granted_at, then id. Errors: those of the Store.
func (d *Directory) ActiveOwners(ctx context.Context) ([]identity.OwnerView, error) {
	recs, err := d.store.ActiveOwners(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]identity.OwnerView, 0, len(recs))
	for _, r := range recs {
		out = append(out, identity.OwnerView{
			User:      principalView(r.User),
			GrantedBy: r.Owner.GrantedBy,
			GrantedAt: r.Owner.GrantedAt,
			Reason:    r.Owner.Reason,
		})
	}
	return out, nil
}

// principalView converts a principal into the port DTO.
func principalView(p domain.Principal) identity.PrincipalView {
	return identity.PrincipalView{
		ID:           p.ID,
		Kind:         actor.Kind(p.Kind),
		Status:       identity.Status(p.Status),
		Handle:       p.Handle.String(),
		DisplayName:  p.DisplayName.String(),
		AvatarFileID: p.AvatarFileID,
		CreatedAt:    p.CreatedAt,
	}
}

// notFound is the not_found error of the directory; it names the kind of object, never the id
// or handle the caller asked for.
func notFound(what string) *errs.Error {
	return errs.New(errs.CodeNotFound, what+" not found")
}
