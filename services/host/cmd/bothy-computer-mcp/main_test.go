// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/host/internal/buildinfo"
)

func TestRunFlags(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run(context.Background(), []string{"--version"}, func(string) string { return "" }, strings.NewReader(""), &out, &errOut); code != exitOK || strings.TrimSpace(out.String()) != buildinfo.Version {
		t.Errorf("--version: %d %q", code, out.String())
	}
	out.Reset()
	if code := run(context.Background(), []string{"--help"}, func(string) string { return "" }, strings.NewReader(""), &out, &errOut); code != exitOK || !strings.Contains(out.String(), "usage") {
		t.Errorf("--help: %d %q", code, out.String())
	}
	errOut.Reset()
	if code := run(context.Background(), []string{"serve"}, func(string) string { return "" }, strings.NewReader(""), &out, &errOut); code != exitUsage || !strings.Contains(errOut.String(), "usage") {
		t.Errorf("unknown argument: %d %q", code, errOut.String())
	}
}

func TestRunNeedsTheEnvironment(t *testing.T) {
	var out, errOut bytes.Buffer
	env := map[string]string{"BOTHY_COMPUTER_NONCE": "nonce-of-alex"}
	code := run(context.Background(), nil, func(k string) string { return env[k] }, strings.NewReader(""), &out, &errOut)
	if code != exitUsage || !strings.Contains(errOut.String(), "BOTHY_COMPUTER_DIR") || strings.Contains(errOut.String(), "nonce-of-alex") {
		t.Errorf("missing environment: %d %q", code, errOut.String())
	}
}

// TestRunServesStdio answers initialize and tools/list on stdin and ends at EOF with 0.
func TestRunServesStdio(t *testing.T) {
	dir := t.TempDir()
	env := map[string]string{
		"BOTHY_COMPUTER_DIR": dir, "BOTHY_COMPUTER_NONCE": "nonce-of-alex", "BOTHY_COMPUTER_TIMEOUT_MS": "1000",
		"BOTHY_COMPUTER_PINS": dir + "/pins.json", "BOTHY_COMPUTER_SERVER": "https://bothy.example.com",
	}
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{}}}` + "\n" +
		`{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}` + "\n")
	var out, errOut bytes.Buffer
	if code := run(context.Background(), nil, func(k string) string { return env[k] }, in, &out, &errOut); code != exitOK {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("%d answer lines: %q", len(lines), out.String())
	}
	var init struct {
		Result struct {
			ProtocolVersion string `json:"protocolVersion"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &init); err != nil || init.Result.ProtocolVersion != "2025-06-18" {
		t.Errorf("initialize answer %s", lines[0])
	}
	if !strings.Contains(lines[1], `"name":"batch"`) {
		t.Errorf("tools/list answer %s", lines[1])
	}
}
