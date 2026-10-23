// SPDX-License-Identifier: Apache-2.0

package canary

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/url"

	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// search drives the full-text search and the prefix search of the quick switcher with a canary
// search term (SEC-T10, M3-14 design note 14): in Support/Eskalation a message, the title of a
// thread and the name of an attached file carry the term, and Alex searches it with every type
// and with searchThreadTitles. The term may reach the answers of readers of Eskalation only and
// never a log, span, metric, audit row or problem body.
func (sc *Scenario) search() {
	sc.T.Helper()
	o, alex := sc.W.Owner, sc.W.People["Alex"]
	esk := sc.W.Channels["Support/Eskalation"]
	term := sc.Reg.New(ClassContent, esk, "search term")
	main := str(obj(sc.get(o, "/channels/"+esk, ok...)), "channel.main_thread_id")
	parent := id(sc.post(alex, "/threads/"+main+"/messages", map[string]any{"client_msg_id": newID(), "content_md": "search " + term}, created...))
	thread := str(obj(sc.post(alex, "/channels/"+esk+"/threads", map[string]any{
		"title": term + " title", "parent_message_id": parent,
		"first_message": map[string]any{"client_msg_id": newID(), "content_md": "first " + term},
	}, created...)), "thread.id")
	file := sc.searchFile(alex, esk, term+".txt")
	sc.post(alex, "/threads/"+thread+"/messages", map[string]any{
		"client_msg_id": newID(), "content_md": "attached " + term, "attachment_file_ids": []string{file},
	}, created...)
	for _, typ := range []string{"messages", "threads", "files"} {
		sc.get(alex, "/search?"+url.Values{"q": {term}, "type": {typ}}.Encode(), ok...)
	}
	sc.get(alex, "/search?"+url.Values{"q": {term}, "sort": {"newest"}, "limit": {"1"}}.Encode(), ok...)
	sc.get(alex, "/search/thread-titles?"+url.Values{"q": {term}}.Encode(), ok...)
	// A too long query with the term answers a problem that must not carry it.
	long := term
	for len(long) <= 256 {
		long += " " + term
	}
	sc.get(alex, "/search?"+url.Values{"q": {long}}.Encode(), http.StatusBadRequest)
}

// searchFile uploads a small file named name for channel as c and waits until it is final, so a
// message can attach it.
func (sc *Scenario) searchFile(c *harness.Client, channel, name string) string {
	sc.T.Helper()
	body := []byte("search canary")
	sum := sha256.Sum256(body)
	file := str(obj(sc.post(c, "/channels/"+channel+"/uploads", map[string]any{
		"name": name, "size_bytes": len(body), "sha256": hex.EncodeToString(sum[:]),
	}, created...)), "file_id")
	h := header("Content-Type", "application/octet-stream", "Content-Digest", "sha-256=:"+base64.StdEncoding.EncodeToString(sum[:])+":")
	etag := str(obj(sc.call(c, req{method: http.MethodPut, path: "/uploads/" + file + "/parts/1", body: body, header: h}, ok...)), "etag")
	sc.post(c, "/uploads/"+file+"/complete", map[string]any{"parts": []map[string]any{{"part_number": 1, "etag": etag}}}, ok...)
	testkit.Until(sc.T, "the scan of the search file", func() bool {
		return str(obj(sc.get(c, "/files/"+file, ok...)), "scan_status") != "pending"
	})
	return file
}
