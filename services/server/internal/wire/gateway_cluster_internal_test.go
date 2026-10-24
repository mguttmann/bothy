// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
)

// TestGatewayPlan pins the table of M7-17 Design note 3 (D-M7-17-7): role all with single runs
// nothing across nodes; role api always listens (a separate worker sends hints) and writes
// gateway_presence, and sends signals only with multi; the worker reads the table and never
// listens or writes; role all with multi does everything.
func TestGatewayPlan(t *testing.T) {
	t.Parallel()
	cases := []struct {
		role Role
		mode string
		want GatewayClusterPlan
	}{
		{RoleAll, clusterModeSingle, GatewayClusterPlan{}},
		{RoleAll, clusterModeMulti, GatewayClusterPlan{Multi: true, Signals: true, Listener: true, PresenceWrite: true, PresenceTable: true}},
		{RoleAPI, clusterModeSingle, GatewayClusterPlan{Listener: true, PresenceWrite: true}},
		{RoleAPI, clusterModeMulti, GatewayClusterPlan{Multi: true, Signals: true, Listener: true, PresenceWrite: true}},
		{RoleWorker, clusterModeSingle, GatewayClusterPlan{PresenceTable: true}},
		{RoleWorker, clusterModeMulti, GatewayClusterPlan{Multi: true, PresenceTable: true}},
	}
	for _, c := range cases {
		if got := GatewayPlan(c.mode, c.role); got != c.want {
			t.Errorf("%s %s: %+v, want %+v", c.role, c.mode, got, c.want)
		}
	}
}

// TestPresenceConfig: the defaults bind, and cluster.presence_ttl at or below twice
// cluster.presence_flush_interval is a configuration problem naming both keys.
func TestPresenceConfig(t *testing.T) {
	t.Parallel()
	cfg, err := PresenceConfigOf(loadValues(t))
	if err != nil || cfg.FlushInterval <= 0 || cfg.TTL <= 2*cfg.FlushInterval {
		t.Fatalf("defaults: %+v %v", cfg, err)
	}
	s, err := ConfigSchema()
	if err != nil {
		t.Fatal(err)
	}
	for _, env := range [][]string{
		{"BOTHY_CLUSTER__PRESENCE_FLUSH_INTERVAL=5s", "BOTHY_CLUSTER__PRESENCE_TTL=10s"},
		{"BOTHY_CLUSTER__PRESENCE_FLUSH_INTERVAL=5s", "BOTHY_CLUSTER__PRESENCE_TTL=9s"},
	} {
		_, err := config.Load(s, config.Sources{Env: append([]string{"BOTHY_SERVER__PUBLIC_URL=https://chat.example.com"}, env...)})
		if err == nil {
			t.Errorf("%v: loaded", env)
		}
	}
	if _, err := config.Load(s, config.Sources{Env: []string{"BOTHY_SERVER__PUBLIC_URL=https://chat.example.com",
		"BOTHY_CLUSTER__PRESENCE_FLUSH_INTERVAL=5s", "BOTHY_CLUSTER__PRESENCE_TTL=11s"}}); err != nil {
		t.Errorf("ttl above twice the interval: %v", err)
	}
}
