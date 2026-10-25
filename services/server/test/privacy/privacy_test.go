// SPDX-License-Identifier: Apache-2.0

// Package privacy_test holds the integration tests of brief M7-06: the real server of
// internal/wire in role all through test/harness on a fake clock, PostgreSQL from pgtest. River
// never runs the jobs of the module within a test: the hook of the job modules keeps the use
// cases and gives River no-ops, and the test runs the export, the deletion, the expiry and the
// reminder itself with the controlled clock.
package privacy_test

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
	privacyjobs "github.com/OWNER/bothy/services/server/internal/privacy/adapters/jobs"
	"github.com/OWNER/bothy/services/server/internal/wire"
	"github.com/OWNER/bothy/services/server/test/harness"
)

func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }

// schemaDir is docs/api/gdpr-export/v1 relative to this package.
var schemaDir = filepath.Join("..", "..", "..", "..", "docs", "api", "gdpr-export", "v1")

// world is one test server with the use cases of the privacy jobs.
type world struct {
	t   *testing.T
	s   *harness.Server
	clk *clock.Fake
	db  *pgxpool.Pool
	w   *harness.World

	mu   sync.Mutex
	jobs privacyjobs.UseCases
}

// newWorld starts the server with cfg (dotted keys) and the participant hook wrap (may be nil),
// and builds the example world of 07.
func newWorld(t *testing.T, cfg map[string]string, wrap func([]personaldata.Participant) []personaldata.Participant) *world {
	t.Helper()
	all := harness.RaisedRateLimits()
	for k, v := range cfg {
		all[k] = v
	}
	// The clock moves by days; the clients sign their proofs with the real time.
	all["auth.clock_leeway"], all["auth.dpop_proof_max_age"] = "96h", "96h"
	w := &world{t: t, clk: clock.NewFake(time.Now().UTC().Truncate(time.Millisecond))}
	w.s = harness.Start(t, harness.Options{Config: all, Clock: w.clk, EgressDenyAll: true, JobModules: w.take, PersonalData: wrap})
	w.db = w.s.AdminDB()
	w.w = harness.BuildMatrixWorld(t, w.s)
	return w
}

// take keeps the use cases of the privacy jobs and gives River no-ops in their place.
func (w *world) take(m *wire.JobModules) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.jobs = m.Privacy
	m.Privacy = noJobs{}
}

// noJobs is the River side of the privacy jobs: it does nothing.
type noJobs struct{}

func (noJobs) RunExport(context.Context, ids.ID) error          { return nil }
func (noJobs) RunErase(context.Context, ids.ID) error           { return nil }
func (noJobs) Fail(context.Context, ids.ID, string) error       { return nil }
func (noJobs) ExpirePackages(context.Context, int) (int, error) { return 0, nil }
func (noJobs) Remind(context.Context, int) (int, error)         { return 0, nil }

func (w *world) uc() privacyjobs.UseCases {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.jobs
}

func (w *world) scalar(sql string, args ...any) string {
	w.t.Helper()
	var v string
	if err := w.db.QueryRow(w.t.Context(), sql, args...).Scan(&v); err != nil {
		w.t.Fatalf("%s: %v", sql, err)
	}
	return v
}

// post posts text as c into thread and returns the message id.
func post(t *testing.T, c *harness.Client, thread, text string) string {
	t.Helper()
	var m struct {
		ID string `json:"id"`
	}
	harness.MustDo(t, c, harness.Request{Method: http.MethodPost, Path: "/threads/" + thread + "/messages",
		Body: map[string]any{"client_msg_id": ids.New().String(), "content_md": text}}, http.StatusCreated).JSON(t, &m)
	return m.ID
}

// request is the part of the schema GdprRequest the tests read.
type request struct {
	ID              string `json:"id"`
	State           string `json:"state"`
	Kind            string `json:"kind"`
	ApprovedBy      string `json:"approved_by"`
	ResultExpiresAt string `json:"result_expires_at"`
	Error           string `json:"error"`
	Progress        struct {
		LastStep int `json:"last_step"`
	} `json:"progress"`
}

// create files a request as c and returns it.
func create(t *testing.T, c *harness.Client, body map[string]any) request {
	t.Helper()
	var r request
	harness.MustDo(t, c, harness.Request{Method: http.MethodPost, Path: "/gdpr-requests", Body: body}, http.StatusCreated).JSON(t, &r)
	return r
}

