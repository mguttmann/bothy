// SPDX-License-Identifier: Apache-2.0

package apns_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/notify/adapters/apns"
	"github.com/OWNER/bothy/services/server/internal/notify/adapters/apns/apnstest"
	"github.com/OWNER/bothy/services/server/internal/notify/app"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

var update = flag.Bool("update", false, "rewrite testdata/payload_message.golden")

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// The allowed keys and values of 13 6.1.
var (
	apsKeys   = []string{"alert", "mutable-content", "thread-id", "category", "sound", "badge", "interruption-level"}
	alertKeys = []string{"title-loc-key", "loc-key"}
	ownKeys   = []string{"v", "nid", "k", "i", "t", "m"}
	levels    = []string{"active", "passive", "time-sensitive"}
)

// variant is one combination of TestPayloadAllowList.
type variant struct {
	kind          domain.Kind
	quiet, ts     bool
	message       bool
	badge         int64
	name          string
	notification  app.Notification
	timeSensitive bool
}

// variants returns every kind times Quiet times push.apns.time_sensitive_enabled times with and
// without message ID times the badges.
func variants(w *world, badges ...int64) []variant {
	var out []variant
	for _, k := range domain.Kinds() {
		for _, quiet := range []bool{false, true} {
			for _, ts := range []bool{false, true} {
				for _, msg := range []bool{false, true} {
					for _, b := range badges {
						n := w.hint()
						n.Kind, n.Quiet, n.Badge = string(k), quiet, b
						if !k.HasThread() {
							n.ThreadID = ids.ID{}
						}
						if !msg {
							n.MessageID = ids.ID{}
						}
						out = append(out, variant{kind: k, quiet: quiet, ts: ts, message: msg, badge: b, notification: n, timeSensitive: ts,
							name: string(k) + "/quiet=" + strconv.FormatBool(quiet) + "/ts=" + strconv.FormatBool(ts) + "/msg=" + strconv.FormatBool(msg)})
					}
				}
			}
		}
	}
	return out
}

// sync returns the badge sync of 13 8 with badge b.
func (w *world) sync(b int64) app.Notification {
	return app.Notification{ID: ids.New(), InstanceID: w.instance, Kind: app.KindSync, Badge: b, HasBadge: true, Cleared: []ids.ID{ids.New(), ids.New()}}
}

// adapters returns the adapter of the world and one with push.apns.time_sensitive_enabled.
func (w *world) adapters(t *testing.T) map[bool]*apns.Adapter {
	t.Helper()
	cfg := w.cfg
	cfg.TimeSensitiveEnabled = true
	ts, err := apns.New(t.Context(), cfg, w.deps())
	if err != nil {
		t.Fatal(err)
	}
	return map[bool]*apns.Adapter{false: w.a, true: ts}
}

// lastBody returns the body of the last request at s, parsed as generic JSON.
func lastBody(t *testing.T, s *apnstest.Server) (map[string]any, []byte) {
	t.Helper()
	reqs := s.Requests()
	if len(reqs) == 0 {
		t.Fatal("no request")
	}
	raw := reqs[len(reqs)-1].Body
	var m map[string]any
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if err := d.Decode(&m); err != nil {
		t.Fatalf("body %q: %v", raw, err)
	}
	return m, raw
}

// onlyKeys fails unless every key of m is in allowed.
func onlyKeys(t *testing.T, what string, m map[string]any, allowed []string) {
	t.Helper()
	for k := range m {
		if !slices.Contains(allowed, k) {
			t.Errorf("%s: key %q is not allowed", what, k)
		}
	}
}

func isNumber(v any) bool { _, ok := v.(json.Number); return ok }

func isUUID(v any) bool { s, ok := v.(string); return ok && uuidPattern.MatchString(s) }

