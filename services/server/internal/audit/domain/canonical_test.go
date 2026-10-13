// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/audit/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// goldenFile is the chain the tests recompute; its hashes were computed independently (see the
// comment member of the file).
const goldenFile = "testdata/chain_v1.json"

// row2Canonical is the canonical form of row 2 of the golden file, written out by hand: no
// optional column, empty details.
const row2Canonical = `{"action":"instance.bootstrap","decision":"allow","details":{},` +
	`"id":"01928f3a-7b1c-7000-8a00-000000000002","occurred_at":"2026-10-12T08:16:00.000Z",` +
	`"prev_hash":"7e2b6b7637943064977cc3da42e1c1d74ba2dc3ceadb121c331bf4d97707de25","seq":2}`

// goldenRows reads the golden file into rows and the stored hashes.
func goldenRows(t *testing.T) ([]domain.Row, []string) {
	t.Helper()
	raw, err := os.ReadFile(goldenFile)
	if err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var file struct {
		Rows []map[string]any `json:"rows"`
	}
	if err := dec.Decode(&file); err != nil {
		t.Fatal(err)
	}
	rows := make([]domain.Row, 0, len(file.Rows))
	hashes := make([]string, 0, len(file.Rows))
	for _, m := range file.Rows {
		rows = append(rows, rowFromEvent(t, m))
		hashes = append(hashes, m["hash"].(string))
	}
	return rows, hashes
}

// rowFromEvent converts the API representation AuditEvent into a Row.
func rowFromEvent(t *testing.T, m map[string]any) domain.Row {
	t.Helper()
	str := func(k string) string { s, _ := m[k].(string); return s }
	uuid := func(k string) ids.ID {
		s := str(k)
		if s == "" {
			return ids.ID{}
		}
		id, err := domain.ParseUUID(s)
		if err != nil {
			t.Fatalf("%s: %v", k, err)
		}
		return id
	}
	seq, err := m["seq"].(json.Number).Int64()
	if err != nil {
		t.Fatal(err)
	}
	at, err := time.Parse(time.RFC3339Nano, str("occurred_at"))
	if err != nil {
		t.Fatal(err)
	}
	prev, err := hex.DecodeString(str("prev_hash"))
	if err != nil || len(prev) != 32 {
		t.Fatalf("prev_hash: %v", err)
	}
	r := domain.Row{
		Seq: seq, ID: uuid("id"), OccurredAt: at,
		ActorPrincipalID: uuid("actor_principal_id"), ActorDeviceID: uuid("actor_device_id"),
		ActorSessionID: uuid("actor_session_id"), Action: str("action"), Decision: str("decision"),
		DenyReason: str("deny_reason"), ResourceKind: str("resource_kind"), ResourceID: uuid("resource_id"),
		Details: m["details"].(map[string]any), CorrelationID: uuid("correlation_id"),
	}
	copy(r.PrevHash[:], prev)
	return r
}

func TestCanonicalRow_Golden(t *testing.T) {
	t.Parallel()
	rows, hashes := goldenRows(t)
	if len(rows) < 3 {
		t.Fatalf("the golden file has %d rows, want at least 3", len(rows))
	}
	prev := domain.ZeroHash
	for i, r := range rows {
		if r.PrevHash != prev {
			t.Fatalf("row %d: prev_hash is not the hash of the row before", r.Seq)
		}
		c, err := domain.CanonicalRow(r)
		if err != nil {
			t.Fatalf("row %d: %v", r.Seq, err)
		}
		h := domain.ChainHash(prev, c)
		if got := hex.EncodeToString(h[:]); got != hashes[i] {
			t.Fatalf("row %d: hash %s, want %s\ncanonical: %s", r.Seq, got, hashes[i], c)
		}
		if r.Seq == 2 && string(c) != row2Canonical {
			t.Fatalf("row 2 canonical:\n got %s\nwant %s", c, row2Canonical)
		}
		prev = h
	}
}

