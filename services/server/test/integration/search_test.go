// SPDX-License-Identifier: Apache-2.0

package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/searchlang"
)

// The texts of the search tests with their non-ASCII letters written out: U+00C4 capital A with
// diaeresis, U+00E4 small a with diaeresis, U+00FC small u with diaeresis.
const (
	verfuegbar = "verfügbar"
	aerger     = "Ärger mit dem Abruf"
)

// searchWorld is the world of the M3-13 tests (brief "Integration with the real modules"): the
// space Support with the text channel Allgemein and the forum channel Fragen; Alex (owner) and
// Robin with users.locale de, Sam with en; the agent lotse with an API token and write in
// Support; Sam writes in Support too.
type searchWorld struct {
	*world
	alex, sam, robin, lotse ids.ID
	support, allgemein      ids.ID
	fragen, main            ids.ID
}

func newSearchWorld(t *testing.T, search searchlang.Map) *searchWorld {
	t.Helper()
	w := &searchWorld{world: newEmptyWorldWith(t, search)}
	// The instance row bothy migrate up writes; putInstanceSetting reads it.
	w.sql(`INSERT INTO instance (instance_id, schema_version, sync_epoch, setup_completed_at) VALUES ($1, 35, $2, now())`,
		ids.New().String(), ids.New().String())
	w.alex, w.sam, w.robin = w.person("alex", true), w.person("sam", false), w.person("robin", false)
	for id, locale := range map[ids.ID]string{w.alex: "de", w.sam: "en", w.robin: "de"} {
		w.sql(`UPDATE users SET locale = $2 WHERE principal_id = $1`, id.String(), locale)
	}
	w.support = w.space("Support")
	w.allgemein, w.fragen = w.channel(w.support, "text", "Allgemein"), w.channel(w.support, "forum", "Fragen")
	w.main = ids.MustParse(w.text(`SELECT id::text FROM threads WHERE channel_id = $1 AND kind = 'main'`, w.allgemein.String()))
	w.lotse = w.agent("lotse")
	w.grant(`"subject_kind":"user","subject_id":"` + w.sam.String() + `","target_kind":"space","target_id":"` + w.support.String() +
		`","level":"write","reason":"Support"`)
	w.grant(`"subject_kind":"agent","subject_id":"` + w.lotse.String() + `","target_kind":"space","target_id":"` + w.support.String() +
		`","level":"write","reason":"Support agent"`)
	return w
}

// messageBody is a MessageCreate body with a fresh client_msg_id and content.
func messageBody(content string, extra ...string) string {
	c, err := json.Marshal(content)
	if err != nil {
		panic(err)
	}
	b := `{"client_msg_id":"` + ids.New().String() + `","content_md":` + string(c)
	for _, e := range extra {
		b += "," + e
	}
	return b + "}"
}

// post sends createMessage as name into thread and returns the new message.
func (w *searchWorld) post(name string, thread ids.ID, content string) ids.ID {
	w.t.Helper()
	m := w.want(w.do(name, http.MethodPost, "/api/v1/threads/"+thread.String()+"/messages", messageBody(content)), http.StatusCreated, "")
	return ids.MustParse(m["id"].(string))
}

// forumPost sends createThread as name into forum with title and a first message and returns the
// thread and the message.
func (w *searchWorld) forumPost(name string, forum ids.ID, title, content string) (ids.ID, ids.ID) {
	w.t.Helper()
	t, err := json.Marshal(title)
	must(w.t, err)
	m := w.want(w.do(name, http.MethodPost, "/api/v1/channels/"+forum.String()+"/threads",
		`{"title":`+string(t)+`,"first_message":`+messageBody(content)+`}`), http.StatusCreated, "")
	th, first := m["thread"].(map[string]any), m["first_message"].(map[string]any)
	return ids.MustParse(th["id"].(string)), ids.MustParse(first["id"].(string))
}

