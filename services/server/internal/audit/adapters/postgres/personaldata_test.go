// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/OWNER/bothy/services/server/internal/audit/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/audit/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
)

// chainBuilder appends chained rows: every row links the hash of the one before it.
type chainBuilder struct {
	rows []domain.Row
	prev [domain.HashSize]byte
	at   time.Time
}

// add appends one row; actor zero is the system, details nil an empty object.
func (b *chainBuilder) add(actor ids.ID, action, decision, reason, kind string, resource ids.ID, details map[string]any) domain.Row {
	if details == nil {
		details = map[string]any{}
	}
	b.at = b.at.Add(time.Second)
	r := domain.Row{
		Seq: int64(len(b.rows) + 1), ID: ids.New(), OccurredAt: b.at, ActorPrincipalID: actor,
		Action: action, Decision: decision, DenyReason: reason, ResourceKind: kind, ResourceID: resource,
		Details: details, PrevHash: b.prev,
	}
	if !actor.IsZero() {
		r.ActorDeviceID, r.ActorSessionID, r.CorrelationID = ids.New(), ids.New(), ids.New()
	}
	c, err := domain.CanonicalRow(r)
	if err != nil {
		panic(err)
	}
	r.Hash = domain.ChainHash(b.prev, c)
	b.prev = r.Hash
	b.rows = append(b.rows, r)
	return r
}

// personalFixture is a log with entries of Alex (the subject), Sam and Robin.
type personalFixture struct {
	e                *env
	alex, sam, robin ids.ID
	// alexSeqs are the seqs of the entries with Alex as actor or as user/principal resource.
	alexSeqs []int64
}

// pagedRows is above one page of the export, so the keyset paging is exercised.
const pagedRows = 1200

func newPersonalFixture(t *testing.T) *personalFixture {
	t.Helper()
	f := &personalFixture{e: newEnv(t, 4), alex: ids.New(), sam: ids.New(), robin: ids.New()}
	b := &chainBuilder{at: time.Date(2026, 10, 12, 8, 0, 0, 0, time.UTC)}
	mark := func(r domain.Row) { f.alexSeqs = append(f.alexSeqs, r.Seq) }
	ip := map[string]any{"ip_hmac": strings.Repeat("ab", 32), "ip_key_id": ids.New().String()}
	// Alex acts: allowed, and denied with a reason code.
	mark(b.add(f.alex, "message.create", "allow", "", "message", ids.New(), ip))
	mark(b.add(f.alex, "grant.create", "deny", "no_grant", "grant", ids.New(), ip))
	// Sam (an owner) disables Alex with a free-text reason: Sam's text must not leave.
	mark(b.add(f.sam, "user.disable", "allow", "", "user", f.alex, map[string]any{"reason": "Sam wrote this about Alex", "ip_hmac": strings.Repeat("cd", 32)}))
	// The system acts on Alex's principal.
	mark(b.add(ids.ID{}, "gdpr.request.decide", "allow", "", "principal", f.alex, map[string]any{"code": "approved"}))
	// Entries of others only: Sam on Sam, Robin on a grant, Sam on Robin, a message with Alex's ID
	// as resource of another kind (no person reference).
	b.add(f.sam, "user.update", "allow", "", "user", f.sam, nil)
	b.add(f.robin, "grant.create", "allow", "", "grant", ids.New(), nil)
	b.add(f.sam, "user.disable", "allow", "", "principal", f.robin, map[string]any{"reason": "about Robin"})
	b.add(f.robin, "message.read", "allow", "", "message", f.alex, nil)
	// A long history across several pages, interleaved with Sam.
	for i := range pagedRows {
		if i%2 == 0 {
			mark(b.add(f.alex, "message.read", "allow", "", "message", ids.New(), nil))
		} else {
			b.add(f.sam, "message.read", "allow", "", "message", ids.New(), nil)
		}
	}
	ctx := context.Background()
	if err := f.e.uow.Do(ctx, func(ctx context.Context) error { return f.e.repo.InsertRows(ctx, b.rows) }); err != nil {
		t.Fatal(err)
	}
	return f
}

