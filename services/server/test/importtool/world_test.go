// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool"
	importapi "github.com/OWNER/bothy/services/server/cmd/bothy/importtool/bothyapi"
	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/discord"
	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/discord/discordtest"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// Discord IDs of the fixture (invented digit strings): the guild, categories, channels and
// threads of guild_basic.json, the messages this file adds and the authors.
const (
	guildID      = "13000001"
	catProjects  = "13000101"
	catCommunity = "13000102"
	chGeneral    = "13000103" // text channel, category Projects
	chNews       = "13000104" // announcement channel, category Community
	chSupport    = "13000105" // forum, category Projects
	chRandom     = "13000107" // text channel without category, lead `bothy`, planned operator Lotse
	thRelease    = "13001001" // active thread of general
	thLogin      = "13000302" // active forum post of support (tag open)
	thExport     = "13000321" // archived forum post of support (tag done)
	userAlex     = "13000011"
	userSam      = "13000012"
	userRobin    = "13000013"

	msgSmallFile = "13001004" // message of general with an attachment within the limit
	msgBigFile   = "13001013" // message of general with an attachment above the limit
	msgPinned    = "13001001" // pinned message of general (also the starter of thRelease)
	msgSameReply = "13002004" // reply in thRelease to a message of thRelease
	msgCrossRepl = "13002003" // reply in thRelease to a message of the main thread of general
	msgReplied   = "13002002" // the message of thRelease msgSameReply answers
	msgForumPin  = "13003002" // pinned message of thLogin

	attSmall = "13005001"
	attBig   = "13005002"
)

// archivedThreads are the archived threads of general in guild_basic.json (all done).
var archivedThreads = []string{"13000311", "13000312", "13000313", "13000314"}

// Sizes of the two attachments around the limit import.attachment_max_bytes of toolEnviron.
const (
	smallFileBytes     = 1234
	bigFileBytes       = 5000
	attachmentMaxBytes = "2000"
)

// Tool values of every run (22 19): small batches so a thread takes several, and short lease
// windows so a test can pass import.lease_stale_after on the fake clock with few steps.
const (
	batchMaxItems   = "4"
	pollInterval    = 500 * time.Millisecond
	leaseStaleAfter = 2 * time.Second
	maxRateWait     = 10 * time.Second
)

// The counts the fixture yields (22 5, 6.1): what TestApplyImportsFixture and the resume tests
// expect in the database after a complete import.
const (
	// wantMessages: general 9 (of 13: types 6 and 18 skipped, the two messages of the unmapped bot
	// skipped by defaults.unmapped_bot), thRelease 3 (the type 21 starter is in the main thread),
	// four archived threads with 1 each, thLogin 2, thExport 1, news 1, random 1.
	wantMessages = 21
	// wantThreadsOpen: thRelease and thLogin; wantThreadsDone: four archived threads and thExport.
	wantThreadsOpen = 2
	wantThreadsDone = 5
	// wantUsers: Alex and Robin mapped, Sam (unmapped person) and the webhook as placeholders.
	wantUsers        = 4
	wantPlaceholders = 2
	wantPins         = 2
)

// wantItems are the posted import items per kind after a complete import.
var wantItems = map[string]int{"space": 2, "channel": 4, "forum_tag": 2, "thread": wantThreadsOpen + wantThreadsDone,
	"message": wantMessages, "user": wantUsers, "pin": wantPins, "file": 1}

// world is one harness server with Robin (owner) and Alex (person) and the agent Lotse, the
// fake Discord and the fake CDN with the fixture of this package, and the fake clock of the tool.
type world struct {
	t     *testing.T
	ctx   context.Context
	s     *harness.Server
	robin *harness.Client
	alex  *harness.Client
	lotse *harness.Client
	cdn   *discordtest.CDN
	fake  *discordtest.Fake
	clk   *clock.Fake
}

