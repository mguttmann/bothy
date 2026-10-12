// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/otel"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

// telemetryKeyVar holds the test master key; telemetry.pseudonym_key_ref points to it with env:.
const telemetryKeyVar = "BOTHY_SECRET_TELEMETRY_TEST"

// telemetryTestKey is a master key of 33 bytes; the EXAMPLE marker tells secret scanners it is none.
const telemetryTestKey = "EXAMPLE-pseudonym-key-of-33-bytes"

// telemetryTestID is a principal id in canonical lowercase form.
const telemetryTestID = "4f6c2a1e-9b3d-4c8a-8e2f-1a2b3c4d5e6f"

// telemetryEnv is the environment of the command with key as the master key.
func telemetryEnv(key string) []string {
	return []string{
		"BOTHY_TELEMETRY__PSEUDONYM_KEY_REF=env:" + telemetryKeyVar,
		telemetryKeyVar + "=" + key,
	}
}

// fixedNow is the clock of the tests that leave out --at.
func fixedNow() time.Time { return time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC) }

// runTelemetry runs "bothy telemetry <args>" with env and the fixed clock.
func runTelemetry(t *testing.T, env []string, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := telemetryCommand(t.Context(), args, env, fixedNow, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// libraryOutput is the output the command must print: the values of otel.Pseudonymizer for the
// same key, id, instant and the default rotation of 30 days.
func libraryOutput(t *testing.T, key, id string, at time.Time) string {
	t.Helper()
	p, err := secrets.NewProvider(secrets.Options{Dir: t.TempDir(), Env: []string{telemetryKeyVar + "=" + key}})
	if err != nil {
		t.Fatalf("provider: %v", err)
	}
	ps, err := otel.NewPseudonymizer(t.Context(), p, "env:"+telemetryKeyVar, 30*24*time.Hour, nil)
	if err != nil {
		t.Fatalf("pseudonymizer: %v", err)
	}
	pseudo, start, err := ps.PseudonymWithPeriod(t.Context(), id, at)
	if err != nil {
		t.Fatalf("pseudonym: %v", err)
	}
	return "pseudonym " + pseudo + "\nperiod_start " + start.Format(time.RFC3339) + "\n"
}

// TestTelemetryPseudonymCommand: the output for a known key, id and instant equals the library
// value, and that value follows privacy.md "Pseudonyms" item 1 computed here by hand.
func TestTelemetryPseudonymCommand(t *testing.T) {
	t.Parallel()
	// Another period than fixedNow, so a command that ignored --at would fail.
	at := time.Date(2026, 8, 1, 9, 30, 0, 0, time.UTC)
	want := libraryOutput(t, telemetryTestKey, telemetryTestID, at)

	// By hand: period index, period key, pseudonym.
	const rotation = 30 * 24 * 3600
	period := at.Unix() / rotation
	pk := hmac.New(sha256.New, []byte(telemetryTestKey))
	pk.Write([]byte("bothy/telemetry-pseudonym/v1/" + strconv.FormatInt(period, 10)))
	m := hmac.New(sha256.New, pk.Sum(nil))
	m.Write([]byte(telemetryTestID))
	byHand := "pseudonym " + hex.EncodeToString(m.Sum(nil))[:32] + "\nperiod_start " +
		time.Unix(period*rotation, 0).UTC().Format(time.RFC3339) + "\n"
	if want != byHand {
		t.Fatalf("library %q, by hand %q", want, byHand)
	}

	for _, args := range [][]string{
		{"pseudonym", telemetryTestID, "--at", "2026-08-01T09:30:00Z"},
		{"pseudonym", "--at", "2026-08-01T09:30:00Z", telemetryTestID},
		{"pseudonym", strings.ToUpper(telemetryTestID), "--at=2026-08-01T11:30:00+02:00"},
	} {
		code, stdout, stderr := runTelemetry(t, telemetryEnv(telemetryTestKey), args...)
		if code != exitOK || stdout != want {
			t.Errorf("%q: exit %d, stdout %q, stderr %q; want 0 and %q", args, code, stdout, stderr, want)
		}
		if strings.Contains(stdout+stderr, telemetryTestKey) {
			t.Errorf("%q: output contains the key", args)
		}
	}

	// Without --at the period of now.
	code, stdout, _ := runTelemetry(t, telemetryEnv(telemetryTestKey), "pseudonym", telemetryTestID)
	if code != exitOK || stdout != libraryOutput(t, telemetryTestKey, telemetryTestID, fixedNow()) || stdout == want {
		t.Errorf("without --at: exit %d, stdout %q", code, stdout)
	}

	// Registered in the command table; the clock there is the system clock.
	var out, errOut bytes.Buffer
	args := []string{"telemetry", "pseudonym", telemetryTestID, "--at", "2026-08-01T09:30:00Z"}
	if code := run(context.Background(), args, telemetryEnv(telemetryTestKey), &out, &errOut); code != exitOK || out.String() != want {
		t.Errorf("run telemetry pseudonym: exit %d, stdout %q, stderr %q", code, out.String(), errOut.String())
	}
}

// TestTelemetryPseudonymPeriods: another master key and the next period give other pseudonyms.
func TestTelemetryPseudonymPeriods(t *testing.T) {
	t.Parallel()
	_, base, _ := runTelemetry(t, telemetryEnv(telemetryTestKey), "pseudonym", telemetryTestID)
	_, otherKey, _ := runTelemetry(t, telemetryEnv(strings.Repeat("k", 32)), "pseudonym", telemetryTestID)
	_, nextPeriod, _ := runTelemetry(t, telemetryEnv(telemetryTestKey), "pseudonym", telemetryTestID,
		"--at", fixedNow().Add(30*24*time.Hour).Format(time.RFC3339))
	first := func(s string) string { line, _, _ := strings.Cut(s, "\n"); return line }
	if first(base) == first(otherKey) || first(base) == first(nextPeriod) || first(base) == "" {
		t.Fatalf("pseudonyms do not change: %q, %q, %q", base, otherKey, nextPeriod)
	}
}

// TestTelemetryPseudonymErrors: usage errors and a configuration that does not load exit 2, a
// key that cannot be read or is too short exits 1; no output carries the key.
func TestTelemetryPseudonymErrors(t *testing.T) {
	t.Parallel()
	short := strings.Repeat("s", 31)
	cases := []struct {
		name string
		env  []string
		args []string
		code int
	}{
		{"no subcommand", telemetryEnv(telemetryTestKey), nil, exitUsage},
		{"unknown subcommand", telemetryEnv(telemetryTestKey), []string{"unknown"}, exitUsage},
		{"missing id", telemetryEnv(telemetryTestKey), []string{"pseudonym"}, exitUsage},
		{"two ids", telemetryEnv(telemetryTestKey), []string{"pseudonym", telemetryTestID, telemetryTestID}, exitUsage},
		{"id not a UUID", telemetryEnv(telemetryTestKey), []string{"pseudonym", "alex"}, exitUsage},
		{"bad --at", telemetryEnv(telemetryTestKey), []string{"pseudonym", telemetryTestID, "--at", "yesterday"}, exitUsage},
		{"unknown flag", telemetryEnv(telemetryTestKey), []string{"pseudonym", telemetryTestID, "--role", "api"}, exitUsage},
		{"empty --config", telemetryEnv(telemetryTestKey), []string{"pseudonym", telemetryTestID, "--config", ""}, exitUsage},
		{"configuration does not load", append(telemetryEnv(telemetryTestKey), "BOTHY_SERVER__ROLE=nobody"), []string{"pseudonym", telemetryTestID}, exitUsage},
		{"key too short", telemetryEnv(short), []string{"pseudonym", telemetryTestID}, exitFailure},
		{"key missing", []string{"BOTHY_TELEMETRY__PSEUDONYM_KEY_REF=env:" + telemetryKeyVar}, []string{"pseudonym", telemetryTestID}, exitFailure},
	}
	for _, tc := range cases {
		code, stdout, stderr := runTelemetry(t, tc.env, tc.args...)
		if code != tc.code || stdout != "" || stderr == "" {
			t.Errorf("%s: exit %d, stdout %q, stderr %q; want %d, empty stdout, a reason on stderr", tc.name, code, stdout, stderr, tc.code)
		}
		if strings.Contains(stderr, telemetryTestKey) || strings.Contains(stderr, short) {
			t.Errorf("%s: stderr contains the key", tc.name)
		}
	}
}

// TestTelemetryHelpExit0: help goes to stdout with exit 0.
func TestTelemetryHelpExit0(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"help"}, {"--help"}, {"pseudonym", "-h"}} {
		code, stdout, stderr := runTelemetry(t, nil, args...)
		if code != exitOK || !strings.HasPrefix(stdout, "Usage: bothy telemetry pseudonym") || stderr != "" {
			t.Errorf("%q: exit %d, stdout %q, stderr %q", args, code, stdout, stderr)
		}
	}
}
