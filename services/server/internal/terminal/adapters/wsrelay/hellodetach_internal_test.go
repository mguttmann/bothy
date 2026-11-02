// SPDX-License-Identifier: Apache-2.0

package wsrelay

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/internal/terminal/app"
	"github.com/OWNER/bothy/services/server/test/terminal/fakehost"
)

// helloUC answers the key set of hello and nothing else.
type helloUC struct{ UseCases }

func (helloUC) TerminalJWKS(context.Context) ([]app.JWK, error) { return nil, nil }
func (helloUC) AuthFailed(context.Context) error                { return nil }

// graceSessions names a detach value per session.
type graceSessions map[ids.ID]time.Duration

func (g graceSessions) DataSession(context.Context, app.Login, ids.ID) (bool, error) {
	return false, nil
}

func (g graceSessions) HelloDetachGrace(_ context.Context, id ids.ID) (time.Duration, error) {
	if d, ok := g[id]; ok {
		return d, nil
	}
	return 15 * time.Minute, nil
}

// TestDataHelloCarriesTheSessionDetachGrace (FIX-terminal-session-lifecycle, D-TSL-1): the hello
// of a data connection carries the detach value of the session of its path, so the host service
// keeps a detached session in the owner account mode for terminal.owner_account_detach_grace; the
// hello of the control connection keeps terminal.detach_grace.
func TestDataHelloCarriesTheSessionDetachGrace(t *testing.T) {
	t.Parallel()
	schemas, err := httpserver.NewSchemaValidator(rest.Spec, SchemaNames())
	if err != nil {
		t.Fatal(err)
	}
	h, err := NewHandler(Config{HeartbeatInterval: 20 * time.Second, HeartbeatMissLimit: 3, AuthTimeout: time.Minute, DuplicateProbeTimeout: time.Second,
		MaxFrameBytes: 1 << 16, MaxJSONDepth: 32, OutboundQueueMax: 16, WriteTimeout: time.Second, IdleTimeout: 30 * time.Minute,
		DetachGrace: 15 * time.Minute}, Deps{UseCases: helloUC{}, Links: NewLinks(nil, nil), Schemas: schemas})
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewRelay(RelayConfig{BufferMaxBytes: 1 << 16, OpenTimeout: time.Minute}, &fakeRelayUC{}, nil, manualTimers{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	owner, plain := ids.New(), ids.New()
	d, err := NewDataHandler(h, r, graceSessions{owner: 24 * time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/hosts/sessions/{session_id}/stream", d)
	mux.Handle("GET /api/v1/hosts/connect", h)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	ctx := testkit.Context(t)
	hello := func(path string) int64 {
		t.Helper()
		ws, _, err := fakehost.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+path)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = ws.CloseNow() }()
		_, b, err := ws.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var f struct {
			Op string                `json:"op"`
			D  rest.HostChannelHello `json:"d"`
		}
		if err := json.Unmarshal(b, &f); err != nil || f.Op != opHello {
			t.Fatalf("first frame %s: %v", b, err)
		}
		return f.D.DetachGraceMs
	}
	if got := hello(app.DataPath(owner)); got != (24 * time.Hour).Milliseconds() {
		t.Errorf("owner account session: detach_grace_ms %d", got)
	}
	if got := hello(app.DataPath(plain)); got != (15 * time.Minute).Milliseconds() {
		t.Errorf("separate session user: detach_grace_ms %d", got)
	}
	if got := hello("/api/v1/hosts/connect"); got != (15 * time.Minute).Milliseconds() {
		t.Errorf("control connection: detach_grace_ms %d", got)
	}
}