// find returns the messages whose vector matches search_tsquery over the default languages and
// q, read as bothy_app the way the search coordinator (M3-14) will call the function.
func (w *searchWorld) find(q string) []ids.ID {
	w.t.Helper()
	rows, err := w.pool.PGX().Query(context.Background(),
		`SELECT id::text FROM messages WHERE search_vector @@ search_tsquery($1::text[]::regconfig[], $2)`, []string{"german", "english"}, q)
	must(w.t, err)
	defer rows.Close()
	var out []ids.ID
	for rows.Next() {
		var id string
		must(w.t, rows.Scan(&id))
		out = append(out, ids.MustParse(id))
	}
	must(w.t, rows.Err())
	return out
}

// language returns search_language of the row id of table.
func (w *searchWorld) language(table string, id ids.ID) string {
	w.t.Helper()
	return w.text(`SELECT search_language::text FROM `+table+` WHERE id = $1`, id.String())
}

// weights returns per lexeme of the vector expression vec of the row id of table its distinct
// weights, sorted.
func (w *searchWorld) weights(table, vec string, id ids.ID) map[string][]string {
	w.t.Helper()
	rows, err := w.db.Query(context.Background(), `SELECT u.lexeme, u.weights::text[] FROM `+table+` m, unnest(m.`+vec+`) AS u
		WHERE m.id = $1`, id.String())
	must(w.t, err)
	defer rows.Close()
	out := map[string][]string{}
	for rows.Next() {
		var lexeme string
		var ws []string
		must(w.t, rows.Scan(&lexeme, &ws))
		slices.Sort(ws)
		out[lexeme] = slices.Compact(ws)
	}
	must(w.t, rows.Err())
	return out
}

// TestLanguageExamplesFromChapter14 holds the language rows of 14 section 12 and the limits of
// 14 3.3 and 4.2 on PostgreSQL 18 through the real write path.
func TestLanguageExamplesFromChapter14(t *testing.T) {
	t.Parallel()
	w := newSearchWorld(t, chapterSearch(t))
	rechnung := w.post("alex", w.main, "Rechnung")
	adresse := w.post("alex", w.main, "Rechnungsadresse")
	kennung := w.post("alex", w.main, "Fehler ERR_CONN_RESET beim Abruf")
	verfuegbarMsg := w.post("alex", w.main, verfuegbar)
	offen := w.post("alex", w.main, "Rechnung offen")
	aergerMsg := w.post("alex", w.main, aerger)
	invoices := w.post("sam", w.main, "invoices are running")
	for _, c := range []struct {
		q          string
		want, not  []ids.ID
		limitation string
	}{
		{q: "Rechnungen", want: []ids.ID{rechnung, offen}, limitation: "German stem"},
		{q: "invoice run", want: []ids.ID{invoices}, limitation: "English stem"},
		{q: "err_conn_reset", want: []ids.ID{kennung}, limitation: "identifier by its parts"},
		{q: `"err_conn_reset"`, want: []ids.ID{kennung}, limitation: "identifier as a phrase"},
		{q: "Rechnung", want: []ids.ID{rechnung, offen}, not: []ids.ID{adresse}, limitation: "no compound splitting (14 3.3)"},
		{q: verfuegbar, want: []ids.ID{verfuegbarMsg}, limitation: "umlaut"},
		{q: "ärger", want: []ids.ID{aergerMsg}, limitation: "case folding of non-ASCII letters"},
		{q: "offen -Rechnungen", want: []ids.ID{offen}, limitation: "negation per sub-query is best effort (14 4.2)"},
	} {
		got := w.find(c.q)
		for _, id := range c.want {
			if !slices.Contains(got, id) {
				t.Errorf("%q (%s): %s not found in %v", c.q, c.limitation, id, got)
			}
		}
		for _, id := range c.not {
			if slices.Contains(got, id) {
				t.Errorf("%q (%s): %s found", c.q, c.limitation, id)
			}
		}
	}
}