// newWorld starts a server, seeds the people and starts the fakes. The fake clock is the time of
// the run lease only (Env.Clock); the server client signs its DPoP proofs with the system clock,
// which the server checks against its own, so a slow run never ages a proof.
func newWorld(t *testing.T) *world {
	t.Helper()
	return newWorldWith(t, harness.Options{EgressDenyAll: true, Config: harness.RaisedRateLimits()})
}

// newWorldWith is newWorld with the harness options opts (the sessions tests read the server log at
// debug level).
func newWorldWith(t *testing.T, opts harness.Options) *world {
	t.Helper()
	s := harness.Start(t, opts)
	w := &world{t: t, ctx: t.Context(), s: s, clk: clock.NewFake(clock.System().Now())}
	w.robin = s.NewUser(t, harness.UserSpec{Handle: "robin", DisplayName: "Robin", Email: "robin@example.com"})
	harness.MustDo(t, s.Owner, harness.Request{Method: http.MethodPost, Path: "/owners",
		Body: map[string]any{"user_id": w.robin.PrincipalID, "reason": "import owner"}}, http.StatusCreated)
	w.alex = s.NewUser(t, harness.UserSpec{Handle: "alex", DisplayName: "Alex", Email: "alex@example.com"})
	w.lotse = s.NewAgent(t, harness.AgentSpec{Handle: "lotse", DisplayName: "Lotse", BillingMode: "organization"})
	w.cdn = discordtest.NewCDN(t)
	w.cdn.AddFile(filePath(attSmall), discordtest.CDNFile{Size: smallFileBytes})
	w.cdn.AddFile(filePath(attBig), discordtest.CDNFile{Size: bigFileBytes})
	w.fake = discordtest.New(t, w.fixture())
	return w
}

// must sends one request as c with the step-up retry of the harness and fails unless want.
func (w *world) must(c *harness.Client, method, path string, body any, h http.Header, want int) harness.Response {
	w.t.Helper()
	return harness.MustDo(w.t, c, harness.Request{Method: method, Path: path, Body: body, Header: h}, want)
}

// filePath is the CDN path of an attachment of the general channel.
func filePath(attachment string) string {
	return "/attachments/" + chGeneral + "/" + attachment + "/file"
}

// author is the raw Discord user object of a fixture author.
func author(id, username, global string) map[string]any {
	return map[string]any{"id": id, "username": username, "global_name": global, "bot": false}
}

// authors of the messages this file adds.
var (
	alex  = author(userAlex, "alex", "Alex")
	sam   = author(userSam, "sam", "Sam")
	robin = author(userRobin, "robin", "Robin")
)

// message returns a raw Discord message of type typ by who in channel at the given time, with the
// members of extra added.
func message(id, channel string, typ int, who map[string]any, content, at string, extra map[string]any) json.RawMessage {
	m := map[string]any{"id": id, "channel_id": channel, "type": typ, "author": who, "content": content,
		"timestamp": at, "pinned": false, "mentions": []any{}}
	for k, v := range extra {
		m[k] = v
	}
	b, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return b
}

// replyTo is the message_reference of a reply to message in channel.
func replyTo(channel, msg string) map[string]any {
	return map[string]any{"message_reference": map[string]any{"type": 0, "message_id": msg, "channel_id": channel, "guild_id": guildID}}
}

