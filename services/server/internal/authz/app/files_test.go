// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// file is a file of owner with purpose and attachment state; an attachment names its scope
// channel, an attached one also the thread of its message.
func (w *world) file(owner, channel ids.ID, purpose string, attached bool) authorize.Resource {
	r := authorize.Resource{Kind: authorize.ResourceFile, ID: ids.New(), OwnerID: owner,
		File: &authorize.FileFacts{Purpose: purpose, Attached: attached}}
	if purpose == "attachment" {
		r.ChannelID = channel
		if attached {
			r.ThreadID = w.mainThread[channel]
		}
	}
	return r
}

// TestFileVisibility holds the rule of file.read for kind file (brief M3-03 design note 15,
// docs/DECISIONS.md D-M3-03-17): an unattached attachment is visible only to its uploader with read
// on the scope channel; an attached one with read on the message's path; avatar, branding and
// gdpr_export are not visible to file.read (TestFileReadRequirement holds avatar and branding); a
// file without its facts is an internal error in request mode and resolves its hint path in filter
// mode.
func TestFileVisibility(t *testing.T) {
	w := newWorld(t)
	mine := w.file(w.alex, w.allgemein, "attachment", false)
	w.allow(t, w.user(w.alex), authorize.ActionFileRead, mine)
	for name, p := range map[string]authorize.Principal{
		"Sam with read on the channel": w.user(w.sam), "Robin": w.user(w.robin), "owner": w.user(w.owner),
	} {
		t.Run(name, func(t *testing.T) {
			w.expect(t, p, authorize.ActionFileRead, mine, authorize.OutcomeNotFound, "no_grant")
		})
	}
	attached := w.file(w.alex, w.allgemein, "attachment", true)
	w.allow(t, w.user(w.sam), authorize.ActionFileRead, attached)
	w.expect(t, w.user(w.robin), authorize.ActionFileRead, attached, authorize.OutcomeNotFound, "no_grant")
	w.expect(t, w.user(w.alex), authorize.ActionFileRead, w.file(w.alex, ids.ID{}, "gdpr_export", false), authorize.OutcomeNotFound, "no_grant")

	bare := authorize.Resource{Kind: authorize.ResourceFile, ID: ids.New(), ChannelID: w.allgemein, OwnerID: w.alex}
	if _, err := w.az.Authorize(context.Background(), w.user(w.alex), authorize.ActionFileRead, bare); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("file without facts in request mode: %v, want an internal error", err)
	}
	ev := authorize.ResourceFromEvent("file", bare.ID, w.support, w.allgemein, ids.ID{})
	if d := w.decide(t, authorize.Filter(context.Background()), w.user(w.sam), authorize.ActionFileRead, ev); !d.Allowed() {
		t.Fatalf("file event in filter mode for a reader of the channel: %s", d.Outcome)
	}
	if d := w.decide(t, authorize.Filter(context.Background()), w.user(w.robin), authorize.ActionFileRead, ev); d.Allowed() {
		t.Fatal("file event in filter mode allowed for Robin without read")
	}

	w.revoke(t, w.g1)
	w.expect(t, w.user(w.alex), authorize.ActionFileRead, mine, authorize.OutcomeNotFound, "no_grant")
}

// TestFileDeleteVisibility holds file.delete (05 4.8, self or manage) with the visibility of
// file.read: the uploader deletes the own unattached file, nobody else sees it.
func TestFileDeleteVisibility(t *testing.T) {
	w := newWorld(t)
	mine := w.file(w.alex, w.allgemein, "attachment", false)
	w.allow(t, w.user(w.alex), authorize.ActionFileDelete, mine)
	w.expect(t, w.user(w.sam), authorize.ActionFileDelete, mine, authorize.OutcomeNotFound, "no_grant")
	w.expect(t, w.user(w.owner), authorize.ActionFileDelete, mine, authorize.OutcomeNotFound, "no_grant")
	w.expect(t, w.user(w.robin), authorize.ActionFileDelete, mine, authorize.OutcomeNotFound, "no_grant")
}

