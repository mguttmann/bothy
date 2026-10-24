// SPDX-License-Identifier: Apache-2.0

package bothytest_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/restclient"
	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/bothyapi"
	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/bothyapi/bothytest"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
)

// The Discord IDs of the test mapping; invented digit strings.
const (
	guildID    = "1029384000"
	categoryID = "1029384001"
	channelID  = "1029384010"
	threadID   = "1029384700"
	samID      = "1029384100"
	robinID    = "1029384101"
	botID      = "1029384102"
	hookID     = "1029384103"
)

// shaA and shaB are two content hashes.
var (
	shaA = hex.EncodeToString(bytes.Repeat([]byte{0xa1}, 32))
	shaB = hex.EncodeToString(bytes.Repeat([]byte{0xb2}, 32))
)

// importEnv is a fake with a logged-in client of the import tool.
type importEnv struct {
	fake *bothytest.Server
	c    *bothyapi.Client
	clk  *clock.Fake
}

// newImportEnv starts a fake on a fake clock and logs a client in; the login sleeper approves.
func newImportEnv(t *testing.T) *importEnv {
	t.Helper()
	clk := clock.NewFake(time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC))
	fake := bothytest.New(t, bothytest.Options{Clock: clk})
	path := filepath.Join(t.TempDir(), "bothy", "session.json")
	deps := bothyapi.Deps{Clock: clk, Sleep: func(ctx context.Context, _ time.Duration) error { fake.Approve(); return ctx.Err() }}
	if err := bothyapi.Login(t.Context(), bothyapi.LoginOptions{ServerURL: fake.URL, SessionFile: path,
		Limits: bothyapi.DefaultLimits(), Deps: deps, Stdout: &bytes.Buffer{}}); err != nil {
		t.Fatal(err)
	}
	c, err := bothyapi.New(t.Context(), bothyapi.Options{SessionFile: path, Limits: bothyapi.DefaultLimits(), Deps: deps})
	if err != nil {
		t.Fatal(err)
	}
	return &importEnv{fake: fake, c: c, clk: clk}
}

// mapping is the normalized mapping of the test source: one category, one channel, Sam mapped to
// the person sam, Robin to the agent robin-agent, bots skipped.
func mapping() map[string]any {
	return map[string]any{
		"version": 1, "guild_id": guildID,
		"defaults": map[string]any{"unmapped_human": "placeholder", "unmapped_bot": "skip", "uncategorized_space": "skip"},
		"spaces":   []any{map[string]any{"discord_category": categoryID, "target": map[string]any{"create": map[string]any{"name": "Support"}}}},
		"channels": []any{map[string]any{"discord_channel": channelID, "target": map[string]any{"create": map[string]any{"name": "tickets"}},
			"planned_operator": "none", "lead": "discord"}},
		"users": []any{
			map[string]any{"discord_user": samID, "target": "user:sam"},
			map[string]any{"discord_user": robinID, "target": "agent:robin-agent"},
		},
	}
}

// uuid parses s or fails the test.
func uuid(t *testing.T, s string) restclient.Uuid {
	t.Helper()
	var u restclient.Uuid
	if err := u.UnmarshalText([]byte(s)); err != nil {
		t.Fatalf("uuid %q: %v", s, err)
	}
	return u
}

// ptr returns a pointer to v.
func ptr[T any](v T) *T { return &v }

// obj is one ImportObject.
func obj(kind restclient.ImportObjectItemKind, key, sha string, data map[string]any) restclient.ImportObject {
	o := restclient.ImportObject{ItemKind: kind, OriginKey: key, Data: data}
	if sha != "" {
		o.ContentSha256 = ptr(sha)
	}
	return o
}

// structure is the structure batch of the test source: the space, the channel and the users.
func structure() restclient.ImportBatch {
	return restclient.ImportBatch{Items: []restclient.ImportObject{
		obj(restclient.ImportObjectItemKindSpace, "space:"+categoryID, shaA, map[string]any{"name": "Support", "sort_key": "a0"}),
		obj(restclient.ImportObjectItemKindChannel, "channel:"+channelID, shaA,
			map[string]any{"kind": "text", "name": "tickets", "sort_key": "a0", "space_origin_key": "space:" + categoryID}),
		obj(restclient.ImportObjectItemKindUser, "user:"+samID, "", map[string]any{"discord_kind": "user", "username": "sam"}),
	}}
}

