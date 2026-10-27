// SPDX-License-Identifier: Apache-2.0

package suite

import (
	"bytes"
	"fmt"
	"maps"
	"os"
	"regexp"
	"slices"
	"strconv"
	"time"

	"go.yaml.in/yaml/v3"
)

// TMT03Budget is the time TM-T03 needs between the first opening and the replay of its message 1
// (disconnect, attach, connect); the first token must live at least this long (Design note 4). A
// test constant, not a product limit.
const TMT03Budget = 15 * time.Second

// Bounds of Design note 4 that the profiles must keep (test constants).
const (
	// defaultFloor: every window of the profile default is at least this long, so no scenario
	// reaches it by accident.
	defaultFloor = 10 * time.Minute
	// minRate is the least rate limit for the parallel cases of a package.
	minRate = 1000
	// minSessions is the least session limit per person and host.
	minSessions = 50
	// maxPeriod bounds the periods the scenarios wait for (sweeps, periodic runs, probes).
	maxPeriod = 10 * time.Second
	// maxPairingTTL bounds terminal.pairing_code_ttl and terminal.step_up_max_age, for which
	// TM-T20, TM-T07 and ID-T28 wait.
	maxPairingTTL = time.Minute
	// mobileQuickFloor: terminal.quick_reattach_window of the profile mobile is at least the
	// patience of the client suites (apps/client/e2e_test/support/e2e_app.dart), so the phone
	// suite's return from the background within the window never depends on the machine's speed
	// (brief M5-13).
	mobileQuickFloor = 3 * time.Minute
)

// Profiles is profiles.yaml of the suite.
type Profiles struct {
	Format   int                `yaml:"format"`
	Profiles map[string]Profile `yaml:"profiles"`
}

// Profile holds the server keys and the host-service keys of one profile.
type Profile struct {
	// Server are dotted server keys with their values as text.
	Server map[string]string `yaml:"server"`
	// HostService are host_service.* keys with their YAML values.
	HostService map[string]any `yaml:"host_service"`
}

// Value returns the value of key in the profile as text.
func (p Profile) Value(key string) (string, bool) {
	if v, ok := p.Server[key]; ok {
		return v, true
	}
	if v, ok := p.HostService[key]; ok {
		return fmt.Sprint(v), true
	}
	return "", false
}

// Duration returns the value of key as a duration of the configuration grammar.
//
// Errors: a missing key or a value that is no duration.
func (p Profile) Duration(key string) (time.Duration, error) {
	v, ok := p.Value(key)
	if !ok {
		return 0, fmt.Errorf("suite: %s is not set in the profile", key)
	}
	return ParseDuration(v)
}

// Int returns the value of key as a whole number.
func (p Profile) Int(key string) (int, error) {
	v, ok := p.Value(key)
	if !ok {
		return 0, fmt.Errorf("suite: %s is not set in the profile", key)
	}
	return strconv.Atoi(v)
}

// durationRE is the duration grammar of the configuration (M1-07): a whole number and one unit.
var durationRE = regexp.MustCompile(`^(0|[1-9][0-9]*)(ms|s|m|h|d)$`)

// ParseDuration parses a duration of the configuration grammar ("30s", "15m", "200ms").
//
// Errors: a value outside the grammar.
func ParseDuration(s string) (time.Duration, error) {
	m := durationRE.FindStringSubmatch(s)
	if m == nil {
		return 0, fmt.Errorf("suite: %q is no duration of the configuration grammar", s)
	}
	n, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		return 0, err
	}
	unit := map[string]time.Duration{"ms": time.Millisecond, "s": time.Second, "m": time.Minute, "h": time.Hour, "d": 24 * time.Hour}[m[2]]
	return time.Duration(n) * unit, nil
}

// LoadProfiles decodes profiles.yaml strictly.
//
// Errors: a decode error naming the field.
func LoadProfiles(path string) (*Profiles, error) {
	b, err := os.ReadFile(path) //nolint:gosec // G304: the profiles file of the suite
	if err != nil {
		return nil, err
	}
	var p Profiles
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("profiles.yaml: %w", err)
	}
	if p.Format != 1 {
		return nil, fmt.Errorf("profiles.yaml: format %d, want 1", p.Format)
	}
	return &p, nil
}