// fixture is guild_basic.json with the messages an import needs: the attachment of general served
// by the fake CDN, a message whose attachment is above the limit, replies in the same thread and
// across threads, a pinned forum message and messages for every thread, forum post, news and
// random.
func (w *world) fixture() *discordtest.Fixture {
	f := discordtest.BasicFixture(w.t)
	signed := "ex=6a6b6c6d&is=6a6a6a6a&hm=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	general := f.Messages[chGeneral]
	for i, raw := range general {
		var m struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(raw, &m); err != nil {
			w.t.Fatal(err)
		}
		if m.ID == msgSmallFile {
			general[i] = discordtest.AttachmentMessage(msgSmallFile, chGeneral, attSmall, "notes.txt", smallFileBytes,
				w.cdn.URL(discordtest.CDNHost, filePath(attSmall), signed))
		}
	}
	f.Messages[chGeneral] = append(general, discordtest.AttachmentMessage(msgBigFile, chGeneral, attBig, "recording.bin", bigFileBytes,
		w.cdn.URL(discordtest.CDNHost, filePath(attBig), signed)))
	f.Messages[thRelease] = append(f.Messages[thRelease],
		message(msgCrossRepl, thRelease, 19, alex, "Agreed, Monday it is.", "2026-03-01T10:05:00.000000+00:00", replyTo(chGeneral, "13001012")),
		message(msgSameReply, thRelease, 19, robin, "Thank you, Sam.", "2026-03-01T10:10:00.000000+00:00", replyTo(thRelease, msgReplied)))
	for i, id := range archivedThreads {
		who := []map[string]any{alex, sam}[i%2]
		f.Messages[id] = []json.RawMessage{message("1300401"+strconv.Itoa(i+1), id, 0, who, "An old idea, part "+strconv.Itoa(i+1),
			"2026-03-0"+strconv.Itoa(i+1)+"T08:00:00.000000+00:00", nil)}
	}
	f.Messages[thLogin] = []json.RawMessage{
		message(thLogin, thLogin, 0, sam, "The second device cannot sign in.", "2026-03-09T11:00:00.000000+00:00", nil),
		message(msgForumPin, thLogin, 0, alex, "Clear the app data and pair again.", "2026-03-09T11:30:00.000000+00:00",
			map[string]any{"pinned": true}),
	}
	f.Messages[thExport] = []json.RawMessage{
		message(thExport, thExport, 0, robin, "Can we export the tickets?", "2026-02-20T08:00:00.000000+00:00", nil),
	}
	f.Messages[chNews] = []json.RawMessage{
		message("13001101", chNews, 0, robin, "Version 1.2.0 is out.", "2026-03-02T08:00:00.000000+00:00", nil),
	}
	f.Messages[chRandom] = []json.RawMessage{
		message("13001201", chRandom, 0, alex, "@lotse please look at the build", "2026-03-03T08:00:00.000000+00:00", nil),
	}
	return f
}

// mapping is the mapping of import_source.discord.v1 Robin stores and the tool reads: both
// categories to new spaces, every text channel and the forum to new channels, random (without
// category) into Projects with lead `bothy` and the planned operator Lotse, Alex and Robin mapped,
// the unmapped person a placeholder, unmapped bots skipped.
func mapping() map[string]any {
	create := func(name string) map[string]any { return map[string]any{"create": map[string]any{"name": name}} }
	return map[string]any{
		"version": 1, "guild_id": guildID,
		"defaults": map[string]any{"unmapped_human": "placeholder", "unmapped_bot": "skip", "uncategorized_space": catProjects},
		"spaces": []any{
			map[string]any{"discord_category": catProjects, "target": create("Projects")},
			map[string]any{"discord_category": catCommunity, "target": create("Community")},
		},
		"channels": []any{
			map[string]any{"discord_channel": chGeneral, "target": create("general")},
			map[string]any{"discord_channel": chNews, "target": create("news")},
			map[string]any{"discord_channel": chSupport, "target": create("support")},
			map[string]any{"discord_channel": chRandom, "target": create("random"), "planned_operator": "agent:lotse", "lead": "bothy"},
		},
		"users": []any{
			map[string]any{"discord_user": userAlex, "target": "user:alex"},
			map[string]any{"discord_user": userRobin, "target": "user:robin"},
		},
	}
}

// mapFile writes m as the YAML mapping file of --map and returns its path.
func (w *world) mapFile(m map[string]any) string {
	w.t.Helper()
	b, err := yaml.Marshal(m)
	if err != nil {
		w.t.Fatal(err)
	}
	path := filepath.Join(w.t.TempDir(), "mapping.yaml")
	if err := os.WriteFile(path, b, 0o600); err != nil {
		w.t.Fatal(err)
	}
	return path
}

