// SPDX-License-Identifier: Apache-2.0

package attachments_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"reflect"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/test/harness"
)

// The search tests of M3-14 that need real files: they share the server of this package with
// its scripted clamd, because attachments, scan outcomes and thread moves exist only there.

// hitPage is the decoded SearchHitPage.
type hitPage struct {
	Items []struct {
		Kind        string         `json:"kind"`
		ThreadTitle *string        `json:"thread_title"`
		ChannelName *string        `json:"channel_name"`
		SpaceName   *string        `json:"space_name"`
		Message     map[string]any `json:"message"`
		Thread      map[string]any `json:"thread"`
		File        map[string]any `json:"file"`
	} `json:"items"`
	NextCursor *string `json:"next_cursor"`
}

// ids returns the ids of the hit objects in order.
func (p hitPage) ids() []string {
	out := []string{}
	for _, it := range p.Items {
		for _, o := range []map[string]any{it.Message, it.Thread, it.File} {
			if o != nil {
				out = append(out, o["id"].(string))
			}
		}
	}
	return out
}

// search sends GET /search as c with params and decodes the 200 page.
func search(t *testing.T, c *harness.Client, params url.Values) hitPage {
	t.Helper()
	r := c.Do(t.Context(), harness.Request{Method: http.MethodGet, Path: "/search?" + params.Encode()})
	if r.Status != http.StatusOK {
		t.Fatalf("search %v: %d %s", params, r.Status, r.Body)
	}
	var p hitPage
	r.JSON(t, &p)
	return p
}

// sq returns query parameters with q and pairs of further names and values.
func sq(q string, pairs ...string) url.Values {
	v := url.Values{"q": {q}}
	for i := 0; i+1 < len(pairs); i += 2 {
		v.Add(pairs[i], pairs[i+1])
	}
	return v
}

// titledThread creates a thread with title in the text channel ch on a new message of c in its
// main thread and returns the thread id.
func (e *env) titledThread(t *testing.T, c *harness.Client, ch, title string) string {
	t.Helper()
	parent := msgOf(t, e.send(t, c, e.mainThread(t, ch), "Start", nil, nil), http.StatusCreated)
	var th struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	harness.MustDo(t, c, harness.Request{Method: http.MethodPost, Path: "/channels/" + ch + "/threads",
		Body: map[string]any{"parent_message_id": parent.ID, "title": title}}, http.StatusCreated).JSON(t, &th)
	return th.Thread.ID
}

// TestSearch_TypesFiles: type files finds an attached clean file by name and by transcript with
// thread_title, channel_name and space_name. The names hold a space: the parser of PostgreSQL reads
// "name.txt" as one file token, so a word of a name is found when it stands apart (14 3.3).
// has_file keeps exactly the messages with or without attachments.
func TestSearch_TypesFiles(t *testing.T) {
	t.Parallel()
	e := start(t)
	th := e.titledThread(t, e.alex, e.allg, "Quarter close")
	f := e.clean(t, e.alex, e.allg, "quarterly ledger.txt", []byte("ledger"))
	with := msgOf(t, e.send(t, e.alex, th, "see the ledger", []string{f}, nil), http.StatusCreated)
	without := msgOf(t, e.send(t, e.alex, th, "the ledger follows", nil, nil), http.StatusCreated)
	// The transcript comes from an agent or the transcription service (12 3.6); the fixture
	// writes it directly, because this server runs neither.
	e.exec(t, `UPDATE files SET transcript_status = 'done', transcript_text = 'reconciliation of accounts' WHERE id = $1`, f)
	for _, q := range []string{"quarterly", "reconciliation"} {
		p := search(t, e.alex, sq(q, "type", "files"))
		if len(p.Items) != 1 || p.Items[0].Kind != "file" || p.Items[0].File["id"] != f || deref(p.Items[0].ThreadTitle) != "Quarter close" ||
			deref(p.Items[0].ChannelName) != "Allgemein" || deref(p.Items[0].SpaceName) != "Support" {
			t.Fatalf("files %q: %+v", q, p.Items)
		}
	}
	if got := search(t, e.alex, sq("ledger", "has_file", "true")).ids(); !slices.Equal(got, []string{with.ID}) {
		t.Fatalf("has_file true: %v, want %s", got, with.ID)
	}
	if got := search(t, e.alex, sq("ledger", "has_file", "false")).ids(); !slices.Equal(got, []string{without.ID}) {
		t.Fatalf("has_file false: %v, want %s", got, without.ID)
	}
	// Robin reads no channel of Support: nothing.
	if got := search(t, e.robin, sq("quarterly", "type", "files")); len(got.Items) != 0 {
		t.Fatalf("Robin found %v", got.ids())
	}
}

