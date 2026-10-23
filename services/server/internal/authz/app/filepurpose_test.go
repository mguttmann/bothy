// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/adapters/fake"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// filePurposeCase is one principal of TestFileReadByPurpose with its expected answer on avatar and
// branding (gdpr_export is not_found no_grant for everyone).
type filePurposeCase struct {
	name    string
	p       authorize.Principal
	outcome authorize.Outcome
	reason  string
}

// filePurposeCases are the principals of TestFileReadByPurpose: Alex and Sam as active people, an
// agent, the app Tracker with and without files:read, anonymous, Robin on a pending device and a
// disabled person.
func filePurposeCases(w *world) []filePurposeCase {
	reader := w.app()
	reader.GrantedScopes, reader.Scopes = []string{domain.ScopeFilesRead}, []string{domain.ScopeFilesRead}
	w.creds.PutDevice(w.devices[w.robin], fake.Device{PrincipalID: w.robin, State: authz.DevicePending})
	disabled := ids.New()
	w.principals.Put(authz.PrincipalFacts{ID: disabled, Kind: actor.KindUser, Status: authz.PrincipalDisabled})
	w.addSession(disabled)
	w.versions.Bump()
	return []filePurposeCase{
		{"Alex", w.user(w.alex), authorize.OutcomeAllow, ""},
		{"Sam", w.user(w.sam), authorize.OutcomeAllow, ""},
		{"agent", w.agent(w.lotse), authorize.OutcomeAllow, ""},
		{"app with files:read", reader, authorize.OutcomeAllow, ""},
		{"app without files:read", w.app(), authorize.OutcomeForbidden, "scope_missing"},
		{"anonymous", actor.Anonymous(), authorize.OutcomeUnauthenticated, "unauthenticated"},
		{"pending device", w.user(w.robin), authorize.OutcomeForbidden, "device_pending"},
		{"disabled person", w.user(disabled), authorize.OutcomeUnauthenticated, "principal_inactive"},
	}
}

// TestFileReadByPurpose holds file.read by purpose (brief M3-10 design note 6, 05 section 2.2, 07
// sections 4.2 and 4.3): every principal that passes step 1 reads an avatar or a logo file, an app
// only with files:read; gdpr_export is not_found through file.read for everyone, also for its
// uploader and the owner.
func TestFileReadByPurpose(t *testing.T) {
	w := newWorld(t)
	cases := filePurposeCases(w)
	for _, purpose := range []string{"avatar", "branding"} {
		f := w.file(w.alex, ids.ID{}, purpose, false)
		for _, c := range cases {
			t.Run(purpose+"/"+c.name, func(t *testing.T) {
				w.expect(t, c.p, authorize.ActionFileRead, f, c.outcome, c.reason)
			})
		}
	}
	export := w.file(w.alex, ids.ID{}, "gdpr_export", false)
	for _, c := range cases {
		if c.outcome != authorize.OutcomeAllow {
			continue
		}
		t.Run("gdpr_export/"+c.name, func(t *testing.T) {
			w.expect(t, c.p, authorize.ActionFileRead, export, authorize.OutcomeNotFound, "no_grant")
		})
	}
	w.expect(t, w.user(w.owner), authorize.ActionFileRead, w.file(w.owner, ids.ID{}, "gdpr_export", false),
		authorize.OutcomeNotFound, "no_grant")
}

// TestFileReadByPurpose_OtherActions holds that the read rule of avatar and branding opens only
// file.read: file.delete of such a file stays not_found, also for its uploader (the avatar and
// logo operations remove them, brief M3-10 design note 3).
func TestFileReadByPurpose_OtherActions(t *testing.T) {
	w := newWorld(t)
	for _, purpose := range []string{"avatar", "branding"} {
		t.Run(purpose, func(t *testing.T) {
			w.expect(t, w.user(w.alex), authorize.ActionFileDelete, w.file(w.alex, ids.ID{}, purpose, false),
				authorize.OutcomeNotFound, "no_grant")
		})
	}
}

// TestFileReadEventDescriptor holds the receive check of a personal file.updated (brief M3-10
// design note 7): the descriptor of an event row of kind file without scope takes OwnerID from the
// audience, so only the uploader receives it; a broadcast row without scope fails closed.
func TestFileReadEventDescriptor(t *testing.T) {
	w := newWorld(t)
	filter := authorize.Filter(context.Background())
	ev := authorize.ResourceFromEvent("file", ids.New(), ids.ID{}, ids.ID{}, ids.ID{}, w.alex)
	if d := w.decide(t, filter, w.user(w.alex), authorize.ActionFileRead, ev); !d.Allowed() {
		t.Fatalf("personal file event for its audience: %s/%s", d.Outcome, d.Reason)
	}
	for name, p := range map[string]authorize.Principal{"Sam": w.user(w.sam), "owner": w.user(w.owner), "agent": w.agent(w.lotse)} {
		t.Run(name, func(t *testing.T) {
			if d := w.decide(t, filter, p, authorize.ActionFileRead, ev); d.Outcome != authorize.OutcomeNotFound {
				t.Fatalf("personal file event of Alex for %s: %s/%s, want not_found", name, d.Outcome, d.Reason)
			}
		})
	}
	broadcast := authorize.ResourceFromEvent("file", ids.New(), ids.ID{}, ids.ID{}, ids.ID{}, ids.ID{})
	if d := w.decide(t, filter, w.user(w.alex), authorize.ActionFileRead, broadcast); d.Outcome != authorize.OutcomeNotFound {
		t.Fatalf("file event without scope and audience: %s/%s, want not_found", d.Outcome, d.Reason)
	}
}
