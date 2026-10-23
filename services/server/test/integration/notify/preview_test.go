// SPDX-License-Identifier: Apache-2.0

package notify_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"slices"
	"testing"
)

// The tests of M3-22 for the preview fields of notification.created (brief "Tests",
// TestGatewayHint_PreviewFieldsByMode and TestGatewayHint_LostAccessGetsNoNames; 13 6.3, design
// note 9 f) over the world wired as role all (withLive) with the preview builder (withPreview).

// previewCanary is a word of the message text that must never appear in a none hint or after a
// revocation.
const previewCanary = "Zebrafink"

// hintWorld returns a world in which Sam has a live macos desktop with an empty focus and follows
// a thread T of Support/Allgemein with all, and Alex wrote a Markdown message in T that mentions
// Sam; the consumer ran, so one notify.gateway_send for Sam waits.
func hintWorld(t *testing.T, mode string) *world {
	t.Helper()
	w := newWorld(t, withLive(), withPreview())
	th := w.newThread(w.allgemein)
	w.setLevel("sam", th, "all")
	if mode != "" {
		w.setMySetting("sam", "notify.preview_mode", `"`+mode+`"`)
	}
	desk := w.desktop(w.sam, "macos")
	w.live.set(w.sam, desk, false)
	w.want(w.postBody("alex", th, "**Look** @sam, the "+previewCanary+" `report` is [here](https://example.com/r)",
		`"mention_ids":["`+w.sam.String()+`"]`), http.StatusCreated)
	w.consume()
	if n := len(w.gatewayJobsFor(w.sam)); n != 1 {
		t.Fatalf("%d gateway jobs for Sam, want 1", n)
	}
	return w
}

// hintMembers returns the payload of the only notification.created to Sam as a JSON object and
// its raw bytes.
func (w *world) hintMembers() (map[string]any, []byte) {
	w.t.Helper()
	evs := w.published.to(w.sam)
	if len(evs) != 1 {
		w.t.Fatalf("%d events to Sam, want 1", len(evs))
	}
	w.payloadOf(evs[0])
	raw, err := json.Marshal(evs[0].Payload)
	must(w.t, err)
	var m map[string]any
	must(w.t, json.Unmarshal(raw, &m))
	return m, raw
}

// keys returns the sorted member names of m.
func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// TestGatewayHint_PreviewFieldsByMode: full carries names and excerpt (no image_url), names only
// the names, none exactly the IDs.
func TestGatewayHint_PreviewFieldsByMode(t *testing.T) {
	t.Parallel()
	t.Run("full", func(t *testing.T) {
		t.Parallel()
		w := hintWorld(t, "full")
		w.runGateway()
		m, _ := w.hintMembers()
		if m["space_name"] != "Support" || m["channel_name"] != "Allgemein" || m["sender_display_name"] != "alex" {
			t.Errorf("full names %v", m)
		}
		ex, _ := m["excerpt"].(string)
		if !bytes.Contains([]byte(ex), []byte("sam")) || !bytes.Contains([]byte(ex), []byte(previewCanary)) || bytes.ContainsAny([]byte(ex), "*`[]@") {
			t.Errorf("full excerpt %q", ex)
		}
		if _, ok := m["image_url"]; ok {
			t.Error("notification.created carries image_url")
		}
	})
	t.Run("names", func(t *testing.T) {
		t.Parallel()
		w := hintWorld(t, "names")
		w.runGateway()
		m, _ := w.hintMembers()
		if m["space_name"] != "Support" || m["sender_display_name"] != "alex" {
			t.Errorf("names %v", m)
		}
		for _, k := range []string{"excerpt", "attachment_count", "image_url"} {
			if _, ok := m[k]; ok {
				t.Errorf("names carries %s", k)
			}
		}
	})
	t.Run("none", func(t *testing.T) {
		t.Parallel()
		w := hintWorld(t, "none")
		w.runGateway()
		m, raw := w.hintMembers()
		if got, want := keys(m), []string{"count", "kind", "message_id", "nid", "quiet", "thread_id"}; !slices.Equal(got, want) {
			t.Errorf("none members %v, want %v", got, want)
		}
		for _, s := range []string{"Support", "Allgemein", "alex", previewCanary} {
			if bytes.Contains(raw, []byte(s)) {
				t.Errorf("none frame holds %q", s)
			}
		}
	})
}

// TestGatewayHint_LostAccessGetsNoNames: Sam loses read on Allgemein after the round and before the
// job: no notification.created and no name or text in the log of the push path; a message deleted
// between round and job gives an event without any preview field.
func TestGatewayHint_LostAccessGetsNoNames(t *testing.T) {
	t.Parallel()
	t.Run("revoked", func(t *testing.T) {
		t.Parallel()
		w := hintWorld(t, "full")
		mark := w.logMark()
		w.removeMember(w.leser, w.sam)
		w.runGateway()
		if evs := w.published.to(w.sam); len(evs) != 0 {
			t.Fatalf("%d events to Sam after the revocation", len(evs))
		}
		for _, r := range w.logSince(mark) {
			b, _ := json.Marshal(r)
			for _, s := range []string{"Support", "Allgemein", previewCanary} {
				if bytes.Contains(b, []byte(s)) {
					t.Errorf("log record holds %q", s)
				}
			}
		}
	})
	t.Run("deleted", func(t *testing.T) {
		t.Parallel()
		w := hintWorld(t, "full")
		jobs := w.gatewayJobsFor(w.sam)
		w.want(w.do("alex", http.MethodDelete, "/api/v1/messages/"+jobs[0].Args.MessageID.String(), ""), http.StatusNoContent)
		w.runGateway()
		m, raw := w.hintMembers()
		for _, k := range []string{"space_name", "channel_name", "thread_title", "sender_display_name", "excerpt", "attachment_count"} {
			if _, ok := m[k]; ok {
				t.Errorf("deleted message: event carries %s", k)
			}
		}
		if bytes.Contains(raw, []byte(previewCanary)) {
			t.Error("deleted message: the frame holds the text")
		}
	})
}