func TestPayloadAllowList(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	as := w.adapters(t)
	upper := map[string]bool{}
	for _, k := range domain.Kinds() {
		upper[strings.ToUpper(string(k))] = true
	}
	for _, v := range variants(w, 0, 3) {
		if res, err := as[v.timeSensitive].Send(t.Context(), w.phone, v.notification); err != nil || res.Class != domain.ResultAccepted {
			t.Fatalf("%s: %+v %v", v.name, res, err)
		}
		m, _ := lastBody(t, w.prod)
		onlyKeys(t, v.name, m, []string{"aps", "b"})
		a, _ := m["aps"].(map[string]any)
		b, _ := m["b"].(map[string]any)
		if a == nil || b == nil {
			t.Fatalf("%s: aps or b missing", v.name)
		}
		onlyKeys(t, v.name+" aps", a, apsKeys)
		onlyKeys(t, v.name+" b", b, ownKeys)
		alert, _ := a["alert"].(map[string]any)
		onlyKeys(t, v.name+" alert", alert, alertKeys)
		if alert["title-loc-key"] != "PUSH_TITLE" || alert["loc-key"] != "PUSH_BODY_"+strings.ToUpper(string(v.kind)) {
			t.Errorf("%s: alert %v", v.name, alert)
		}
		if a["badge"] == nil || a["badge"].(json.Number).String() != strconv.FormatInt(v.badge, 10) {
			t.Errorf("%s: badge %v, want %d", v.name, a["badge"], v.badge)
		}
		for _, k := range []string{"mutable-content", "badge"} {
			if !isNumber(a[k]) {
				t.Errorf("%s: aps.%s %v is not a number", v.name, k, a[k])
			}
		}
		if !isNumber(b["v"]) {
			t.Errorf("%s: b.v is not a number", v.name)
		}
		if !upper[a["category"].(string)] || !slices.Contains(levels, a["interruption-level"].(string)) {
			t.Errorf("%s: category %v level %v", v.name, a["category"], a["interruption-level"])
		}
		if s, ok := a["sound"]; ok && s != "default" {
			t.Errorf("%s: sound %v", v.name, s)
		}
		if !domain.Kind(b["k"].(string)).Valid() {
			t.Errorf("%s: k %v", v.name, b["k"])
		}
		for _, k := range []string{"nid", "i"} {
			if !isUUID(b[k]) {
				t.Errorf("%s: b.%s %v", v.name, k, b[k])
			}
		}
		for _, pair := range [][2]any{{a["thread-id"], "thread-id"}, {b["t"], "t"}, {b["m"], "m"}} {
			if pair[0] != nil && !isUUID(pair[0]) {
				t.Errorf("%s: %s %v is not a UUID", v.name, pair[1], pair[0])
			}
		}
		if !v.kind.HasThread() && (a["thread-id"] != nil || b["t"] != nil) {
			t.Errorf("%s: a thread on a hint without thread", v.name)
		}
		if v.kind.HasThread() && (a["thread-id"] == nil || b["t"] == nil) {
			t.Errorf("%s: thread missing", v.name)
		}
		if (b["m"] != nil) != v.message {
			t.Errorf("%s: m %v", v.name, b["m"])
		}
	}
	for _, badge := range []int64{0, 3} {
		w.send(w.phone, w.sync(badge))
		m, _ := lastBody(t, w.prod)
		onlyKeys(t, "sync", m, []string{"aps"})
		a, _ := m["aps"].(map[string]any)
		onlyKeys(t, "sync aps", a, []string{"badge"})
		if a["badge"].(json.Number).String() != strconv.FormatInt(badge, 10) {
			t.Errorf("sync badge %v", a["badge"])
		}
	}
	walk(t, reflect.TypeOf(apns.PayloadTypes()), "payload")
}

// allowedStrings are the string fields of the payload structs: IDs and fixed values only.
var allowedStrings = []string{"TitleLocKey", "LocKey", "ThreadID", "Category", "Sound", "InterruptionLevel", "NID", "K", "I", "T", "M"}

// walk checks that the payload types hold only structs, pointers to structs, numbers and the
// allowed string fields: no map, no interface, no slice.
func walk(t *testing.T, typ reflect.Type, path string) {
	t.Helper()
	k := typ.Kind()
	if k == reflect.Pointer {
		walk(t, typ.Elem(), path)
		return
	}
	if k == reflect.Struct {
		for i := range typ.NumField() {
			f := typ.Field(i)
			if f.Type.Kind() == reflect.String && !slices.Contains(allowedStrings, f.Name) {
				t.Errorf("%s.%s: string field outside the enums and IDs", path, f.Name)
			}
			walk(t, f.Type, path+"."+f.Name)
		}
		return
	}
	if k != reflect.String && k != reflect.Int && k != reflect.Int64 {
		t.Errorf("%s: kind %s is not allowed in the payload", path, k)
	}
}

func TestPayloadSizeUnderAppleLimit(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	as := w.adapters(t)
	for _, v := range variants(w, math.MaxInt64) {
		if _, err := as[v.timeSensitive].Send(t.Context(), w.phone, v.notification); err != nil {
			t.Fatal(err)
		}
		if _, raw := lastBody(t, w.prod); len(raw) >= apns.AppleMaxPayloadBytes {
			t.Errorf("%s: %d bytes, limit %d", v.name, len(raw), apns.AppleMaxPayloadBytes)
		}
	}
	before := len(w.prod.Requests())
	apns.SetInflate(w.a, func(b []byte) []byte { return append(b, bytes.Repeat([]byte(" "), apns.AppleMaxPayloadBytes)...) })
	if res := w.send(w.phone, w.hint()); res.Class != domain.ResultProgramError {
		t.Fatalf("oversize payload: %+v", res)
	}
	if n := len(w.prod.Requests()); n != before {
		t.Fatalf("the oversize payload reached the Fake")
	}
}

