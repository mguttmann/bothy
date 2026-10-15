// SPDX-License-Identifier: Apache-2.0

package discord

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"

	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/discord/discordtest"
)

// packageDir returns the directory of this package.
func packageDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("no caller")
	}
	return filepath.Dir(file)
}

// typeCheckPackage parses the non-test files of this package and type-checks them against the
// export data of their imports (like internal/wire/counters_static_test.go, without the parallel
// type check of packages.Load).
func typeCheckPackage(t *testing.T) (*token.FileSet, []*ast.File, *types.Package, *types.Info) {
	t.Helper()
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedCompiledGoFiles | packages.NeedImports | packages.NeedDeps |
			packages.NeedExportFile | packages.NeedModule,
		Dir: packageDir(t),
	}
	pkgs, err := packages.Load(cfg, ".")
	if err != nil || len(pkgs) != 1 {
		t.Fatalf("load: %v (%d packages)", err, len(pkgs))
	}
	p := pkgs[0]
	for _, e := range p.Errors {
		t.Fatalf("load: %v", e)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, name := range p.CompiledGoFiles {
		f, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	conf := types.Config{Importer: importer.ForCompiler(fset, "gc", func(path string) (io.ReadCloser, error) {
		dep, ok := p.Imports[path]
		if !ok || dep.ExportFile == "" {
			return nil, fmt.Errorf("no export data for %q", path)
		}
		return os.Open(dep.ExportFile)
	})}
	info := &types.Info{Uses: map[*ast.Ident]types.Object{}, Types: map[ast.Expr]types.TypeAndValue{}}
	pkg, err := conf.Check(p.PkgPath, fset, files, info)
	if err != nil {
		t.Fatalf("type-check: %v", err)
	}
	return fset, files, pkg, info
}

// writeVerb matches names that mention a write verb.
var writeVerb = regexp.MustCompile(`(?i)post|put|patch|delete`)

// TestClientHasOnlyGet is the static check of 22 16 "Discord client without a write method":
// one request construction in the package, with the constant http.MethodGet; no request built
// as a literal or with its Method changed; no exported name with a write verb and no exported
// function that takes a method.
func TestClientHasOnlyGet(t *testing.T) {
	fset, files, pkg, info := typeCheckPackage(t)
	var sites []string
	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.CallExpr:
				fn := calledFunc(info, n.Fun)
				if fn == nil || fn.Pkg() == nil || fn.Pkg().Path() != "net/http" {
					return true
				}
				if fn.Name() != "NewRequest" && fn.Name() != "NewRequestWithContext" {
					return true
				}
				sites = append(sites, fset.Position(n.Pos()).String())
				arg := n.Args[0]
				if fn.Name() == "NewRequestWithContext" {
					arg = n.Args[1]
				}
				if obj := usedObject(info, arg); obj == nil || obj.Pkg() == nil || obj.Pkg().Path() != "net/http" || obj.Name() != "MethodGet" {
					t.Errorf("%s: the method is not the constant http.MethodGet", fset.Position(n.Pos()))
				}
			case *ast.CompositeLit:
				if tv, ok := info.Types[n]; ok && types.TypeString(tv.Type, nil) == "net/http.Request" {
					t.Errorf("%s: an http.Request literal", fset.Position(n.Pos()))
				}
			case *ast.SelectorExpr:
				if sel, ok := info.Types[n.X]; ok && strings.HasSuffix(types.TypeString(sel.Type, nil), "net/http.Request") && n.Sel.Name == "Method" {
					t.Errorf("%s: the request method is read or changed", fset.Position(n.Pos()))
				}
			}
			return true
		})
	}
	if len(sites) != 1 {
		t.Errorf("request constructions = %v, want exactly one", sites)
	}
	checkExportedNames(t, pkg)
}