// source creates the source of the guild with mapping() as Robin with step-up and starts it (the
// web console's job in production, 22 8); it returns the source ID.
func (w *world) source() string {
	w.t.Helper()
	return w.sourceWith(mapping())
}

// sourceWith is source with the mapping m.
func (w *world) sourceWith(m map[string]any) string {
	w.t.Helper()
	var b struct {
		ID string `json:"id"`
	}
	w.must(w.robin, http.MethodPost, "/imports", map[string]any{"kind": "discord", "name": "Example Workshop", "config": m},
		nil, http.StatusCreated).JSON(w.t, &b)
	w.must(w.robin, http.MethodPost, "/imports/"+b.ID+"/start", nil, nil, http.StatusAccepted)
	return b.ID
}

// login logs the tool in as Robin's cli device through device pairing (M8-10, the flow of
// TestLoginAgainstRealServer) and returns the session file. The sleeper of the poll approves the
// pairing code with Robin's step-up session on its first call and never waits on the wall clock.
func (w *world) login() string {
	w.t.Helper()
	return w.loginTo(w.s.PublicURL, w.t.TempDir())
}

// loginTo is login against the server origin server with the session file below dir (the
// rehearsal takes the origin from the --server of the guide's command and keeps the file beyond
// the subtest of the login).
func (w *world) loginTo(server, dir string) string {
	w.t.Helper()
	path := filepath.Join(dir, "bothy", "import-session.json")
	var stdout, stderr syncBuffer
	approved := false
	sleep := func(ctx context.Context, _ time.Duration) error {
		if approved {
			return ctx.Err()
		}
		approved = true
		resp, err := harness.DoWithStepUp(ctx, w.robin, harness.Request{Method: http.MethodPost, Path: "/me/devices/pairings",
			Body: map[string]any{"pairing_code": strings.TrimSpace(stdout.String())}})
		if err != nil {
			return err
		}
		if resp.Status/100 != 2 {
			return &importapi.StatusError{Op: "approvePairing", Status: resp.Status}
		}
		return ctx.Err()
	}
	if err := importapi.Login(w.ctx, importapi.LoginOptions{ServerURL: server, SessionFile: path, AppVersion: "1.0.0",
		Hostname: "workstation", Limits: importapi.DefaultLimits(), Deps: importapi.Deps{Sleep: sleep}, Stdout: &stdout,
		Stderr: &stderr}); err != nil {
		w.t.Fatalf("Login: %v\n%s", err, stderr.String())
	}
	return path
}

// run is one in-process run of bothy import discord.
type run struct {
	// session is the session file of login.
	session string
	// fake is the Discord fake; nil means the world's.
	fake *discordtest.Fake
	// mapFile is the --map file; "" means mapping().
	mapFile string
	// takeover adds --takeover.
	takeover bool
	// sleep replaces the rate-limit sleeper (default: returns at once); leaseSleep the sleeper of
	// the lease ticker (default: waits for the end of the run, so no renewal happens).
	sleep, leaseSleep importtool.Sleeper
	// afterBatch is the test hook of the run.
	afterBatch func(batch int) error
	// extra are further arguments (--once, --follow).
	extra []string
	// ctx is the context of the run (a cancel stops a --follow mirror like Ctrl-C); nil means the
	// test's.
	ctx context.Context
	// followSleep is the FollowSleep of the run; nil means the rate-limit sleeper.
	followSleep importtool.Sleeper
}

// result is the exit code and the output of a run.
type result struct {
	code           int
	stdout, stderr string
}

// noWait is the rate-limit sleeper of a run that never waits.
func noWait(ctx context.Context, _ time.Duration) error { return ctx.Err() }

// untilDone is the lease sleeper of a run that renews only through its batches: it returns when
// the run ends.
func untilDone(ctx context.Context, _ time.Duration) error {
	<-ctx.Done()
	return ctx.Err()
}