// get reads the request id as c.
func get(t *testing.T, c *harness.Client, id string) request {
	t.Helper()
	var r request
	harness.MustDo(t, c, harness.Request{Method: http.MethodGet, Path: "/gdpr-requests/" + id}, http.StatusOK).JSON(t, &r)
	return r
}

// export files and runs the export of Alex and returns the request and the archive.
func (w *world) export(alex *harness.Client) (request, []byte) {
	w.t.Helper()
	r := create(w.t, alex, map[string]any{"kind": "export", "reason": "Request by the person"})
	if r.State != "approved" || r.ApprovedBy != "" {
		w.t.Fatalf("own export not approved by the system: %+v", r)
	}
	if err := w.uc().RunExport(w.t.Context(), ids.MustParse(r.ID)); err != nil {
		w.t.Fatal(err)
	}
	r = get(w.t, alex, r.ID)
	if r.State != "completed" || r.ResultExpiresAt == "" {
		w.t.Fatalf("export = %+v", r)
	}
	resp := harness.MustDo(w.t, alex, harness.Request{Method: http.MethodGet, Path: "/gdpr-requests/" + r.ID + "/export"}, http.StatusOK)
	return r, resp.Body
}

// unzip returns the files of the archive by path and the order of the entries.
func unzip(t *testing.T, raw []byte) (map[string][]byte, []string) {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]byte{}
	var order []string
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		out[f.Name], order = b, append(order, f.Name)
	}
	return out, order
}

// schema compiles the schema file name of docs/api/gdpr-export/v1.
func schema(t *testing.T, name string) *jsonschema.Schema {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join(schemaDir, name))
	if err != nil {
		t.Fatal(err)
	}
	s, err := jsonschema.NewCompiler().Compile(abs)
	if err != nil {
		t.Fatalf("schema %s: %v", name, err)
	}
	return s
}

// validate checks b (one JSON value) against the schema file name.
func validate(t *testing.T, name string, b []byte) {
	t.Helper()
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("%s: %v: %s", name, err, b)
	}
	if err := schema(t, name).Validate(v); err != nil {
		t.Fatalf("%s does not validate: %v\n%s", name, err, b)
	}
}