// checkExportedNames fails on exported names with a write verb and on exported functions that
// take a parameter named like a method or a verb.
func checkExportedNames(t *testing.T, pkg *types.Package) {
	t.Helper()
	check := func(name string) {
		if writeVerb.MatchString(name) {
			t.Errorf("exported name %s mentions a write verb", name)
		}
	}
	checkSig := func(name string, sig *types.Signature) {
		for v := range sig.Params().Variables() {
			if p := strings.ToLower(v.Name()); strings.Contains(p, "method") || strings.Contains(p, "verb") {
				t.Errorf("%s takes a method parameter %s", name, v.Name())
			}
		}
	}
	scope := pkg.Scope()
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		if !obj.Exported() {
			continue
		}
		check(name)
		if fn, ok := obj.(*types.Func); ok {
			checkSig(name, fn.Signature())
		}
		named, ok := obj.Type().(*types.Named)
		if !ok || obj.Type() == nil {
			continue
		}
		for m := range named.Methods() {
			if m.Exported() {
				check(name + "." + m.Name())
				checkSig(name+"."+m.Name(), m.Signature())
			}
		}
		if st, ok := named.Underlying().(*types.Struct); ok {
			for f := range st.Fields() {
				if f.Exported() {
					check(name + "." + f.Name())
				}
			}
		}
	}
}

// calledFunc returns the function a call expression calls, if it is a named function.
func calledFunc(info *types.Info, fun ast.Expr) *types.Func {
	switch f := fun.(type) {
	case *ast.SelectorExpr:
		obj, _ := info.Uses[f.Sel].(*types.Func)
		return obj
	case *ast.Ident:
		obj, _ := info.Uses[f].(*types.Func)
		return obj
	}
	return nil
}

// usedObject returns the object an identifier or selector refers to.
func usedObject(info *types.Info, e ast.Expr) types.Object {
	switch e := e.(type) {
	case *ast.SelectorExpr:
		return info.Uses[e.Sel]
	case *ast.Ident:
		return info.Uses[e]
	}
	return nil
}

// callEveryMethod drives every exported read method once.
func callEveryMethod(ctx context.Context, c *Client) []error {
	collect := func([]Channel) error { return nil }
	var errs []error
	add := func(err error) { errs = append(errs, err) }
	_, err := c.Me(ctx)
	add(err)
	_, err = c.Guild(ctx, guildID)
	add(err)
	_, err = c.GuildChannels(ctx, guildID)
	add(err)
	_, err = c.ActiveThreads(ctx, guildID)
	add(err)
	add(c.ArchivedPublicThreads(ctx, generalID, collect))
	add(c.ArchivedPrivateThreads(ctx, forumID, collect))
	_, err = c.Channel(ctx, generalID)
	add(err)
	_, err = c.MessagesAfter(ctx, generalID, firstMsgID, 50)
	add(err)
	_, err = c.MessagesBefore(ctx, generalID, lastMsgID, 50)
	add(err)
	_, err = c.MessagesAround(ctx, generalID, middleMsgID, 5)
	add(err)
	return errs
}

// TestFakeSeesOnlyGet drives every exported method against the fake: the method set it logs is
// exactly {GET}, and every call carried the token.
func TestFakeSeesOnlyGet(t *testing.T) {
	h := newHarness(t, nil, nil)
	for i, err := range callEveryMethod(t.Context(), h.client) {
		if err != nil && !errors.Is(err, ErrForbidden) {
			t.Errorf("call %d: %v", i, err)
		}
	}
	if got := h.fake.Methods(); !slices.Equal(got, []string{http.MethodGet}) {
		t.Fatalf("methods = %v, want [GET]", got)
	}
	if n := len(h.fake.Requests()); n < 10 {
		t.Fatalf("requests = %d, want at least one per method", n)
	}
	for _, r := range h.fake.Requests() {
		if !r.Authorization || r.Host != discordtest.Host {
			t.Errorf("request %+v: want the token on the API host", r)
		}
	}
}

// TestUserAgentFormat: every request carries "DiscordBot (<url>, <version>)" [R6].
func TestUserAgentFormat(t *testing.T) {
	h := newHarness(t, nil, nil)
	if _, err := h.client.Me(t.Context()); err != nil {
		t.Fatal(err)
	}
	want := regexp.MustCompile(`^DiscordBot \(https://[^ ,()]+, 1\.2\.3\)$`)
	for _, r := range h.fake.Requests() {
		if !want.MatchString(r.UserAgent) {
			t.Errorf("User-Agent = %q, want the DiscordBot (<url>, <version>) form", r.UserAgent)
		}
	}
}

