// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"net/http"
	"testing"

	instanceapp "github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestGetInstance_GrafanaEmbedOwnerOnly (brief M6-17 Design note 3, 25 9.2 item 6): getInstance
// carries grafana_embed_url only for an owner and only when console.grafana_embed_url is set, with
// the same filter as telemetry_dashboard_url: the check runs in filter mode, so the call of a
// person who is no owner writes no deny audit and simply has no member.
func TestGetInstance_GrafanaEmbedOwnerOnly(t *testing.T) {
	t.Parallel()
	const url = "https://grafana.example.com/d/ops/operations?orgId=1"
	instanceRow := func(w *world) {
		w.sql(`INSERT INTO instance (instance_id, schema_version, sync_epoch, setup_completed_at) VALUES ($1, 14, $2, now())`,
			ids.New().String(), ids.New().String())
	}
	t.Run("configured", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t, func(c *instanceapp.Config) { c.GrafanaEmbedURL = url })
		instanceRow(w)
		w.person("alex", "active", true)
		w.person("sam", "active", false)
		owner := w.want(w.do("alex", http.MethodGet, "/api/v1/instance", "", ""), http.StatusOK, "")
		if owner["grafana_embed_url"] != url {
			t.Errorf("owner grafana_embed_url %v, want %q", owner["grafana_embed_url"], url)
		}
		sam := w.want(w.do("sam", http.MethodGet, "/api/v1/instance", "", ""), http.StatusOK, "")
		if v, ok := sam["grafana_embed_url"]; ok {
			t.Errorf("Sam got grafana_embed_url %v, want none", v)
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
		if v, ok := owner["grafana_embed_url"]; ok {
			t.Errorf("owner got grafana_embed_url %v without the key set", v)
		}
	})
}
