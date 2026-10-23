// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

func TestAttachable(t *testing.T) {
	t.Parallel()
	alex, sam := ids.New(), ids.New()
	allgemein, eskalation := ids.New(), ids.New()
	ok := domain.AttachFacts{Found: true, UploadedBy: alex, AuthorID: alex, Purpose: string(domain.PurposeAttachment),
		ScopeChannelID: allgemein, ChannelID: allgemein, SessionState: domain.SessionCompleted,
		UploadState: domain.UploadPending, ScanStatus: domain.ScanPending}
	with := func(change func(*domain.AttachFacts)) domain.AttachFacts {
		f := ok
		change(&f)
		return f
	}
	cases := []struct {
		name  string
		facts domain.AttachFacts
		want  bool
	}{
		{"completed and pending scan", ok, true},
		{"unknown id", domain.AttachFacts{AuthorID: alex, ChannelID: allgemein}, false},
		{"foreign uploader", with(func(f *domain.AttachFacts) { f.UploadedBy = sam }), false},
		{"purpose avatar", with(func(f *domain.AttachFacts) { f.Purpose = string(domain.PurposeAvatar) }), false},
		{"other channel (SEC-UP-9)", with(func(f *domain.AttachFacts) { f.ScopeChannelID = eskalation }), false},
		{"no scope channel", with(func(f *domain.AttachFacts) { f.ScopeChannelID = ids.ID{} }), false},
		{"session open", with(func(f *domain.AttachFacts) { f.SessionState = domain.SessionOpen }), false},
		{"session aborted", with(func(f *domain.AttachFacts) { f.SessionState = domain.SessionAborted }), false},
		{"session expired", with(func(f *domain.AttachFacts) { f.SessionState = domain.SessionExpired }), false},
		{"no session", with(func(f *domain.AttachFacts) { f.SessionState = "" }), false},
		{"upload failed", with(func(f *domain.AttachFacts) { f.UploadState = domain.UploadFailed }), false},
		{"deleted", with(func(f *domain.AttachFacts) { f.Deleted = true }), false},
		{"already attached", with(func(f *domain.AttachFacts) { f.Attached = true }), false},
		{"infected", with(func(f *domain.AttachFacts) { f.ScanStatus = domain.ScanInfected }), true},
		{"scan error", with(func(f *domain.AttachFacts) { f.ScanStatus = domain.ScanError }), true},
		{"clean and complete", with(func(f *domain.AttachFacts) {
			f.UploadState, f.ScanStatus = domain.UploadComplete, domain.ScanClean
		}), true},
		{"skipped", with(func(f *domain.AttachFacts) {
			f.UploadState, f.ScanStatus = domain.UploadComplete, domain.ScanSkipped
		}), true},
		{"zero author", with(func(f *domain.AttachFacts) { f.AuthorID, f.UploadedBy = ids.ID{}, ids.ID{} }), false},
	}
	for _, c := range cases {
		if got := domain.Attachable(c.facts); got != c.want {
			t.Errorf("%s: Attachable = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestEventTarget(t *testing.T) {
	t.Parallel()
	if got := domain.EventTarget(false); !got.AudienceUploader || got.ThreadScope {
		t.Errorf("unattached: %+v, want the uploader as audience and the scope of the channel", got)
	}
	if got := domain.EventTarget(true); got.AudienceUploader || !got.ThreadScope {
		t.Errorf("attached: %+v, want no audience and the scope of the message's thread", got)
	}
}

func TestBecameFinal(t *testing.T) {
	t.Parallel()
	type state = domain.StateFacts
	var all []state
	for _, u := range []string{domain.UploadPending, domain.UploadComplete, domain.UploadFailed} {
		for _, s := range []string{domain.ScanPending, domain.ScanClean, domain.ScanInfected, domain.ScanError, domain.ScanSkipped} {
			for _, del := range []bool{false, true} {
				all = append(all, state{UploadState: u, ScanStatus: s, Deleted: del})
			}
		}
	}
	for _, before := range all {
		for _, after := range all {
			want := !domain.Final(before.UploadState, before.ScanStatus, before.Deleted) &&
				domain.Final(after.UploadState, after.ScanStatus, after.Deleted)
			if got := domain.BecameFinal(before, after); got != want {
				t.Errorf("BecameFinal(%+v, %+v) = %v, want %v", before, after, got, want)
			}
		}
	}
	pending := state{UploadState: domain.UploadPending, ScanStatus: domain.ScanPending}
	named := []struct {
		name  string
		after state
		want  bool
	}{
		{"clean", state{UploadState: domain.UploadComplete, ScanStatus: domain.ScanClean}, true},
		{"skipped", state{UploadState: domain.UploadComplete, ScanStatus: domain.ScanSkipped}, true},
		{"infected", state{UploadState: domain.UploadComplete, ScanStatus: domain.ScanInfected}, true},
		{"scan error", state{UploadState: domain.UploadComplete, ScanStatus: domain.ScanError}, true},
		{"checksum failure", state{UploadState: domain.UploadFailed, ScanStatus: domain.ScanPending}, true},
		{"abort", state{UploadState: domain.UploadFailed, ScanStatus: domain.ScanPending}, true},
		{"image waits for its original", state{UploadState: domain.UploadPending, ScanStatus: domain.ScanClean}, false},
		{"still pending", pending, false},
		{"deleted clean is not deliverable", state{UploadState: domain.UploadComplete, ScanStatus: domain.ScanClean, Deleted: true}, false},
	}
	for _, c := range named {
		if got := domain.BecameFinal(pending, c.after); got != c.want {
			t.Errorf("%s: BecameFinal = %v, want %v", c.name, got, c.want)
		}
	}
	clean := state{UploadState: domain.UploadComplete, ScanStatus: domain.ScanClean}
	if domain.BecameFinal(clean, clean) {
		t.Error("a change of a final file (thumbnails, media info) must not count as becoming final")
	}
}
