// SPDX-License-Identifier: Apache-2.0

package importer_test

import (
	"bytes"
	"context"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/restclient"
	importapi "github.com/OWNER/bothy/services/server/cmd/bothy/importtool/bothyapi"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// syncBuffer is a bytes.Buffer safe for the writer of Login and the reader in the sleeper.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// cliClient logs Robin in as a device with platform cli through device pairing (08 5.3, the
// flow of bothy import login) and returns the client of that session.
func (w *iw) cliClient() *importapi.Client {
	w.t.Helper()
	path := filepath.Join(w.t.TempDir(), "bothy", "import-session.json")
	var stdout, stderr syncBuffer
	approved := false
	limits := importapi.DefaultLimits()
	limits.LoginPollInterval = 50 * time.Millisecond
	sleep := func(ctx context.Context, d time.Duration) error {
		if !approved {
			approved = true
			code := strings.TrimSpace(stdout.String())
			if _, err := harness.DoWithStepUp(ctx, w.robin, harness.Request{Method: http.MethodPost, Path: "/me/devices/pairings",
				Body: map[string]any{"pairing_code": code}}); err != nil {
				return err
			}
		}
		t := time.NewTimer(d)
		defer t.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			return nil
		}
	}
	if err := importapi.Login(w.ctx, importapi.LoginOptions{ServerURL: w.s.PublicURL, SessionFile: path, AppVersion: "1.0.0",
		Hostname: "workstation", Limits: limits, Deps: importapi.Deps{Sleep: sleep}, Stdout: &stdout, Stderr: &stderr}); err != nil {
		w.t.Fatalf("Login: %v\n%s", err, stderr.String())
	}
	c, err := importapi.New(w.ctx, importapi.Options{SessionFile: path, Limits: limits})
	if err != nil {
		w.t.Fatal(err)
	}
	return c
}