// chapterExample returns the JSON block of 13 "6.1 APNs".
func chapterExample(t *testing.T) []byte {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		for _, name := range []string{"13-push-notifications.md", "13-push-benachrichtigungen.md"} {
			b, err := os.ReadFile(filepath.Join(dir, "docs", "spec", name)) // #nosec G304 -- a chapter of the repository's specification
			if err != nil {
				continue
			}
			text := string(b)
			i := strings.Index(text, "### 6.1 APNs")
			if i < 0 {
				t.Fatalf("%s has no section 6.1 APNs", name)
			}
			text = text[i:]
			s := strings.Index(text, "```json\n")
			e := strings.Index(text[s+8:], "```")
			if s < 0 || e < 0 {
				t.Fatal("no JSON block in 6.1")
			}
			return []byte(text[s+8 : s+8+e])
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("docs/spec not found")
		}
		dir = parent
	}
}

func TestPayloadMatchesChapterExample(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	var want struct {
		B struct{ NID, I, T, M string } `json:"b"`
	}
	example := chapterExample(t)
	if err := json.Unmarshal(example, &want); err != nil {
		t.Fatal(err)
	}
	n := app.Notification{ID: ids.MustParse(want.B.NID), InstanceID: ids.MustParse(want.B.I), ThreadID: ids.MustParse(want.B.T),
		MessageID: ids.MustParse(want.B.M), Kind: "message", Count: 1, Badge: 3, HasBadge: true}
	w.send(w.phone, n)
	_, raw := lastBody(t, w.prod)
	var got, exp any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(example, &exp); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, exp) {
		t.Fatalf("payload %s differs from the example of 13 6.1", raw)
	}
	// The golden file holds the exact bytes plus one newline, which the end-of-file hook keeps.
	golden := filepath.Join("testdata", "payload_message.golden")
	line := append(slices.Clone(raw), '\n')
	if *update {
		if err := os.WriteFile(golden, line, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	g, err := os.ReadFile(golden) // #nosec G304 -- the golden file of this package
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(g, line) {
		t.Fatalf("payload bytes differ from %s:\n got %s\nwant %s", golden, line, g)
	}
}

func TestInterruptionLevel(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	as := w.adapters(t)
	level := func(a *apns.Adapter, k domain.Kind, quiet bool) (string, bool) {
		n := w.hint()
		n.Kind, n.Quiet = string(k), quiet
		if !k.HasThread() {
			n.ThreadID = ids.ID{}
		}
		if _, err := a.Send(t.Context(), w.phone, n); err != nil {
			t.Fatal(err)
		}
		m, _ := lastBody(t, w.prod)
		a2 := m["aps"].(map[string]any)
		_, sound := a2["sound"]
		return a2["interruption-level"].(string), sound
	}
	for _, k := range domain.Kinds() {
		for _, ts := range []bool{false, true} {
			got, sound := level(as[ts], k, true)
			if got != "passive" || sound {
				t.Errorf("%s quiet ts=%v: %s sound %v", k, ts, got, sound)
			}
			got, sound = level(as[ts], k, false)
			want := "active"
			if k == domain.KindInputNeeded && ts {
				want = "time-sensitive"
			}
			if got != want || !sound {
				t.Errorf("%s ts=%v: %s sound %v, want %s with sound", k, ts, got, sound, want)
			}
		}
	}
}

// adapterHeaders are the headers the adapter sets; net/http adds Accept-Encoding (the transport's
// gzip) and Content-Length on its own.
var transportHeaders = []string{"Accept-Encoding", "Content-Length"}

func TestHeaders(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	check := func(name string, n app.Notification, priority string, collapse bool) {
		t.Helper()
		w.send(w.phone, n)
		reqs := w.prod.Requests()
		r := reqs[len(reqs)-1]
		h := r.Header
		want := map[string]string{
			"Authorization": "bearer " + r.JWT, "Apns-Push-Type": "alert", "Apns-Id": n.ID.String(), "Apns-Topic": topicPhone,
			"Apns-Priority": priority, "Apns-Expiration": strconv.FormatInt(w.clk.Now().Add(ttl).Unix(), 10), "Content-Type": "application/json",
		}
		if collapse {
			want["Apns-Collapse-Id"] = n.ThreadID.String()
		}
		for k, v := range want {
			if got := h.Values(k); len(got) != 1 || got[0] != v {
				t.Errorf("%s: %s = %v, want %q", name, k, got, v)
			}
		}
		for k := range h {
			if _, ok := want[k]; !ok && !slices.Contains(transportHeaders, k) {
				t.Errorf("%s: unexpected header %s", name, k)
			}
		}
		if r.Method != http.MethodPost || r.Token != w.phone.Token || r.JWT == "" {
			t.Errorf("%s: %s %s", name, r.Method, r.Token)
		}
	}
	check("message", w.hint(), "10", true)
	quiet := w.hint()
	quiet.Quiet = true
	check("quiet", quiet, "5", true)
	for _, k := range []domain.Kind{domain.KindSecurity, domain.KindAdmin} {
		n := w.hint()
		n.Kind, n.ThreadID, n.MessageID = string(k), ids.ID{}, ids.ID{}
		check(string(k), n, "10", false)
	}
	check("sync", w.sync(2), "5", false)
}

func TestBadgeSyncForm(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	n := w.sync(2)
	w.send(w.phone, n)
	r := w.prod.Requests()[0]
	if string(r.Body) != `{"aps":{"badge":2}}` {
		t.Errorf("body %s", r.Body)
	}
	if r.Header.Get("Apns-Priority") != "5" || r.Header.Get("Apns-Collapse-Id") != "" || r.Header.Get("Apns-Id") != n.ID.String() {
		t.Errorf("headers %v", r.Header)
	}
	for _, c := range n.Cleared {
		for k, vs := range r.Header {
			if strings.Contains(strings.Join(vs, " "), c.String()) {
				t.Errorf("header %s carries a cleared thread", k)
			}
		}
		if bytes.Contains(r.Body, []byte(c.String())) {
			t.Error("body carries a cleared thread")
		}
	}
	w.send(w.phone, w.sync(0))
	if r := w.prod.Requests()[1]; string(r.Body) != `{"aps":{"badge":0}}` {
		t.Errorf("badge 0: body %s", r.Body)
	}
	w.prod.Script(w.phone.Token, apnstest.Response{Status: 410, Reason: "Unregistered"})
	if res := w.send(w.phone, w.sync(1)); res.Class != domain.ResultDisable || res.DisableReason != domain.DisableTokenInvalid {
		t.Errorf("410 on a sync: %+v", res)
	}
}

func TestPrechecksWithoutRequest(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	program := map[string]func(*app.Registration, *app.Notification){
		"provider fcm":        func(r *app.Registration, _ *app.Notification) { r.Provider = "fcm" },
		"environment staging": func(r *app.Registration, _ *app.Notification) { r.Environment = "staging" },
		"unknown kind":        func(_ *app.Registration, n *app.Notification) { n.Kind = "gossip" },
		"zero nid":            func(_ *app.Registration, n *app.Notification) { n.ID = ids.ID{} },
		"zero instance":       func(_ *app.Registration, n *app.Notification) { n.InstanceID = ids.ID{} },
		"hint without badge":  func(_ *app.Registration, n *app.Notification) { n.HasBadge = false },
		"sync without badge":  func(_ *app.Registration, n *app.Notification) { *n = w.sync(1); n.HasBadge = false },
	}
	for name, f := range program {
		reg, n := w.phone, w.hint()
		f(&reg, &n)
		if res := w.send(reg, n); res.Class != domain.ResultProgramError {
			t.Errorf("%s: %+v", name, res)
		}
	}
	for name, tok := range map[string]string{"empty": "", "non-hex": w.phone.Token[:10] + "zz", "path": "ab/cd"} {
		reg := w.phone
		reg.Token = tok
		if res := w.send(reg, w.hint()); res.Class != domain.ResultDisable || res.DisableReason != domain.DisableTokenInvalid {
			t.Errorf("token %s: %+v", name, res)
		}
	}
	reg := w.phone
	reg.AppID = "com.example.other"
	if res := w.send(reg, w.hint()); res.Class != domain.ResultDisable || res.DisableReason != domain.DisableTopicMismatch {
		t.Errorf("topic: %+v", res)
	}
	if n := len(w.prod.Requests()) + len(w.sand.Requests()); n != 0 {
		t.Errorf("%d requests at the Fakes", n)
	}
	if n := apns.JWTCreations(w.a, "production"); n != 0 {
		t.Errorf("%d JWTs created", n)
	}
}
