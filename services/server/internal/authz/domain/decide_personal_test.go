// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestDecide_PersonalDataOnlyForPeople (M1-61 note 22): the actions of the own read states,
// drafts and settings, and the notification levels and quiet hours of M3-18, are open to people (in a session or with a personal token) and refused to
// agents and apps with subject_kind_not_allowed, even when the resource names the agent itself
// and the agent may read the channel.
func TestDecide_PersonalDataOnlyForPeople(t *testing.T) {
	t.Parallel()
	read := domain.Rights{Level: domain.LevelRead}
	for _, c := range []struct {
		action, kind string
	}{
		{"read_state.update", "channel"}, {"read_state.mark_unread", "channel"}, {"draft.write", "channel"},
		{"draft.read", "user"}, {"settings.read", "user"}, {"settings.update", "user"},
		// M3-18: the notification levels and quiet hours of a person.
		{"notification_pref.update", "space"}, {"notification_pref.update", "channel"}, {"notification_pref.update", "thread"},
		{"quiet_hours.update", "user"},
	} {
		for _, p := range []struct {
			name string
			p    domain.Principal
			want domain.Decision
		}{
			{"person", domain.Principal{ID: ids.New(), Kind: domain.KindUser, HasSession: true}, domain.Allow},
			{"personal token", domain.Principal{ID: ids.New(), Kind: domain.KindUser}, domain.Allow},
			{"agent", domain.Principal{ID: ids.New(), Kind: domain.KindAgent},
				domain.Decision{Outcome: domain.OutcomeForbidden, Reason: domain.ReasonSubjectKindNotAllowed}},
		} {
			in := input(t, c.action, c.kind, p.p, read)
			in.Resource.OwnerID = p.p.ID
			if c.kind == "user" {
				in.Resource.ID = p.p.ID
			}
			got, err := domain.Decide(in)
			if err != nil || got != p.want {
				t.Errorf("%s by %s: %+v, %v; want %+v", c.action, p.name, got, err, p.want)
			}
		}
	}
}