// auditSchema compiles docs/api/gdpr-export/v1/lines/audit.schema.json.
func auditSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(file), "../../../../../../docs/api/gdpr-export/v1/lines/audit.schema.json")
	raw, err := os.ReadFile(path) //nolint:gosec // G304: a fixed path of the repository's schema files.
	if err != nil {
		t.Fatal(err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	if err := c.AddResource("audit.schema.json", doc); err != nil {
		t.Fatal(err)
	}
	s, err := c.Compile("audit.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestPersonalDataExport(t *testing.T) {
	t.Parallel()
	f := newPersonalFixture(t)
	p := postgres.NewPersonalData(f.e.pool)
	if p.Module() != "audit" || !slices.Equal(p.Tables(), []string{"audit_events"}) {
		t.Fatalf("Module %q, Tables %v", p.Module(), p.Tables())
	}
	w := personaldata.NewMemoryWriter()
	if err := p.Export(context.Background(), personaldata.Export{Subject: f.alex}, w); err != nil {
		t.Fatal(err)
	}
	if len(w.Sections) != 0 || len(w.Contents) != 0 || len(w.Lines) != 1 {
		t.Fatalf("wrote outside audit.jsonl: sections %d, contents %d, files %d", len(w.Sections), len(w.Contents), len(w.Lines))
	}
	lines := w.Lines[personaldata.FileAudit]
	if len(lines) != len(f.alexSeqs) {
		t.Fatalf("%d lines, want %d", len(lines), len(f.alexSeqs))
	}
	schema := auditSchema(t)
	allowed := []string{"seq", "occurred_at", "action", "decision", "reason_code", "resource_kind", "resource_id", "actor_kind", "actor_id"}
	for i, raw := range lines {
		for _, banned := range []string{"hash", "ip_", "signature", "details", "Sam wrote", "about Robin", "device", "session", "correlation", "null"} {
			if bytes.Contains(raw, []byte(banned)) {
				t.Fatalf("line %d holds %q: %s", i, banned, raw)
			}
		}
		v, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		if err := schema.Validate(v); err != nil {
			t.Fatalf("line %d does not match the schema: %v\n%s", i, err, raw)
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		for k := range m {
			if !slices.Contains(allowed, k) {
				t.Fatalf("line %d has the undocumented member %q", i, k)
			}
		}
		if int64(m["seq"].(float64)) != f.alexSeqs[i] {
			t.Fatalf("line %d has seq %v, want %d (ascending, Alex only)", i, m["seq"], f.alexSeqs[i])
		}
		if m["actor_id"] != nil && m["actor_id"] != f.alex.String() && m["resource_id"] != f.alex.String() {
			t.Fatalf("line %d names neither Alex as actor nor as resource: %s", i, raw)
		}
	}
	var first, second, third, fourth map[string]any
	for i, dst := range []*map[string]any{&first, &second, &third, &fourth} {
		if err := json.Unmarshal(lines[i], dst); err != nil {
			t.Fatal(err)
		}
	}
	if first["actor_kind"] != "principal" || first["actor_id"] != f.alex.String() || first["occurred_at"] != "2026-10-12T08:00:01Z" || first["reason_code"] != nil {
		t.Fatalf("first line %v", first)
	}
	if second["decision"] != "deny" || second["reason_code"] != "no_grant" || second["resource_kind"] != "grant" {
		t.Fatalf("second line %v", second)
	}
	if third["actor_id"] != f.sam.String() || third["resource_kind"] != "user" || third["resource_id"] != f.alex.String() || third["action"] != "user.disable" {
		t.Fatalf("third line %v", third)
	}
	if fourth["actor_kind"] != "system" || fourth["actor_id"] != nil || fourth["resource_kind"] != "principal" {
		t.Fatalf("fourth line %v", fourth)
	}

	// A person without entries gets no line and no error.
	empty := personaldata.NewMemoryWriter()
	if err := p.Export(context.Background(), personaldata.Export{Subject: ids.New()}, empty); err != nil || len(empty.Lines) != 0 {
		t.Fatalf("export of a person without entries: %v, %d files", err, len(empty.Lines))
	}
}

// snapshot returns every column of every row of audit_events and audit_checkpoints, in order.
func snapshot(t *testing.T, f *personalFixture) string {
	t.Helper()
	var s string
	err := f.e.pool.QueryRow(context.Background(), `
SELECT coalesce((SELECT string_agg(t::text, E'\n' ORDER BY t.seq) FROM audit_events t), '') || '|' ||
       coalesce((SELECT string_agg(c::text, E'\n' ORDER BY c.seq) FROM audit_checkpoints c), '')`).Scan(&s)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// verifyChain recomputes every hash of audit_events from row 1.
func verifyChain(t *testing.T, f *personalFixture) {
	t.Helper()
	first, head, ok, err := f.e.repo.Bounds(context.Background())
	if err != nil || !ok || first != 1 {
		t.Fatalf("Bounds = %d, %d, %v, %v", first, head, ok, err)
	}
	rows, err := f.e.repo.Rows(context.Background(), first, head)
	if err != nil {
		t.Fatal(err)
	}
	prev := domain.ZeroHash
	for _, r := range rows {
		c, err := domain.CanonicalRow(r)
		if err != nil {
			t.Fatal(err)
		}
		if r.PrevHash != prev || domain.ChainHash(prev, c) != r.Hash {
			t.Fatalf("the chain breaks at seq %d", r.Seq)
		}
		prev = r.Hash
	}
}

func TestPersonalDataErase(t *testing.T) {
	t.Parallel()
	for _, mode := range []personaldata.ContentMode{personaldata.KeepPseudonymized, personaldata.DeleteContent} {
		t.Run(string(mode), func(t *testing.T) {
			t.Parallel()
			f := newPersonalFixture(t)
			p := postgres.NewPersonalData(f.e.pool)
			before := snapshot(t, f)
			e := personaldata.Erasure{Subject: f.alex, RequestID: ids.New(), DecidedBy: f.sam, Content: mode}
			// Twice: the second run is the resume after a crash.
			for range 2 {
				for _, step := range personaldata.ParticipantSteps() {
					if err := p.Erase(context.Background(), step, e); err != nil {
						t.Fatalf("step %s: %v", step, err)
					}
				}
			}
			if after := snapshot(t, f); after != before {
				t.Fatal("Erase changed audit_events or audit_checkpoints")
			}
			verifyChain(t, f)
		})
	}
}
