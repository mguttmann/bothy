// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/importer/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// TestOriginKeyForms checks the forms of design note 6 and that the item kind matches the prefix.
func TestOriginKeyForms(t *testing.T) {
	t.Parallel()
	cases := []struct {
		kind, key, code string
		ids             []string
	}{
		{domain.KindSpace, "space:1", "", []string{"1"}},
		{domain.KindChannel, "channel:1029384000", "", []string{"1029384000"}},
		{domain.KindForumTag, "forum_tag:10:20", "", []string{"10", "20"}},
		{domain.KindUser, "user:42", "", []string{"42"}},
		{domain.KindForumTag, "forum_tag:20", domain.FieldInvalid, nil},
		{domain.KindSpace, "space:1:2", domain.FieldInvalid, nil},
		{domain.KindSpace, "space:", domain.FieldInvalid, nil},
		{domain.KindSpace, "space:1a", domain.FieldInvalid, nil},
		{domain.KindSpace, "space", domain.FieldInvalid, nil},
		{domain.KindSpace, "channel:1", domain.FieldKindMismatch, nil},
		{domain.KindUser, "user:-1", domain.FieldInvalid, nil},
	}
	for _, c := range cases {
		k, code := domain.ParseOriginKey(c.kind, c.key)
		if code != c.code || !reflect.DeepEqual(k.IDs, c.ids) {
			t.Errorf("%s %q = %+v %q, want %v %q", c.kind, c.key, k, code, c.ids, c.code)
		}
	}
	if domain.Key(domain.KindForumTag, "10", "20") != "forum_tag:10:20" {
		t.Error("Key")
	}
	for _, k := range []string{domain.KindSpace, domain.KindChannel, domain.KindForumTag, domain.KindUser} {
		if !domain.Structure(k) {
			t.Errorf("%s is structure", k)
		}
	}
	for _, k := range []string{domain.KindThread, domain.KindMessage, domain.KindPin, domain.KindFile} {
		if domain.Structure(k) {
			t.Errorf("%s is not structure", k)
		}
	}
}

// TestItemData checks the data schemas of design note 9.
func TestItemData(t *testing.T) {
	t.Parallel()
	has := func(p *domain.Problems, ptr, code string) bool {
		return slices.ContainsFunc(p.Fields(), func(f errs.FieldError) bool { return f.Pointer == ptr && f.Code == code })
	}
	var p domain.Problems
	s := domain.ParseSpaceData(json.RawMessage(`{"name": "Support", "sort_key": "a0"}`), "/items/0/data", &p)
	if !p.Empty() || s.Name != "Support" || s.SortKey != "a0" {
		t.Errorf("space %+v %+v", s, p.Fields())
	}
	p = domain.Problems{}
	domain.ParseSpaceData(json.RawMessage(`{"sort_key": "a0"}`), "/items/0/data", &p)
	if !has(&p, "/items/0/data", domain.FieldInvalid) {
		t.Errorf("space without name %+v", p.Fields())
	}
	p = domain.Problems{}
	c := domain.ParseChannelData(json.RawMessage(`{"space_origin_key": "space:1", "name": "tickets", "kind": "forum", "sort_key": "a1", "read_only": true}`), "/items/1/data", &p)
	if !has(&p, "/items/1/data/read_only", domain.FieldUnknownMember) || c.Kind != "forum" || c.SpaceOriginKey != "space:1" {
		t.Errorf("channel %+v %+v", c, p.Fields())
	}
	p = domain.Problems{}
	domain.ParseChannelData(json.RawMessage(`{"space_origin_key": "channel:1", "kind": "voice", "sort_key": "a1"}`), "/d", &p)
	if !has(&p, "/d/name", domain.FieldRequired) || !has(&p, "/d/kind", domain.FieldInvalid) || !has(&p, "/d/space_origin_key", domain.FieldKindMismatch) {
		t.Errorf("channel errors %+v", p.Fields())
	}
	p = domain.Problems{}
	tag := domain.ParseForumTagData(json.RawMessage(`{"channel_origin_key": "channel:7", "name": "bug", "emoji": "x"}`), "/d", &p)
	if !p.Empty() || tag.Name != "bug" {
		t.Errorf("tag %+v %+v", tag, p.Fields())
	}
	p = domain.Problems{}
	u := domain.ParseUserData(json.RawMessage(`{"discord_kind": "webhook", "username": "Deploy"}`), "/d", &p)
	if !p.Empty() || u.DiscordKind != domain.DiscordWebhook {
		t.Errorf("user %+v %+v", u, p.Fields())
	}
	p = domain.Problems{}
	domain.ParseUserData(json.RawMessage(`"x"`), "/d", &p)
	if !has(&p, "/d", domain.FieldInvalid) {
		t.Errorf("user not an object %+v", p.Fields())
	}
}
