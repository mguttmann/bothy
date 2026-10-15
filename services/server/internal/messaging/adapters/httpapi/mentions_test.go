// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	mw "github.com/OWNER/bothy/services/server/test/messagingworld"
)

// myMentions is the path of listMyMentions.
const myMentions = "/api/v1/me/mentions"

// mentionWorld is Alex with read on Support and none on Marketing, a text channel and its main
// thread in each space, and the agent lotse.
type mentionWorld struct {
	*mw.World
	alex                   ids.ID
	supportMain, mktMain   ids.ID
	supportChan, mktChanID ids.ID
}

func newMentionWorld(t *testing.T) mentionWorld {
	t.Helper()
	w := mw.New(t, subscribe)
	support, marketing := w.Space("Support"), w.Space("Marketing")
	m := mentionWorld{World: w}
	m.supportChan = w.Channel(support, "text", "general")
	m.mktChanID = w.Channel(marketing, "text", "general")
	m.supportMain, m.mktMain = w.MainThread(m.supportChan), w.MainThread(m.mktChanID)
	m.alex = w.Person("alex", false)
	w.Grant("user", m.alex, support, "read")
	w.Agent("lotse")
	return m
}

// mention sends "@alex <text>" as the owner into thread one second after the previous one and
// returns the message id.
func (w mentionWorld) mention(thread ids.ID, text string) string {
	w.Clock.Advance(time.Second)
	return w.Send("owner", thread, "@alex "+text, http.StatusCreated)["id"].(string)
}

// page reads one page of Alex's mentions.
func (w mentionWorld) page(name, query string, status int, code string) map[string]any {
	return w.Want(w.Do(name, http.MethodGet, myMentions+query, ""), status, code)
}

// messageIDs returns the message ids of a MentionItemPage.
func messageIDs(p map[string]any) []string {
	var out []string
	for _, it := range p["items"].([]any) {
		out = append(out, it.(map[string]any)["message"].(map[string]any)["id"].(string))
	}
	return out
}

// TestListMyMentions_NewestFirstAndPaging: seven mentions of Alex, limit 3, three pages newest
// first, no duplicates, the last page without next_cursor.
func TestListMyMentions_NewestFirstAndPaging(t *testing.T) {
	t.Parallel()
	w := newMentionWorld(t)
	var sent []string
	for i := range 7 {
		sent = append(sent, w.mention(w.supportMain, strconv.Itoa(i)))
	}
	w.Send("owner", w.supportMain, "no mention", http.StatusCreated)
	var got []string
	cursor := ""
	for pageNo := range 3 {
		q := "?limit=3"
		if cursor != "" {
			q += "&cursor=" + url.QueryEscape(cursor)
		}
		p := w.page("alex", q, http.StatusOK, "")
		ids := messageIDs(p)
		got = append(got, ids...)
		next, has := p["next_cursor"].(string)
		if pageNo < 2 && (!has || len(ids) != 3) {
			t.Fatalf("page %d: %d items, next %v", pageNo, len(ids), p["next_cursor"])
		}
		if pageNo == 2 && (has || len(ids) != 1) {
			t.Fatalf("last page: %d items, next %q", len(ids), next)
		}
		cursor = next
	}
	for i, id := range got {
		if id != sent[len(sent)-1-i] {
			t.Fatalf("order %v, sent %v", got, sent)
		}
	}
	first := w.page("alex", "?limit=1", http.StatusOK, "")["items"].([]any)[0].(map[string]any)
	if first["created_at"] == nil || first["message"].(map[string]any)["content_md"] != "@alex 6" {
		t.Fatalf("item %v", first)
	}
}