// Check returns every broken relation of Design note 4, each naming its keys: keys that are no
// row of the configuration reference (keys), the relations of the windows, the bounds for the
// parallel cases and the periods the scenarios wait for, and cluster.mode of twonode against the
// value of M2-47's overlay (overlayMode).
func (ps *Profiles) Check(keys map[string]bool, overlayMode string) []string {
	var probs []string
	for _, name := range slices.Sorted(maps.Keys(ps.Profiles)) {
		p := ps.Profiles[name]
		add := func(format string, args ...any) { probs = append(probs, name+": "+fmt.Sprintf(format, args...)) }
		for _, k := range slices.Sorted(maps.Keys(p.Server)) {
			if !keys[k] {
				add("%s is no row of docs/operations/configuration.md", k)
			}
		}
		for _, k := range slices.Sorted(maps.Keys(p.HostService)) {
			if !keys[k] {
				add("%s is no row of docs/operations/configuration.md", k)
			}
		}
		if _, ok := p.Server["auth.session_max_age"]; ok {
			add("auth.session_max_age is never shortened: it would end the session of the bootstrap owner")
		}
		d := func(k string) time.Duration {
			v, err := p.Duration(k)
			if err != nil {
				add("%v", err)
			}
			return v
		}
		n := func(k string) int {
			v, err := p.Int(k)
			if err != nil {
				add("%s: %v", k, err)
			}
			return v
		}
		ttl, leeway, open := d("terminal.session_token_ttl"), d("auth.clock_leeway"), d("terminal.open_timeout")
		if ttl+leeway >= open {
			add("terminal.session_token_ttl plus auth.clock_leeway (%s) must be below terminal.open_timeout (%s): TM-T04 sees token_expired before the opening times out", ttl+leeway, open)
		}
		if ttl+leeway < TMT03Budget {
			add("terminal.session_token_ttl plus auth.clock_leeway (%s) must leave TM-T03 %s to disconnect, attach and replay", ttl+leeway, TMT03Budget)
		}
		quick, grace := d("terminal.quick_reattach_window"), d("terminal.detach_grace")
		if quick >= grace {
			add("terminal.quick_reattach_window (%s) must be below terminal.detach_grace (%s)", quick, grace)
		}
		if name == "mobile" && quick < mobileQuickFloor {
			add("terminal.quick_reattach_window (%s) is below %s: the phone suite of the profile mobile returns within it on any machine", quick, mobileQuickFloor)
		}
		hb, miss := d("terminal.host_heartbeat_interval"), n("terminal.host_heartbeat_miss_limit")
		if hb*time.Duration(miss) >= grace {
			add("terminal.host_heartbeat_interval times terminal.host_heartbeat_miss_limit (%s) must be below terminal.detach_grace (%s): TM-T16", hb*time.Duration(miss), grace)
		}
		idle, idleGrace, maxDur := d("terminal.idle_timeout"), d("terminal.idle_grace"), d("terminal.max_session_duration")
		if name == "limits" && idle+idleGrace >= maxDur {
			add("terminal.idle_timeout plus terminal.idle_grace (%s) must be below terminal.max_session_duration (%s)", idle+idleGrace, maxDur)
		}
		if name == "limits" && grace >= idle {
			add("terminal.detach_grace (%s) must be below terminal.idle_timeout (%s): TM-T11 sees detach_expired before idle", grace, idle)
		}
		if name == "default" {
			for k, v := range map[string]time.Duration{"terminal.idle_timeout": idle, "terminal.detach_grace": grace, "terminal.max_session_duration": maxDur} {
				if v < defaultFloor {
					add("%s (%s) is below %s: a scenario of the profile default could reach it by accident", k, v, defaultFloor)
				}
			}
		}
		for _, k := range []string{"ratelimit.rl.terminal", "ratelimit.rl.host", "ratelimit.rl.public_auth", "ratelimit.rl.admin", "ratelimit.rl.read", "ratelimit.rl.write"} {
			for _, f := range []string{".per_minute", ".burst"} {
				if n(k+f) < minRate {
					add("%s is below %d: the parallel cases of a package exceed it", k+f, minRate)
				}
			}
		}
		for _, k := range []string{"terminal.max_sessions_per_person", "terminal.max_sessions_per_host"} {
			if n(k) < minSessions {
				add("%s is below %d", k, minSessions)
			}
		}
		for _, k := range []string{"authz.expiry_sweep_interval", "terminal.sweep_interval", "terminal.host_duplicate_probe_timeout", "host_service.credential_check_interval", "host_service.entry_scan_timeout"} {
			if v := d(k); v > maxPeriod {
				add("%s (%s) is above %s: TM-T10, TM-T11, TM-T23 and TM-T29 wait for it", k, v, maxPeriod)
			}
		}
		for _, k := range []string{"terminal.pairing_code_ttl", "terminal.step_up_max_age"} {
			if v := d(k); v > maxPairingTTL {
				add("%s (%s) is above %s: TM-T20, TM-T07 and ID-T28 wait for it", k, v, maxPairingTTL)
			}
		}
		if name == "twonode" {
			if v := p.Server["cluster.mode"]; v != overlayMode {
				add("cluster.mode %q differs from BOTHY_CLUSTER__MODE %q of compose.multi-node.yaml", v, overlayMode)
			}
		}
	}
	return probs
}
