// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"slices"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
)

// Templates implements messaging.Templates over the use cases (Service.Templates).
type Templates struct{ *Service }

// Templates returns the incoming port of the message templates.
func (s *Service) Templates() Templates { return Templates{s} }

// channelResource describes the channel id for authorize; authz resolves its path, so an
// unknown and an invisible channel are the same not_found.
func channelResource(id ids.ID) authorize.Resource {
	return authorize.Resource{Kind: authorize.ResourceChannel, ID: id}
}

// templateResource is the resource of a template for authorize (03 message_templates, 06 K15):
// the user of its owner for a personal template, its channel otherwise.
func templateResource(t domain.Template) authorize.Resource {
	if t.Scope() == domain.ScopePersonal {
		return userResource(t.OwnerUserID)
	}
	return channelResource(t.ChannelID)
}

// rules are the template limits of the configuration.
func (s *Service) rules() domain.TemplateRules {
	return domain.TemplateRules{NameMaxChars: s.cfg.TemplateNameMaxChars, ContentMaxBytes: s.cfg.MaxContentBytes}
}

// ListMine implements messaging.Templates (brief note 7): template.read on the caller, then the
// personal templates; a person without any readable channel still lists them.
func (s Templates) ListMine(ctx context.Context) ([]messaging.TemplateView, error) {
	a := caller(ctx)
	if err := s.checkPerson(ctx, a, authorize.ActionTemplateRead); err != nil {
		return nil, err
	}
	rows, err := s.d.Store.PersonalTemplates(ctx, a.ID)
	if err != nil {
		return nil, err
	}
	return templateViews(rows), nil
}

// List implements messaging.Templates (brief note 7): template.read on the channel (404 for a
// missing or invisible one), then one statement over the caller's personal templates and those
// of the checked channel. The personal items pass template.read on the caller in filter mode,
// checked once; the channel items are covered by the channel check.
func (s Templates) List(ctx context.Context, channelID ids.ID) ([]messaging.TemplateView, error) {
	a := caller(ctx)
	if err := s.check(ctx, a, authorize.ActionTemplateRead, channelResource(channelID)); err != nil {
		return nil, err
	}
	var owner ids.ID
	if a.Kind == actor.KindUser {
		owner = a.ID
	}
	rows, err := s.d.Store.ChannelTemplates(ctx, owner, channelID, []ids.ID{channelID})
	if err != nil {
		return nil, err
	}
	if !slices.ContainsFunc(rows, isPersonal) {
		return templateViews(rows), nil
	}
	ok, err := s.allowed(authorize.Filter(ctx), a, authorize.ActionTemplateRead, userResource(a.ID))
	if err != nil {
		return nil, err
	}
	if !ok {
		rows = slices.DeleteFunc(rows, isPersonal)
	}
	return templateViews(rows), nil
}

// isPersonal reports a personal template.
func isPersonal(t domain.Template) bool { return t.Scope() == domain.ScopePersonal }

// CreatePersonal implements messaging.Templates (brief notes 8 and 9).
func (s Templates) CreatePersonal(ctx context.Context, c messaging.TemplateInput) (messaging.TemplateView, error) {
	a := caller(ctx)
	if err := s.checkPerson(ctx, a, authorize.ActionTemplateManage); err != nil {
		return messaging.TemplateView{}, err
	}
	return s.create(ctx, a, domain.Template{OwnerUserID: a.ID}, c)
}

// CreateForChannel implements messaging.Templates (brief notes 8 and 9): template.manage on the
// channel needs manage; read without it is 403, no read 404.
func (s Templates) CreateForChannel(ctx context.Context, channelID ids.ID, c messaging.TemplateInput) (messaging.TemplateView, error) {
	a := caller(ctx)
	if err := s.check(ctx, a, authorize.ActionTemplateManage, channelResource(channelID)); err != nil {
		return messaging.TemplateView{}, err
	}
	return s.create(ctx, a, domain.Template{ChannelID: channelID}, c)
}

// create validates c and inserts the template of the scope of t in one unit of work: the
// advisory lock of the scope, the count against template.max_per_scope, the insert. The lock
// makes the bound exact under concurrency (brief note 9).
func (s *Service) create(ctx context.Context, a actor.Actor, t domain.Template, c messaging.TemplateInput) (messaging.TemplateView, error) {
	if err := s.rules().Check(c.Name, c.ContentMD); err != nil {
		return messaging.TemplateView{}, err
	}
	t.ID, t.Name, t.ContentMD, t.CreatedBy, t.CreatedAt = ids.New(), c.Name, c.ContentMD, a.ID, s.now()
	scopeID := t.OwnerUserID
	if t.Scope() == domain.ScopeChannel {
		scopeID = t.ChannelID
	}
	var out domain.Template
	err := s.d.UoW.Do(ctx, func(ctx context.Context) error {
		if err := s.d.Store.LockTemplateScope(ctx, domain.ScopeLockKey(t.Scope(), scopeID)); err != nil {
			return err
		}
		n, err := s.d.Store.CountTemplates(ctx, t)
		if err != nil {
			return err
		}
		if n >= s.cfg.TemplateMaxPerScope {
			return domain.LimitReached()
		}
		out, err = s.d.Store.InsertTemplate(ctx, t)
		return err
	})
	if err != nil {
		return messaging.TemplateView{}, err
	}
	return templateView(out), nil
}

