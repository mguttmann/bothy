// SPDX-License-Identifier: Apache-2.0

package httpservertest_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

func TestActors(t *testing.T) {
	t.Parallel()
	s := httpservertest.NewActors()
	id, sess, dev, tok := ids.New(), ids.New(), ids.New(), ids.New()
	at := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	s.Set("alex", httpservertest.InSession(id, sess, dev, at))
	s.Set("bot", httpservertest.WithToken(id, actor.KindAgent, tok))
	ctx := context.Background()
	if a, err := s.Authenticate(ctx, actor.Credentials{Authorization: "Bearer alex"}); err != nil || a.SessionID != sess || a.DeviceID != dev || !a.StepUpAt.Equal(at) {
		t.Fatalf("alex = %+v, %v", a, err)
	}
	if a, err := s.Authenticate(ctx, actor.Credentials{Authorization: "Bearer bot"}); err != nil || a.Kind != actor.KindAgent || a.TokenID != tok {
		t.Fatalf("bot = %+v, %v", a, err)
	}
	if a, err := s.Authenticate(ctx, actor.Credentials{}); err != nil || !a.IsAnonymous() {
		t.Fatalf("anonymous = %+v, %v", a, err)
	}
	if _, err := s.Authenticate(ctx, actor.Credentials{Authorization: "Bearer nobody"}); errs.KindOf(err) != errs.KindUnauthenticated {
		t.Fatalf("unknown token = %v", err)
	}
	if s.Get("alex").ID != id || s.Revalidate(ctx, actor.Actor{}) != nil {
		t.Fatal("Get or Revalidate")
	}
}

func TestSharedResponseValidator(t *testing.T) {
	t.Parallel()
	a, err := httpservertest.SharedResponseValidator(rest.Spec, []string{"getMe", "getUser"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := httpservertest.SharedResponseValidator(rest.Spec, []string{"getUser", "getMe"})
	if err != nil || a != b {
		t.Fatalf("second call = %p, %v; want the cached %p", b, err, a)
	}
	if _, err := httpservertest.SharedResponseValidator(rest.Spec, []string{"noSuchOperation"}); err == nil {
		t.Fatal("an unknown operation gave a validator")
	}
	if _, err := httpservertest.NewResponseValidator([]byte("not a spec: ["), nil); err == nil {
		t.Fatal("a broken spec gave a validator")
	}
}

// recordingTB records Errorf instead of failing.
type recordingTB struct {
	testing.TB
	errors []string
}

func (r *recordingTB) Errorf(format string, args ...any) {
	r.errors = append(r.errors, fmt.Sprintf(format, args...))
}

// TestHarness_ValidateResponses: with the option, a response outside the contract is reported.
func TestHarness_ValidateResponses(t *testing.T) {
	t.Parallel()
	op := httpserver.Operation{ID: "getMe", Method: http.MethodGet, Path: "/api/v1/me", Pattern: "GET /api/v1/me", Action: "self.read"}
	rec := &recordingTB{TB: t}
	h := httpservertest.New(rec, httpservertest.Options{Contract: rest.Spec, ValidateResponses: true, Routes: []httpservertest.Route{{
		Op: op, Handler: httpservertest.Authorized("self.read", func(w http.ResponseWriter, _ *http.Request) {
			httpservertest.JSON(w, http.StatusOK, map[string]any{"is_owner": false})
		}),
	}}})
	r := h.Do(h.NewRequest(http.MethodGet, "/api/v1/me", "", "Authorization", httpservertest.GoodToken))
	if r.StatusCode != http.StatusOK || len(rec.errors) != 1 {
		t.Fatalf("status %d, findings %v; want one finding for the missing members", r.StatusCode, rec.errors)
	}
}

// TestHarness_ValidateResponsesNDJSON: an application/x-ndjson body is validated line by line
// against the schema of the media type (exportAuditEvents, M1-50); a valid stream passes and one
// line outside the schema is reported.
func TestHarness_ValidateResponsesNDJSON(t *testing.T) {
	t.Parallel()
	op := httpserver.Operation{ID: "exportThings", Method: http.MethodGet, Path: "/api/v1/things-export", Pattern: "GET /api/v1/things-export", Action: "thing.read", Download: true}
	for _, c := range []struct {
		body     string
		findings int
	}{
		{"{\"a\":1}\n{\"b\":2}\n", 0},
		{"{\"a\":1}\n[1]\n", 1},
	} {
		rec := &recordingTB{TB: t}
		h := httpservertest.New(rec, httpservertest.Options{ValidateResponses: true, Routes: []httpservertest.Route{{
			Op: op, Handler: httpservertest.Authorized("thing.read", func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/x-ndjson")
				_, _ = w.Write([]byte(c.body))
			}),
		}}})
		r := h.Do(h.NewRequest(http.MethodGet, "/api/v1/things-export", "", "Authorization", httpservertest.GoodToken))
		if r.StatusCode != http.StatusOK || len(rec.errors) != c.findings {
			t.Errorf("%q: status %d, findings %v; want %d", c.body, r.StatusCode, rec.errors, c.findings)
		}
	}
}