// toolEnviron is the environment of a run: the invented bot token and the tool values pointing
// the tool at the fakes (22 19).
func (w *world) toolEnviron(fake *discordtest.Fake) []string {
	return []string{
		"BOTHY_IMPORT_DISCORD_TOKEN=invented-bot-token-0123456789",
		"BOTHY_IMPORT__DISCORD__API_BASE=" + fake.BaseURL,
		"BOTHY_IMPORT__DISCORD__CDN_HOSTS=" + strings.Join(w.cdn.Hosts(), ","),
		"BOTHY_IMPORT__DISCORD__MAX_RATE_WAIT=" + maxRateWait.String(),
		"BOTHY_IMPORT__ATTACHMENT_MAX_BYTES=" + attachmentMaxBytes,
		"BOTHY_IMPORT__BATCH_MAX_ITEMS=" + batchMaxItems,
		"BOTHY_IMPORT__FOLLOW__POLL_INTERVAL=" + pollInterval.String(),
		"BOTHY_IMPORT__LEASE_STALE_AFTER=" + leaseStaleAfter.String(),
	}
}

// apply runs bothy import discord --apply with r and returns its exit code and output.
func (w *world) apply(r run) result {
	w.t.Helper()
	file := r.mapFile
	if file == "" {
		file = w.mapFile(mapping())
	}
	args := []string{"--guild", guildID, "--map", file, "--apply"}
	if r.takeover {
		args = append(args, "--takeover")
	}
	return w.invoke(r, append(args, r.extra...)...)
}

