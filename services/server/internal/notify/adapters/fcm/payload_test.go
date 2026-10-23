// SPDX-License-Identifier: Apache-2.0

package fcm

import (
	"bytes"
	"encoding/json"
	"flag"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/notify/app"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

var update = flag.Bool("update", false, "rewrite testdata/payload_message.golden")

var (
	uuidPattern  = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	digits       = regexp.MustCompile(`^[0-9]+$`)
	ttlPattern   = regexp.MustCompile(`^[0-9]+s$`)
	hintDataKeys = []string{"v", "nid", "k", "i", "t", "m", "badge", "quiet"}
	syncDataKeys = []string{"v", "nid", "k", "i", "badge", "cleared"}
)

const samDevice = "fcm-token-of-sams-phone:APA91bEXAMPLE"

// hint returns a Notification of kind message with thread, message, nid, instance and badge 3.
func hint() app.Notification {
	return app.Notification{ID: ids.New(), InstanceID: ids.New(), ThreadID: ids.New(), MessageID: ids.New(),
		Kind: string(domain.KindMessage), Count: 1, Badge: 3, HasBadge: true}
}

// syncHint returns the badge sync of 13 8 with badge b and the cleared threads.
func syncHint(b int64, cleared ...ids.ID) app.Notification {
	return app.Notification{ID: ids.New(), InstanceID: ids.New(), Kind: app.KindSync, Badge: b, HasBadge: true, Cleared: cleared}
}

// variants returns every kind times Quiet times with and without message ID times the badges.
func variants(badges ...int64) []app.Notification {
	var out []app.Notification
	for _, k := range domain.Kinds() {
		for _, quiet := range []bool{false, true} {
			for _, msg := range []bool{false, true} {
				for _, b := range badges {
					n := hint()
					n.Kind, n.Quiet, n.Badge = string(k), quiet, b
					if !k.HasThread() {
						n.ThreadID = ids.ID{}
					}
					if !msg {
						n.MessageID = ids.ID{}
					}
					out = append(out, n)
				}
			}
		}
	}
	out = append(out, syncHint(badges[len(badges)-1], ids.New(), ids.New()), syncHint(badges[0]))
	return out
}

// build serializes n for the test token with push.ttl 24h; it fails the test when n does not fit.
func build(t *testing.T, n app.Notification) []byte {
	t.Helper()
	b, ok := buildMessage(samDevice, n, 24*time.Hour, nil)
	if !ok {
		t.Fatal("message does not fit")
	}
	return b
}

// keysOf returns the keys of the JSON object v.
func keysOf(t *testing.T, v any) []string {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("%v is not an object", v)
	}
	var out []string
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

func subset(keys, allowed []string) bool {
	for _, k := range keys {
		if !slices.Contains(allowed, k) {
			return false
		}
	}
	return true
}

// walk fails when a forbidden key appears at any depth.
func walk(t *testing.T, v any) {
	t.Helper()
	switch x := v.(type) {
	case map[string]any:
		for k, c := range x {
			if k == "collapse_key" || k == "notification" || k == "apns" || k == "webpush" || k == "fcm_options" {
				t.Fatalf("forbidden key %q", k)
			}
			walk(t, c)
		}
	case []any:
		for _, c := range x {
			walk(t, c)
		}
	}
}

func TestPayloadAllowList(t *testing.T) {
	for _, n := range variants(0, 3) {
		body := build(t, n)
		var root map[string]any
		if err := json.Unmarshal(body, &root); err != nil {
			t.Fatal(err)
		}
		walk(t, root)
		if got := keysOf(t, root); !slices.Equal(got, []string{"message"}) {
			t.Fatalf("top level keys %v", got)
		}
		msg := root["message"].(map[string]any)
		if got := keysOf(t, msg); !slices.Equal(got, []string{"android", "data", "token"}) {
			t.Fatalf("message keys %v", got)
		}
		if msg["token"] != samDevice {
			t.Fatal("token differs")
		}
		android := msg["android"].(map[string]any)
		if got := keysOf(t, android); !slices.Equal(got, []string{"priority", "ttl"}) {
			t.Fatalf("android keys %v", got)
		}
		if p := android["priority"]; p != "high" && p != "normal" {
			t.Fatalf("priority %v", p)
		}
		if !ttlPattern.MatchString(android["ttl"].(string)) {
			t.Fatalf("ttl %v", android["ttl"])
		}
		data := msg["data"].(map[string]any)
		keys := keysOf(t, data)
		allowed := hintDataKeys
		if n.Kind == app.KindSync {
			allowed = syncDataKeys
		}
		if !subset(keys, allowed) {
			t.Fatalf("data keys %v outside %v", keys, allowed)
		}
		for k, v := range data {
			s, ok := v.(string)
			if !ok {
				t.Fatalf("data.%s is not a string", k)
			}
			switch k {
			case "nid", "i", "t", "m":
				if !uuidPattern.MatchString(s) {
					t.Fatalf("data.%s = %q is no canonical UUID", k, s)
				}
			case "v", "badge", "quiet":
				if !digits.MatchString(s) {
					t.Fatalf("data.%s = %q is no number", k, s)
				}
			case "k":
				if s != app.KindSync && !domain.Kind(s).Valid() {
					t.Fatalf("data.k = %q", s)
				}
			case "cleared":
				for _, p := range strings.Split(s, ",") {
					if !uuidPattern.MatchString(p) {
						t.Fatalf("cleared %q", s)
					}
				}
			}
		}
		if data["badge"] != strconv.FormatInt(n.Badge, 10) {
			t.Fatalf("badge %v, want %d", data["badge"], n.Badge)
		}
		for _, k := range []string{"v", "nid", "k", "i"} {
			if _, ok := data[k]; !ok {
				t.Fatalf("data.%s missing", k)
			}
		}
		if n.Kind == string(domain.KindSecurity) || n.Kind == string(domain.KindAdmin) {
			if _, ok := data["t"]; ok {
				t.Fatalf("%s has a thread", n.Kind)
			}
		}
	}
	checkStructs(t, reflect.TypeFor[fcmMessage]())
}

// allowedStrings are the string fields of the payload structs: the token, the two enums of
// android and the data values that hold IDs, numbers and fixed keys.
var allowedStrings = []string{"Token", "Priority", "TTL", "V", "NID", "K", "I", "T", "M", "Badge", "Quiet", "Cleared"}

// checkStructs walks the payload structs: no map, no interface, no string field outside the
// allowed list.
func checkStructs(t *testing.T, typ reflect.Type) {
	t.Helper()
	switch typ.Kind() { //nolint:exhaustive // only containers need a check; scalars are allowed
	case reflect.Struct:
		for i := range typ.NumField() {
			f := typ.Field(i)
			if f.Type.Kind() == reflect.String && !slices.Contains(allowedStrings, f.Name) {
				t.Fatalf("string field %s.%s outside the allow list", typ.Name(), f.Name)
			}
			checkStructs(t, f.Type)
		}
	case reflect.Map, reflect.Interface:
		t.Fatalf("payload struct holds a %v", typ.Kind())
	case reflect.Pointer, reflect.Slice, reflect.Array:
		checkStructs(t, typ.Elem())
	}
}

// chapterExample returns the JSON block of 13 "6.2 FCM", walking up to the directory that holds
// docs/spec.
func chapterExample(t *testing.T) []byte {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if raw, err := os.ReadFile(filepath.Join(dir, "docs", "spec", "13-push-notifications.md")); err == nil { // #nosec G304 -- a fixed file of the repository
			s := string(raw)
			i := strings.Index(s, "### 6.2 FCM")
			if i < 0 {
				t.Fatal("section 6.2 FCM not found")
			}
			s = s[i:]
			start := strings.Index(s, "```json\n")
			end := strings.Index(s[start+8:], "```")
			return []byte(s[start+8 : start+8+end])
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("docs/spec not found")
		}
		dir = parent
	}
}