// TestCreateImport_StepUpAndAudit (design note 2, 08 6.4, SR5): without step-up 401
// step_up_required; a cli session 401; a personal token of Robin 403; with step-up 201 draft with
// one audit entry; a mapping with violations answers all of them at once.
func TestCreateImport_StepUpAndAudit(t *testing.T) {
	t.Parallel()
	w := newIW(t)
	body := map[string]any{"kind": "discord", "name": "Old community server", "config": w.mapping()}
	w.wantProblem(w.do(w.robin, http.MethodPost, "/imports", body, nil), http.StatusUnauthorized, "step_up_required", "")

	cli := w.cliClient()
	resp, err := cli.CreateImportWithResponse(w.ctx, &restclient.CreateImportParams{}, restclient.CreateImportJSONRequestBody{
		Kind: restclient.ImportKind("discord"), Name: "Old community server", Config: restclient.JsonObject(w.mapping()),
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != http.StatusUnauthorized {
		t.Errorf("a cli session: %d %s, want 401", resp.StatusCode(), resp.Body)
	}

	w.must(w.s.Owner, http.MethodPut, "/feature-flags/tokens.personal_enabled", map[string]any{"enabled": true, "reason": "owner token"}, nil, http.StatusOK)
	// The harness creates personal tokens through the setup owner, which is an owner like Robin.
	token := w.s.NewTokenClient(t, w.s.Owner)
	if r := w.do(token, http.MethodPost, "/imports", body, nil); r.Status != http.StatusForbidden {
		t.Errorf("a personal token: %d %s, want 403", r.Status, r.Body)
	}

	id := w.create()
	s := w.get(id)
	if s.State != "draft" || s.Config["guild_id"] != "100" {
		t.Errorf("source %+v", s)
	}
	if n := w.count(`SELECT count(*) FROM audit_events WHERE action = 'import.manage' AND decision = 'allow' AND resource_id = $1`, id); n != 1 {
		t.Errorf("audit entries %d, want 1", n)
	}

	bad := w.mapping()
	bad["guild_id"] = "12x"
	bad["users"] = []any{map[string]any{"discord_user": "1", "target": "user:nobody"}, map[string]any{"discord_user": "1", "target": "placeholder"}}
	bad["channels"] = []any{map[string]any{"discord_channel": "2", "target": map[string]any{"existing": w.existingSpace}}}
	p := w.wantProblem(w.must(w.robin, http.MethodPost, "/imports", map[string]any{"kind": "discord", "name": "x", "config": bad}, nil,
		http.StatusBadRequest), http.StatusBadRequest, "validation_failed", "")
	for _, e := range []struct{ pointer, code string }{
		{"/config/guild_id", "not_digits"}, {"/config/users/0/target", "unknown_handle"}, {"/config/users/1/discord_user", "duplicate"},
		{"/config/channels/0/target/existing", "unknown_channel"},
	} {
		if !p.hasError(e.pointer, e.code) {
			t.Errorf("missing %s %s in %+v", e.pointer, e.code, p.Errors)
		}
	}
	p = w.wantProblem(w.must(w.robin, http.MethodPost, "/imports", map[string]any{"kind": "archive", "name": "x", "config": map[string]any{}}, nil,
		http.StatusBadRequest), http.StatusBadRequest, "validation_failed", "")
	if !p.hasError("/kind", "not_enabled") {
		t.Errorf("archive: %+v", p.Errors)
	}
}

// TestStateTransitions (design note 3): every allowed change, the no-effect repeats without an
// event, import_state for a refused change and the lead reason for an early completion.
func TestStateTransitions(t *testing.T) {
	t.Parallel()
	w := newIW(t)
	id := w.create()
	w.wantProblem(w.must(w.robin, http.MethodPost, "/imports/"+id+"/pause", nil, nil, http.StatusConflict), http.StatusConflict, "conflict", "import_state")
	w.wantProblem(w.must(w.robin, http.MethodPost, "/imports/"+id+"/complete", nil, nil, http.StatusConflict), http.StatusConflict, "conflict", "import_state")
	w.must(w.robin, http.MethodPost, "/imports/"+id+"/start", nil, nil, http.StatusAccepted)
	s := w.get(id)
	if s.State != "running" || s.StartedAt == "" {
		t.Fatalf("after start %+v", s)
	}
	seq := w.maxSeq()
	w.must(w.robin, http.MethodPost, "/imports/"+id+"/start", nil, nil, http.StatusAccepted)
	if ev := w.events(seq); len(ev) != 0 {
		t.Errorf("a repeated start appended %+v", ev)
	}
	w.must(w.robin, http.MethodPost, "/imports/"+id+"/pause", nil, nil, http.StatusOK)
	w.must(w.robin, http.MethodPost, "/imports/"+id+"/pause", nil, nil, http.StatusOK)
	if s := w.get(id); s.State != "paused" {
		t.Fatalf("after pause %+v", s)
	}
	w.must(w.robin, http.MethodPost, "/imports/"+id+"/start", nil, nil, http.StatusAccepted)
	w.mustBatch(id, structure()...)
	w.wantProblem(w.must(w.robin, http.MethodPost, "/imports/"+id+"/complete", nil, nil, http.StatusConflict), http.StatusConflict, "conflict", "lead_not_"+"bothy")
	for _, ch := range []string{chText, chForum} {
		w.must(w.robin, http.MethodPut, "/imports/"+id+"/leads", map[string]any{"source_channel": ch, "lead": "bothy"}, nil, http.StatusOK)
	}
	w.must(w.robin, http.MethodPost, "/imports/"+id+"/complete", nil, nil, http.StatusOK)
	s = w.get(id)
	if s.State != "completed" || s.FinishedAt == "" {
		t.Fatalf("after complete %+v", s)
	}
	for _, op := range []string{"start", "pause", "cancel", "complete"} {
		w.wantProblem(w.must(w.robin, http.MethodPost, "/imports/"+id+"/"+op, nil, nil, http.StatusConflict), http.StatusConflict, "conflict", "import_state")
	}
	w.wantProblem(w.must(w.robin, http.MethodPut, "/imports/"+id+"/leads", map[string]any{"source_channel": chText, "lead": "discord"}, nil,
		http.StatusConflict), http.StatusConflict, "conflict", "import_state")

	other := w.create()
	w.must(w.robin, http.MethodPost, "/imports/"+other+"/cancel", nil, nil, http.StatusOK)
	s = w.get(other)
	server, _ := s.Stats["server"].(map[string]any)
	if s.State != "failed" || s.FinishedAt == "" || server["cancelled"] != true {
		t.Errorf("after cancel %+v", s)
	}
	if n := w.count(`SELECT count(*) FROM audit_events WHERE action = 'import.manage' AND resource_id = $1`, id); n < 6 {
		t.Errorf("audit entries %d", n)
	}
}

// TestImportRoutesOwnerOnly: Alex gets the 404 or 403 of the catalogue for every route.
func TestImportRoutesOwnerOnly(t *testing.T) {
	t.Parallel()
	w := newIW(t)
	id := w.running()
	for _, r := range []struct{ method, path string }{
		{http.MethodGet, "/imports"}, {http.MethodPost, "/imports"}, {http.MethodGet, "/imports/" + id},
		{http.MethodGet, "/imports/" + id + "/items"}, {http.MethodPost, "/imports/" + id + "/start"},
		{http.MethodPost, "/imports/" + id + "/pause"}, {http.MethodPost, "/imports/" + id + "/cancel"},
		{http.MethodPost, "/imports/" + id + "/complete"}, {http.MethodPut, "/imports/" + id + "/leads"},
		{http.MethodPost, "/imports/" + id + "/batches"}, {http.MethodPut, "/imports/" + id + "/cursor"},
	} {
		var body any
		switch {
		case r.method == http.MethodPost && r.path == "/imports":
			body = map[string]any{"kind": "discord", "name": "x", "config": w.mapping()}
		case strings.HasSuffix(r.path, "/leads"):
			body = map[string]any{"source_channel": chText, "lead": "bothy"}
		case strings.HasSuffix(r.path, "/batches"):
			body = map[string]any{"items": structure()[:1]}
		case strings.HasSuffix(r.path, "/cursor"):
			body = map[string]any{"cursor": map[string]any{}}
		}
		resp, err := harness.DoWithStepUp(w.ctx, w.alex, harness.Request{Method: r.method, Path: r.path, Body: body})
		if err != nil {
			t.Fatal(err)
		}
		if resp.Status != http.StatusNotFound && resp.Status != http.StatusForbidden {
			t.Errorf("%s %s as Alex = %d %s", r.method, r.path, resp.Status, resp.Body)
		}
	}
	if s := w.get(id); s.State != "running" {
		t.Errorf("Alex changed the source: %+v", s)
	}
}