// invoke runs bothy import discord with exactly args and the environment of r (its session, fake
// and sleepers) and returns its exit code and output.
func (w *world) invoke(r run, args ...string) result {
	w.t.Helper()
	fake := r.fake
	if fake == nil {
		fake = w.fake
	}
	sleep, leaseSleep := r.sleep, r.leaseSleep
	if sleep == nil {
		sleep = noWait
	}
	if leaseSleep == nil {
		leaseSleep = untilDone
	}
	var stdout, stderr syncBuffer
	env := importtool.Env{
		Environ: w.toolEnviron(fake), Stdout: &stdout, Stderr: &stderr, Version: "1.0.0",
		SessionPath: func() (string, error) { return r.session, nil },
		Clock:       w.clk, Sleep: sleep, LeaseSleep: leaseSleep,
		Discord:       discord.Deps{Resolver: fake.Resolver, RootCAs: fake.RootCAs},
		CDN:           discord.Deps{Resolver: w.cdn.Resolver, RootCAs: w.cdn.RootCAs},
		DiscordEgress: fake.EgressConfig(), CDNEgress: w.cdn.EgressConfig(w.t), AfterBatch: r.afterBatch, FollowSleep: r.followSleep,
		// The debug log of the clients carries path templates and statuses, never content; it lands
		// on stderr so a failed run shows why.
		Logger: slog.New(slog.NewTextHandler(&stderr, &slog.HandlerOptions{Level: slog.LevelDebug})),
	}
	ctx := r.ctx
	if ctx == nil {
		ctx = w.ctx
	}
	code := importtool.Main(ctx, args, env)
	return result{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

// mustApply runs the tool and fails unless it exits with want.
func (w *world) mustApply(r run, want int) result {
	w.t.Helper()
	res := w.apply(r)
	if res.code != want {
		w.t.Fatalf("exit %d, want %d\nstderr: %s", res.code, want, res.stderr)
	}
	return res
}

// count runs a count query on the admin pool.
func (w *world) count(query string, args ...any) int {
	w.t.Helper()
	var n int
	if err := w.s.AdminDB().QueryRow(w.ctx, query, args...).Scan(&n); err != nil {
		w.t.Fatalf("%s: %v", query, err)
	}
	return n
}

// text reads one text column of a query on the admin pool.
func (w *world) text(query string, args ...any) string {
	w.t.Helper()
	var s string
	if err := w.s.AdminDB().QueryRow(w.ctx, query, args...).Scan(&s); err != nil {
		w.t.Fatalf("%s: %v", query, err)
	}
	return s
}

// target returns the target of the posted item with the origin key of the source.
func (w *world) target(src, key string) string {
	w.t.Helper()
	return w.text(`SELECT target_id::text FROM import_items WHERE source_id = $1 AND origin_key = $2 AND state = 'posted'`, src, key)
}

// cursor reads the stored cursor object of the source.
type cursor struct {
	Run *struct {
		ID        string `json:"id"`
		RenewedAt string `json:"renewed_at"`
	} `json:"run"`
	Channels map[string]string `json:"channels"`
	Threads  map[string]string `json:"threads"`
}

// cursor reads the cursor of the source from the database.
func (w *world) cursor(src string) cursor {
	w.t.Helper()
	var c cursor
	if err := json.Unmarshal([]byte(w.text(`SELECT cursor::text FROM import_sources WHERE id = $1`, src)), &c); err != nil {
		w.t.Fatal(err)
	}
	return c
}

// idLess orders two Discord IDs numerically.
func idLess(a, b string) bool {
	if len(a) != len(b) {
		return len(a) < len(b)
	}
	return a < b
}

// checkComplete holds the guarantees of a complete import of the fixture (22 7, 16 "No duplicate
// after a crash"): the posted items per kind equal the fixture, every origin key exists once, no
// item failed, every imported message exists once and silent, and the messages of every thread in
// thread_seq order are in Discord ID order.
func (w *world) checkComplete(src string) {
	w.t.Helper()
	rows, err := w.s.AdminDB().Query(w.ctx, `SELECT item_kind, count(*) FROM import_items WHERE source_id = $1 AND state = 'posted'
		GROUP BY item_kind`, src)
	if err != nil {
		w.t.Fatal(err)
	}
	got := map[string]int{}
	for rows.Next() {
		var kind string
		var n int
		if err := rows.Scan(&kind, &n); err != nil {
			w.t.Fatal(err)
		}
		got[kind] = n
	}
	rows.Close()
	for kind, n := range wantItems {
		if got[kind] != n {
			w.t.Errorf("posted %s items = %d, want %d (all %v)", kind, got[kind], n, got)
		}
	}
	if n := w.count(`SELECT count(*) FROM (SELECT origin_key FROM import_items WHERE source_id = $1 GROUP BY origin_key HAVING count(*) > 1) d`,
		src); n != 0 {
		w.t.Errorf("%d origin keys more than once", n)
	}
	if n := w.count(`SELECT count(*) FROM import_items WHERE source_id = $1 AND state = 'failed'`, src); n != 0 {
		w.t.Errorf("%d failed items", n)
	}
	var all, distinct, silent int
	if err := w.s.AdminDB().QueryRow(w.ctx, `SELECT count(*), count(DISTINCT import_item_id), count(*) FILTER (WHERE silent)
		FROM messages WHERE origin = 'import'`).Scan(&all, &distinct, &silent); err != nil {
		w.t.Fatal(err)
	}
	if all != wantMessages || distinct != wantMessages || silent != wantMessages {
		w.t.Errorf("imported messages %d, distinct items %d, silent %d; want %d each", all, distinct, silent, wantMessages)
	}
	w.checkOrder()
}

// checkOrder holds that the imported messages of every thread in thread_seq order are in
// ascending Discord ID order.
func (w *world) checkOrder() {
	w.t.Helper()
	rows, err := w.s.AdminDB().Query(w.ctx, `SELECT m.thread_id::text, i.origin_key FROM messages m JOIN import_items i ON i.id = m.import_item_id
		WHERE m.origin = 'import' ORDER BY m.thread_id, m.thread_seq`)
	if err != nil {
		w.t.Fatal(err)
	}
	defer rows.Close()
	last := map[string]string{}
	for rows.Next() {
		var thread, key string
		if err := rows.Scan(&thread, &key); err != nil {
			w.t.Fatal(err)
		}
		id := strings.TrimPrefix(key, "message:")
		if prev, ok := last[thread]; ok && !idLess(prev, id) {
			w.t.Errorf("thread %s: message %s after %s in thread_seq order", thread, id, prev)
		}
		last[thread] = id
	}
}