// TestDPT03_Export (DP-T03): the package of Alex validates against the JSON Schemas of
// docs/api/gdpr-export/v1, every checksum of the manifest matches, it holds no message of another
// person, no secret and no hash, and a channel Alex may no longer read appears only by its ID.
func TestDPT03_Export(t *testing.T) {
	t.Parallel()
	w := newWorld(t, nil, nil)
	o, alex, sam := w.s.Owner, w.w.People["Alex"], w.w.People["Sam"]
	// Alex may write in Engineering/Werkzeuge as well; Support is taken away after Alex wrote.
	harness.MustDo(t, o, harness.Request{Method: http.MethodPost, Path: "/grants", Body: map[string]any{
		"subject_kind": "user", "subject_id": alex.PrincipalID, "target_kind": "channel",
		"target_id": w.w.Channels["Engineering/Werkzeuge"], "level": "write", "reason": "test",
	}}, http.StatusCreated)
	lost := post(t, alex, w.w.Threads["Support/Allgemein"], "alex-own-text-in-support")
	kept := post(t, alex, w.w.Threads["Engineering/Werkzeuge"], "alex-own-text-in-werkzeuge")
	post(t, sam, w.w.Threads["Marketing/Kampagnen"], "sam-foreign-text")
	post(t, o, w.w.Threads["Engineering/Werkzeuge"], "owner-foreign-text")
	harness.MustDo(t, o, harness.Request{Method: http.MethodPost, Path: "/grants/" + w.w.Grants["G-1"] + "/revoke",
		Body: map[string]any{"reason": "test"}}, http.StatusOK)

	_, raw := w.export(alex)
	files, order := unzip(t, raw)
	if order[len(order)-1] != "manifest.json" {
		t.Fatalf("manifest.json is not last: %v", order)
	}
	validate(t, "manifest.schema.json", files["manifest.json"])
	var m struct {
		SchemaVersion int    `json:"schema_version"`
		SubjectUserID string `json:"subject_user_id"`
		Files         []struct {
			Path   string `json:"path"`
			Size   int64  `json:"size"`
			SHA256 string `json:"sha256"`
		} `json:"files"`
	}
	if err := json.Unmarshal(files["manifest.json"], &m); err != nil {
		t.Fatal(err)
	}
	if m.SchemaVersion != 1 || m.SubjectUserID != alex.PrincipalID || len(m.Files) != len(files)-1 {
		t.Fatalf("manifest = %+v", m)
	}
	for _, f := range m.Files {
		sum := sha256.Sum256(files[f.Path])
		if hex.EncodeToString(sum[:]) != f.SHA256 || int64(len(files[f.Path])) != f.Size {
			t.Errorf("checksum of %s does not match", f.Path)
		}
	}
	for _, f := range personaldata.Files() {
		b, ok := files[string(f)]
		if !ok {
			t.Fatalf("%s missing", f)
		}
		stem := strings.TrimSuffix(filepath.Base(string(f)), filepath.Ext(string(f)))
		if !f.Lines() {
			validate(t, stem+".schema.json", b)
			continue
		}
		for _, line := range bytes.Split(bytes.TrimSpace(b), []byte("\n")) {
			if len(line) > 0 {
				validate(t, filepath.Join("lines", stem+".schema.json"), line)
			}
		}
	}
	all := string(raw)
	for _, z := range files {
		all += string(z)
	}
	for _, foreign := range []string{"sam-foreign-text", "owner-foreign-text", "Second message in"} {
		if strings.Contains(all, foreign) {
			t.Errorf("the package holds a message of another person: %q", foreign)
		}
	}
	access, refresh := alex.Tokens()
	for _, secret := range []string{access, refresh, "$argon2", "BEGIN " + "PRIVATE KEY"} {
		if secret != "" && strings.Contains(all, secret) {
			t.Errorf("the package holds a secret (%d bytes)", len(secret))
		}
	}
	for path, b := range files {
		if strings.HasSuffix(path, ".json") || strings.HasSuffix(path, ".jsonl") {
			for _, k := range forbiddenKeys(t, b) {
				t.Errorf("%s holds the member %q", path, k)
			}
		}
	}
	msgs := map[string]map[string]any{}
	for _, line := range bytes.Split(bytes.TrimSpace(files["messages.jsonl"]), []byte("\n")) {
		var v map[string]any
		if err := json.Unmarshal(line, &v); err != nil {
			t.Fatal(err)
		}
		msgs[fmt.Sprint(v["id"])] = v
	}
	if _, ok := msgs[lost]["channel_name"]; ok || msgs[lost]["channel_id"] != w.w.Channels["Support/Allgemein"] {
		t.Errorf("the message in the channel Alex lost = %v", msgs[lost])
	}
	if name, _ := msgs[kept]["channel_name"].(string); name != "Werkzeuge" {
		t.Errorf("the message in the readable channel = %v", msgs[kept])
	}
}

// forbiddenKeys returns the members of the JSON document(s) b whose name says secret, hash or key.
func forbiddenKeys(t *testing.T, b []byte) []string {
	t.Helper()
	var out []string
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, c := range x {
				l := strings.ToLower(k)
				if strings.Contains(l, "hash") || strings.Contains(l, "secret") || strings.Contains(l, "password") ||
					strings.Contains(l, "public_key") || strings.Contains(l, "private") || l == "token" || strings.HasSuffix(l, "_token") ||
					strings.Contains(l, "ciphertext") || strings.Contains(l, "ip_hmac") {
					out = append(out, k)
				}
				walk(c)
			}
		case []any:
			for _, c := range x {
				walk(c)
			}
		}
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	for dec.More() {
		var v any
		if err := dec.Decode(&v); err != nil {
			t.Fatal(err)
		}
		walk(v)
	}
	return out
}

// TestDPT04_ExportExpiry (DP-T04): after privacy.export_ttl the job privacy.export_expiry deletes
// the package (the file is marked deleted, result_file_id cleared) and the download answers 410.
func TestDPT04_ExportExpiry(t *testing.T) {
	t.Parallel()
	// A short privacy.export_ttl keeps the sessions of the clients valid while the clock moves.
	w := newWorld(t, map[string]string{"privacy.export_ttl": "1h"}, nil)
	alex := w.w.People["Alex"]
	r, _ := w.export(alex)
	fileID := w.scalar(`SELECT result_file_id::text FROM gdpr_requests WHERE id = $1`, r.ID)
	const ttl = time.Hour
	// Just before the end the package stays.
	w.clk.Advance(ttl - time.Minute)
	if n, err := w.uc().ExpirePackages(t.Context(), 10); err != nil || n != 0 {
		t.Fatalf("early expiry = %d, %v", n, err)
	}
	w.clk.Advance(time.Minute)
	if n, err := w.uc().ExpirePackages(t.Context(), 10); err != nil || n != 1 {
		t.Fatalf("expiry = %d, %v", n, err)
	}
	if got := w.scalar(`SELECT (deleted_at IS NOT NULL)::text FROM files WHERE id = $1`, fileID); got != "true" {
		t.Fatal("the package file is not deleted")
	}
	if got := w.scalar(`SELECT (result_file_id IS NULL)::text FROM gdpr_requests WHERE id = $1`, r.ID); got != "true" {
		t.Fatal("result_file_id stays")
	}
	harness.MustDo(t, alex, harness.Request{Method: http.MethodGet, Path: "/gdpr-requests/" + r.ID + "/export"}, http.StatusGone)
}