// deref returns *s or "".
func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// TestSearch_NotSearchable (14 12, DP-T01 search part): a deleted message, an attached file that
// is pending, infected, error or of a deleted message, a file in state failed, a clean file never
// attached, and a draft give no hit in any type; an attached clean file with the same word is the
// positive control and the only hit. No state depends on the retry clock of the scan pass: the
// files that must end clean are scanned before clamd stalls, the stall lasts until the end, so the
// pending file stays pending, and infected and error come from the fixture.
func TestSearch_NotSearchable(t *testing.T) {
	t.Parallel()
	e := start(t)
	th := e.newThread(t, e.alex, e.allg)
	gone := msgOf(t, e.send(t, e.alex, th, "obsolete zebra", nil, nil), http.StatusCreated)
	harness.MustDo(t, e.alex, harness.Request{Method: http.MethodDelete, Path: "/messages/" + gone.ID}, http.StatusNoContent)

	control := e.clean(t, e.alex, e.allg, "zebra control.txt", []byte("control"))
	msgOf(t, e.send(t, e.alex, th, "control file", []string{control}, nil), http.StatusCreated)
	e.clean(t, e.alex, e.allg, "zebra never.txt", []byte("never attached"))
	ofDeleted := e.clean(t, e.alex, e.allg, "zebra deleted.txt", []byte("of a deleted message"))
	carrier := msgOf(t, e.send(t, e.alex, th, "carrier", []string{ofDeleted}, nil), http.StatusCreated)
	harness.MustDo(t, e.alex, harness.Request{Method: http.MethodDelete, Path: "/messages/" + carrier.ID}, http.StatusNoContent)
	infected := e.clean(t, e.alex, e.allg, "zebra infected.txt", []byte("infected"))
	msgOf(t, e.send(t, e.alex, th, "infected file", []string{infected}, nil), http.StatusCreated)
	broken := e.clean(t, e.alex, e.allg, "zebra error.txt", []byte("error"))
	msgOf(t, e.send(t, e.alex, th, "error file", []string{broken}, nil), http.StatusCreated)
	// The scan outcomes infected and error need clamd answers at the right retry; the fixture
	// sets them directly (12 2: neither is deliverable).
	e.exec(t, `UPDATE files SET scan_status = 'infected' WHERE id = $1`, infected)
	e.exec(t, `UPDATE files SET scan_status = 'error' WHERE id = $1`, broken)

	failed := e.open(t, e.alex, e.allg, "zebra failed.txt", []byte("aborted"))
	harness.MustDo(t, e.alex, harness.Request{Method: http.MethodDelete, Path: "/uploads/" + failed.FileID}, http.StatusNoContent)

	e.stallScans() // until the end of the test: every retry stalls again
	pending := e.file(t, e.alex, e.allg, "zebra pending.txt", []byte("pending"))
	msgOf(t, e.send(t, e.alex, th, "pending file", []string{pending}, nil), http.StatusCreated)

	harness.MustDo(t, e.alex, harness.Request{Method: http.MethodPut, Path: "/threads/" + th + "/draft",
		Body: map[string]any{"content_md": "draft zebra"}}, http.StatusOK)

	if st := e.state(t, pending); st != "pending/pending" {
		t.Fatalf("pending file is %s", st)
	}
	for _, typ := range []string{"messages", "threads"} {
		if got := search(t, e.alex, sq("zebra", "type", typ)); len(got.Items) != 0 {
			t.Errorf("type %s found %v", typ, got.ids())
		}
	}
	if got := search(t, e.alex, sq("zebra", "type", "files")).ids(); !slices.Equal(got, []string{control}) {
		t.Errorf("type files found %v, want only the control %s", got, control)
	}
}

