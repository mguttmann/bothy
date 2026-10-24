// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"net/http"
	"testing"

	instanceapp "github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestGetInstance_TelemetryURL: getInstance carries telemetry_dashboard_url only for an owner
// and only when telemetry.dashboard_url is set (brief M6-06 Design note 4); the owner check runs
// in filter mode, so the call of a person who is no owner writes no deny audit of stats.read.
func TestGetInstance_TelemetryURL(t *testing.T) {
	t.Parallel()
	const url = "https://telemetry.example.com/d/bothy"
	instanceRow := func(w *world) {
		w.sql(`INSERT INTO instance (instance_id, schema_version, sync_epoch, setup_completed_at) VALUES ($1, 14, $2, now())`,
			ids.New().String(), ids.New().String())
	}
	t.Run("configured", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t, func(c *instanceapp.Config) { c.TelemetryDashboardURL = url })
		instanceRow(w)
		w.person("alex", "active", true)
		w.person("sam", "active", false)
		owner := w.want(w.do("alex", http.MethodGet, "/api/v1/instance", "", ""), http.StatusOK, "")
		if owner["telemetry_dashboard_url"] != url {
			t.Errorf("owner telemetry_dashboard_url %v, want %q", owner["telemetry_dashboard_url"], url)
		}
		sam := w.want(w.do("sam", http.MethodGet, "/api/v1/instance", "", ""), http.StatusOK, "")
		if v, ok := sam["telemetry_dashboard_url"]; ok {
			t.Errorf("Sam got telemetry_dashboard_url %v, want none", v)
		}
		if a := w.audits("stats.read"); len(a) != 0 {
			t.Errorf("audit of stats.read %+v, want none (filter mode)", a)
		}
	})
	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		instanceRow(w)
		w.person("alex", "active", true)
		owner := w.want(w.do("alex", http.MethodGet, "/api/v1/instance", "", ""), http.StatusOK, "")
		if v, ok := owner["telemetry_dashboard_url"]; ok {
			t.Errorf("owner got telemetry_dashboard_url %v without the key set", v)
		}
	})
}