// crasher is a participant that fails once in step 4 and records every step it was called in.
type crasher struct {
	mu     sync.Mutex
	failed bool
	steps  []personaldata.Step
}

func (*crasher) Module() string   { return "crasher" }
func (*crasher) Tables() []string { return []string{"crasher_test_table"} }
func (*crasher) Export(context.Context, personaldata.Export, personaldata.Writer) error {
	return nil
}

func (c *crasher) Erase(_ context.Context, step personaldata.Step, _ personaldata.Erasure) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.steps = append(c.steps, step)
	if step == personaldata.StepAccess && !c.failed {
		c.failed = true
		return fmt.Errorf("crash after step 3")
	}
	return nil
}

func (c *crasher) seen() []personaldata.Step {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.steps)
}

// TestDPT02_Deletion (DP-T02) for both values of privacy.account_deletion_content: the owner
// approves the deletion of Alex, the job crashes in step 4 and resumes there; afterwards Alex is
// disabled with no clear data in principals and users, the security and personal data are gone,
// the grants revoked, the messages kept or deleted by the mode, and audit.verify passes.
func TestDPT02_Deletion(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"keep_pseudonymized", "delete"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			c := &crasher{}
			w := newWorld(t, map[string]string{"privacy.account_deletion_content": mode},
				func(ps []personaldata.Participant) []personaldata.Participant { return append(ps, c) })
			o, alex := w.s.Owner, w.w.People["Alex"]
			msg := post(t, alex, w.w.Threads["Support/Allgemein"], "alex text to delete")
			harness.MustDo(t, alex, harness.Request{Method: http.MethodPut, Path: "/me/settings/composer.enter_sends",
				Body: map[string]any{"value": true}}, http.StatusOK)
			r := create(t, alex, map[string]any{"kind": "deletion", "reason": "Delete my account"})
			if r.State != "requested" {
				t.Fatalf("deletion = %+v", r)
			}
			var d request
			harness.MustDo(t, o, harness.Request{Method: http.MethodPost, Path: "/gdpr-requests/" + r.ID + "/decide",
				Body: map[string]any{"decision": "approve", "reason": "No retention duty"}}, http.StatusOK).JSON(t, &d)
			if d.State != "approved" {
				t.Fatalf("decision = %+v", d)
			}
			id := ids.MustParse(r.ID)
			if err := w.uc().RunErase(t.Context(), id); err == nil {
				t.Fatal("the crash in step 4 was not reported")
			}
			if got := w.scalar(`SELECT state || ':' || (progress->>'last_step') FROM gdpr_requests WHERE id = $1`, r.ID); got != "running:3" {
				t.Fatalf("after the crash: %s", got)
			}
			if err := w.uc().RunErase(t.Context(), id); err != nil {
				t.Fatal(err)
			}
			want := []personaldata.Step{1, 2, 3, 4, 4, 5, 6}
			if got := c.seen(); !slices.Equal(got, want) {
				t.Fatalf("steps = %v, want %v (resume at step 4)", got, want)
			}
			if got := w.scalar(`SELECT state FROM gdpr_requests WHERE id = $1`, r.ID); got != "completed" {
				t.Fatalf("state = %s", got)
			}
			// Step 1 and 6: disabled, pseudonymized, no clear data.
			row := w.scalar(`SELECT p.status || '|' || p.handle || '|' || p.display_name || '|' || (p.avatar_file_id IS NULL)::text || '|' ||
				(p.deleted_at IS NOT NULL)::text || '|' || COALESCE(u.email, '<null>') FROM principals p JOIN users u ON u.principal_id = p.id
				WHERE p.id = $1`, alex.PrincipalID)
			parts := strings.Split(row, "|")
			if parts[0] != "deleted" || !strings.HasPrefix(parts[1], "deleted-") || !strings.HasPrefix(parts[2], "Deleted person") ||
				parts[3] != "true" || parts[4] != "true" || parts[5] != "<null>" {
				t.Fatalf("principal and user = %s", row)
			}
			if strings.Contains(strings.ToLower(row), "alex") {
				t.Fatalf("clear data stays: %s", row)
			}
			// Step 2 and 3.
			for _, q := range []string{
				`SELECT count(*)::text FROM auth_sessions WHERE user_id = $1`,
				`SELECT count(*)::text FROM devices WHERE user_id = $1`,
				`SELECT count(*)::text FROM user_identities WHERE user_id = $1`,
				`SELECT count(*)::text FROM user_settings WHERE user_id = $1`,
			} {
				if n := w.scalar(q, alex.PrincipalID); n != "0" {
					t.Errorf("%s = %s", q, n)
				}
			}
			// Step 4: the grants of Alex are revoked, the rows stay.
			if n := w.scalar(`SELECT count(*)::text FROM grants WHERE subject_principal_id = $1 AND revoked_at IS NULL`, alex.PrincipalID); n != "0" {
				t.Errorf("open grants = %s", n)
			}
			if n := w.scalar(`SELECT count(*)::text FROM grants WHERE subject_principal_id = $1`, alex.PrincipalID); n == "0" {
				t.Error("the grant rows are gone")
			}
			// Step 5.
			deleted := w.scalar(`SELECT (deleted_at IS NOT NULL)::text || ':' || content_md FROM messages WHERE id = $1`, msg)
			if mode == "delete" && deleted != "true:" || mode == "keep_pseudonymized" && deleted != "false:alex text to delete" {
				t.Errorf("message after %s = %q", mode, deleted)
			}
			// Step 7 and the chain.
			if n := w.scalar(`SELECT count(*)::text FROM audit_events WHERE action = 'privacy.erase' AND resource_id = $1`, alex.PrincipalID); n != "1" {
				t.Errorf("audit entries privacy.erase = %s", n)
			}
			var v struct {
				OK bool `json:"ok"`
			}
			harness.MustDo(t, o, harness.Request{Method: http.MethodPost, Path: "/audit/verify", Body: map[string]any{}}, http.StatusOK).JSON(t, &v)
			if !v.OK {
				t.Fatalf("audit.verify = %+v", v)
			}
		})
	}
}