// TestTokenOnlyToAPIBase: the header setter refuses a request to another host than the API
// base before anything is sent, and a redirect to another host is not followed, so the fake
// sees the token only on its own host.
func TestTokenOnlyToAPIBase(t *testing.T) {
	h := newHarness(t, nil, nil)
	for _, target := range []string{
		"https://" + discordtest.OtherHost + "/api/v10/users/@me",
		"http://" + discordtest.Host + "/api/v10/users/@me",
		"https://" + discordtest.Host + ":1/api/v10/users/@me",
	} {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := h.client.authorize(req); !errors.Is(err, ErrTokenHostMismatch) {
			t.Errorf("authorize(%s) = %v, want ErrTokenHostMismatch", target, err)
		}
		if req.Header.Get("Authorization") != "" {
			t.Errorf("authorize(%s) set the header", target)
		}
	}
	other := "https://" + discordtest.OtherHost + "/api/v10/users/@me"
	h.fake.Inject(path("/users/@me"), discordtest.Redirect(other))
	_, err := h.client.Me(t.Context())
	if !errors.Is(err, ErrUnexpectedStatus) {
		t.Fatalf("Me after a redirect = %v, want ErrUnexpectedStatus", err)
	}
	for _, r := range h.fake.Requests() {
		if r.Host != discordtest.Host {
			t.Errorf("request to %s: the client followed the redirect", r.Host)
		}
	}
	if len(h.fake.Requests()) != 1 {
		t.Errorf("requests = %d, want 1", len(h.fake.Requests()))
	}
}

// TestNewRejectsInvalidConfig: New refuses configurations that would send the token in clear
// text or without the limits of 22 19.
func TestNewRejectsInvalidConfig(t *testing.T) {
	fake := discordtest.New(t, discordtest.BasicFixture(t))
	tok, err := NewToken(testToken)
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*Config){
		"http base":          func(c *Config) { c.APIBase = "http://" + discordtest.Host + "/api/v10" },
		"user info":          func(c *Config) { c.APIBase = "https://alex:secret@" + discordtest.Host + "/api/v10" },
		"query":              func(c *Config) { c.APIBase = fake.BaseURL + "?x=1" },
		"no host":            func(c *Config) { c.APIBase = "https:///api/v10" },
		"bad url":            func(c *Config) { c.APIBase = "https://[::1" },
		"no version":         func(c *Config) { c.UserAgentVersion = "" },
		"version with paren": func(c *Config) { c.UserAgentVersion = "1)" },
		"page size 0":        func(c *Config) { c.PageSize = 0 },
		"page size 101":      func(c *Config) { c.PageSize = 101 },
		"negative retries":   func(c *Config) { c.MaxRetries = -1 },
		"no rate wait":       func(c *Config) { c.MaxRateWait = 0 },
		"no timeout":         func(c *Config) { c.HTTPTimeout = 0 },
		"no egress":          func(c *Config) { c.Egress.Timeout = 0 },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := testConfig(fake)
			change(&cfg)
			if _, err := New(cfg, tok, Deps{}); err == nil {
				t.Fatal("New accepted the configuration")
			}
		})
	}
	if _, err := New(testConfig(fake), Token{}, Deps{}); err == nil {
		t.Fatal("New accepted an empty token")
	}
}