// TestAZM48_MentionInInvisibleChannelStoredNotListed: a Marketing message mentions Alex; the row
// exists in mentions, but neither listMyMentions nor CountUnread show it.
func TestAZM48_MentionInInvisibleChannelStoredNotListed(t *testing.T) {
	t.Parallel()
	w := newMentionWorld(t)
	hidden := w.mention(w.mktMain, "marketing")
	if n := w.Count(`SELECT count(*)::int FROM mentions WHERE message_id = $1 AND principal_id = $2`, hidden, w.alex.String()); n != 1 {
		t.Fatalf("%d mention rows", n)
	}
	if got := messageIDs(w.page("alex", "", http.StatusOK, "")); len(got) != 0 {
		t.Fatalf("listed %v", got)
	}
	visible := w.mention(w.supportMain, "support")
	if got := messageIDs(w.page("alex", "", http.StatusOK, "")); len(got) != 1 || got[0] != visible {
		t.Fatalf("listed %v", got)
	}
	// CountUnread with every channel id the owner can name: the invisible one is excluded by the
	// caller's scope, and with the scope of Alex only Support counts.
	counts, err := w.Svc.Mentions().CountUnread(context.Background(), messaging.UnreadMentionQuery{
		PrincipalID: w.alex, VisibleChannelIDs: alexScope(t, w),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(counts) != 1 || counts[0].ThreadID != w.supportMain || counts[0].Count != 1 {
		t.Fatalf("counts %+v", counts)
	}
}

// alexScope is the message.read scope of Alex as sync builds it.
func alexScope(t *testing.T, w mentionWorld) []ids.ID {
	t.Helper()
	sc, err := w.Authorizer.Scope(context.Background(), actor.Actor{ID: w.alex, Kind: actor.KindUser}, "message.read")
	if err != nil {
		t.Fatal(err)
	}
	return sc.ChannelIDs
}

// TestListMyMentions_ExcludesTombstones: a deleted message (a tombstone set by a fixture, since
// M1-58 is not on the base) is not listed.
func TestListMyMentions_ExcludesTombstones(t *testing.T) {
	t.Parallel()
	w := newMentionWorld(t)
	gone := w.mention(w.supportMain, "gone")
	kept := w.mention(w.supportMain, "kept")
	w.SQL(`UPDATE messages SET deleted_at = now(), deleted_by = author_principal_id, content_md = '', version = version + 1 WHERE id = $1`, gone)
	if got := messageIDs(w.page("alex", "", http.StatusOK, "")); len(got) != 1 || got[0] != kept {
		t.Fatalf("listed %v", got)
	}
}

// TestListMyMentions_CursorInvalid: a cursor of another list, of another principal and a
// tampered one each give 400 cursor_invalid.
func TestListMyMentions_CursorInvalid(t *testing.T) {
	t.Parallel()
	w := newMentionWorld(t)
	w.Person("sam", false)
	for i := range 3 {
		w.mention(w.supportMain, strconv.Itoa(i))
	}
	alexCursor := w.page("alex", "?limit=1", http.StatusOK, "")["next_cursor"].(string)
	// Another principal: Sam replays Alex's cursor.
	w.page("sam", "?limit=1&cursor="+url.QueryEscape(alexCursor), http.StatusBadRequest, "cursor_invalid")
	// Tampered: one character changed.
	b := []byte(alexCursor)
	if b[5] == 'A' {
		b[5] = 'B'
	} else {
		b[5] = 'A'
	}
	w.page("alex", "?limit=1&cursor="+url.QueryEscape(string(b)), http.StatusBadRequest, "cursor_invalid")
	// Another list: a cursor of listChannelThreads.
	space := w.Space("Forum")
	forum := w.Channel(space, "forum", "posts")
	for i := range 3 {
		w.Want(w.Do("owner", http.MethodPost, mw.ThreadsPath(forum), `{"title":"Post `+strconv.Itoa(i)+`","first_message":`+
			mw.Body(ids.New(), "post")+`}`), http.StatusCreated, "")
	}
	tp := w.Want(w.Do("owner", http.MethodGet, "/api/v1/channels/"+forum.String()+"/threads?limit=1", ""), http.StatusOK, "")
	other, ok := tp["next_cursor"].(string)
	if !ok || strings.TrimSpace(other) == "" {
		t.Fatalf("no thread cursor: %v", tp)
	}
	w.page("owner", "?cursor="+url.QueryEscape(other), http.StatusBadRequest, "cursor_invalid")
	// The valid cursor still works.
	w.page("alex", "?limit=1&cursor="+url.QueryEscape(alexCursor), http.StatusOK, "")
}

// TestListMyMentions_AgentGets403: agents are not users (07 section 4.1).
func TestListMyMentions_AgentGets403(t *testing.T) {
	t.Parallel()
	w := newMentionWorld(t)
	p := w.page("lotse", "", http.StatusForbidden, "forbidden")
	if p["reason"] != "subject_kind_not_allowed" {
		t.Fatalf("reason %v", p["reason"])
	}
	w.page("", "", http.StatusUnauthorized, "")
}
