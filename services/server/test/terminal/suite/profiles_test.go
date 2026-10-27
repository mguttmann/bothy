// SPDX-License-Identifier: Apache-2.0

package suite

import (
	"maps"
	"strings"
	"testing"
)

// TestProfiles_Relations proves the loader of Design note 4: the profiles of the repository pass,
// every key is a row of the configuration reference, and each relation the scenarios rely on fails
// with both key names when it is broken.
func TestProfiles_Relations(t *testing.T) {
	repo := mustRepo(t)
	p, err := LoadProfiles(repo.ProfilesPath())
	if err != nil {
		t.Fatal(err)
	}
	keys, err := ConfigKeys(repo.Root)
	if err != nil {
		t.Fatal(err)
	}
	overlay, err := MultiNodeClusterMode(repo.Root)
	if err != nil {
		t.Fatal(err)
	}
	if probs := p.Check(keys, overlay); len(probs) > 0 {
		t.Fatalf("the profiles of the repository break relations:\n%s", strings.Join(probs, "\n"))
	}
	broken := []struct {
		name    string
		profile string
		set     map[string]string
		want    []string
	}{
		{"token outlives the opening", "default", map[string]string{"terminal.open_timeout": "20s"}, []string{"terminal.session_token_ttl", "auth.clock_leeway", "terminal.open_timeout"}},
		{"token too short for TM-T03", "default", map[string]string{"terminal.session_token_ttl": "2s", "terminal.open_timeout": "20s"}, []string{"terminal.session_token_ttl", "auth.clock_leeway"}},
		{"quick reattach beyond grace", "limits", map[string]string{"terminal.quick_reattach_window": "40s"}, []string{"terminal.quick_reattach_window", "terminal.detach_grace"}},
		{"heartbeat beyond grace", "limits", map[string]string{"terminal.host_heartbeat_interval": "20s"}, []string{"terminal.host_heartbeat_interval", "terminal.host_heartbeat_miss_limit", "terminal.detach_grace"}},
		{"idle beyond maximum", "limits", map[string]string{"terminal.max_session_duration": "20s"}, []string{"terminal.idle_timeout", "terminal.idle_grace", "terminal.max_session_duration"}},
		{"detach beyond idle", "limits", map[string]string{"terminal.detach_grace": "40s", "terminal.max_session_duration": "2m"}, []string{"terminal.detach_grace", "terminal.idle_timeout"}},
		{"default reaches idle", "default", map[string]string{"terminal.idle_timeout": "1m"}, []string{"terminal.idle_timeout"}},
		{"low rate limit", "default", map[string]string{"ratelimit.rl.terminal.burst": "5"}, []string{"ratelimit.rl.terminal.burst"}},
		{"long sweep", "default", map[string]string{"terminal.sweep_interval": "5m"}, []string{"terminal.sweep_interval"}},
		{"session max age shortened", "default", map[string]string{"auth.session_max_age": "1h"}, []string{"auth.session_max_age"}},
		{"unknown key", "default", map[string]string{"terminal.no_such_key": "1s"}, []string{"terminal.no_such_key"}},
		{"cluster mode differs", "twonode", map[string]string{"cluster.mode": "single"}, []string{"cluster.mode"}},
		{"phone window below the patience", "mobile", map[string]string{"terminal.quick_reattach_window": "20s"}, []string{"terminal.quick_reattach_window", "mobile"}},
	}
	for _, b := range broken {
		t.Run(b.name, func(t *testing.T) {
			q := cloneProfiles(p)
			prof := q.Profiles[b.profile]
			for k, v := range b.set {
				prof.Server[k] = v
			}
			probs := q.Check(keys, overlay)
			if len(probs) == 0 {
				t.Fatalf("a broken profile passes: %v", b.set)
			}
			all := strings.Join(probs, "\n")
			for _, w := range b.want {
				if !strings.Contains(all, w) {
					t.Errorf("the problems do not name %s:\n%s", w, all)
				}
			}
		})
	}
	q := cloneProfiles(p)
	q.Profiles["default"].HostService["host_service.entry_scan_timeout"] = "30m"
	if probs := q.Check(keys, overlay); !strings.Contains(strings.Join(probs, "\n"), "host_service.entry_scan_timeout") {
		t.Errorf("a long host_service.entry_scan_timeout passes: %v", probs)
	}
}

// cloneProfiles returns a deep copy of the server and host-service maps of p.
func cloneProfiles(p *Profiles) *Profiles {
	q := &Profiles{Format: p.Format, Profiles: map[string]Profile{}}
	for n, pr := range p.Profiles {
		q.Profiles[n] = Profile{Server: maps.Clone(pr.Server), HostService: maps.Clone(pr.HostService)}
	}
	return q
}
