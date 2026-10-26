// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/files/adapters/fs"
	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
)

// recorder records the side effects the participant and the export packages cause through the
// ports of the files use cases: file.updated events, redactions and files.purge_objects.
type recorder struct {
	mu      sync.Mutex
	events  []eventlog.Event
	redacts []ids.ID
	purges  []ids.ID
}

// Append implements eventlog.Appender.
func (r *recorder) Append(_ context.Context, evs ...eventlog.Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, evs...)
	return nil
}

// Redact implements eventlog.Redactor.
func (r *recorder) Redact(_ context.Context, _ string, id ids.ID, _ eventlog.RedactFunc) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.redacts = append(r.redacts, id)
	return 0, nil
}

// Enqueue implements app.PurgeQueue.
func (r *recorder) Enqueue(_ context.Context, id ids.ID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.purges = append(r.purges, id)
	return nil
}

// counts returns the numbers of events, redactions and purges so far.
func (r *recorder) counts() (int, int, int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.events), len(r.redacts), len(r.purges)
}

// pdWorld is the world of the attachment tests with a real object store of the file system
// driver and a files service whose events, redactions and purges are recorded.
type pdWorld struct {
	*world
	store *fs.Store
	rec   *recorder
	svc   *app.Service
	robin ids.ID
}

func newPDWorld(t *testing.T) *pdWorld {
	t.Helper()
	w := newWorld(t)
	// The file system driver wants a root of mode 0700 that it does not create itself.
	root := filepath.Join(t.TempDir(), "objects")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	store, err := fs.Open(fs.Config{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	rec := &recorder{}
	svc, err := app.New(app.Config{MaxUploadBytes: 1 << 20, PartBytes: 1 << 20, SessionTTL: time.Hour, MaxOpenSessions: 3, NameMaxBytes: 255},
		app.Deps{Repo: w.repo, Parts: app.NewFSPartStore(store), UoW: w.uow, Authorizer: allow{}, Events: rec,
			Channels: oneSpace{w.space}, Names: asIs{}, Types: noTypes{}, ScanQueue: noQueue{}, Purge: rec,
			Redactor: rec, Cleanup: w.repo, Clock: w.clk, Store: store})
	if err != nil {
		t.Fatal(err)
	}
	p := &pdWorld{world: w, store: store, rec: rec, svc: svc, robin: ids.New()}
	p.exec(`INSERT INTO principals (id, kind, status, handle, display_name) VALUES ($1, 'user', 'active', 'robin', 'Robin')`, p.robin)
	return p
}

// stored inserts a file of uploader whose bytes are in the object store: purpose, scope channel
// (zero for none), upload state, scan status, name and media type. It returns the id.
func (p *pdWorld) stored(uploader, channel ids.ID, purpose, upload, scan, name, mediaType string, body []byte) ids.ID {
	p.t.Helper()
	id := ids.New()
	key, err := domain.OriginalKey(id, 1)
	if err != nil {
		p.t.Fatal(err)
	}
	if err := p.store.Put(context.Background(), key, bytes.NewReader(body), int64(len(body))); err != nil {
		p.t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	var scope any
	if !channel.IsZero() {
		scope = channel.String()
	}
	p.exec(`INSERT INTO files (id, object_key, sha256, size_bytes, mime_type, original_name, uploaded_by, purpose, scope_channel_id,
		upload_state, scan_status, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		id, key, sum[:], len(body), mediaType, name, uploader, purpose, scope, upload, scan, p.clk.Now())
	return id
}

// openSession inserts an open upload session of uploader with one confirmed part and returns the
// file id.
func (p *pdWorld) openSession(uploader ids.ID) ids.ID {
	p.t.Helper()
	id := p.file(uploader, p.allgemein, "attachment", "open", "pending", "pending")
	p.exec(`INSERT INTO file_upload_parts (file_id, part_number, size_bytes, sha256) VALUES ($1, 1, 4, '\x01')`, id)
	return id
}

// attachTo inserts the attachment row of file on message m in Allgemein.
func (p *pdWorld) attachTo(m, file ids.ID, position int) {
	p.t.Helper()
	p.exec(`INSERT INTO message_attachments (message_id, file_id, position, thread_id, channel_id) VALUES ($1, $2, $3, $4, $5)`,
		m, file, position, p.thread, p.allgemein)
}

// deleted reports files.deleted_at IS NOT NULL of id.
func (p *pdWorld) deleted(id ids.ID) bool {
	p.t.Helper()
	return p.count(`SELECT count(*) FROM files WHERE id = $1 AND deleted_at IS NOT NULL`, id) == 1
}

// sessionState returns the state of the upload session of id.
func (p *pdWorld) sessionState(id ids.ID) string {
	p.t.Helper()
	var s string
	if err := p.conn.QueryRow(context.Background(), `SELECT state FROM file_upload_sessions WHERE file_id = $1`, id.String()).Scan(&s); err != nil {
		p.t.Fatal(err)
	}
	return s
}

// filesSchema compiles docs/api/gdpr-export/v1/sections/files.files.schema.json.
func filesSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../../../../docs/api/gdpr-export/v1/sections/files.files.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	if err := c.AddResource("files.files.schema.json", doc); err != nil {
		t.Fatal(err)
	}
	s, err := c.Compile("files.files.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// TestPersonalDataExport: the export holds exactly Alex's own complete and deliverable uploads of
// purpose attachment or avatar, each with its bytes under files/<id> and one entry of the section
// "files" of files/files.json that validates against its schema; no file of Sam, no export
// package, nothing deleted, open, failed or blocked, and no other file of the package.
func TestPersonalDataExport(t *testing.T) {
	p := newPDWorld(t)
	pd, err := app.NewPersonalData(p.svc, p.repo)
	if err != nil {
		t.Fatal(err)
	}
	if pd.Module() != "files" {
		t.Fatalf("Module %q", pd.Module())
	}
	wantTables := []string{"files", "file_thumbnails", "file_upload_sessions", "file_upload_parts", "file_text_extracts"}
	if got := pd.Tables(); !slices.Equal(got, wantTables) {
		t.Fatalf("Tables %v, want %v", got, wantTables)
	}
	attached := p.stored(p.alex, p.allgemein, "attachment", "complete", "clean", "report.pdf", "application/pdf", []byte("alex report"))
	p.attachTo(p.m1, attached, 1)
	unattached := p.stored(p.alex, p.eskalation, "attachment", "complete", "skipped", "notes.txt", "text/plain", []byte("alex notes"))
	voice := p.stored(p.alex, p.allgemein, "attachment", "complete", "clean", "voice.m4a", "audio/mp4", []byte("alex voice"))
	p.exec(`UPDATE files SET transcript_status = 'done', transcript_text = 'Hello Sam', transcript_language = 'en', transcript_source = 'service' WHERE id = $1`, voice)
	avatar := p.stored(p.alex, ids.ID{}, "avatar", "complete", "clean", "avatar", "image/png", []byte("alex avatar"))
	// None of these goes into the package.
	p.stored(p.alex, p.allgemein, "attachment", "pending", "pending", "pending.txt", "text/plain", []byte("pending"))
	p.stored(p.alex, p.allgemein, "attachment", "failed", "pending", "failed.txt", "text/plain", []byte("failed"))
	p.stored(p.alex, p.allgemein, "attachment", "complete", "infected", "infected.txt", "text/plain", []byte("infected"))
	gone := p.stored(p.alex, p.allgemein, "attachment", "complete", "clean", "gone.txt", "text/plain", []byte("gone"))
	p.exec(`UPDATE files SET deleted_at = now() WHERE id = $1`, gone)
	p.stored(p.alex, ids.ID{}, "gdpr_export", "complete", "skipped", "export.zip", "application/zip", []byte("old export"))
	p.stored(p.sam, p.allgemein, "attachment", "complete", "clean", "sam.txt", "text/plain", []byte("sam notes"))
	p.stored(p.sam, ids.ID{}, "avatar", "complete", "clean", "avatar", "image/png", []byte("sam avatar"))

	w := personaldata.NewMemoryWriter()
	if err := pd.Export(context.Background(), personaldata.Export{Subject: p.alex}, w); err != nil {
		t.Fatal(err)
	}
	if len(w.Lines) != 0 || len(w.Sections) != 1 || len(w.Sections[personaldata.FileFiles]) != 1 {
		t.Fatalf("wrote outside files/files.json section files: lines %d, sections %v", len(w.Lines), w.Sections)
	}
	raw := w.Sections[personaldata.FileFiles]["files"]
	var inst any
	if err := json.Unmarshal(raw, &inst); err != nil {
		t.Fatal(err)
	}
	if err := filesSchema(t).Validate(inst); err != nil {
		t.Fatalf("files.json files does not match its schema: %v\n%s", err, raw)
	}
	var entries []map[string]any
	if err := json.Unmarshal(raw, &entries); err != nil {
		t.Fatal(err)
	}
	bodies := map[ids.ID]string{attached: "alex report", unattached: "alex notes", voice: "alex voice", avatar: "alex avatar"}
	if len(entries) != len(bodies) || len(w.Contents) != len(bodies) {
		t.Fatalf("%d entries and %d contents, want %d each: %s", len(entries), len(w.Contents), len(bodies), raw)
	}
	channels := map[ids.ID]string{attached: p.allgemein.String(), unattached: p.eskalation.String(), voice: p.allgemein.String()}
	for _, e := range entries {
		id := ids.MustParse(e["id"].(string))
		body, ok := bodies[id]
		if !ok {
			t.Fatalf("entry of a file that is not Alex's own deliverable upload: %v", e)
		}
		if string(w.Contents[id]) != body {
			t.Fatalf("content of %s = %q, want %q", id, w.Contents[id], body)
		}
		sum := sha256.Sum256([]byte(body))
		if e["sha256"] != hex.EncodeToString(sum[:]) || e["size_bytes"] != float64(len(body)) {
			t.Fatalf("sha256 or size of %s wrong: %v", id, e)
		}
		if got, _ := e["channel_id"].(string); got != channels[id] {
			t.Fatalf("channel_id of %s = %q, want %q", id, got, channels[id])
		}
		_, hasTranscript := e["transcript"]
		if hasTranscript != (id == voice) {
			t.Fatalf("transcript of %s: %v", id, e)
		}
	}
	for _, e := range entries {
		switch ids.MustParse(e["id"].(string)) {
		case voice:
			if e["transcript"] != "Hello Sam" || e["transcript_language"] != "en" || e["name"] != "voice.m4a" || e["media_type"] != "audio/mp4" {
				t.Fatalf("voice entry %v", e)
			}
		case avatar:
			if e["purpose"] != "avatar" || e["created_at"] != p.clk.Now().UTC().Format(time.RFC3339) {
				t.Fatalf("avatar entry %v", e)
			}
		}
	}
}

// TestPersonalDataExportEmpty: a person without uploads gets an empty array, never null.
func TestPersonalDataExportEmpty(t *testing.T) {
	p := newPDWorld(t)
	pd, err := app.NewPersonalData(p.svc, p.repo)
	if err != nil {
		t.Fatal(err)
	}
	w := personaldata.NewMemoryWriter()
	if err := pd.Export(context.Background(), personaldata.Export{Subject: p.robin}, w); err != nil {
		t.Fatal(err)
	}
	if got := string(w.Sections[personaldata.FileFiles]["files"]); got != "[]" {
		t.Fatalf("files = %s, want []", got)
	}
}

// eraseFixture are the files of TestPersonalDataErase.
type eraseFixture struct {
	open, unattached, attached, avatar, export, failed ids.ID
	samOpen, samFile, samAvatar                        ids.ID
}

func newEraseFixture(p *pdWorld) eraseFixture {
	f := eraseFixture{
		open:       p.openSession(p.alex),
		unattached: p.stored(p.alex, p.allgemein, "attachment", "complete", "clean", "draft.txt", "text/plain", []byte("draft")),
		attached:   p.stored(p.alex, p.allgemein, "attachment", "complete", "clean", "sent.txt", "text/plain", []byte("sent")),
		avatar:     p.stored(p.alex, ids.ID{}, "avatar", "complete", "clean", "avatar", "image/png", []byte("avatar")),
		export:     p.stored(p.alex, ids.ID{}, "gdpr_export", "complete", "skipped", "export.zip", "application/zip", []byte("zip")),
		failed:     p.stored(p.alex, p.allgemein, "attachment", "failed", "pending", "failed.txt", "text/plain", []byte("failed")),
		samOpen:    p.openSession(p.sam),
		samFile:    p.stored(p.sam, p.allgemein, "attachment", "complete", "clean", "sam.txt", "text/plain", []byte("sam")),
		samAvatar:  p.stored(p.sam, ids.ID{}, "avatar", "complete", "clean", "avatar", "image/png", []byte("sam avatar")),
	}
	p.attachTo(p.m1, f.attached, 1)
	return f
}

// TestPersonalDataErase runs the steps 1 to 6 twice (the second run is the resume after a crash)
// in both content modes: step 3 aborts Alex's open upload session with file.updated and the
// purge; step 5 with delete marks her unattached uploads deleted (attached ones go with their
// message through messaging), with keep_pseudonymized it keeps them; step 6 removes her avatar
// file in both modes; export packages stay for privacy. Sam's rows are untouched and the second
// run changes nothing.
func TestPersonalDataErase(t *testing.T) {
	for _, mode := range []personaldata.ContentMode{personaldata.KeepPseudonymized, personaldata.DeleteContent} {
		t.Run(string(mode), func(t *testing.T) {
			p := newPDWorld(t)
			pd, err := app.NewPersonalData(p.svc, p.repo)
			if err != nil {
				t.Fatal(err)
			}
			f := newEraseFixture(p)
			e := personaldata.Erasure{Subject: p.alex, RequestID: ids.New(), Content: mode}
			var first [3]int
			for run := range 2 {
				for _, step := range personaldata.ParticipantSteps() {
					if err := pd.Erase(context.Background(), step, e); err != nil {
						t.Fatalf("run %d step %s: %v", run, step, err)
					}
					if run == 0 && step == personaldata.StepPersonal {
						if got := p.sessionState(f.open); got != "aborted" {
							t.Fatalf("after step personal the open session is %q, want aborted", got)
						}
					}
				}
				if run == 0 {
					first[0], first[1], first[2] = p.rec.counts()
				}
			}
			if n, r, q := p.rec.counts(); n != first[0] || r != first[1] || q != first[2] {
				t.Fatalf("the second run changed something: events %d->%d, redactions %d->%d, purges %d->%d", first[0], n, first[1], r, first[2], q)
			}
			if got := p.sessionState(f.open); got != "aborted" {
				t.Fatalf("Alex's open session is %q, want aborted", got)
			}
			if p.count(`SELECT count(*) FROM files WHERE id = $1 AND upload_state = 'failed'`, f.open) != 1 ||
				p.count(`SELECT count(*) FROM file_upload_parts WHERE file_id = $1`, f.open) != 0 {
				t.Fatal("the aborted upload is not failed or still has parts")
			}
			if !slices.ContainsFunc(p.rec.events, func(ev eventlog.Event) bool { return ev.Type == "file.updated" && ev.ResourceID == f.open }) {
				t.Fatal("no file.updated for the aborted upload")
			}
			if ev := p.rec.events[0]; !ev.ActorPrincipalID.IsZero() {
				t.Fatalf("file.updated names an actor: %v", ev.ActorPrincipalID)
			}
			if !slices.Contains(p.rec.purges, f.open) {
				t.Fatal("no files.purge_objects for the aborted upload")
			}
			wantDeleted := map[ids.ID]bool{
				f.unattached: mode == personaldata.DeleteContent, f.failed: mode == personaldata.DeleteContent,
				f.attached: false, f.avatar: true, f.export: false, f.samFile: false, f.samAvatar: false,
			}
			for id, want := range wantDeleted {
				if got := p.deleted(id); got != want {
					t.Fatalf("file %s deleted = %v, want %v", id, got, want)
				}
				if want && (!slices.Contains(p.rec.purges, id) || !slices.Contains(p.rec.redacts, id)) {
					t.Fatalf("deleted file %s without purge or redaction", id)
				}
			}
			if got := p.sessionState(f.samOpen); got != "open" {
				t.Fatalf("Sam's session is %q, want open", got)
			}
		})
	}
}

// TestExportPackages holds files.ExportPackages: Store writes the object and a complete
// gdpr_export row of the subject without scan and channel, Open returns the bytes, a wrong length
// or digest stores nothing, Delete marks the row in the caller's unit with redaction and purge
// and leaves other files and a second call alone.
func TestExportPackages(t *testing.T) {
	p := newPDWorld(t)
	pkgs, err := app.NewExportPackages(p.svc, p.repo)
	if err != nil {
		t.Fatal(err)
	}
	var _ files.ExportPackages = pkgs
	ctx := context.Background()
	body := []byte("PK zip of Alex")
	sum := sha256.Sum256(body)
	hexSum := hex.EncodeToString(sum[:])
	name := "export-" + p.alex.String() + "-2026-10-21.zip"
	id, err := pkgs.Store(ctx, files.StoreExport{SubjectID: p.alex, Name: name, SizeBytes: int64(len(body)), SHA256: hexSum, Body: bytes.NewReader(body)})
	if err != nil {
		t.Fatal(err)
	}
	if p.count(`SELECT count(*) FROM files WHERE id = $1 AND purpose = 'gdpr_export' AND uploaded_by = $2 AND scope_channel_id IS NULL
		AND upload_state = 'complete' AND scan_status = 'skipped' AND size_bytes = $3 AND sha256 = $4 AND original_name = $5
		AND mime_type = 'application/zip' AND deleted_at IS NULL AND created_at = $6`, id, p.alex, len(body), sum[:], name, p.clk.Now()) != 1 {
		t.Fatal("the export row is not as Store must write it")
	}
	if p.count(`SELECT count(*) FROM file_upload_sessions WHERE file_id = $1`, id) != 0 {
		t.Fatal("Store wrote an upload session")
	}
	if n, _, _ := p.rec.counts(); n != 0 {
		t.Fatalf("Store appended %d events", n)
	}
	c, err := pkgs.Open(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(c.Body)
	_ = c.Body.Close()
	if err != nil || !bytes.Equal(got, body) || c.Name != name || c.SizeBytes != int64(len(body)) || c.SHA256 != hexSum {
		t.Fatalf("Open = %q %q %d %s, %v", got, c.Name, c.SizeBytes, c.SHA256, err)
	}
	if used, err := p.repo.QuotaUsedBytes(ctx, p.alex); err != nil || used != 0 {
		t.Fatalf("the export package counts against the upload quota: %d, %v", used, err)
	}

	before := p.count(`SELECT count(*) FROM files`)
	for _, bad := range []files.StoreExport{
		{SubjectID: p.alex, Name: name, SizeBytes: int64(len(body)) + 1, SHA256: hexSum, Body: bytes.NewReader(body)},
		{SubjectID: p.alex, Name: name, SizeBytes: int64(len(body)), SHA256: hex.EncodeToString(make([]byte, 32)), Body: bytes.NewReader(body)},
		{SubjectID: p.alex, Name: "", SizeBytes: int64(len(body)), SHA256: hexSum, Body: bytes.NewReader(body)},
		{SubjectID: ids.ID{}, Name: name, SizeBytes: int64(len(body)), SHA256: hexSum, Body: bytes.NewReader(body)},
	} {
		if _, err := pkgs.Store(ctx, bad); errs.KindOf(err) != errs.KindInternal {
			t.Fatalf("Store of a bad package: %v, want internal", err)
		}
	}
	if p.count(`SELECT count(*) FROM files`) != before {
		t.Fatal("a refused Store left a row")
	}
	err = p.uow.Do(ctx, func(ctx context.Context) error {
		_, err := pkgs.Store(ctx, files.StoreExport{SubjectID: p.alex, Name: name, SizeBytes: int64(len(body)), SHA256: hexSum, Body: bytes.NewReader(body)})
		return err
	})
	if errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("Store inside a unit of work: %v, want internal", err)
	}

	other := p.stored(p.alex, p.allgemein, "attachment", "complete", "clean", "a.txt", "text/plain", []byte("a"))
	if _, err := pkgs.Open(ctx, other); errs.KindOf(err) != errs.KindNotFound {
		t.Fatalf("Open of an attachment: %v, want not_found", err)
	}
	if _, err := pkgs.Open(ctx, ids.New()); errs.KindOf(err) != errs.KindNotFound {
		t.Fatalf("Open of an unknown id: %v, want not_found", err)
	}
	if err := pkgs.Delete(ctx, id); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("Delete outside a unit of work: %v, want internal", err)
	}
	for range 2 {
		if err := p.uow.Do(ctx, func(ctx context.Context) error {
			if err := pkgs.Delete(ctx, other); err != nil {
				return err
			}
			if err := pkgs.Delete(ctx, ids.New()); err != nil {
				return err
			}
			return pkgs.Delete(ctx, id)
		}); err != nil {
			t.Fatal(err)
		}
	}
	if !p.deleted(id) || p.deleted(other) {
		t.Fatal("Delete marked the wrong rows")
	}
	if _, r, q := p.rec.counts(); r != 1 || q != 1 || p.rec.purges[0] != id {
		t.Fatalf("Delete: %d redactions and purges %v, want one of the package", r, p.rec.purges)
	}
	if _, err := pkgs.Open(ctx, id); errs.KindOf(err) != errs.KindNotFound {
		t.Fatalf("Open of a deleted package: %v, want not_found", err)
	}
}