func TestPayloadMatchesChapterExample(t *testing.T) {
	var example struct {
		Message struct {
			Data struct {
				NID, K, I, T, M, Badge string
			} `json:"data"`
		} `json:"message"`
	}
	raw := chapterExample(t)
	if err := json.Unmarshal(raw, &example); err != nil {
		t.Fatal(err)
	}
	d := example.Message.Data
	badge, err := strconv.ParseInt(d.Badge, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	n := app.Notification{ID: ids.MustParse(d.NID), InstanceID: ids.MustParse(d.I), ThreadID: ids.MustParse(d.T),
		MessageID: ids.MustParse(d.M), Kind: d.K, Count: 1, Badge: badge, HasBadge: true}
	got := build(t, n)
	var want, have any
	if err := json.Unmarshal(bytes.Replace(raw, []byte(`"<registration token>"`), []byte(`"`+samDevice+`"`), 1), &want); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(got, &have); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, have) {
		t.Fatalf("message differs from 13 6.2:\n got %s\nwant %s", got, raw)
	}
	golden := filepath.Join("testdata", "payload_message.golden")
	if *update {
		if err := os.WriteFile(golden, append(got, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	g, err := os.ReadFile(golden) // #nosec G304 -- the golden file of this test
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.TrimSuffix(g, []byte("\n")), got) {
		t.Fatalf("golden differs:\n got %s\nwant %s", got, g)
	}
}

// decoded is the request body in the terms of the tests.
type decoded struct {
	Message struct {
		Android struct{ Priority, TTL string } `json:"android"`
		Data    map[string]string              `json:"data"`
	} `json:"message"`
}

func decode(t *testing.T, b []byte) decoded {
	t.Helper()
	var d decoded
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestPriorityAndTTL(t *testing.T) {
	for _, k := range domain.Kinds() {
		for _, quiet := range []bool{false, true} {
			n := hint()
			n.Kind, n.Quiet = string(k), quiet
			d := decode(t, build(t, n))
			want, q := "high", "0"
			if quiet {
				want, q = "normal", "1"
			}
			if d.Message.Android.Priority != want || d.Message.Data["quiet"] != q {
				t.Fatalf("%s quiet=%v: priority %q quiet %q", k, quiet, d.Message.Android.Priority, d.Message.Data["quiet"])
			}
		}
	}
	if d := decode(t, build(t, syncHint(1))); d.Message.Android.Priority != "normal" {
		t.Fatalf("sync priority %q", d.Message.Android.Priority)
	}
	for ttl, want := range map[time.Duration]string{24 * time.Hour: "86400s", 90 * time.Minute: "5400s", time.Second: "1s"} {
		b, ok := buildMessage(samDevice, hint(), ttl, nil)
		if !ok {
			t.Fatal("does not fit")
		}
		if got := decode(t, b).Message.Android.TTL; got != want {
			t.Fatalf("ttl %v gives %q, want %q", ttl, got, want)
		}
	}
}

func TestBadgeAndSyncForm(t *testing.T) {
	n := hint()
	n.Badge = 7
	if got := decode(t, build(t, n)).Message.Data["badge"]; got != "7" {
		t.Fatalf("badge %q", got)
	}
	t1, t2 := ids.New(), ids.New()
	s := syncHint(2, t1, t2)
	d := decode(t, build(t, s))
	want := map[string]string{"v": "1", "nid": s.ID.String(), "k": "sync", "i": s.InstanceID.String(), "badge": "2",
		"cleared": t1.String() + "," + t2.String()}
	if !reflect.DeepEqual(d.Message.Data, want) {
		t.Fatalf("sync data %v, want %v", d.Message.Data, want)
	}
	if d.Message.Android.Priority != "normal" {
		t.Fatalf("sync priority %q", d.Message.Android.Priority)
	}
	d = decode(t, build(t, syncHint(2)))
	if _, ok := d.Message.Data["cleared"]; ok {
		t.Fatal("cleared without threads")
	}
}

func TestDataSizeLimit(t *testing.T) {
	for _, n := range variants(math.MaxInt64) {
		b := build(t, n)
		var m struct {
			Message struct {
				Data json.RawMessage `json:"data"`
			} `json:"message"`
		}
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		if len(m.Message.Data) > fcmMaxDataBytes {
			t.Fatalf("data of %d bytes", len(m.Message.Data))
		}
	}
	if _, ok := buildMessage(samDevice, hint(), 24*time.Hour, func(d *fcmData) { d.NID = strings.Repeat("0", fcmMaxDataBytes) }); ok {
		t.Fatal("an oversize hint fits")
	}
	var many []ids.ID
	for range fcmMaxDataBytes / 36 {
		many = append(many, ids.New())
	}
	d := decode(t, build(t, syncHint(4, many...)))
	if _, ok := d.Message.Data["cleared"]; ok {
		t.Fatal("cleared sent beyond the limit")
	}
	if d.Message.Data["badge"] != "4" {
		t.Fatal("badge missing")
	}
}