// TestSearch_HitObjectsEqualModuleRepresentation: the message of a hit equals its item of
// listThreadMessages, the thread equals getThread and the file equals getFile.
func TestSearch_HitObjectsEqualModuleRepresentation(t *testing.T) {
	t.Parallel()
	e := start(t)
	th := e.titledThread(t, e.alex, e.allg, "Walrus planning")
	f := e.clean(t, e.alex, e.allg, "walrus plan.txt", []byte("walrus"))
	m := msgOf(t, e.send(t, e.alex, th, "walrus with a file", []string{f}, nil), http.StatusCreated)

	var list struct {
		Items []map[string]any `json:"items"`
	}
	get(t, e.alex, "/threads/"+th+"/messages?limit=50").JSON(t, &list)
	var listed map[string]any
	for _, it := range list.Items {
		if it["id"] == m.ID {
			listed = it
		}
	}
	hit := search(t, e.alex, sq("walrus"))
	if len(hit.Items) != 1 || !reflect.DeepEqual(hit.Items[0].Message, listed) {
		t.Fatalf("message hit %v\nlist item %v", jsonOf(hit.Items), jsonOf(listed))
	}
	var thread map[string]any
	get(t, e.alex, "/threads/"+th).JSON(t, &thread)
	tp := search(t, e.alex, sq("walrus", "type", "threads"))
	if len(tp.Items) != 1 || !reflect.DeepEqual(tp.Items[0].Thread, thread) {
		t.Fatalf("thread hit %v\ngetThread %v", jsonOf(tp.Items), jsonOf(thread))
	}
	var file map[string]any
	get(t, e.alex, "/files/"+f).JSON(t, &file)
	fp := search(t, e.alex, sq("walrus", "type", "files"))
	if len(fp.Items) != 1 || !reflect.DeepEqual(fp.Items[0].File, file) {
		t.Fatalf("file hit %v\ngetFile %v", jsonOf(fp.Items), jsonOf(file))
	}
}

// jsonOf renders v for a failure message.
func jsonOf(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// TestSearchFiles_FollowsThreadMove: after moveThread from Support/Allgemein (Alex and Sam read
// it) to Support/Eskalation (Alex reads it, Sam not), the attached file of a message in the moved
// thread is found by Alex, a reader of the target, and no longer by Sam, a reader of the source
// only.
func TestSearchFiles_FollowsThreadMove(t *testing.T) {
	t.Parallel()
	e := start(t)
	th := e.newThread(t, e.alex, e.allg)
	f := e.clean(t, e.alex, e.allg, "narwhal notes.txt", []byte("narwhal"))
	msgOf(t, e.send(t, e.alex, th, "narwhal file", []string{f}, nil), http.StatusCreated)
	for _, c := range []*harness.Client{e.alex, e.sam} {
		if got := search(t, c, sq("narwhal", "type", "files")).ids(); !slices.Equal(got, []string{f}) {
			t.Fatalf("before the move: %v", got)
		}
	}
	if r := e.move(t, th, e.esk); r.Status != http.StatusOK {
		t.Fatalf("moveThread: %d %s", r.Status, r.Body)
	}
	if got := search(t, e.alex, sq("narwhal", "type", "files")).ids(); !slices.Equal(got, []string{f}) {
		t.Fatalf("Alex after the move: %v", got)
	}
	if got := search(t, e.sam, sq("narwhal", "type", "files")); len(got.Items) != 0 {
		t.Fatalf("Sam after the move: %v", got.ids())
	}
}
