// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"net/http"
	"testing"
	"time"

	instanceapp "github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// TestGetInstance_AppLock (brief M5-08 Design note 1): getInstance carries app_lock with
// required from security.app_lock_required and after_ms from client.app_lock_after for every
// authenticated caller, owner or not; the public discovery call does not name the policy, and
// an unauthenticated getInstance is refused.
func TestGetInstance_AppLock(t *testing.T) {
	t.Parallel()
	instanceRow := func(w *world) {
		w.sql(`INSERT INTO instance (instance_id, schema_version, sync_epoch, setup_completed_at) VALUES ($1, 14, $2, now())`,
			ids.New().String(), ids.New().String())
	}
	policy := func(t *testing.T, body map[string]any) map[string]any {
		t.Helper()
		lock, ok := body["app_lock"].(map[string]any)
		if !ok {
			t.Fatalf("app_lock %v, want an object", body["app_lock"])
		}
		return lock
	}
	t.Run("defaults", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		instanceRow(w)
		w.person("alex", "active", true)
		w.person("sam", "active", false)
		want := float64(wire.DefaultInstanceConfig().AppLockAfter.Milliseconds())
		for _, name := range []string{"alex", "sam"} {
			lock := policy(t, w.want(w.do(name, http.MethodGet, "/api/v1/instance", "", ""), http.StatusOK, ""))
			if lock["required"] != false || lock["after_ms"] != want {
				t.Errorf("%s: app_lock %v, want required false and after_ms %v", name, lock, want)
			}
		}
		discovery := w.want(w.do("", http.MethodGet, "/api/v1/instance/discovery", "", ""), http.StatusOK, "")
		if v, ok := discovery["app_lock"]; ok {
			t.Errorf("discovery names app_lock %v, want none", v)
		}
		w.want(w.do("", http.MethodGet, "/api/v1/instance", "", ""), http.StatusUnauthorized, "")
	})
	t.Run("required", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t, func(c *instanceapp.Config) {
			c.AppLockRequired = true
			c.AppLockAfter = 90 * time.Second
		})
		instanceRow(w)
		w.person("sam", "active", false)
		lock := policy(t, w.want(w.do("sam", http.MethodGet, "/api/v1/instance", "", ""), http.StatusOK, ""))
		if lock["required"] != true || lock["after_ms"] != float64(90000) {
			t.Errorf("app_lock %v, want required true and after_ms 90000", lock)
		}
	})
}
