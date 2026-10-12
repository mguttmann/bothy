// SPDX-License-Identifier: Apache-2.0

package ratelimit_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/platform/ratelimit"
)

// t0 is the start instant of the fake clock in every test.
var t0 = time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)

// chapterClass is one row of the table "Rate-Limit-Klassen" of chapter 05.
type chapterClass struct {
	class ratelimit.Class
	key   string // the column "Schlüssel", verbatim
}

// chapterClasses reads the table "Rate-Limit-Klassen" of docs/spec/05-aktionen.md, so the tests
// follow the chapter instead of a hard-coded count.
func chapterClasses(t *testing.T) []chapterClass {
	t.Helper()
	text := readSpec(t, "05-aktionen.md")
	start := strings.Index(text, "<!-- BEGIN GENERATED: rate_limit_classes -->")
	end := strings.Index(text, "<!-- END GENERATED: rate_limit_classes -->")
	if start < 0 || end < start {
		t.Fatal("05-aktionen.md has no generated table rate_limit_classes")
	}
	var out []chapterClass
	for _, line := range strings.Split(text[start:end], "\n") {
		if !strings.HasPrefix(line, "| `") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		out = append(out, chapterClass{
			class: ratelimit.Class(strings.Trim(strings.TrimSpace(cells[0]), "`")),
			key:   strings.TrimSpace(cells[1]),
		})
	}
	if len(out) == 0 {
		t.Fatal("the table rate_limit_classes of 05-aktionen.md has no rows")
	}
	return out
}

// readSpec returns the chapter file of docs/spec found above the working directory.
func readSpec(t *testing.T, chapter string) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if info, err := os.Stat(filepath.Join(dir, "docs", "spec")); err == nil && info.IsDir() {
			root, err := os.OpenRoot(filepath.Join(dir, "docs", "spec"))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = root.Close() }()
			data, err := root.ReadFile(chapter)
			if err != nil {
				t.Fatal(err)
			}
			return string(data)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no directory with docs/spec above the working directory")
		}
		dir = parent
	}
}

// defaultLimits are the defaults of 05 "Konfigurationswerte dieses Kapitels".
var defaultLimits = map[ratelimit.Class]ratelimit.Limits{
	"rl.public_auth": {PerMinute: 30, Burst: 10}, "rl.gateway_connect": {PerMinute: 12, Burst: 4},
	"rl.read": {PerMinute: 600, Burst: 100}, "rl.write": {PerMinute: 120, Burst: 30},
	"rl.message": {PerMinute: 60, Burst: 20}, "rl.trigger": {PerMinute: 30, Burst: 10},
	"rl.interaction": {PerMinute: 60, Burst: 20}, "rl.upload": {PerMinute: 30, Burst: 10},
	"rl.search": {PerMinute: 60, Burst: 20}, "rl.typing": {PerMinute: 30, Burst: 5},
	"rl.admin": {PerMinute: 60, Burst: 20}, "rl.export": {PerMinute: 2, Burst: 2},
	"rl.agent_runtime": {PerMinute: 1200, Burst: 200}, "rl.webhook_in": {PerMinute: 60, Burst: 20},
	"rl.import": {PerMinute: 120, Burst: 30}, "rl.terminal": {PerMinute: 20, Burst: 5},
	"rl.host": {PerMinute: 120, Burst: 30},
}

// config returns a configuration with the defaults of 05, max_tracked_keys and sweep_interval of
// 02, and the given classes replaced.
func config(override map[ratelimit.Class]ratelimit.Limits) ratelimit.Config {
	classes := map[ratelimit.Class]ratelimit.Limits{}
	for c, l := range defaultLimits {
		classes[c] = l
	}
	for c, l := range override {
		classes[c] = l
	}
	return ratelimit.Config{Classes: classes, MaxTrackedKeys: 100000, SweepInterval: time.Minute}
}

// newLimiter builds a limiter on a fake clock at t0 without metrics.
func newLimiter(t testing.TB, cfg ratelimit.Config) (*ratelimit.Limiter, *clock.Fake) {
	t.Helper()
	clk := clock.NewFake(t0)
	l, err := ratelimit.New(cfg, clk, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return l, clk
}

// allow calls Allow and fails the test on an error.
func allow(t testing.TB, l *ratelimit.Limiter, class ratelimit.Class, key string) ratelimit.Decision {
	t.Helper()
	d, err := l.Allow(class, key)
	if err != nil {
		t.Fatalf("Allow(%s, %s): %v", class, key, err)
	}
	return d
}
