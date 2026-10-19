// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"

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
// gdpr_export are not visible yet; a file without its facts is an internal error in request mode
// and resolves its hint path in filter mode.
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
	for _, purpose := range []string{"avatar", "branding", "gdpr_export"} {
		t.Run(purpose, func(t *testing.T) {
			w.expect(t, w.user(w.alex), authorize.ActionFileRead, w.file(w.alex, ids.ID{}, purpose, false), authorize.OutcomeNotFound, "no_grant")
		})
	}

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
