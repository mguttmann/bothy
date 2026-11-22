// SPDX-License-Identifier: Apache-2.0

package attachments_test

import (
	"bytes"
	"net/http"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// idOf returns the member id of a JSON answer.
func idOf(t *testing.T, r harness.Response) string {
	t.Helper()
	var v struct {
		ID string `json:"id"`
	}
	r.JSON(t, &v)
	return v.ID
}

// TestTeam_OwnerScreenshotReachesTheMember (FIX-TEAM-CHANNEL-ATTACHMENTS, owner report of
// 2026-11-22): on the real server, an owner's message with a screenshot in a team channel reaches
// the member's work thread as a copy that lists the screenshot, the member's agent downloads it
// with its own token although it cannot read the team channel, and a person who reads neither
// place gets 404. The original stays the one row of message_attachments; the copy holds a relay.
func TestTeam_OwnerScreenshotReachesTheMember(t *testing.T) {
	t.Parallel()
	e := start(t)
	labs := idOf(t, harness.MustDo(t, e.owner, harness.Request{Method: http.MethodPost, Path: "/spaces", Body: map[string]any{"name": "Labs"}},
		http.StatusCreated))
	channel := func(name string) string {
		return idOf(t, harness.MustDo(t, e.owner, harness.Request{Method: http.MethodPost, Path: "/spaces/" + labs + "/channels",
			Body: map[string]any{"kind": "text", "name": name}}, http.StatusCreated))
	}
	team, lab := channel("Team"), channel("Lab")
	version := e.text(t, `SELECT version::text FROM channels WHERE id = $1`, lab)
	h := http.Header{}
	h.Set("If-Match", `"`+version+`"`)
	harness.MustDo(t, e.owner, harness.Request{Method: http.MethodPut, Path: "/channels/" + lab + "/operator", Header: h,
		Body: map[string]any{"mode": "agent", "agent_id": e.lotse.PrincipalID, "reason": "Lab agent"}}, http.StatusOK)
	harness.MustDo(t, e.owner, harness.Request{Method: http.MethodPut, Path: "/channels/" + team + "/team"}, http.StatusOK)
	var member struct {
		WorkThreadID string `json:"work_thread_id"`
	}
	harness.MustDo(t, e.owner, harness.Request{Method: http.MethodPost, Path: "/channels/" + team + "/team/members",
		Body: map[string]any{"agent_id": e.lotse.PrincipalID, "home_channel_id": lab, "handle": "lab"}}, http.StatusCreated).JSON(t, &member)

	shot := bytes.Repeat([]byte("screenshot bytes "), 64)
	file := e.clean(t, e.owner, team, "screen.txt", shot)
	src := msgOf(t, e.send(t, e.owner, e.mainThread(t, team), "Here is the screenshot of the last chat", []string{file}, nil), http.StatusCreated)

	// The job runs.team_input runs in the worker after the commit.
	var copyID string
	testkit.Until(t, "the copy in the work thread", func() bool {
		n := e.count(t, `SELECT count(*) FROM messages WHERE thread_id = $1 AND has_attachments`, member.WorkThreadID)
		if n == 1 {
			copyID = e.text(t, `SELECT id::text FROM messages WHERE thread_id = $1 AND has_attachments`, member.WorkThreadID)
		}
		return n == 1
	})
	copied := msgOf(t, harness.MustDo(t, e.lotse, harness.Request{Method: http.MethodGet, Path: "/messages/" + copyID}, http.StatusOK), http.StatusOK)
	if got := fileIDs(t, copied); !slices.Equal(got, []string{file}) {
		t.Fatalf("the copy lists %v, want the screenshot %s", got, file)
	}
	if r := e.lotse.Do(t.Context(), harness.Request{Method: http.MethodGet, Path: "/messages/" + src.ID}); r.Status != http.StatusNotFound {
		t.Fatalf("the member's agent reads the team channel: %d", r.Status)
	}
	got := e.lotse.Do(t.Context(), harness.Request{Method: http.MethodGet, Path: "/files/" + file + "/content"})
	if got.Status != http.StatusOK || !bytes.Equal(got.Body, shot) {
		t.Fatalf("download by the member's agent: %d, %d bytes", got.Status, len(got.Body))
	}
	harness.MustDo(t, e.lotse, harness.Request{Method: http.MethodGet, Path: "/files/" + file}, http.StatusOK)
	if r := e.sam.Do(t.Context(), harness.Request{Method: http.MethodGet, Path: "/files/" + file + "/content"}); r.Status != http.StatusNotFound {
		t.Fatalf("download by a person who reads neither place: %d", r.Status)
	}
	if n := e.count(t, `SELECT count(*) FROM message_attachments WHERE file_id = $1`, file); n != 1 {
		t.Fatalf("message_attachments rows of the screenshot: %d, want the original", n)
	}
	if n := e.count(t, `SELECT count(*) FROM message_attachment_relays WHERE file_id = $1 AND message_id = $2`, file, copyID); n != 1 {
		t.Fatalf("relays of the screenshot on the copy: %d", n)
	}
}