func TestChainHash_FirstRowZeroPrevHash(t *testing.T) {
	t.Parallel()
	if domain.ZeroHash != ([32]byte{}) {
		t.Fatal("ZeroHash is not 32 zero bytes")
	}
	rows, _ := goldenRows(t)
	if rows[0].Seq != 1 || rows[0].PrevHash != domain.ZeroHash {
		t.Fatal("the first golden row does not chain to ZeroHash")
	}
	c, err := domain.CanonicalRow(rows[0])
	if err != nil {
		t.Fatal(err)
	}
	want := sha256.Sum256(append(make([]byte, 32), c...))
	if got := domain.ChainHash(domain.ZeroHash, c); got != want {
		t.Fatal("ChainHash is not SHA-256(prev_hash || canonical)")
	}
	if !strings.Contains(string(c), `"prev_hash":"`+strings.Repeat("0", 64)+`"`) {
		t.Fatalf("the canonical form of row 1 does not carry 64 zero hex digits: %s", c)
	}
}

// minimalRow returns a valid row without optional columns.
func minimalRow() domain.Row {
	return domain.Row{
		Seq: 7, ID: ids.MustParse("01928f3a-7b1c-7000-8a00-000000000007"),
		OccurredAt: time.Date(2026, 10, 12, 8, 0, 0, 0, time.UTC),
		Action:     "grant.create", Decision: "allow", Details: map[string]any{},
	}
}

func TestCanonicalRow_Escaping(t *testing.T) {
	t.Parallel()
	r := minimalRow()
	r.Details = map[string]any{"s": "q\" b\\ c\b\t\n\f\r \x01\x1f\x7f <>&    é/"}
	c, err := domain.CanonicalRow(r)
	if err != nil {
		t.Fatal(err)
	}
	want := `"details":{"s":"q\" b\\ c\b\t\n\f\r \u0001\u001f` + "\x7f <>&    é/" + `"}`
	if !strings.Contains(string(c), want) {
		t.Fatalf("canonical %s\ndoes not contain %s", c, want)
	}
}

func TestCanonicalRow_OmitsNull(t *testing.T) {
	t.Parallel()
	c, err := domain.CanonicalRow(minimalRow())
	if err != nil {
		t.Fatal(err)
	}
	for _, col := range []string{"actor_principal_id", "actor_device_id", "actor_session_id", "deny_reason",
		"resource_kind", "resource_id", "correlation_id", `"hash"`, "null"} {
		if strings.Contains(string(c), col) {
			t.Errorf("canonical form contains %s: %s", col, c)
		}
	}
	for _, col := range []string{`"details":{}`, `"seq":7`, `"decision":"allow"`} {
		if !strings.Contains(string(c), col) {
			t.Errorf("canonical form lacks %s: %s", col, c)
		}
	}
}