// TestReminder (R-AZ-35): when due_at minus privacy.request_reminder_lead has passed, every active
// owner gets one gdpr.request_due for an open request, once, with only its ID, kind and deadline.
func TestReminder(t *testing.T) {
	t.Parallel()
	w := newWorld(t, map[string]string{"privacy.export_requires_approval": "true"}, nil)
	o, alex, robin := w.s.Owner, w.w.People["Alex"], w.w.People["Robin"]
	// A second owner may trigger the personal agent of the owner (I1): it becomes an organization
	// agent first.
	assistent := w.w.Agents["Assistent"].PrincipalID
	var agent struct {
		Version int64 `json:"version"`
	}
	harness.MustDo(t, o, harness.Request{Method: http.MethodGet, Path: "/agents/" + assistent}, http.StatusOK).JSON(t, &agent)
	harness.MustDo(t, o, harness.Request{Method: http.MethodPut, Path: "/agents/" + assistent + "/billing-mode",
		Header: http.Header{"If-Match": {fmt.Sprintf("%q", fmt.Sprint(agent.Version))}},
		Body:   map[string]any{"billing_mode": "organization", "reason": "test"}}, http.StatusOK)
	harness.MustDo(t, o, harness.Request{Method: http.MethodPost, Path: "/owners",
		Body: map[string]any{"user_id": robin.PrincipalID, "reason": "second owner"}}, http.StatusCreated)
	r := create(t, alex, map[string]any{"kind": "export", "reason": "Request by the person"})
	if n, err := w.uc().Remind(t.Context(), 10); err != nil || n != 0 {
		t.Fatalf("reminder before the lead = %d, %v", n, err)
	}
	w.clk.Advance(30 * 24 * time.Hour)
	for range 2 {
		if _, err := w.uc().Remind(t.Context(), 10); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := w.db.Query(t.Context(), `SELECT audience_principal_id::text, payload::text FROM event_log
		WHERE type = 'gdpr.request_due' ORDER BY seq`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var audiences []string
	for rows.Next() {
		var aud, payload string
		if err := rows.Scan(&aud, &payload); err != nil {
			t.Fatal(err)
		}
		var p map[string]any
		if err := json.Unmarshal([]byte(payload), &p); err != nil {
			t.Fatal(err)
		}
		if len(p) != 3 || p["request_id"] != r.ID || p["kind"] != "export" || p["due_at"] == nil {
			t.Fatalf("payload = %s", payload)
		}
		audiences = append(audiences, aud)
	}
	slices.Sort(audiences)
	want := []string{o.PrincipalID, robin.PrincipalID}
	slices.Sort(want)
	if !slices.Equal(audiences, want) {
		t.Fatalf("reminders to %v, want one each to %v", audiences, want)
	}
}

// TestHandlers: step-up for the writes, self or owner for the reads, 409 for a second open
// request, an owner's decision and a person who is no owner.
func TestHandlers(t *testing.T) {
	t.Parallel()
	w := newWorld(t, nil, nil)
	ctx := t.Context()
	o, alex, sam := w.s.Owner, w.w.People["Alex"], w.w.People["Sam"]
	first := alex.Do(ctx, harness.Request{Method: http.MethodPost, Path: "/gdpr-requests", Header: http.Header{"Idempotency-Key": {ids.New().String()}},
		Body: map[string]any{"kind": "deletion", "reason": "Delete my account"}})
	if first.Status != http.StatusUnauthorized || !strings.Contains(string(first.Body), "step_up_required") {
		t.Fatalf("create without step-up = %d %s", first.Status, first.Body)
	}
	r := create(t, alex, map[string]any{"kind": "deletion", "reason": "Delete my account"})
	again, err := harness.DoWithStepUp(ctx, alex, harness.Request{Method: http.MethodPost, Path: "/gdpr-requests",
		Body: map[string]any{"kind": "deletion", "reason": "again"}})
	if err != nil || again.Status != http.StatusConflict || !strings.Contains(string(again.Body), r.ID) {
		t.Fatalf("second open deletion = %d %s %v", again.Status, again.Body, err)
	}
	harness.MustDo(t, sam, harness.Request{Method: http.MethodGet, Path: "/gdpr-requests/" + r.ID}, http.StatusNotFound)
	get(t, o, r.ID)
	var page struct {
		Items []request `json:"items"`
	}
	harness.MustDo(t, sam, harness.Request{Method: http.MethodGet, Path: "/gdpr-requests"}, http.StatusOK).JSON(t, &page)
	if len(page.Items) != 0 {
		t.Fatalf("Sam lists %d requests of others", len(page.Items))
	}
	harness.MustDo(t, o, harness.Request{Method: http.MethodGet, Path: "/gdpr-requests"}, http.StatusOK).JSON(t, &page)
	if len(page.Items) != 1 || page.Items[0].ID != r.ID {
		t.Fatalf("owner list = %+v", page.Items)
	}
	harness.MustDo(t, alex, harness.Request{Method: http.MethodPost, Path: "/gdpr-requests/" + r.ID + "/decide",
		Body: map[string]any{"decision": "approve", "reason": "x"}}, http.StatusForbidden)
	harness.MustDo(t, sam, harness.Request{Method: http.MethodPost, Path: "/gdpr-requests",
		Body: map[string]any{"kind": "export", "reason": "x", "subject_user_id": alex.PrincipalID}}, http.StatusForbidden)
	var d request
	harness.MustDo(t, o, harness.Request{Method: http.MethodPost, Path: "/gdpr-requests/" + r.ID + "/decide",
		Body: map[string]any{"decision": "reject", "reason": "Retention duty", "extend_months": 1}}, http.StatusOK).JSON(t, &d)
	if d.State != "rejected" {
		t.Fatalf("decision = %+v", d)
	}
	// The deletion of the last owner is refused.
	resp, err := harness.DoWithStepUp(ctx, o, harness.Request{Method: http.MethodPost, Path: "/gdpr-requests",
		Body: map[string]any{"kind": "deletion", "reason": "x"}})
	if err != nil || resp.Status != http.StatusConflict || !strings.Contains(string(resp.Body), "last_owner") {
		t.Fatalf("deletion of the last owner = %d %s", resp.Status, resp.Body)
	}
	// The package of a request without one is not ready.
	harness.MustDo(t, alex, harness.Request{Method: http.MethodGet, Path: "/gdpr-requests/" + r.ID + "/export"}, http.StatusConflict)
}
