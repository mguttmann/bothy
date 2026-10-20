// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	messagingapp "github.com/OWNER/bothy/services/server/internal/messaging/app"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	mw "github.com/OWNER/bothy/services/server/test/messagingworld"
)

// Symbols of the reaction tests.
const (
	thumbsUp = "\U0001F44D"
	party    = "\U0001F389"
	heart    = "\u2764\uFE0F"
)

// reactionPath is the path of addReaction and removeReaction for the message id and emoji.
func reactionPath(id, emoji string) string {
	return messagePath(id) + "/reactions/" + url.PathEscape(emoji)
}

// react sends addReaction as name.
func react(w *mw.World, name, id, emoji string) *httpservertest.Response {
	return w.Do(name, http.MethodPut, reactionPath(id, emoji), "")
}

// unreact sends removeReaction as name.
func unreact(w *mw.World, name, id, emoji string) *httpservertest.Response {
	return w.Do(name, http.MethodDelete, reactionPath(id, emoji), "")
}

// worldWith is world with the messaging options opts.
func worldWith(t *testing.T, opts ...mw.Option) (w *mw.World, space, ch, main ids.ID, alex ids.ID, msg sent) {
	t.Helper()
	w = mw.New(t, subscribe, opts...)
	space, ch, main = w.TextChannel()
	alex = w.Person("alex", false)
	w.Grant("user", alex, space, "write")
	msg = post(w, "alex", main, "first version")
	return w, space, ch, main, alex, msg
}

// eventCounts returns the count of each event of typ, in seq order.
func eventCounts(w *mw.World, typ string) []int {
	var out []int
	for _, e := range w.Events(typ) {
		out = append(out, int(e.Payload["count"].(float64)))
	}
	return out
}

// TestAddReaction_IdempotentSingleEvent: two identical calls give 204 twice, one row and one
// reaction.added with count 1 and the payload ReactionPayload.
func TestAddReaction_IdempotentSingleEvent(t *testing.T) {
	t.Parallel()
	w, _, _, main, alex, msg := world(t)
	w.Want(react(w, "alex", msg.id, thumbsUp), http.StatusNoContent, "")
	w.Want(react(w, "alex", msg.id, thumbsUp), http.StatusNoContent, "")
	if n := w.Count(`SELECT count(*)::int FROM reactions WHERE message_id = $1`, msg.id); n != 1 {
		t.Fatalf("rows %d", n)
	}
	evs := w.Events("reaction.added")
	if len(evs) != 1 {
		t.Fatalf("events %+v", evs)
	}
	p := evs[0].Payload
	if p["message_id"] != msg.id || p["thread_id"] != main.String() || p["emoji"] != thumbsUp || p["principal_id"] != alex.String() ||
		p["count"] != float64(1) {
		t.Fatalf("payload %v", p)
	}
	validate(t, "ReactionPayload", evs[0].Raw)
}

// TestAddReaction_CountsPerSymbol: Alex and the owner react with the same symbol (counts 1 and
// 2); another symbol has its own count.
func TestAddReaction_CountsPerSymbol(t *testing.T) {
	t.Parallel()
	w, _, _, _, _, msg := world(t)
	w.Want(react(w, "alex", msg.id, thumbsUp), http.StatusNoContent, "")
	w.Want(react(w, "owner", msg.id, thumbsUp), http.StatusNoContent, "")
	w.Want(react(w, "owner", msg.id, party), http.StatusNoContent, "")
	if got := eventCounts(w, "reaction.added"); !slices.Equal(got, []int{1, 2, 1}) {
		t.Fatalf("counts %v", got)
	}
}

// TestAddReaction_ConcurrentCountsExact: parallel adds of one symbol by several people give the
// counts 1 to n exactly once each.
func TestAddReaction_ConcurrentCountsExact(t *testing.T) {
	t.Parallel()
	w, space, _, _, _, msg := world(t)
	const n = 6
	names := make([]string, n)
	for i := range names {
		names[i] = "person" + string(rune('a'+i))
		w.Grant("user", w.Person(names[i], false), space, "write")
	}
	var wg sync.WaitGroup
	gate := make(chan struct{})
	codes := make([]int, n)
	for i, name := range names {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-gate
			codes[i] = w.H.Do(w.H.NewRequest(http.MethodPut, reactionPath(msg.id, thumbsUp), "", "Authorization", "Bearer "+name)).StatusCode
		}()
	}
	close(gate)
	wg.Wait()
	for _, c := range codes {
		if c != http.StatusNoContent {
			t.Fatalf("statuses %v", codes)
		}
	}
	got := eventCounts(w, "reaction.added")
	if !slices.Equal(got, []int{1, 2, 3, 4, 5, 6}) {
		t.Fatalf("counts in seq order %v", got)
	}
}

