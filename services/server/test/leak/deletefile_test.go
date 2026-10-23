// SPDX-License-Identifier: Apache-2.0

package leak

import (
	"net/http"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// deleteFileWorld is a server with the matrix world: Sam's unattached upload in
// Marketing/Kampagnen (foreign to Alex) and an id of Alex whose file was deleted and purged.
type deleteFileWorld struct {
	alex            *harness.Client
	foreign, purged string
}

// newDeleteFileWorld starts the server, uploads Sam's file and deletes one of Alex's files and
// waits until files.purge_objects removed its row.
func newDeleteFileWorld(t *testing.T) deleteFileWorld {
	t.Helper()
	s := harness.Start(t, harness.Options{EgressDenyAll: true, Config: harness.RaisedRateLimits()})
	w := harness.BuildMatrixWorld(t, s)
	alex := w.People["Alex"]
	r := harness.MustDo(t, alex, harness.Request{Method: http.MethodPost, Path: "/channels/" + w.Channels["Support/Allgemein"] + "/uploads",
		Body: map[string]any{"name": "gone.txt", "size_bytes": 4, "sha256": "88d4266fd4e6338d13b845fcf289579d209c897823b9217da3e161936f031589"}},
		http.StatusCreated)
	var up struct {
		FileID string `json:"file_id"`
	}
	r.JSON(t, &up)
	harness.MustDo(t, alex, harness.Request{Method: http.MethodDelete, Path: "/files/" + up.FileID}, http.StatusNoContent)
	db := s.AdminDB()
	testkit.Until(t, "files.purge_objects removes the row", func() bool {
		var n int
		if err := db.QueryRow(t.Context(), `SELECT count(*) FROM files WHERE id = $1`, up.FileID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n == 0
	})
	return deleteFileWorld{alex: alex, foreign: uploadOfSam(t, w), purged: up.FileID}
}

// deleteAs sends deleteFile for id as c.
func deleteAs(t *testing.T, c *harness.Client, id string) harness.Response {
	t.Helper()
	r := c.Do(t.Context(), harness.Request{Method: http.MethodDelete, Path: "/files/" + id})
	if r.Status == 0 {
		t.Fatalf("DELETE /files/%s: %s", id, r.Body)
	}
	return r
}

// TestAZ_P4_DeleteFile (AZ-P4, brief M3-08 "Suites of other tickets"): deleteFile answers a
// foreign, an unknown and a purged id alike in status, headers and body (404 not_found), and the
// medians of the foreign and the purged id equal the unknown one within
// authz.not_found_timing_tolerance. It does not run in parallel (TimingSame).
func TestAZ_P4_DeleteFile(t *testing.T) {
	p := newDeleteFileWorld(t)
	for name, id := range map[string]string{"foreign": p.foreign, "purged": p.purged} {
		unknown := ids.New().String()
		pr := pair{foreign: []string{id}, random: []string{unknown}, content: true}
		assertEqualAnswers(t, pr, deleteAs(t, p.alex, id), deleteAs(t, p.alex, unknown))
		tolerance := notFoundTimingTolerance(t)
		measure := func(target string) func() time.Duration {
			return func() time.Duration { return deleteAs(t, p.alex, target).Elapsed }
		}
		if d, ok := testkit.TimingSame(tolerance, measure(id), measure(unknown)); !ok {
			t.Errorf("%s: medians differ by %v > %v", name, d, testkit.TimingBound(tolerance))
		}
	}
	if r := deleteAs(t, p.alex, p.foreign); r.Status != http.StatusNotFound {
		t.Fatalf("the foreign file: %d", r.Status)
	}
}
