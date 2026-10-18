// SPDX-License-Identifier: Apache-2.0

package canary

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/OWNER/bothy/services/server/internal/kernel/designkeys"
)

// The status sets of the steps.
var (
	ok      = []int{http.StatusOK}
	created = []int{http.StatusCreated}
	none    = []int{http.StatusNoContent}
)

// content drives spaces, channels, forum tags, threads, messages, reactions, pins, drafts,
// templates, read states and mentions with canary text, in scopes no observer may read
// (Support/Eskalation, Research and a new space).
func (sc *Scenario) content() {
	t := sc.T
	t.Helper()
	o, alex := sc.W.Owner, sc.W.People["Alex"]
	eskalation := sc.W.Channels["Support/Eskalation"]
	research := sc.W.Spaces["Research"]
	notizen := sc.W.Channels["Research/Notizen"]

	// Spaces.
	spaceName := sc.Reg.New(ClassContent, "", "space name")
	space := id(sc.post(o, "/spaces", map[string]any{"name": spaceName, "icon": designkeys.SpaceIcons()[0], "color": designkeys.CategoryColors()[0]}, created...))
	sc.Reg.Rescope(spaceName, space)
	sc.get(o, "/spaces/"+space, ok...)
	sc.get(o, "/spaces", ok...)
	newName := sc.Reg.New(ClassContent, research, "space name (update)")
	sc.matched(o, "/spaces/"+research, req{method: http.MethodPatch, path: "/spaces/" + research, body: map[string]any{"name": newName}}, ok...)
	sc.post(o, "/spaces/"+space+"/move", map[string]any{"before_id": sc.W.Spaces["Support"]}, ok...)
	sc.get(o, "/spaces/"+research+"/access", ok...)

	// Channels.
	chName := sc.Reg.New(ClassContent, "", "channel name")
	topic := sc.Reg.New(ClassContent, "", "channel topic")
	ch := sc.post(o, "/spaces/"+space+"/channels", map[string]any{"kind": "text", "name": chName, "topic": topic}, created...)
	chID, chMain := id(ch), str(obj(ch), "main_thread_id")
	sc.Reg.Rescope(chName, chID)
	sc.Reg.Rescope(topic, chID)
	sc.get(o, "/channels/"+chID, ok...)
	sc.get(o, "/spaces/"+space+"/channels", ok...)
	sc.matched(o, "/channels/"+chID, req{method: http.MethodPatch, path: "/channels/" + chID, body: map[string]any{
		"name": sc.Reg.New(ClassContent, chID, "channel name (update)"), "topic": sc.Reg.New(ClassContent, chID, "channel topic (update)"),
	}}, ok...)
	forumName := sc.Reg.New(ClassContent, research, "forum channel name")
	forum := id(sc.post(o, "/spaces/"+research+"/channels", map[string]any{"kind": "forum", "name": forumName}, created...))
	sc.Reg.Rescope(forumName, forum)
	sc.post(o, "/channels/"+forum+"/move", map[string]any{"before_id": notizen}, ok...)
	sc.get(o, "/channels/"+eskalation+"/access", ok...)
	// Operators: Eskalation without one and back to the space's, the new space without one.
	for _, mode := range []string{"none", "inherit"} {
		sc.matched(o, "/channels/"+eskalation, req{method: http.MethodPut, path: "/channels/" + eskalation + "/operator", body: map[string]any{
			"mode": mode, "reason": sc.Reg.New(ClassAdminReason, eskalation, "channel operator reason ("+mode+")"),
		}}, ok...)
	}
	sc.matched(o, "/spaces/"+space, req{method: http.MethodPut, path: "/spaces/" + space + "/operator", body: map[string]any{
		"mode": "none", "reason": sc.Reg.New(ClassAdminReason, space, "space operator reason"),
	}}, ok...)

	// Forum tags.
	tagA := id(sc.post(o, "/channels/"+forum+"/tags", map[string]any{"name": sc.Reg.New(ClassContent, forum, "forum tag name"), "color": designkeys.CategoryColors()[1]}, created...))
	tagB := id(sc.post(o, "/channels/"+forum+"/tags", map[string]any{"name": sc.Reg.New(ClassContent, forum, "forum tag name 2")}, created...))
	sc.patch(o, "/channels/"+forum+"/tags/"+tagA, map[string]any{"name": sc.Reg.New(ClassContent, forum, "forum tag name (update)")}, ok...)
	sc.post(o, "/channels/"+forum+"/tags/"+tagB+"/move", map[string]any{"before_id": tagA}, ok...)
	sc.get(o, "/channels/"+forum+"/tags", ok...)
	post := sc.post(o, "/channels/"+forum+"/threads", map[string]any{
		"title": sc.Reg.New(ClassContent, forum, "forum post title"), "tag_ids": []string{tagA},
		"first_message": map[string]any{"client_msg_id": newID(), "content_md": sc.Reg.New(ClassContent, forum, "forum post text")},
	}, created...)
	postID := str(obj(post), "thread.id")
	sc.matched(o, "/threads/"+postID, req{method: http.MethodPut, path: "/threads/" + postID + "/tags", body: map[string]any{"tag_ids": []string{tagA, tagB}}}, ok...)
	sc.get(o, "/channels/"+forum+"/threads?sort=created", ok...)
	sc.del(o, "/channels/"+forum+"/tags/"+tagB, none...)

	// Threads and messages in Support/Eskalation (Alex has trigger on Support through G-1).
	mainEsk := str(obj(sc.get(o, "/channels/"+eskalation, ok...)), "channel.main_thread_id")
	parent := id(sc.post(alex, "/threads/"+mainEsk+"/messages", map[string]any{"client_msg_id": newID(), "content_md": sc.Reg.New(ClassContent, eskalation, "message text")}, created...))
	th := sc.post(alex, "/channels/"+eskalation+"/threads", map[string]any{
		"title": sc.Reg.New(ClassContent, eskalation, "thread title"), "parent_message_id": parent,
		"first_message": map[string]any{"client_msg_id": newID(), "content_md": sc.Reg.New(ClassContent, eskalation, "first message text")},
	}, created...)
	thread := str(obj(th), "thread.id")
	msg := sc.post(alex, "/threads/"+thread+"/messages", map[string]any{
		"client_msg_id": newID(), "content_md": sc.Reg.New(ClassContent, eskalation, "message text with mention") + " @owner",
	}, created...)
	msgID := id(msg)
	sc.matched(alex, "/messages/"+msgID, req{method: http.MethodPatch, path: "/messages/" + msgID, body: map[string]any{
		"content_md": sc.Reg.New(ClassContent, eskalation, "message edit"),
	}}, ok...)
	sc.get(alex, "/messages/"+msgID+"/revisions", ok...)
	sc.get(o, "/threads/"+thread+"/messages?limit=50", ok...)
	sc.get(o, "/threads/"+thread, ok...)
	sc.put(o, "/messages/"+msgID+"/reactions/"+url.PathEscape("👍"), nil, none...)
	sc.get(o, "/messages/"+msgID+"/reactions", ok...)
	sc.del(o, "/messages/"+msgID+"/reactions/"+url.PathEscape("👍"), none...)
	sc.put(o, "/messages/"+msgID+"/pin", nil, none...)
	sc.get(o, "/threads/"+thread+"/pins", ok...)
	sc.del(o, "/messages/"+msgID+"/pin", none...)
	sc.matched(o, "/threads/"+thread, req{method: http.MethodPut, path: "/threads/" + thread + "/title", body: map[string]any{
		"title": sc.Reg.New(ClassContent, eskalation, "thread title (update)"),
	}}, ok...)
	sc.post(o, "/threads/"+thread+"/close", nil, ok...)
	sc.post(o, "/threads/"+thread+"/reopen", nil, ok...)
	sc.get(o, "/me/mentions", ok...)
	sc.get(o, "/channels/"+eskalation+"/threads", ok...)

	// Read states.
	seq := lastSeq(sc.get(o, "/threads/"+thread+"/messages", ok...))
	sc.put(o, "/threads/"+thread+"/read-state", map[string]any{"last_read_seq": seq}, ok...)
	rs := sc.get(o, "/me/read-states?channel_id="+eskalation, ok...)
	sc.markUnread(thread, rs)

	// Drafts.
	sc.put(alex, "/threads/"+thread+"/draft", map[string]any{"content_md": sc.Reg.New(ClassContent, eskalation, "thread draft")}, ok...)
	sc.put(o, "/channels/"+forum+"/draft", map[string]any{"content_md": sc.Reg.New(ClassContent, forum, "forum channel draft")}, ok...)
	sc.get(alex, "/me/drafts", ok...)
	sc.del(alex, "/threads/"+thread+"/draft", none...)
	sc.del(o, "/channels/"+forum+"/draft", none...)

	// Templates.
	tpl := id(sc.post(o, "/channels/"+eskalation+"/templates", map[string]any{
		"name": sc.Reg.New(ClassContent, eskalation, "channel template name"), "content_md": sc.Reg.New(ClassContent, eskalation, "channel template content"),
	}, created...))
	sc.get(o, "/channels/"+eskalation+"/templates", ok...)
	sc.matchedTemplate(tpl, eskalation)
	mine := id(sc.post(alex, "/me/templates", map[string]any{
		"name": sc.Reg.New(ClassContent, "", "personal template name"), "content_md": sc.Reg.New(ClassContent, "", "personal template content"),
	}, created...))
	sc.get(alex, "/me/templates", ok...)
	sc.del(alex, "/templates/"+mine, none...)
	sc.del(o, "/templates/"+tpl, none...)

	// A thread moved between channels with a reason, the message delete (redacted event), an
	// archived channel with a reason.
	moved := str(obj(sc.post(o, "/channels/"+chID+"/threads", map[string]any{
		"title": sc.Reg.New(ClassContent, chID, "thread title to move"), "parent_message_id": id(sc.post(o, "/threads/"+chMain+"/messages",
			map[string]any{"client_msg_id": newID(), "content_md": sc.Reg.New(ClassContent, chID, "message text new space")}, created...)),
	}, created...)), "thread.id")
	ch2Name := sc.Reg.New(ClassContent, space, "channel name 2")
	ch2 := id(sc.post(o, "/spaces/"+space+"/channels", map[string]any{"kind": "text", "name": ch2Name}, created...))
	sc.Reg.Rescope(ch2Name, ch2)
	sc.post(o, "/threads/"+moved+"/move", map[string]any{"target_channel_id": ch2, "reason": sc.Reg.New(ClassAdminReason, ch2, "thread move reason")}, ok...)
	sc.del(alex, "/messages/"+msgID, none...)
	sc.post(o, "/channels/"+chID+"/archive", map[string]any{"reason": sc.Reg.New(ClassAdminReason, chID, "channel archive reason")}, ok...)
	sc.post(o, "/spaces/"+space+"/archive", map[string]any{"reason": sc.Reg.New(ClassAdminReason, space, "space archive reason")}, ok...)
	sc.threads = append(sc.threads, thread)
	sc.messages = append(sc.messages, parent)
}

