// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/platform/lifecycle"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// setMinServer sets instance.min_server_version of db as the superuser.
func setMinServer(t *testing.T, db *pgtest.DB, v string) {
	t.Helper()
	if _, err := db.SuperConn(t).Exec(context.Background(), "UPDATE instance SET min_server_version = $1", v); err != nil {
		t.Fatal(err)
	}
}

// TestStart_MinServerVersionAboveOwn_NotReady: a server below instance.min_server_version keeps
// running for every role but never becomes ready; /readyz names only the check, the log line
// names both versions, /healthz stays 200 (02 "Start" step 5, 16 "Upgrade" item 4).
func TestStart_MinServerVersionAboveOwn_NotReady(t *testing.T) {
	t.Parallel()
	db := newServeDB(t)
	setMinServer(t, db, "1.0.1")
	for _, role := range []wire.Role{wire.RoleAPI, wire.RoleWorker, wire.RoleAll} {
		t.Run(string(role), func(t *testing.T) {
			t.Parallel()
			p := start(t, role, serveEnv(t, db), wire.Deps{Version: "1.0.0"})
			internal := p.waitInternal(t)
			deadline := time.Now().Add(startupWait)
			for !strings.Contains(p.logs.String(), `"msg":"server started, not ready"`) && time.Now().Before(deadline) {
				time.Sleep(20 * time.Millisecond)
			}
			if logs := p.logs.String(); strings.Contains(logs, `"msg":"server ready"`) || !strings.Contains(logs, `"msg":"server started, not ready"`) {
				t.Fatalf("want the line server started, not ready and no server ready line:\n%s", logs)
			}
			code, body := get(t, internal, "/readyz")
			if code != http.StatusServiceUnavailable || body != `{"status":"unavailable","failed":["schema_requires_newer_server"]}`+"\n" {
				t.Fatalf("/readyz = %d %q; logs:\n%s", code, body, p.logs)
			}
			if code, _ := get(t, internal, "/healthz"); code != http.StatusOK {
				t.Fatalf("/healthz = %d", code)
			}
			logs := p.logs.String()
			if !strings.Contains(logs, `"msg":"server version below instance.min_server_version"`) ||
				!strings.Contains(logs, `"bothy.version.own":"1.0.0"`) || !strings.Contains(logs, `"bothy.version.required":"1.0.1"`) {
				t.Fatalf("no log line with both versions:\n%s", logs)
			}
		})
	}
}

// TestStart_MinServerVersionEqualOrMissing_Ready: an equal minimum and a row without one (a
// newer, compatible schema without the lock) let the server become ready.
func TestStart_MinServerVersionEqualOrMissing_Ready(t *testing.T) {
	t.Parallel()
	for _, minimum := range []any{"1.0.0", nil} {
		db := newServeDB(t)
		if _, err := db.SuperConn(t).Exec(context.Background(), "UPDATE instance SET min_server_version = $1", minimum); err != nil {
			t.Fatal(err)
		}
		p := start(t, wire.RoleAll, serveEnv(t, db), wire.Deps{Version: "1.0.0"})
		waitReadyz(t, p.waitInternal(t), http.StatusOK)
	}
}

// TestStart_InstanceRowMissing_AbortsWithHint: without the instance row the start ends with
// exit code 1 and the hint to run bothy migrate up.
func TestStart_InstanceRowMissing_AbortsWithHint(t *testing.T) {
	t.Parallel()
	db := newServeDB(t)
	if _, err := db.SuperConn(t).Exec(context.Background(), "DELETE FROM instance"); err != nil {
		t.Fatal(err)
	}
	p := start(t, wire.RoleAll, serveEnv(t, db), wire.Deps{})
	if code := p.exit(t); code != lifecycle.ExitFailure {
		t.Fatalf("exit = %d; logs:\n%s", code, p.logs)
	}
	if err := p.runner.Err(); err == nil || !strings.Contains(err.Error(), "instance row is missing") || !strings.Contains(err.Error(), `"bothy migrate up"`) {
		t.Fatalf("Err() = %v", err)
	}
	if p.logs.port("public listener started") != 0 {
		t.Fatal("public listener opened")
	}
}

// TestStart_SetupStateLogged: the start loads the record and logs the setup state, which later
// tickets read from it (02 "Start" step 5).
func TestStart_SetupStateLogged(t *testing.T) {
	t.Parallel()
	db := newServeDB(t)
	p := start(t, wire.RoleAll, serveEnv(t, db), wire.Deps{})
	waitReadyz(t, p.waitInternal(t), http.StatusOK)
	if !strings.Contains(p.logs.String(), `"msg":"instance loaded, setup pending"`) {
		t.Fatalf("no instance line:\n%s", p.logs)
	}
}

// TestInstanceCounters_GuardsPhase: the counters wire builds carry the phase guard; outside a
// unit of work every call is refused before the pool is touched.
func TestInstanceCounters_GuardsPhase(t *testing.T) {
	t.Parallel()
	c := wire.InstanceCounters(nil)
	if _, err := c.Advance(context.Background(), instance.CounterEventSeq, 1); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("Advance outside a unit = %v, want kind internal", err)
	}
	if _, err := c.Current(context.Background(), instance.CounterAuditSeq); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("Current outside a unit = %v, want kind internal", err)
	}
}