// TestFileReadRequirement holds file.read per purpose (brief M3-05 design note 7): an attachment
// needs the uploader with read on its scope channel (everybody else not_found, never forbidden), an
// avatar any authenticated principal, a branding file every principal, and a gdpr_export is
// invisible even to its uploader; file.delete keeps the attachment-only visibility.
func TestFileReadRequirement(t *testing.T) {
	w := newWorld(t)
	attachment := w.file(w.alex, w.allgemein, "attachment", false)
	avatar := w.file(w.alex, ids.ID{}, "avatar", false)
	branding := w.file(w.owner, ids.ID{}, "branding", false)
	export := w.file(w.alex, ids.ID{}, "gdpr_export", false)
	for _, c := range []struct {
		name   string
		p      authorize.Principal
		a      authorize.Action
		r      authorize.Resource
		o      authorize.Outcome
		reason string
	}{
		{"attachment uploader with read", w.user(w.alex), authorize.ActionFileRead, attachment, authorize.OutcomeAllow, ""},
		{"attachment Sam with read on the channel", w.user(w.sam), authorize.ActionFileRead, attachment, authorize.OutcomeNotFound, "no_grant"},
		{"attachment Robin without read", w.user(w.robin), authorize.ActionFileRead, attachment, authorize.OutcomeNotFound, "no_grant"},
		{"attachment owner not uploader", w.user(w.owner), authorize.ActionFileRead, attachment, authorize.OutcomeNotFound, "no_grant"},
		{"avatar Sam", w.user(w.sam), authorize.ActionFileRead, avatar, authorize.OutcomeAllow, ""},
		{"avatar Robin", w.user(w.robin), authorize.ActionFileRead, avatar, authorize.OutcomeAllow, ""},
		{"avatar agent", w.agent(w.lotse), authorize.ActionFileRead, avatar, authorize.OutcomeAllow, ""},
		{"branding Robin", w.user(w.robin), authorize.ActionFileRead, branding, authorize.OutcomeAllow, ""},
		{"gdpr_export uploader", w.user(w.alex), authorize.ActionFileRead, export, authorize.OutcomeNotFound, "no_grant"},
		{"file.delete avatar non-uploader", w.user(w.sam), authorize.ActionFileDelete, avatar, authorize.OutcomeNotFound, "no_grant"},
	} {
		t.Run(c.name, func(t *testing.T) {
			w.expect(t, c.p, c.a, c.r, c.o, c.reason)
		})
	}
}

// TestFileReadIssueTicketNeedsSession holds the session rule of createFileTicket (brief M3-05
// design note 7): a ticket binds to a login session, so a principal without one (API token, agent)
// is forbidden step_up_not_possible after the visibility check, and an invisible file stays
// not_found.
func TestFileReadIssueTicketNeedsSession(t *testing.T) {
	w := newWorld(t)
	w.addToken(w.alex)
	token := actor.Actor{ID: w.alex, Kind: actor.KindUser, TokenID: w.tokens[w.alex]}
	mine := w.file(w.alex, w.allgemein, "attachment", false)
	mine.IssueTicket = true
	attached := w.file(w.sam, w.allgemein, "attachment", true)
	attached.IssueTicket = true
	w.expect(t, token, authorize.ActionFileRead, mine, authorize.OutcomeForbidden, "step_up_not_possible")
	w.expect(t, w.agent(w.lotse), authorize.ActionFileRead, attached, authorize.OutcomeForbidden, "step_up_not_possible")
	hidden := w.file(w.sam, w.allgemein, "attachment", false)
	hidden.IssueTicket = true
	w.expect(t, token, authorize.ActionFileRead, hidden, authorize.OutcomeNotFound, "no_grant")
	w.allow(t, w.user(w.alex), authorize.ActionFileRead, mine)
	mine.IssueTicket = false
	w.allow(t, token, authorize.ActionFileRead, mine)
}