// TestSearchVectorWeights holds the weights of 14 section 2: message stems B and unchanged words
// D, both forms of an umlaut (14 3.3), and only A in a thread title.
func TestSearchVectorWeights(t *testing.T) {
	t.Parallel()
	w := newSearchWorld(t, chapterSearch(t))
	offen := w.post("alex", w.main, "Rechnung offen")
	if got, want := w.weights("messages", "search_vector", offen), map[string][]string{
		"off": {"B"}, "rechnung": {"B", "D"}, "offen": {"D"},
	}; !equalWeights(got, want) {
		t.Errorf("Rechnung offen: %v, want %v", got, want)
	}
	v := w.post("alex", w.main, verfuegbar)
	if got, want := w.weights("messages", "search_vector", v), map[string][]string{
		"verfugbar": {"B"}, verfuegbar: {"D"},
	}; !equalWeights(got, want) {
		t.Errorf("%s: %v, want %v", verfuegbar, got, want)
	}
	th, _ := w.forumPost("alex", w.fragen, "Ärger mit Rechnungen", "Siehe Anhang")
	if got, want := w.weights("threads", "title_search", th), map[string][]string{
		"arg": {"A"}, "mit": {"A"}, "rechnung": {"A"}, "rechnungen": {"A"}, "ärger": {"A"},
	}; !equalWeights(got, want) {
		t.Errorf("thread title: %v, want %v", got, want)
	}
}

func equalWeights(a, b map[string][]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if !slices.Equal(v, b[k]) {
			return false
		}
	}
	return true
}

// TestSearchLanguageAssignedAtCreation holds 14 3.1 (R-SU-02, D-M3-13-1 and D-M3-13-2) through
// the API: the author's or creator's language is set at creation, the instance language for an
// agent and a system line, and no request sets or changes it.
func TestSearchLanguageAssignedAtCreation(t *testing.T) {
	t.Parallel()
	w := newSearchWorld(t, chapterSearch(t))
	alexMsg, samMsg := w.post("alex", w.main, "Hallo"), w.post("sam", w.main, "Hello")
	if a, s := w.language("messages", alexMsg), w.language("messages", samMsg); a != "german" || s != "english" {
		t.Errorf("languages alex %s, sam %s; want german, english", a, s)
	}
	if got := w.language("messages", w.post("lotse", w.main, "Bericht")); got != "german" {
		t.Errorf("agent with locale.default de: %s, want german", got)
	}

	// A forum post by Sam takes his language for thread and first message.
	th, first := w.forumPost("sam", w.fragen, "Invoices are running", "since this morning")
	if l, f := w.language("threads", th), w.language("messages", first); l != "english" || f != "english" {
		t.Errorf("forum post: thread %s, first message %s; want english", l, f)
	}
	var match bool
	if err := w.db.QueryRow(context.Background(), `SELECT title_search @@ search_tsquery('{german,english}', 'invoice') FROM threads WHERE id = $1`,
		th.String()).Scan(&match); err != nil || !match {
		t.Errorf("title_search does not match invoice: %v %v", match, err)
	}

	// Closing and a new message of Sam: the reopen line takes the instance language (de), Sam's
	// message his own.
	w.want(w.do("alex", http.MethodPost, "/api/v1/threads/"+th.String()+"/close", "", key()...), http.StatusOK, "")
	reply := w.post("sam", th, "still running")
	line := ids.MustParse(w.text(`SELECT id::text FROM messages WHERE thread_id = $1 AND kind = 'system'`, th.String()))
	if l, r := w.language("messages", line), w.language("messages", reply); l != "german" || r != "english" {
		t.Errorf("reopen: system line %s, message %s; want german, english", l, r)
	}

	// A new channel: its main thread takes the language of its creator.
	m := w.want(w.do("alex", http.MethodPost, "/api/v1/spaces/"+w.support.String()+"/channels", `{"kind":"text","name":"Neu"}`, key()...),
		http.StatusCreated, "")
	if got := w.text(`SELECT search_language::text FROM threads WHERE channel_id = $1 AND kind = 'main'`, m["id"].(string)); got != "german" {
		t.Errorf("main thread of Alex's channel: %s, want german", got)
	}

	// locale.default en: the agent writes English from now on.
	w.want(w.do("alex", http.MethodPut, "/api/v1/instance/settings/locale.default", `{"value":"en"}`), http.StatusOK, "")
	if got := w.language("messages", w.post("lotse", w.main, "Report")); got != "english" {
		t.Errorf("agent with locale.default en: %s, want english", got)
	}

	// Alex changes her locale: her earlier message keeps german, a new one is English.
	w.want(w.do("alex", http.MethodPatch, "/api/v1/me/profile", `{"locale":"en"}`, key()...), http.StatusOK, "")
	if got := w.language("messages", alexMsg); got != "german" {
		t.Errorf("earlier message after the locale change: %s, want german", got)
	}
	if got := w.language("messages", w.post("alex", w.main, "Hello again")); got != "english" {
		t.Errorf("new message after the locale change: %s, want english", got)
	}

	// The language is no member of any request: an unknown member is refused and nothing is
	// written.
	before := w.count(`SELECT count(*) FROM messages`)
	w.want(w.do("alex", http.MethodPost, "/api/v1/threads/"+w.main.String()+"/messages", messageBody("Hallo", `"search_language":"simple"`)),
		http.StatusBadRequest, "validation_failed")
	if after := w.count(`SELECT count(*) FROM messages`); after != before {
		t.Errorf("messages %d after a refused request, want %d", after, before)
	}
}

