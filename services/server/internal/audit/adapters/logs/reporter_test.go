// SPDX-License-Identifier: Apache-2.0

package logs_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/OWNER/bothy/services/server/internal/audit/adapters/logs"
	"github.com/OWNER/bothy/services/server/internal/audit/app"
	"github.com/OWNER/bothy/services/server/internal/audit/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
)

// Compile-time check: the reporter is the outgoing port of app.
var _ app.Reporter = (*logs.Reporter)(nil)

// lines runs report against a JSON logger and returns the decoded records.
func lines(t *testing.T, report func(r *logs.Reporter)) []map[string]any {
	t.Helper()
	var buf bytes.Buffer
	report(logs.New(slog.New(slog.NewJSONHandler(&buf, nil))))
	var out []map[string]any
	for _, l := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatal(err)
		}
		out = append(out, m)
	}
	return out
}

func TestReporter_KeysAreInTheCatalogue(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	got := lines(t, func(r *logs.Reporter) {
		r.ChainBroken(ctx, 41)
		r.UnknownReason(ctx, "grant.create", "brand_new")
		r.DenialsDropped(ctx, "grant.create", "no_grant", 3)
		r.WriteFailed(ctx, fmt.Errorf("wrapped: %w", &pgconn.PgError{Code: "57014", Message: "secret detail 192.0.2.44"}))
		r.IPKeyFailed(ctx, errors.New("secret detail"))
		r.SinkNotConfigured(ctx)
		r.VerifyFailed(ctx, domain.CheckpointMismatch)
		r.ExportCut(ctx, errors.New("secret detail"))
		r.RecordFailed(ctx, errors.New("secret detail"))
	})
	if len(got) != 9 {
		t.Fatalf("%d lines", len(got))
	}
	want := []struct{ level, errType string }{
		{"ERROR", "audit_chain_broken"}, {"ERROR", "audit_unknown_reason"}, {"ERROR", "audit_denials_dropped"},
		{"WARN", "audit_write_failed"}, {"ERROR", "audit_ip_key_failed"},
		{"WARN", "audit_checkpoint_sink_not_configured"}, {"ERROR", "audit_chain_broken"}, {"ERROR", "audit_export_cut"}, {"ERROR", "audit_record_failed"},
	}
	for i, l := range got {
		if l["level"] != want[i].level || l[attr.ErrorType] != want[i].errType {
			t.Errorf("line %d = %v", i, l)
		}
		for k := range l {
			if k != "time" && k != "level" && k != "msg" && !attr.Allowed(k) {
				t.Errorf("line %d has key %s outside the attribute catalogue", i, k)
			}
		}
		if strings.Contains(l["msg"].(string), "secret") || strings.Contains(l["msg"].(string), "192.0.2.44") {
			t.Errorf("line %d carries the error text: %v", i, l)
		}
	}
	if got[3][attr.DBResponseStatusCode] != "57014" || got[4][attr.DBResponseStatusCode] != nil {
		t.Errorf("SQLSTATE: %v / %v", got[3], got[4])
	}
	if got[6][attr.Reason] != "checkpoint_mismatch" {
		t.Errorf("verify failure = %v", got[6])
	}
	if got[1][attr.Reason] != "brand_new" || got[2][attr.Action] != "grant.create" || got[2][attr.Reason] != "no_grant" {
		t.Errorf("lines = %v", got)
	}
}
