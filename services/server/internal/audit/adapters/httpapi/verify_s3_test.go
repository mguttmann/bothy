// SPDX-License-Identifier: Apache-2.0

//go:build unix

package httpapi_test

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/audit/adapters/s3sink"
	auditapp "github.com/OWNER/bothy/services/server/internal/audit/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// s3Sink returns a constructor of the object store sink on a fresh locked bucket of the shared
// SeaweedFS, with the keys in a secrets directory below the temporary sinkDir.
func s3Sink(t *testing.T) func(string, ids.ID) auditapp.CheckpointSink {
	t.Helper()
	srv := sharedSeaweed(t)
	bucket := srv.newLockedBucket(t)
	return func(sinkDir string, inst ids.ID) auditapp.CheckpointSink {
		for name, value := range map[string]string{"audit_checkpoint_s3_access_key": srv.access, "audit_checkpoint_s3_secret_key": srv.secret} {
			if err := os.WriteFile(filepath.Join(sinkDir, name), []byte(value), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		p, err := secrets.NewProvider(secrets.Options{Dir: sinkDir, CacheTTL: time.Minute})
		if err != nil {
			t.Fatal(err)
		}
		u, err := url.Parse(srv.endpoint)
		if err != nil {
			t.Fatal(err)
		}
		rule, err := egress.ParseRule(u.Host + ";allow_private")
		if err != nil {
			t.Fatal(err)
		}
		sink, err := s3sink.New(context.Background(), s3sink.Config{
			Bucket: bucket, Endpoint: srv.endpoint, Region: "us-east-1", UsePathStyle: true,
			AccessKeyRef: "file:<secrets.dir>/audit_checkpoint_s3_access_key", SecretKeyRef: "file:<secrets.dir>/audit_checkpoint_s3_secret_key",
		}, inst, s3sink.Deps{Egress: egress.Config{AllowedHosts: []egress.Rule{rule}, Timeout: 30 * time.Second, MaxResponseBytes: 1 << 20}, Secrets: p})
		if err != nil {
			t.Fatal(err)
		}
		// The S3 gateway may learn of a bucket of weed shell a moment later.
		deadline := testkit.Deadline()
		for _, err = sink.List(context.Background()); err != nil && time.Now().Before(deadline); _, err = sink.List(context.Background()) {
			time.Sleep(500 * time.Millisecond)
		}
		if err != nil {
			t.Fatalf("the bucket is not reachable: %v", err)
		}
		return sink
	}
}

// TestVerifyDetectsRewriteViaS3Checkpoints is SEC-AUD-3 with the object store sink (M7-22): the
// job signs two checkpoints into the bucket, then the attacker changes row k below both,
// recomputes every later hash and rewrites audit_checkpoints to the new hashes. The walk alone
// finds nothing; audit.verify fails at the first rewritten checkpoint, read from the object
// store.
func TestVerifyDetectsRewriteViaS3Checkpoints(t *testing.T) {
	t.Parallel()
	e := newEnvWithSink(t, s3Sink(t))
	e.owner("owner")
	e.write(9, actor.Actor{}, "grant.create", "grant", ids.New())
	e.run()
	first := e.head()
	e.write(3, actor.Actor{}, "grant.create", "grant", ids.New())
	e.run()
	second := e.head()
	e.write(2, actor.Actor{}, "grant.create", "grant", ids.New())
	stored, err := e.sink.List(context.Background())
	if err != nil || len(stored) != 2 || stored[0].Seq != first || stored[1].Seq != second {
		t.Fatalf("the object store holds %+v, %v; want the checkpoints at %d and %d", stored, err, first, second)
	}
	if resp, res := e.verify("owner", "{}"); resp.StatusCode != http.StatusOK || !res.Ok || res.CheckpointsCompared != 2 {
		t.Fatalf("before the attack: %d %s", resp.StatusCode, resp.Body)
	}
	const k = 4
	e.attack("audit_events", "audit_events_no_update_delete", `UPDATE audit_events SET details = '{"changed":true}' WHERE seq = $1`, k)
	e.rewriteFrom(k)
	for _, c := range []int64{first, second} {
		e.attack("audit_checkpoints", "audit_checkpoints_no_update_delete",
			"UPDATE audit_checkpoints SET hash = (SELECT hash FROM audit_events WHERE seq = $1) WHERE seq = $1", c)
	}
	resp, res := e.verify("owner", "{}")
	if resp.StatusCode != http.StatusOK || res.Ok || broken(res) != first || reason(res) != "checkpoint_mismatch" {
		t.Fatalf("rewritten chain: %d %s, want checkpoint_mismatch at %d", resp.StatusCode, resp.Body, first)
	}
}
