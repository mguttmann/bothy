// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestBinaryOverStdio builds the command as the device does (CGO_ENABLED=0) and drives it over
// real stdin and stdout: a start and an error answer, exit code 0 at EOF, nothing on stderr.
func TestBinaryOverStdio(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a binary")
	}
	bin := filepath.Join(t.TempDir(), "noisedevice")
	t.Setenv("CGO_ENABLED", "0")
	build := exec.CommandContext(t.Context(), "go", "build", "-o", bin, ".") //nolint:gosec // G204: fixed arguments
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	in := `{"op":"start","host_static":"` + enc(make([]byte, 32)) + `","session_id":"` + testSessionID + `","host_id":"` + testHostID + `"}` + "\n" +
		`{"op":"seal","frame":"AQ"}` + "\n"
	cmd := exec.CommandContext(t.Context(), bin) //nolint:gosec // G204: the binary this test built
	cmd.Stdin = strings.NewReader(in)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr received %d bytes, want none", stderr.Len())
	}
	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("%d answer lines, want 2", len(lines))
	}
	var first, second map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil || first["ok"] != true || first["cek"] == nil {
		t.Errorf("start answer not ok with cek (decode error %v)", err)
	}
	if err := json.Unmarshal([]byte(lines[1]), &second); err != nil {
		t.Fatal(err)
	}
	if e, _ := second["error"].(map[string]any); e == nil || e["kind"] != "out_of_order" {
		t.Errorf("seal before the handshake: answer %v, want kind out_of_order", second["error"])
	}
}