// message is a message item of author with created_at.
func message(id, author string, extra map[string]any) restclient.ImportObject {
	d := map[string]any{"author_origin_key": "user:" + author, "content_md": "Hello", "created_at": "2025-03-01T08:00:00.000Z"}
	for k, v := range extra {
		d[k] = v
	}
	return obj(restclient.ImportObjectItemKindMessage, "message:"+id, shaA, d)
}

// apply posts batch b and returns the answer.
func (e *importEnv) apply(t *testing.T, src string, b restclient.ImportBatch) *restclient.ApplyImportBatchResponse {
	t.Helper()
	resp, err := e.c.ApplyImportBatchWithResponse(t.Context(), uuid(t, src), b)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

// mustApply posts b and fails unless the answer is 200.
func (e *importEnv) mustApply(t *testing.T, src string, b restclient.ImportBatch) restclient.ImportBatchResult {
	t.Helper()
	resp := e.apply(t, src, b)
	if resp.JSON200 == nil {
		t.Fatalf("applyImportBatch = %d %s", resp.StatusCode(), resp.Body)
	}
	return *resp.JSON200
}

// conflictReason fails unless resp is 409 with reason and returns nothing.
func conflictReason(t *testing.T, status int, problem *restclient.Problem, reason string) {
	t.Helper()
	if status != http.StatusConflict || problem == nil || problem.Reason == nil || *problem.Reason != reason {
		t.Fatalf("answer = %d %+v, want 409 %s", status, problem, reason)
	}
}

// running starts a running source with the people sam and robin-agent and the structure posted.
func (e *importEnv) running(t *testing.T) (string, restclient.ImportBatchResult) {
	t.Helper()
	e.fake.AddUser("sam", "Sam", true)
	e.fake.AddAgent("robin-agent", "Robin", true)
	src := e.fake.AddSource("running", mapping())
	return src, e.mustApply(t, src, structure())
}

// TestFakeBatchReplayAndChanged: a posted key answers the same target; a different hash is
// source_changed and counted under stats.server.
func TestFakeBatchReplayAndChanged(t *testing.T) {
	e := newImportEnv(t)
	src, first := e.running(t)
	if len(first.Items) != 3 || first.Items[0].State != restclient.ImportItemStatePosted || first.Items[0].TargetId == nil {
		t.Fatalf("first = %+v", first.Items)
	}
	if first.Items[2].TargetKind == nil || *first.Items[2].TargetKind != "user" {
		t.Errorf("sam target kind = %v", first.Items[2].TargetKind)
	}
	again := structure()
	again.Items[0].ContentSha256 = ptr(shaB)
	second := e.mustApply(t, src, again)
	for i := range first.Items {
		if *second.Items[i].TargetId != *first.Items[i].TargetId || second.Items[i].Id != first.Items[i].Id {
			t.Errorf("item %d target changed on replay", i)
		}
	}
	if second.Items[0].SourceChanged == nil || !*second.Items[0].SourceChanged || second.Items[1].SourceChanged != nil {
		t.Errorf("source_changed = %v, %v", second.Items[0].SourceChanged, second.Items[1].SourceChanged)
	}
	stats := e.fake.SourceStats(src)
	srv, _ := stats["server"].(map[string]any)
	if srv["source_changed"] != float64(1) {
		t.Errorf("stats.server = %v", srv)
	}
	if items := e.fake.Items(src); len(items) != 3 || items[0].OriginKey != "space:"+categoryID {
		t.Errorf("Items = %+v", items)
	}
	if b := e.fake.Batches(src); len(b) != 2 || b[0].ThreadOriginKey != "" || len(b[0].Keys) != 3 {
		t.Errorf("Batches = %+v", b)
	}
	sp, err := e.c.ListSpacesWithResponse(t.Context())
	if err != nil || sp.JSON200 == nil || len(sp.JSON200.Items) != 1 || sp.JSON200.Items[0].Space.Name != "Support" {
		t.Fatalf("listSpaces = %v %+v", err, sp)
	}
	ch, err := e.c.ListSpaceChannelsWithResponse(t.Context(), sp.JSON200.Items[0].Space.Id)
	if err != nil || ch.JSON200 == nil || len(ch.JSON200.Items) != 1 || ch.JSON200.Items[0].Channel.Id != *first.Items[1].TargetId {
		t.Fatalf("listSpaceChannels = %v %+v", err, ch)
	}
}

// TestFakeBatchRefusals: not running, thread_not_imported, author_not_imported, a wrong kind, an
// empty batch and a missing reply target; a refused batch stores nothing.
func TestFakeBatchRefusals(t *testing.T) {
	e := newImportEnv(t)
	paused := e.fake.AddSource("paused", mapping())
	resp := e.apply(t, paused, structure())
	conflictReason(t, resp.StatusCode(), resp.ApplicationproblemJSON409, "import_not_running")
	if n := len(e.fake.Items(paused)); n != 0 {
		t.Errorf("a refused batch stored %d items", n)
	}
	src, _ := e.running(t)
	thread := restclient.ImportBatch{ThreadOriginKey: ptr("thread:" + threadID), Items: []restclient.ImportObject{message("1029384701", samID, nil)}}
	resp = e.apply(t, src, thread)
	conflictReason(t, resp.StatusCode(), resp.ApplicationproblemJSON409, "thread_not_imported")
	main := restclient.ImportBatch{ThreadOriginKey: ptr("channel:" + channelID), Items: []restclient.ImportObject{
		message("1029384702", samID, nil), message("1029384703", robinID, nil)}}
	resp = e.apply(t, src, main)
	conflictReason(t, resp.StatusCode(), resp.ApplicationproblemJSON409, "author_not_imported")
	if got := e.fake.ThreadMessages(src, "channel:"+channelID); len(got) != 0 {
		t.Errorf("a refused batch appended %v", got)
	}
	wrong := restclient.ImportBatch{Items: []restclient.ImportObject{message("1029384704", samID, nil)}}
	if resp = e.apply(t, src, wrong); resp.StatusCode() != http.StatusBadRequest {
		t.Errorf("content item in a structure batch = %d", resp.StatusCode())
	}
	if resp = e.apply(t, src, restclient.ImportBatch{Items: []restclient.ImportObject{}}); resp.StatusCode() != http.StatusBadRequest {
		t.Errorf("empty batch = %d", resp.StatusCode())
	}
	reply := restclient.ImportBatch{ThreadOriginKey: ptr("channel:" + channelID), Items: []restclient.ImportObject{
		message("1029384705", samID, map[string]any{"reply_to_origin_key": "message:1029384799"})}}
	resp = e.apply(t, src, reply)
	if resp.StatusCode() != http.StatusBadRequest || resp.ApplicationproblemJSON400 == nil || resp.ApplicationproblemJSON400.Errors == nil ||
		(*resp.ApplicationproblemJSON400.Errors)[0].Pointer != "/items/0/data/reply_to_origin_key" {
		t.Errorf("missing reply target = %d %s", resp.StatusCode(), resp.Body)
	}
}

// TestFakeThreadBatchAndPins: a thread created by its first item, messages in order with a reply
// and a pin, the main thread, a skipped bot and a webhook placeholder.
func TestFakeThreadBatchAndPins(t *testing.T) {
	e := newImportEnv(t)
	src, _ := e.running(t)
	people := e.mustApply(t, src, restclient.ImportBatch{Items: []restclient.ImportObject{
		obj(restclient.ImportObjectItemKindUser, "user:"+botID, "", map[string]any{"discord_kind": "bot", "username": "helper"}),
		obj(restclient.ImportObjectItemKindUser, "user:"+hookID, "", map[string]any{"discord_kind": "webhook", "username": "hook"}),
		obj(restclient.ImportObjectItemKindUser, "user:"+robinID, "", map[string]any{"discord_kind": "user", "username": "robin"}),
	}})
	if people.Items[0].State != restclient.ImportItemStateSkipped || people.Items[0].TargetId != nil {
		t.Errorf("bot = %+v", people.Items[0])
	}
	if people.Items[1].State != restclient.ImportItemStatePosted || people.Items[1].TargetId == nil {
		t.Errorf("webhook = %+v", people.Items[1])
	}
	if people.Items[2].TargetKind == nil || *people.Items[2].TargetKind != "agent" {
		t.Errorf("robin = %+v", people.Items[2])
	}
	b := restclient.ImportBatch{ThreadOriginKey: ptr("thread:" + threadID), Items: []restclient.ImportObject{
		obj(restclient.ImportObjectItemKindThread, "thread:"+threadID, shaA, map[string]any{"channel_origin_key": "channel:" + channelID,
			"name": "Login fails", "closed": false, "created_at": "2025-03-01T08:00:00.000Z", "last_activity_at": "2025-03-01T09:00:00.000Z"}),
		message("1029384710", samID, nil),
		message("1029384711", robinID, map[string]any{"reply_to_origin_key": "message:1029384710"}),
		obj(restclient.ImportObjectItemKindPin, "pin:1029384710", "", map[string]any{"message_origin_key": "message:1029384710"}),
	}}
	out := e.mustApply(t, src, b)
	if len(out.Items) != 4 || out.Items[0].TargetKind == nil || *out.Items[0].TargetKind != "thread" {
		t.Fatalf("thread batch = %+v", out.Items)
	}
	if got := e.fake.ThreadMessages(src, "thread:"+threadID); len(got) != 2 || got[0] != "message:1029384710" || got[1] != "message:1029384711" {
		t.Errorf("ThreadMessages = %v", got)
	}
	if p := e.fake.Pins(src); len(p) != 1 || p[0] != "message:1029384710" {
		t.Errorf("Pins = %v", p)
	}
	if d := e.fake.MessageData(src, "message:1029384711"); d["reply_to_origin_key"] != "message:1029384710" {
		t.Errorf("MessageData = %v", d)
	}
	e.mustApply(t, src, restclient.ImportBatch{ThreadOriginKey: ptr("channel:" + channelID), Items: []restclient.ImportObject{
		message("1029384720", samID, nil)}})
	if got := e.fake.ThreadMessages(src, "channel:"+channelID); len(got) != 1 {
		t.Errorf("main thread = %v", got)
	}
	again := e.mustApply(t, src, b)
	if *again.Items[1].TargetId != *out.Items[1].TargetId || len(e.fake.ThreadMessages(src, "thread:"+threadID)) != 2 {
		t.Errorf("a replayed thread batch appended again")
	}
}

// TestFakeFileSlotAndUpload: a slot with a direct session, a part with a wrong and a right
// digest, the completion, the reused slot without upload and a message with the file.
func TestFakeFileSlotAndUpload(t *testing.T) {
	e := newImportEnv(t)
	e.fake.SetPartSize(4)
	src, st := e.running(t)
	data := []byte("0123456789")
	sum := sha256.Sum256(data)
	create := restclient.ImportFileCreate{OriginKey: "file:1029384799", ChannelId: *st.Items[1].TargetId, AuthorId: *st.Items[2].TargetId,
		Name: "report.pdf", SizeBytes: int64(len(data)), Sha256: hex.EncodeToString(sum[:])}
	slot, err := e.c.CreateImportFileWithResponse(t.Context(), uuid(t, src), create)
	if err != nil || slot.JSON200 == nil || slot.JSON200.Upload == nil {
		t.Fatalf("createImportFile = %v %+v", err, slot)
	}
	up := slot.JSON200.Upload
	if up.Mode != restclient.UploadSessionModeDirect || up.PartSizeBytes != 4 || len(up.Parts) != 3 || slot.JSON200.Item.State != restclient.ImportItemStatePending {
		t.Fatalf("upload = %+v", up)
	}
	again, err := e.c.CreateImportFileWithResponse(t.Context(), uuid(t, src), create)
	if err != nil || again.JSON200 == nil || again.JSON200.Upload == nil || again.JSON200.Upload.FileId != up.FileId {
		t.Fatalf("repeated slot = %v %+v", err, again)
	}
	bad, err := e.c.UploadImportFilePartWithBodyWithResponse(t.Context(), uuid(t, src), up.FileId, 1,
		&restclient.UploadImportFilePartParams{ContentDigest: digest([]byte("xxxx"))}, "application/octet-stream", bytes.NewReader(data[:4]))
	if err != nil || bad.StatusCode() != http.StatusBadRequest {
		t.Fatalf("wrong digest = %v %d", err, bad.StatusCode())
	}
	var parts []restclient.UploadPartResult
	for i := 0; i < len(data); i += 4 {
		chunk := data[i:min(i+4, len(data))]
		r, err := e.c.UploadImportFilePartWithBodyWithResponse(t.Context(), uuid(t, src), up.FileId, i/4+1,
			&restclient.UploadImportFilePartParams{ContentDigest: digest(chunk)}, "application/octet-stream", bytes.NewReader(chunk))
		if err != nil || r.JSON200 == nil || r.JSON200.Etag == "" {
			t.Fatalf("part %d = %v %+v", i/4+1, err, r)
		}
		parts = append(parts, *r.JSON200)
	}
	done, err := e.c.CompleteImportFileWithResponse(t.Context(), uuid(t, src), up.FileId, restclient.UploadComplete{Parts: parts})
	if err != nil || done.JSON200 == nil || done.JSON200.Item.State != restclient.ImportItemStatePosted {
		t.Fatalf("completeImportFile = %v %+v", err, done)
	}
	reused, err := e.c.CreateImportFileWithResponse(t.Context(), uuid(t, src), create)
	if err != nil || reused.JSON200 == nil || reused.JSON200.Upload != nil || *reused.JSON200.Item.TargetId != up.FileId {
		t.Fatalf("posted slot = %v %+v", err, reused)
	}
	files := e.fake.Files(src)
	if len(files) != 1 || !files[0].Posted || !bytes.Equal(files[0].Bytes, data) || files[0].Name != "report.pdf" {
		t.Fatalf("Files = %+v", files)
	}
	e.mustApply(t, src, restclient.ImportBatch{ThreadOriginKey: ptr("channel:" + channelID), Items: []restclient.ImportObject{
		message("1029384730", samID, map[string]any{"attachment_file_ids": []any{up.FileId.String()}})}})
}

// digest is the Content-Digest header of b.
func digest(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha-256=:" + base64.StdEncoding.EncodeToString(sum[:]) + ":"
}

// TestFakeFailAfterStore: the batch is stored, then answered 502; the retry answers the same
// target.
func TestFakeFailAfterStore(t *testing.T) {
	e := newImportEnv(t)
	e.fake.AddUser("sam", "Sam", true)
	src := e.fake.AddSource("running", mapping())
	e.fake.FailAfterStore(http.MethodPost, bothytest.APIPrefix+"/imports/", http.StatusBadGateway, 1)
	writes, posts := e.fake.Writes(), e.fake.Methods()[http.MethodPost]
	if resp := e.apply(t, src, structure()); resp.StatusCode() != http.StatusBadGateway {
		t.Fatalf("first = %d", resp.StatusCode())
	}
	stored := e.fake.Items(src)
	if len(stored) != 3 {
		t.Fatalf("stored = %+v", stored)
	}
	out := e.mustApply(t, src, structure())
	if out.Items[1].TargetId.String() != stored[1].TargetID {
		t.Errorf("retry target = %s, stored %s", out.Items[1].TargetId, stored[1].TargetID)
	}
	// A use_dpop_nonce answer and its retry are two requests, so count against the method totals.
	batches := e.fake.Count(http.MethodPost, bothytest.APIPrefix+"/imports/"+src+"/batches")
	posts = e.fake.Methods()[http.MethodPost] - posts
	if batches < 2 || posts != batches {
		t.Errorf("POST = %d, batches = %d", posts, batches)
	}
	if n := e.fake.Writes() - writes; n != posts {
		t.Errorf("Writes = %d, POST = %d", n, posts)
	}
	before := e.fake.Writes()
	if _, err := e.c.GetImportWithResponse(t.Context(), uuid(t, src)); err != nil || e.fake.Writes() != before {
		t.Errorf("a GET counted as a write: %v", err)
	}
}

// TestFakeCursorAndStats: the cursor is replaced, stats merge into stats.tool, getImport shows
// both, and a paused source is 409.
func TestFakeCursorAndStats(t *testing.T) {
	e := newImportEnv(t)
	src := e.fake.AddSource("running", mapping())
	got, err := e.c.GetImportWithResponse(t.Context(), uuid(t, src))
	if err != nil || got.JSON200 == nil || got.JSON200.Cursor == nil || len(*got.JSON200.Cursor) != 0 {
		t.Fatalf("getImport = %v %+v", err, got)
	}
	set := func(cursor, stats restclient.JsonObject) *restclient.SetImportCursorResponse {
		resp, err := e.c.SetImportCursorWithResponse(t.Context(), uuid(t, src), restclient.ImportCursorSet{Cursor: cursor, Stats: &stats})
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}
	if r := set(restclient.JsonObject{"version": 1, "channels": map[string]any{channelID: "1"}}, restclient.JsonObject{"a": 1, "b": 2}); r.JSON200 == nil {
		t.Fatalf("set = %d", r.StatusCode())
	}
	set(restclient.JsonObject{"version": 1}, restclient.JsonObject{"b": 3})
	if c := e.fake.SourceCursor(src); c["channels"] != nil || c["version"] != float64(1) {
		t.Errorf("cursor = %v", c)
	}
	tool, _ := e.fake.SourceStats(src)["tool"].(map[string]any)
	if tool["a"] != float64(1) || tool["b"] != float64(3) {
		t.Errorf("stats.tool = %v", tool)
	}
	e.fake.SetState(src, "paused")
	r := set(restclient.JsonObject{}, restclient.JsonObject{})
	conflictReason(t, r.StatusCode(), r.ApplicationproblemJSON409, "import_not_running")
	list, err := e.c.ListImportsWithResponse(t.Context(), &restclient.ListImportsParams{})
	if err != nil || list.JSON200 == nil || len(list.JSON200.Items) != 1 || list.JSON200.Items[0].State != restclient.ImportSourceStatePaused {
		t.Fatalf("listImports = %v %+v", err, list)
	}
}

// TestFakePagesAndMe: listUsers and listAgents in pages of 2, the status of a disabled person and
// getMe after SetMe.
func TestFakePagesAndMe(t *testing.T) {
	e := newImportEnv(t)
	e.fake.AddUser("sam", "Sam", true)
	e.fake.AddUser("robin", "Robin", false)
	e.fake.AddUser("sam2", "Sam", true)
	e.fake.AddAgent("robin-agent", "Robin", false)
	var users []restclient.User
	var cursor *string
	pages := 0
	for {
		resp, err := e.c.ListUsersWithResponse(t.Context(), &restclient.ListUsersParams{Limit: ptr(2), Cursor: cursor})
		if err != nil || resp.JSON200 == nil {
			t.Fatalf("listUsers = %v %+v", err, resp)
		}
		pages++
		users = append(users, resp.JSON200.Items...)
		if resp.JSON200.NextCursor == nil {
			break
		}
		cursor = resp.JSON200.NextCursor
	}
	// The person of the session (alex) comes first, then the added ones.
	if pages != 2 || len(users) != 4 || users[0].Handle != "alex" || users[2].Handle != "robin" || users[2].Status != restclient.UserStatusDisabled {
		t.Errorf("users in %d pages = %+v", pages, users)
	}
	agents, err := e.c.ListAgentsWithResponse(t.Context(), &restclient.ListAgentsParams{Limit: ptr(2)})
	if err != nil || agents.JSON200 == nil || len(agents.JSON200.Items) != 1 || agents.JSON200.Items[0].Status != restclient.AgentStatusDisabled {
		t.Fatalf("listAgents = %v %+v", err, agents)
	}
	e.fake.SetMe(false, false)
	me, err := e.c.Me(t.Context())
	if err != nil || me.IsOwner {
		t.Fatalf("Me = %v %+v", err, me)
	}
	first, err := e.c.ListUsersWithResponse(t.Context(), &restclient.ListUsersParams{Limit: ptr(1)})
	if err != nil || first.JSON200 == nil || first.JSON200.Items[0].Status != restclient.UserStatusDisabled {
		t.Fatalf("the person of the session = %v %+v", err, first)
	}
}
