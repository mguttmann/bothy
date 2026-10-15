// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

// TestServeAbortsOnUnknownTextSearchConfig holds the start check of the text search (M3-13,
// docs/DECISIONS.md D-M3-13-6): with a search.languages entry the database does not have, bothy
// serve ends with exit code 1 right after the schema check, no public listener opens, and the
// output names search.languages and its entry but not the configured name.
func TestServeAbortsOnUnknownTextSearchConfig(t *testing.T) {
	t.Parallel()
	db := newServeDB(t)
	c := startChild(t, serveTestEnv(t, db, `BOTHY_SEARCH__LANGUAGES=["german","english","klingonisch"]`), "serve")
	if code := c.wait(t, 60*time.Second); code != 1 {
		t.Fatalf("exit %d; stderr:\n%s", code, c.stderr)
	}
	out := c.stdout.String() + c.stderr.String()
	if !strings.Contains(out, "search.languages entry 3") {
		t.Errorf("output does not name search.languages entry 3:\n%s", out)
	}
	if strings.Contains(out, "klingonisch") {
		t.Errorf("output names the configured value:\n%s", out)
	}
	if strings.Contains(out, "public listener started") {
		t.Errorf("a public listener opened:\n%s", out)
	}
}

// TestConfigCheckRejectsLocaleMapLanguageMissing holds the row of 14 section 12 "Start mit
// Sprache in search.locale_map, die in search.languages fehlt": bothy config check exits 1 and
// bothy serve exits with the configuration error code before any listener; both name
// search.locale_map and not the configured value.
func TestConfigCheckRejectsLocaleMapLanguageMissing(t *testing.T) {
	t.Parallel()
	env := withPublicURL([]string{`BOTHY_SEARCH__LOCALE_MAP={"de":"german","fr":"french"}`})
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"config", "check"}, env, &stdout, &stderr); code != 1 {
		t.Fatalf("config check: exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	if msg := stderr.String(); !strings.Contains(msg, "search.locale_map") || strings.Contains(msg, "french") {
		t.Errorf("config check: stderr %q", msg)
	}
	stdout.Reset()
	stderr.Reset()
	if code := run(context.Background(), []string{"serve"}, env, &stdout, &stderr); code != 2 {
		t.Fatalf("serve: exit %d, want the configuration error code 2; stderr %q", code, stderr.String())
	}
	out := stdout.String() + stderr.String()
	if !strings.Contains(out, "search.locale_map") || strings.Contains(out, "french") || strings.Contains(out, "listener started") {
		t.Errorf("serve: output %q", out)
	}
}
