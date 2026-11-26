// SPDX-License-Identifier: Apache-2.0

//go:build computere2e

package computer

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/OWNER/bothy/services/server/test/terminal/suite"
)

// The image sentinels (the method of CU-T18 with the real images): windows of windowLen bytes of
// every image the agent got, at offsets that are multiples of three, so the standard base64 of a
// window is a part of the base64 of the image; windowsPerImage of them from the middle on, where
// the compressed data no other file shares begins.
const (
	windowLen       = 48
	windowsPerImage = 3
)

// sentinel is one window with the forms a server could store it in: the raw bytes, lowercase hex
// (bytea in a dump, a hex log field) and standard and URL-safe base64.
type sentinel struct {
	name  string
	forms [][]byte
}

// sentinels returns the windows of the images of calls.
func sentinels(t *testing.T, calls []call) []sentinel {
	t.Helper()
	var out []sentinel
	for _, c := range calls {
		for _, img := range c.Images {
			raw, err := base64.StdEncoding.DecodeString(img.Data)
			if err != nil {
				t.Fatalf("step %d: image data: %v", c.Step, err)
			}
			for i := range windowsPerImage {
				off := (len(raw)/2 + i*len(raw)/(2*windowsPerImage)) / 3 * 3
				if off+windowLen > len(raw) {
					continue
				}
				w := raw[off : off+windowLen]
				out = append(out, sentinel{
					name:  c.Tool + " step " + strconv.Itoa(c.Step) + " window " + strconv.Itoa(i),
					forms: [][]byte{w, []byte(hex.EncodeToString(w)), []byte(base64.StdEncoding.EncodeToString(w)), []byte(base64.RawURLEncoding.EncodeToString(w))},
				})
			}
		}
	}
	return out
}

// checkNoImageOnServer fails when a window of a returned image is in a data dump of the database
// (rows, audit, events), the logs of the Compose project or the telemetry export of the server
// (CU-T18 through the whole path), or in the journal of the desktop's host service (27 13.2). The
// positive control: the output of the scripted agent on the runner machine holds every window.
func checkNoImageOnServer(t *testing.T, env *suite.Env, d *desktop, r *runnerMachine, calls []call) {
	t.Helper()
	ctx := t.Context()
	ss := sentinels(t, calls)
	if len(ss) < 2*windowsPerImage {
		t.Fatalf("%d image windows, want the windows of at least two images", len(ss))
	}
	control, err := r.h.Exec(ctx, nil, "cat", callsFile)
	if err != nil || control.Code != 0 {
		t.Fatalf("the output of the scripted agent: %v", err)
	}
	for _, s := range ss {
		if !bytes.Contains(control.Stdout, s.forms[2]) {
			t.Fatalf("positive control: the agent's output lacks %s", s.name)
		}
	}
	sources := map[string][]byte{}
	dump, err := env.Stack.Exec(ctx, "postgres", "pg_dump", "-U", "postgres", "--data-only", "bothy")
	if err != nil {
		t.Fatal(err)
	}
	sources["database dump"] = dump
	logs, err := env.Stack.Logs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	sources["compose logs"] = logs
	export := filepath.Join(t.TempDir(), "collector")
	if err := env.Stack.CopyFrom(ctx, "otel-collector", "/var/lib/otelcol-smoke/.", export); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(export)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(export, e.Name())) //nolint:gosec // G304: the copied export in the test's temp dir
		if err != nil {
			t.Fatal(err)
		}
		sources["collector "+e.Name()] = b
	}
	journal, err := d.h.Journal(ctx)
	if err != nil {
		t.Fatal(err)
	}
	sources["desktop host service journal"] = journal
	for where, b := range sources {
		for _, s := range ss {
			for _, f := range s.forms {
				if bytes.Contains(b, f) {
					t.Errorf("%s holds bytes of the image %s", where, s.name)
					break
				}
			}
		}
	}
}