// visibleChannels returns the channels the caller reads (Scope of channel.read), for the load
// predicate of a change.
func (s *Service) visibleChannels(ctx context.Context, a actor.Actor) ([]ids.ID, error) {
	sc, err := s.d.Authorizer.Scope(ctx, a, authorize.ActionChannelRead)
	if err != nil {
		return nil, err
	}
	return sc.ChannelIDs, nil
}

// loadForChange reads and locks the template id inside the unit of work of ctx when it is the
// caller's or in a visible channel (withDeleted includes a deleted one), then authorizes
// template.manage on its user or channel. A missing, foreign personal or invisible template is
// one 404 path (brief notes 10 and 11).
func (s *Service) loadForChange(ctx context.Context, a actor.Actor, id ids.ID, visible []ids.ID, withDeleted bool) (domain.Template, error) {
	t, found, err := s.d.Store.LockVisibleTemplate(ctx, id, a.ID, visible, withDeleted)
	if err != nil {
		return domain.Template{}, err
	}
	if !found {
		return domain.Template{}, invisible()
	}
	if err := s.check(ctx, a, authorize.ActionTemplateManage, templateResource(t)); err != nil {
		return domain.Template{}, err
	}
	return t, nil
}

// Update implements messaging.Templates (brief note 10), in the order 428 (the chain), 404,
// 403, 412, 400: the locked template, template.manage, the version, the given members; a change
// without effect returns the template and writes nothing.
func (s Templates) Update(ctx context.Context, c messaging.TemplateChange) (messaging.TemplateView, error) {
	a := caller(ctx)
	if !c.IfMatch.Present {
		return messaging.TemplateView{}, errs.PreconditionRequired()
	}
	visible, err := s.visibleChannels(ctx, a)
	if err != nil {
		return messaging.TemplateView{}, err
	}
	var out domain.Template
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		t, err := s.loadForChange(ctx, a, c.TemplateID, visible, false)
		if err != nil {
			return err
		}
		if !c.IfMatch.Valid || c.IfMatch.Version != t.Version {
			return errs.VersionConflict(templateBody(templateView(t)))
		}
		if err := s.rules().CheckChange(c.Name, c.ContentMD); err != nil {
			return err
		}
		next, changed := t.Apply(c.Name, c.ContentMD)
		if !changed {
			out = t
			return nil
		}
		out, err = s.d.Store.UpdateTemplate(ctx, next)
		return err
	})
	if err != nil {
		return messaging.TemplateView{}, err
	}
	return templateView(out), nil
}

// Delete implements messaging.Templates (brief note 11): the template including a deleted one,
// template.manage, then the soft delete; a deleted template is a success without write.
func (s Templates) Delete(ctx context.Context, templateID ids.ID) error {
	a := caller(ctx)
	visible, err := s.visibleChannels(ctx, a)
	if err != nil {
		return err
	}
	return s.d.UoW.Do(ctx, func(ctx context.Context) error {
		t, err := s.loadForChange(ctx, a, templateID, visible, true)
		if err != nil {
			return err
		}
		if !t.DeletedAt.IsZero() {
			return nil
		}
		return s.d.Store.DeleteTemplate(ctx, t.ID, s.now())
	})
}

// templateView converts a row into the port DTO.
func templateView(t domain.Template) messaging.TemplateView {
	return messaging.TemplateView{
		ID: t.ID, Scope: string(t.Scope()), ChannelID: t.ChannelID, Name: t.Name, ContentMD: t.ContentMD, Version: t.Version,
	}
}

// templateViews converts rows; never nil.
func templateViews(rows []domain.Template) []messaging.TemplateView {
	out := make([]messaging.TemplateView, 0, len(rows))
	for _, t := range rows {
		out = append(out, templateView(t))
	}
	return out
}

// templateJSON is the schema Template, the member current of 412 version_conflict.
type templateJSON struct {
	ID        string `json:"id"`
	Scope     string `json:"scope"`
	ChannelID string `json:"channel_id,omitempty"`
	Name      string `json:"name"`
	ContentMD string `json:"content_md"`
	Version   int64  `json:"version"`
}

// templateBody converts a template into the JSON form of the schema Template.
func templateBody(v messaging.TemplateView) templateJSON {
	return templateJSON{ID: v.ID.String(), Scope: v.Scope, ChannelID: optionalID(v.ChannelID), Name: v.Name, ContentMD: v.ContentMD, Version: v.Version}
}