// markUnread sends markThreadUnread one message before the read position of thread in rs, with
// the version of that read state as If-Match.
func (sc *Scenario) markUnread(thread string, rs harnessResponse) {
	sc.T.Helper()
	ver, pos := "", int64(0)
	for _, it := range items(rs) {
		if it["thread_id"] == thread {
			ver, pos = jsonText(it["version"]), atoi(jsonText(it["last_read_seq"]))
		}
	}
	if ver == "" || pos < 1 {
		sc.T.Errorf("no read state of the thread in %s", clip(rs.Body))
		return
	}
	sc.versioned(sc.W.Owner, req{method: http.MethodPost, path: "/threads/" + thread + "/read-state/mark-unread",
		body: map[string]any{"last_read_seq": pos - 1}, header: ifMatch(ver)}, ok...)
}

// matchedTemplate updates the template with canary text and its version as If-Match.
func (sc *Scenario) matchedTemplate(tpl, scope string) {
	sc.T.Helper()
	var ver string
	for _, it := range items(sc.get(sc.W.Owner, "/channels/"+scope+"/templates", ok...)) {
		if it["id"] == tpl {
			ver = jsonText(it["version"])
		}
	}
	sc.call(sc.W.Owner, req{method: http.MethodPatch, path: "/templates/" + tpl, header: ifMatch(ver), body: map[string]any{
		"name": sc.Reg.New(ClassContent, scope, "channel template name (update)"), "content_md": sc.Reg.New(ClassContent, scope, "channel template content (update)"),
	}}, ok...)
}

// lastSeq returns the largest member seq of the items of a message list.
func lastSeq(r harnessResponse) int64 {
	var max int64
	for _, it := range items(r) {
		if n, err := strconv.ParseInt(jsonText(it["thread_seq"]), 10, 64); err == nil && n > max {
			max = n
		}
	}
	return max
}