// TestArchivedThreadPaging: three pages newest first, a thread repeated across pages appears
// once, before is the archive_timestamp of the last thread, and a page without a new ID ends
// the loop although it says has_more.
func TestArchivedThreadPaging(t *testing.T) {
	h := newHarness(t, nil, nil)
	var got []string
	err := h.client.ArchivedPublicThreads(t.Context(), generalID, func(page []Channel) error {
		for _, c := range page {
			got = append(got, c.ID)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"13000311", "13000312", "13000313", "13000314"}
	if !slices.Equal(got, want) {
		t.Fatalf("threads = %v, want %v", got, want)
	}
	var befores []string
	for _, r := range h.fake.Requests() {
		q, _ := parseQuery(r.Query)
		befores = append(befores, q.before)
		if q.limit != "100" {
			t.Errorf("limit = %q, want import.discord.page_size", q.limit)
		}
	}
	wantBefores := []string{"", "2026-03-04T10:00:00.000000+00:00", "2026-03-03T10:00:00.000000+00:00", "2026-03-02T10:00:00.000000+00:00"}
	if !slices.Equal(befores, wantBefores) {
		t.Fatalf("before values = %q, want %q", befores, wantBefores)
	}
}

// TestArchivedThreadPagingStops: has_more false ends the loop, an error of the callback ends
// it with that error, and a thread without archive_timestamp ends it with ErrDecode.
func TestArchivedThreadPagingStops(t *testing.T) {
	h := newHarness(t, nil, nil)
	pages := 0
	if err := h.client.ArchivedPublicThreads(t.Context(), forumID, func([]Channel) error { pages++; return nil }); err != nil || pages != 1 {
		t.Fatalf("forum: pages %d, err %v", pages, err)
	}
	stop := errors.New("stop")
	if err := h.client.ArchivedPublicThreads(t.Context(), generalID, func([]Channel) error { return stop }); !errors.Is(err, stop) {
		t.Fatalf("callback error = %v", err)
	}
	fx := discordtest.BasicFixture(t)
	fx.ArchivedPublic[generalID][0].Threads[1] = json.RawMessage(`{"id": "13000312", "type": 11}`)
	h = newHarness(t, fx, nil)
	err := h.client.ArchivedPublicThreads(t.Context(), generalID, func([]Channel) error { return nil })
	if !errors.Is(err, ErrDecode) {
		t.Fatalf("missing archive_timestamp = %v, want ErrDecode", err)
	}
	if !strings.Contains(err.Error(), "archive_timestamp") {
		t.Fatalf("error %q does not name the field", err)
	}
}

// pageQuery is the paging part of a logged query.
type pageQuery struct{ before, after, around, limit string }

func parseQuery(raw string) (pageQuery, error) {
	vals, err := urlParseQuery(raw)
	return pageQuery{before: vals.Get("before"), after: vals.Get("after"), around: vals.Get("around"), limit: vals.Get("limit")}, err
}

// TestPrivateArchivedForbidden: a 403 on the private list returns ErrForbidden with the hint on
// MANAGE_THREADS.
func TestPrivateArchivedForbidden(t *testing.T) {
	h := newHarness(t, nil, nil)
	called := false
	err := h.client.ArchivedPrivateThreads(t.Context(), generalID, func([]Channel) error { called = true; return nil })
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
	if !strings.Contains(err.Error(), "MANAGE_THREADS") {
		t.Fatalf("err %q has no hint on MANAGE_THREADS", err)
	}
	if called {
		t.Fatal("the callback ran")
	}
	var ce *CallError
	if !errors.As(err, &ce) || ce.Status != http.StatusForbidden || ce.Op != "GET /channels/{channel.id}/threads/archived/private" {
		t.Fatalf("call error = %+v", ce)
	}
}

// TestMessagesAscending: after, before and around return ascending IDs although the fake
// answers newest first, and the query names the anchor and the limit.
func TestMessagesAscending(t *testing.T) {
	h := newHarness(t, nil, nil)
	ctx := t.Context()
	cases := []struct {
		name  string
		call  func() ([]Message, error)
		param string
		n     int
	}{
		{"after", func() ([]Message, error) { return h.client.MessagesAfter(ctx, generalID, firstMsgID, 5) }, "after", 5},
		{"before", func() ([]Message, error) { return h.client.MessagesBefore(ctx, generalID, lastMsgID, 5) }, "before", 5},
		{"around", func() ([]Message, error) { return h.client.MessagesAround(ctx, generalID, middleMsgID, 4) }, "around", 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msgs, err := tc.call()
			if err != nil {
				t.Fatal(err)
			}
			if len(msgs) != tc.n {
				t.Fatalf("messages = %d, want %d", len(msgs), tc.n)
			}
			for i := 1; i < len(msgs); i++ {
				if compareSnowflakes(msgs[i-1].ID, msgs[i].ID) >= 0 {
					t.Fatalf("not ascending at %d: %s then %s", i, msgs[i-1].ID, msgs[i].ID)
				}
			}
			reqs := h.fake.Requests()
			last := reqs[len(reqs)-1]
			if !strings.Contains(last.Query, tc.param+"=") || !strings.Contains(last.Query, fmt.Sprintf("limit=%d", tc.n)) {
				t.Fatalf("query %q", last.Query)
			}
		})
	}
	if _, err := h.client.MessagesAfter(ctx, generalID, firstMsgID, 0); err == nil {
		t.Fatal("limit 0 accepted")
	}
	if _, err := h.client.MessagesAfter(ctx, generalID, firstMsgID, 101); err == nil {
		t.Fatal("limit above import.discord.page_size accepted")
	}
}

// TestDecodedTypes: the fields the later tickets map arrive typed.
func TestDecodedTypes(t *testing.T) {
	h := newHarness(t, nil, nil)
	ctx := t.Context()
	me, err := h.client.Me(ctx)
	if err != nil || me.ID != "13000002" || !me.Bot {
		t.Fatalf("Me = %+v, %v", me, err)
	}
	g, err := h.client.Guild(ctx, guildID)
	if err != nil || g.Name != "Example Workshop" || len(g.Roles) != 2 {
		t.Fatalf("Guild = %+v, %v", g, err)
	}
	chans, err := h.client.GuildChannels(ctx, guildID)
	if err != nil || len(chans) != 7 {
		t.Fatalf("GuildChannels = %d, %v", len(chans), err)
	}
	forum := chans[4]
	if forum.Type != 15 || len(forum.AvailableTags) != 2 || forum.AvailableTags[1].EmojiName == "" || forum.ParentID != "13000101" {
		t.Fatalf("forum = %+v", forum)
	}
	if chans[1].PermissionOverwrites == nil || len(chans[1].PermissionOverwrites) != 1 || chans[6].ParentID != "" {
		t.Fatalf("overwrites or parent: %+v %+v", chans[1], chans[6])
	}
	active, err := h.client.ActiveThreads(ctx, guildID)
	if err != nil || len(active) != 2 || active[1].AppliedTags[0] != "13000201" {
		t.Fatalf("ActiveThreads = %+v, %v", active, err)
	}
	if md := active[0].ThreadMetadata; md == nil || md.Archived == nil || *md.Archived {
		t.Fatalf("thread metadata = %+v", md)
	}
	ch, err := h.client.Channel(ctx, threadID)
	if err != nil || ch.OwnerID != "13000011" {
		t.Fatalf("Channel = %+v, %v", ch, err)
	}
	msgs, err := h.client.MessagesAfter(ctx, generalID, "1", 100)
	if err != nil || len(msgs) != 12 {
		t.Fatalf("messages = %d, %v", len(msgs), err)
	}
	checkMessages(t, msgs)
}

// checkMessages checks the typed fields of the fixture messages.
func checkMessages(t *testing.T, msgs []Message) {
	t.Helper()
	byID := map[string]Message{}
	for _, m := range msgs {
		byID[m.ID[len(m.ID)-2:]] = m
	}
	if m := byID["01"]; !m.Pinned || m.Thread == nil || m.Reactions[0].Count != 2 || m.Reactions[0].Emoji.Name == "" {
		t.Errorf("message 01 = %+v", m)
	}
	if m := byID["02"]; m.Type != 19 || m.MessageReference == nil || m.MessageReference.MessageID != firstMsgID || m.EditedTimestamp == "" || len(m.Mentions) != 1 {
		t.Errorf("reply = %+v", m)
	}
	if m := byID["04"]; len(m.Attachments) != 1 || m.Attachments[0].Size != 1234 || m.Attachments[0].ContentType == "" {
		t.Errorf("attachment = %+v", m)
	}
	if u := byID["04"].Attachments[0].URL; !strings.Contains(u.Raw(), "hm=") || strings.Contains(u.String(), "?") ||
		u.String() != "https://cdn.example.com/attachments/13000103/13005001/notes.txt" {
		t.Errorf("attachment URL raw %q safe %q", u.Raw(), u.String())
	}
	if m := byID["05"]; m.WebhookID == "" || len(m.Embeds) != 1 || m.Embeds[0].Title == "" || m.Embeds[0].URL.String() == "" {
		t.Errorf("webhook = %+v", m)
	}
	if m := byID["06"]; len(m.StickerItems) != 1 || m.StickerItems[0].Name == "" {
		t.Errorf("sticker = %+v", m)
	}
	if m := byID["07"]; m.Poll == nil || len(m.Poll.Answers) != 2 || m.Poll.Question.Text == "" {
		t.Errorf("poll = %+v", m)
	}
	if m := byID["08"]; len(m.MessageSnapshots) != 1 || m.MessageSnapshots[0].Message.Content == "" || m.MessageReference.Type != 1 {
		t.Errorf("forward = %+v", m)
	}
	if m := byID["03"]; !m.Author.Bot || m.Type != 20 {
		t.Errorf("bot = %+v", m)
	}
}

// TestInvalidSnowflake: an ID that is not a digit string is ErrInvalidID naming the field and
// never the value, in a response and in an argument (which is refused before sending).
func TestInvalidSnowflake(t *testing.T) {
	fx := discordtest.BasicFixture(t)
	fx.Me = json.RawMessage(`{"id": "13000x0000", "username": "import-bot", "bot": true}`)
	fx.Messages[randomID] = []json.RawMessage{json.RawMessage(
		`{"id": "13003001", "channel_id": "13000107", "type": 0, "author": {"id": "robin-7", "username": "robin"}, "content": "hi"}`)}
	h := newHarness(t, fx, nil)
	_, err := h.client.Me(t.Context())
	if !errors.Is(err, ErrInvalidID) || !strings.Contains(err.Error(), "id") || strings.Contains(err.Error(), "13000x0000") {
		t.Fatalf("Me = %v, want ErrInvalidID naming the field without the value", err)
	}
	_, err = h.client.MessagesAfter(t.Context(), randomID, "1", 10)
	if !errors.Is(err, ErrInvalidID) || !strings.Contains(err.Error(), "author.id") || strings.Contains(err.Error(), "robin-7") {
		t.Fatalf("messages = %v, want ErrInvalidID naming author.id", err)
	}
	before := len(h.fake.Requests())
	for _, bad := range []string{"", "12a", "../users/@me", "1?x=1", "123456789012345678901"} {
		if _, err := h.client.Channel(t.Context(), bad); !errors.Is(err, ErrInvalidID) {
			t.Errorf("Channel(%q) = %v, want ErrInvalidID", bad, err)
		}
		if _, err := h.client.MessagesAfter(t.Context(), generalID, bad, 10); !errors.Is(err, ErrInvalidID) {
			t.Errorf("MessagesAfter(after %q) = %v, want ErrInvalidID", bad, err)
		}
	}
	if len(h.fake.Requests()) != before {
		t.Fatal("an invalid argument reached the server")
	}
}

// TestStatusErrors: 404, 5xx and other statuses map to their sentinels and are not retried.
func TestStatusErrors(t *testing.T) {
	cases := map[int]error{
		http.StatusNotFound:            ErrNotFound,
		http.StatusInternalServerError: ErrServer,
		http.StatusBadGateway:          ErrServer,
		http.StatusBadRequest:          ErrUnexpectedStatus,
	}
	for status, want := range cases {
		t.Run(http.StatusText(status), func(t *testing.T) {
			h := newHarness(t, nil, nil)
			h.fake.Inject(path("/guilds/"+guildID), discordtest.Status(status))
			_, err := h.client.Guild(t.Context(), guildID)
			if !errors.Is(err, want) {
				t.Fatalf("err = %v, want %v", err, want)
			}
			want := fmt.Sprintf("discord: GET /guilds/{guild.id}: %d %s", status, http.StatusText(status))
			if !strings.HasPrefix(err.Error(), want) {
				t.Fatalf("text %q, want prefix %q", err, want)
			}
			if n := len(h.fake.Requests()); n != 1 {
				t.Fatalf("requests = %d, want 1", n)
			}
		})
	}
}

// TestMalformedBody: a body that is not the expected JSON is ErrDecode without its content.
func TestMalformedBody(t *testing.T) {
	fx := discordtest.BasicFixture(t)
	fx.Me = json.RawMessage(`["secret-looking-body"]`)
	h := newHarness(t, fx, nil)
	_, err := h.client.Me(t.Context())
	if !errors.Is(err, ErrDecode) || strings.Contains(err.Error(), "secret-looking-body") {
		t.Fatalf("err = %v", err)
	}
}

// TestTransportError: a failed connection returns the transport error without the token and
// without the query.
func TestTransportError(t *testing.T) {
	h := newHarness(t, nil, nil)
	h.fake.Server.Close()
	_, err := h.client.MessagesAfter(t.Context(), generalID, firstMsgID, 10)
	if err == nil {
		t.Fatal("no error")
	}
	var ce *CallError
	if !errors.As(err, &ce) || ce.Op != "GET /channels/{channel.id}/messages" {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(err.Error(), firstMsgID) || strings.Contains(err.Error(), testToken) {
		t.Fatalf("err %q quotes the query or the token", err)
	}
}