// triggerColumns reads trigger_state, its time and reason and the version of the message id.
func triggerColumns(t *testing.T, w *mw.World, id string) string {
	t.Helper()
	var s string
	if err := w.Owner.QueryRow(t.Context(), `SELECT concat_ws('|', trigger_state, trigger_state_at::text, trigger_state_reason, version::text)
		FROM messages WHERE id = $1`, id).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

// TestAddReaction_IsNotStatus: on a message with trigger_state set, adding and removing a reaction
// leaves trigger_state, its time and reason and the version unchanged; a change of
// trigger_state creates no reaction row (docs/DECISIONS.md M1-59/2: the fixture sets it, because
// the port SetTriggerState is not on the base).
func TestAddReaction_IsNotStatus(t *testing.T) {
	t.Parallel()
	w, _, _, _, _, msg := world(t)
	w.SQL(`UPDATE messages SET trigger_state = 'failed', trigger_state_reason = 'agent_error', trigger_state_at = now() WHERE id = $1`, msg.id)
	before := triggerColumns(t, w, msg.id)
	w.Want(react(w, "alex", msg.id, thumbsUp), http.StatusNoContent, "")
	if got := triggerColumns(t, w, msg.id); got != before {
		t.Fatalf("after add %q, before %q", got, before)
	}
	w.Want(unreact(w, "alex", msg.id, thumbsUp), http.StatusNoContent, "")
	if got := triggerColumns(t, w, msg.id); got != before {
		t.Fatalf("after remove %q, before %q", got, before)
	}
	w.SQL(`UPDATE messages SET trigger_state = 'done', trigger_state_reason = NULL, trigger_state_at = now() WHERE id = $1`, msg.id)
	if n := w.Count(`SELECT count(*)::int FROM reactions WHERE message_id = $1`, msg.id); n != 0 {
		t.Fatalf("reaction rows %d", n)
	}
}

// TestAddReaction_Rights: read only is 403 forbidden; Alex on a Marketing message is the 404 of a
// random id; a read-only channel is 403 channel_read_only; the operator agent of the channel
// reacts.
func TestAddReaction_Rights(t *testing.T) {
	t.Parallel()
	w, space, ch, _, _, msg := world(t)
	reader := w.Person("reader", false)
	w.Grant("user", reader, space, "read")
	w.Want(react(w, "reader", msg.id, thumbsUp), http.StatusForbidden, "forbidden")
	marketing := w.MainThread(w.Channel(w.Space("Marketing"), "text", "general"))
	foreign := post(w, "owner", marketing, "marketing plan")
	mw.Same404(t, react(w, "alex", foreign.id, thumbsUp), react(w, "alex", ids.New().String(), thumbsUp))
	op := w.Agent("operator-agent")
	w.SetOperator(ch, op)
	w.Want(react(w, "operator-agent", msg.id, party), http.StatusNoContent, "")
	w.SQL(`UPDATE channels SET read_only = true, version = version + 1 WHERE id = $1`, ch.String())
	w.Want(react(w, "alex", msg.id, thumbsUp), http.StatusForbidden, "channel_read_only")
	w.Want(unreact(w, "operator-agent", msg.id, party), http.StatusForbidden, "channel_read_only")
}

// TestAddReaction_DeletedMessage410: a tombstone of deleteMessage refuses a reaction with 410;
// removing one is 204 without an event.
func TestAddReaction_DeletedMessage410(t *testing.T) {
	t.Parallel()
	w, _, _, _, _, msg := world(t)
	w.Want(del(w, "alex", msg.id), http.StatusNoContent, "")
	w.Want(react(w, "alex", msg.id, thumbsUp), http.StatusGone, "message_deleted")
	w.Want(unreact(w, "alex", msg.id, thumbsUp), http.StatusNoContent, "")
	if evs := w.Events("reaction.added", "reaction.removed"); len(evs) != 0 {
		t.Fatalf("events %+v", evs)
	}
}

// TestReactionAndPinLimits: with two distinct symbols and one pin per thread, a third symbol and a
// second pin are 400 validation_failed with limit_reached; an existing symbol is still added by
// another person.
func TestReactionAndPinLimits(t *testing.T) {
	t.Parallel()
	w, _, _, main, _, msg := worldWith(t, func(c *messagingapp.Config) { c.MaxDistinctReactions, c.MaxPinsPerThread = 2, 1 })
	w.Want(react(w, "alex", msg.id, thumbsUp), http.StatusNoContent, "")
	w.Want(react(w, "alex", msg.id, party), http.StatusNoContent, "")
	p := w.Want(react(w, "alex", msg.id, heart), http.StatusBadRequest, "validation_failed")
	if ptr, code := mw.Field(p); ptr != "#path/emoji" || code != "limit_reached" {
		t.Fatalf("field %s %s", ptr, code)
	}
	w.Want(react(w, "owner", msg.id, party), http.StatusNoContent, "")
	second := post(w, "alex", main, "second")
	w.Want(pin(w, "owner", msg.id), http.StatusNoContent, "")
	p = w.Want(pin(w, "owner", second.id), http.StatusBadRequest, "validation_failed")
	if ptr, code := mw.Field(p); ptr != "" || code != "limit_reached" {
		t.Fatalf("pin field %q %s", ptr, code)
	}
	w.Want(pin(w, "owner", msg.id), http.StatusNoContent, "")
}

// TestAddReaction_InvalidSymbol400: a symbol that is not one grapheme cluster is 400 with pointer
// #path/emoji, also on an unknown message (the check runs before any lookup).
func TestAddReaction_InvalidSymbol400(t *testing.T) {
	t.Parallel()
	w, _, _, _, _, msg := world(t)
	for _, id := range []string{msg.id, ids.New().String()} {
		for _, sym := range []string{"ab", thumbsUp + thumbsUp, "\u202E"} {
			for _, r := range []*httpservertest.Response{react(w, "alex", id, sym), unreact(w, "alex", id, sym)} {
				p := w.Want(r, http.StatusBadRequest, "validation_failed")
				if ptr, code := mw.Field(p); ptr != "#path/emoji" || code != "invalid_emoji" {
					t.Fatalf("%q: field %s %s", sym, ptr, code)
				}
			}
		}
	}
	if n := w.Count(`SELECT count(*)::int FROM reactions`); n != 0 {
		t.Fatalf("rows %d", n)
	}
}

// TestRemoveReaction_OwnOnlyAndIdempotent: removing deletes only the caller's row; the event
// carries the count after the removal; a second call is 204 without an event.
func TestRemoveReaction_OwnOnlyAndIdempotent(t *testing.T) {
	t.Parallel()
	w, _, _, _, alex, msg := world(t)
	w.Want(react(w, "alex", msg.id, thumbsUp), http.StatusNoContent, "")
	w.Want(react(w, "owner", msg.id, thumbsUp), http.StatusNoContent, "")
	w.Want(unreact(w, "alex", msg.id, thumbsUp), http.StatusNoContent, "")
	w.Want(unreact(w, "alex", msg.id, thumbsUp), http.StatusNoContent, "")
	if n := w.Count(`SELECT count(*)::int FROM reactions WHERE message_id = $1 AND principal_id = $2`, msg.id, w.OwnerID.String()); n != 1 {
		t.Fatalf("owner rows %d", n)
	}
	if n := w.Count(`SELECT count(*)::int FROM reactions WHERE message_id = $1 AND principal_id = $2`, msg.id, alex.String()); n != 0 {
		t.Fatalf("alex rows %d", n)
	}
	evs := w.Events("reaction.removed")
	if len(evs) != 1 || evs[0].Payload["count"] != float64(1) || evs[0].Payload["principal_id"] != alex.String() {
		t.Fatalf("events %+v", evs)
	}
	validate(t, "ReactionPayload", evs[0].Raw)
}

// listReactions requests listReactions of id with query as name.
func listReactions(w *mw.World, name, id, query string) *httpservertest.Response {
	path := messagePath(id) + "/reactions"
	if query != "" {
		path += "?" + query
	}
	return w.Do(name, http.MethodGet, path, "")
}

// reactionItems renders the items of a ReactionPage as emoji/handle pairs.
func reactionItems(body map[string]any) []string {
	var out []string
	for _, it := range body["items"].([]any) {
		m := it.(map[string]any)
		out = append(out, m["emoji"].(string)+"/"+m["principal"].(map[string]any)["handle"].(string))
	}
	return out
}

// TestListReactions_PagingFilterAndPrincipals: keyset order by creation, principal and symbol;
// the emoji filter; the PrincipalRef fields; a cursor of another filter is cursor_invalid; an
// invalid filter is 400; an invisible message 404.
func TestListReactions_PagingFilterAndPrincipals(t *testing.T) {
	t.Parallel()
	w, _, _, _, _, msg := world(t)
	w.Want(react(w, "alex", msg.id, thumbsUp), http.StatusNoContent, "")
	w.Clock.Advance(time.Second)
	w.Want(react(w, "owner", msg.id, party), http.StatusNoContent, "")
	w.Want(react(w, "owner", msg.id, thumbsUp), http.StatusNoContent, "")
	w.Clock.Advance(time.Second)
	w.Want(react(w, "alex", msg.id, party), http.StatusNoContent, "")
	// The owner's two reactions share a time: the symbol decides, bytewise.
	ownerPair := []string{party + "/owner", thumbsUp + "/owner"}
	if thumbsUp < party {
		ownerPair = []string{thumbsUp + "/owner", party + "/owner"}
	}
	want := append(append([]string{thumbsUp + "/alex"}, ownerPair...), party+"/alex")
	var got []string
	cursor := ""
	for range 3 {
		q := "limit=3"
		if cursor != "" {
			q += "&cursor=" + url.QueryEscape(cursor)
		}
		body := w.Want(listReactions(w, "alex", msg.id, q), http.StatusOK, "")
		got = append(got, reactionItems(body)...)
		c, ok := body["next_cursor"].(string)
		if !ok {
			break
		}
		cursor = c
	}
	if !slices.Equal(got, want) {
		t.Fatalf("pages %v, want %v", got, want)
	}
	body := w.Want(listReactions(w, "owner", msg.id, "emoji="+url.QueryEscape(party)+"&limit=1"), http.StatusOK, "")
	if items := reactionItems(body); !slices.Equal(items, []string{party + "/owner"}) {
		t.Fatalf("filtered %v", items)
	}
	first := body["items"].([]any)[0].(map[string]any)
	pr := first["principal"].(map[string]any)
	if pr["id"] != w.OwnerID.String() || pr["kind"] != "user" || pr["display_name"] != "owner" || first["created_at"] == nil {
		t.Fatalf("item %v", first)
	}
	filtered := body["next_cursor"].(string)
	w.Want(listReactions(w, "owner", msg.id, "cursor="+url.QueryEscape(filtered)), http.StatusBadRequest, "cursor_invalid")
	p := w.Want(listReactions(w, "alex", msg.id, "emoji=ab"), http.StatusBadRequest, "validation_failed")
	if ptr, code := mw.Field(p); ptr != "#query/emoji" || code != "invalid_emoji" {
		t.Fatalf("filter field %s %s", ptr, code)
	}
	marketing := w.MainThread(w.Channel(w.Space("Marketing"), "text", "general"))
	foreign := post(w, "owner", marketing, "marketing plan")
	mw.Same404(t, listReactions(w, "alex", foreign.id, ""), listReactions(w, "alex", ids.New().String(), ""))
	w.Want(del(w, "alex", msg.id), http.StatusNoContent, "")
	if items := w.Want(listReactions(w, "alex", msg.id, ""), http.StatusOK, "")["items"].([]any); len(items) != 0 {
		t.Fatalf("deleted message reactions %v", items)
	}
}

// TestMessageRepresentation_ReactionsAndPinned: getMessage, listThreadMessages and the 200 of an
// edit show the summary, my_reactions and pinned; createMessage shows them empty; the payloads of
// message.created and message.updated never carry my_reactions.
func TestMessageRepresentation_ReactionsAndPinned(t *testing.T) {
	t.Parallel()
	w, _, _, main, _, msg := world(t)
	created := w.Send("alex", main, "fresh", http.StatusCreated)
	if mw.JSONList(created["my_reactions"]) != "[]" || len(created["reactions"].([]any)) != 0 || created["pinned"] != false {
		t.Fatalf("created %v", created)
	}
	w.Want(react(w, "alex", msg.id, thumbsUp), http.StatusNoContent, "")
	w.Clock.Advance(time.Second)
	w.Want(react(w, "owner", msg.id, party), http.StatusNoContent, "")
	w.Want(react(w, "owner", msg.id, thumbsUp), http.StatusNoContent, "")
	w.Want(pin(w, "owner", msg.id), http.StatusNoContent, "")
	summary := `[{"count":2,"emoji":"` + thumbsUp + `"},{"count":1,"emoji":"` + party + `"}]`
	check := func(label string, m map[string]any, mine string) {
		t.Helper()
		if got := jsonOf(m["reactions"]); got != summary || mw.JSONList(m["my_reactions"]) != mine || m["pinned"] != true {
			t.Fatalf("%s: reactions %s my %v pinned %v", label, got, m["my_reactions"], m["pinned"])
		}
	}
	check("get alex", w.Want(w.Do("alex", http.MethodGet, messagePath(msg.id), ""), http.StatusOK, ""), "["+thumbsUp+"]")
	check("get owner", w.Want(w.Do("owner", http.MethodGet, messagePath(msg.id), ""), http.StatusOK, ""), "["+thumbsUp+","+party+"]")
	items := w.Want(w.Do("alex", http.MethodGet, mw.MessagesPath(main), ""), http.StatusOK, "")["items"].([]any)
	check("history", items[0].(map[string]any), "["+thumbsUp+"]")
	check("edit", w.Want(patch(w, "alex", msg.id, v(msg.version), content("second version")), http.StatusOK, ""), "["+thumbsUp+"]")
	for _, e := range w.Events("message.created", "message.updated") {
		if strings.Contains(e.Raw, "my_reactions") {
			t.Fatalf("%s payload with my_reactions: %s", e.Type, e.Raw)
		}
	}
	updated := w.Events("message.updated")
	if len(updated) != 1 {
		t.Fatalf("updated %d", len(updated))
	}
	m := updated[0].Payload["message"].(map[string]any)
	if jsonOf(m["reactions"]) != summary || m["pinned"] != true {
		t.Fatalf("message.updated %s", updated[0].Raw)
	}
}

// TestReactionPinEvents_Envelope: the four events have resource kind message, the receive action
// of 05 section 6, space, channel and thread, the caller as actor and no resource version, because
// they change no version of the message (04 section 9 item 3, actions.yaml resource_version false;
// docs/DECISIONS.md M1-59/1).
func TestReactionPinEvents_Envelope(t *testing.T) {
	t.Parallel()
	w, space, ch, main, alex, msg := world(t)
	w.Want(react(w, "alex", msg.id, thumbsUp), http.StatusNoContent, "")
	w.Want(unreact(w, "alex", msg.id, thumbsUp), http.StatusNoContent, "")
	w.Want(pin(w, "owner", msg.id), http.StatusNoContent, "")
	w.Want(unpin(w, "owner", msg.id), http.StatusNoContent, "")
	want := map[string]string{
		"reaction.added": "message.read|" + alex.String(), "reaction.removed": "message.read|" + alex.String(),
		"pin.added": "thread.read|" + w.OwnerID.String(), "pin.removed": "thread.read|" + w.OwnerID.String(),
	}
	for typ, rest := range want {
		n := w.Count(`SELECT count(*)::int FROM event_log WHERE type = $1 AND resource_kind = 'message' AND resource_id = $2
			AND resource_version IS NULL AND space_id = $3 AND channel_id = $4 AND thread_id = $5 AND receive_action || '|' || actor_principal_id = $6`,
			typ, msg.id, space.String(), ch.String(), main.String(), rest)
		if n != 1 {
			t.Errorf("%s: %d matching rows", typ, n)
		}
	}
}