func TestCanonicalRow_KeyOrderIndependent(t *testing.T) {
	t.Parallel()
	decode := func(s string) map[string]any {
		dec := json.NewDecoder(strings.NewReader(s))
		dec.UseNumber()
		var m map[string]any
		if err := dec.Decode(&m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	a, b := minimalRow(), minimalRow()
	a.Details = decode(`{"zeta":1,"alpha":{"y":true,"b":"x"},"mid":[3,1]}`)
	b.Details = decode(`{"mid":[3,1],"alpha":{"b":"x","y":true},"zeta":1}`)
	ca, err := domain.CanonicalRow(a)
	if err != nil {
		t.Fatal(err)
	}
	cb, err := domain.CanonicalRow(b)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(ca, cb) {
		t.Fatalf("key order changed the canonical form:\n%s\n%s", ca, cb)
	}
	if !strings.Contains(string(ca), `"details":{"alpha":{"b":"x","y":true},"mid":[3,1],"zeta":1}`) {
		t.Fatalf("keys are not sorted: %s", ca)
	}
	if !strings.HasPrefix(string(ca), `{"action":`) || !strings.HasSuffix(string(ca), `"seq":7}`) {
		t.Fatalf("columns are not sorted: %s", ca)
	}
}

func TestCanonicalRow_RejectsOutOfDomain(t *testing.T) {
	t.Parallel()
	cases := map[string]func(r *domain.Row){
		"float":               func(r *domain.Row) { r.Details = map[string]any{"x": 1.5} },
		"float number text":   func(r *domain.Row) { r.Details = map[string]any{"x": json.Number("1.0")} },
		"exponent":            func(r *domain.Row) { r.Details = map[string]any{"x": json.Number("1e3")} },
		"null":                func(r *domain.Row) { r.Details = map[string]any{"x": nil} },
		"null in list":        func(r *domain.Row) { r.Details = map[string]any{"x": []any{nil}} },
		"non-ASCII key":       func(r *domain.Row) { r.Details = map[string]any{"é": "x"} },
		"int above 2^53-1":    func(r *domain.Row) { r.Details = map[string]any{"x": int64(1) << 53} },
		"number above":        func(r *domain.Row) { r.Details = map[string]any{"x": json.Number("9007199254740992")} },
		"int below -(2^53-1)": func(r *domain.Row) { r.Details = map[string]any{"x": -(int64(1) << 53)} },
		"seq above 2^53-1":    func(r *domain.Row) { r.Seq = 1 << 53 },
		"seq zero":            func(r *domain.Row) { r.Seq = 0 },
		"NUL in string":       func(r *domain.Row) { r.Details = map[string]any{"x": "a\x00b"} },
		"NUL in column":       func(r *domain.Row) { r.DenyReason = "a\x00" },
		"invalid UTF-8":       func(r *domain.Row) { r.Details = map[string]any{"x": "\xff"} },
		"unknown type":        func(r *domain.Row) { r.Details = map[string]any{"x": struct{}{}} },
		"nil details":         func(r *domain.Row) { r.Details = nil },
		"empty action":        func(r *domain.Row) { r.Action = "" },
		"sub-millisecond":     func(r *domain.Row) { r.OccurredAt = r.OccurredAt.Add(time.Microsecond) },
		"year 10000":          func(r *domain.Row) { r.OccurredAt = time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC) },
	}
	for name, mutate := range cases {
		r := minimalRow()
		mutate(&r)
		if c, err := domain.CanonicalRow(r); err == nil {
			t.Errorf("%s: accepted: %s", name, c)
		}
	}
}

func TestCanonicalRow_TimeHasMilliseconds(t *testing.T) {
	t.Parallel()
	for at, want := range map[time.Time]string{
		time.Date(2026, 10, 12, 8, 0, 0, 0, time.UTC):                                           "2026-10-12T08:00:00.000Z",
		time.Date(2026, 10, 12, 8, 0, 0, 120*int(time.Millisecond), time.UTC):                   "2026-10-12T08:00:00.120Z",
		time.Date(2026, 10, 12, 10, 0, 0, 5*int(time.Millisecond), time.FixedZone("x", 2*3600)): "2026-10-12T08:00:00.005Z",
	} {
		r := minimalRow()
		r.OccurredAt = at
		c, err := domain.CanonicalRow(r)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(c), `"occurred_at":"`+want+`"`) {
			t.Errorf("%v: canonical %s lacks %s", at, c, want)
		}
		if got := domain.FormatTime(at); got != want {
			t.Errorf("FormatTime(%v) = %s, want %s", at, got, want)
		}
	}
	if got := domain.TruncateTime(time.Date(2026, 1, 1, 0, 0, 0, 1_999_999, time.FixedZone("x", 3600))); got != time.Date(2025, 12, 31, 23, 0, 0, 1_000_000, time.UTC) {
		t.Fatalf("TruncateTime = %v", got)
	}
}

func TestParseUUID(t *testing.T) {
	t.Parallel()
	id, err := domain.ParseUUID("4bf92f35-77b3-4da6-a3ce-929d0e0e4736")
	if err != nil || id.String() != "4bf92f35-77b3-4da6-a3ce-929d0e0e4736" {
		t.Fatalf("ParseUUID of a version 4 UUID = %v, %v", id, err)
	}
	for _, bad := range []string{"", "4BF92F35-77b3-4da6-a3ce-929d0e0e4736", "4bf92f3577b34da6a3ce929d0e0e4736", "4bf92f35-77b3-4da6-a3ce-929d0e0e473g"} {
		if _, err := domain.ParseUUID(bad); err == nil {
			t.Errorf("ParseUUID(%q) accepted", bad)
		}
	}
}