// TestSearchLanguageWithoutMapEntry holds "Kein Eintrag in der Abbildung: simple" (14 3.1): with
// search.locale_map {"de":"german"} Sam's message gets simple, Alex's still german.
func TestSearchLanguageWithoutMapEntry(t *testing.T) {
	t.Parallel()
	m, err := searchlang.New([]string{"german", "english"}, map[string]string{"de": "german"})
	must(t, err)
	w := newSearchWorld(t, m)
	if got := w.language("messages", w.post("sam", w.main, "Hello")); got != searchlang.Simple {
		t.Errorf("sam without en in the map: %s, want simple", got)
	}
	if got := w.language("messages", w.post("alex", w.main, "Hallo")); got != "german" {
		t.Errorf("alex: %s, want german", got)
	}
}

// TestDeletedMessageHasEmptyVector holds DP-T01 "search hits" and 14 section 8: a message brought
// into the row state of a deleted message (M1-58) has an empty vector at once and is found by no
// query; an edited message is found by its new text only. Both rows are changed with SQL as the
// owner role, because the edit and delete path of M1-58 is not in this ticket.
func TestDeletedMessageHasEmptyVector(t *testing.T) {
	t.Parallel()
	w := newSearchWorld(t, chapterSearch(t))
	gone := w.post("alex", w.main, "Rechnung offen")
	edited := w.post("alex", w.main, "Abruf geplant")
	w.sql(`UPDATE messages SET content_md = '', deleted_at = now(), deleted_by = author_principal_id WHERE id = $1`, gone.String())
	var empty bool
	if err := w.db.QueryRow(context.Background(), `SELECT search_vector = ''::tsvector FROM messages WHERE id = $1`, gone.String()).Scan(&empty); err != nil || !empty {
		t.Errorf("deleted message: empty vector %v (%v)", empty, err)
	}
	for _, q := range []string{"Rechnung", "offen", "Rechnung offen"} {
		if slices.Contains(w.find(q), gone) {
			t.Errorf("%q finds the deleted message", q)
		}
	}
	w.sql(`UPDATE messages SET content_md = 'Lieferung verschoben', edited_at = now(), version = version + 1 WHERE id = $1`, edited.String())
	if !slices.Contains(w.find("Lieferung"), edited) {
		t.Error("the new text does not find the edited message")
	}
	if slices.Contains(w.find("Abruf"), edited) || slices.Contains(w.find("geplant"), edited) {
		t.Error("the old text still finds the edited message")
	}
	if got := w.language("messages", edited); got != "german" {
		t.Errorf("edited message: language %s, want german (kept)", got)
	}
}
