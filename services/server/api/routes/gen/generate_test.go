// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// Import paths of the fixture packages, as api/generate.go passes them.
const (
	fixtureRestPkg = "github.com/OWNER/bothy/services/server/api/routes/internal/fixture/rest"
	corePkg        = "github.com/OWNER/bothy/services/server/api/routes"
	codegenConfig  = "../../rest/oapi-codegen.yaml"
)

// fixtureConfig returns a generator configuration for the fixture inputs at the given paths,
// writing into fresh output directories.
func fixtureConfig(t *testing.T, spec, catalog, list string) config {
	t.Helper()
	out := t.TempDir()
	return config{
		spec: spec, catalog: catalog, list: list, codegenConfig: codegenConfig,
		restDir: filepath.Join(out, "rest"), routesDir: filepath.Join(out, "routes"),
		restPkg: fixtureRestPkg, corePkg: corePkg,
		tempRoot: t.TempDir(),
		diag:     &bytes.Buffer{},
	}
}

// TestGenerate_Fixture_Golden runs the whole pipeline, including the pinned oapi-codegen, on the
// fixture inputs and requires the checked-in fixture packages byte for byte.
func TestGenerate_Fixture_Golden(t *testing.T) {
	cfg := fixtureConfig(t, fixtureDir+"/openapi.yaml", fixtureDir+"/actions.yaml", fixtureDir+"/operations.yaml")
	if err := generate(context.Background(), cfg); err != nil {
		t.Fatalf("generate: %v", err)
	}
	golden := map[string]string{
		filepath.Join(cfg.restDir, "rest.gen.go"):      "../internal/fixture/rest/rest.gen.go",
		filepath.Join(cfg.restDir, "openapi.gen.yaml"): fixtureDir + "/openapi.yaml",
		filepath.Join(cfg.routesDir, "routes.gen.go"):  "../internal/fixture/routes/routes.gen.go",
		filepath.Join(cfg.routesDir, "catalog.gen.go"): "../internal/fixture/routes/catalog.gen.go",
		filepath.Join(cfg.routesDir, "build.gen.go"):   "../internal/fixture/routes/build.gen.go",
		filepath.Join(cfg.routesDir, "meta.gen.go"):    "../internal/fixture/routes/meta.gen.go",
	}
	for got, want := range golden {
		if g, w := readText(t, got), readText(t, want); g != w {
			t.Errorf("%s differs from the generated %s; run go generate ./api/...", want, filepath.Base(got))
		}
	}
	assertEmptyDir(t, cfg.tempRoot)
}

// fakeCodegen stands in for oapi-codegen: it records the filtered spec it was given and writes
// a placeholder package, or fails with err.
type fakeCodegen struct {
	spec []byte
	dir  string
	err  error
}

func (f *fakeCodegen) run(_ context.Context, _, specPath, dir string) error {
	f.dir = dir
	data, err := os.ReadFile(filepath.Clean(specPath))
	if err != nil {
		return err
	}
	f.spec = data
	if f.err != nil {
		return f.err
	}
	return os.WriteFile(filepath.Join(dir, "rest.gen.go"), []byte("package rest\n"), 0o600)
}

// assertEmptyDir requires dir to exist and to be empty.
func assertEmptyDir(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("%s still holds %d entries, want the temporary directory removed", dir, len(entries))
	}
}

func TestGenerate_RemovesTempDir(t *testing.T) {
	spec, catalog, list := fixturePaths(t, "")
	t.Run("success", func(t *testing.T) {
		cfg := fixtureConfig(t, spec, catalog, list)
		fake := &fakeCodegen{}
		cfg.codegen = fake.run
		if err := generate(context.Background(), cfg); err != nil {
			t.Fatal(err)
		}
		if filepath.Dir(fake.dir) != cfg.tempRoot {
			t.Errorf("filtered spec in %s, want a directory made below %s", fake.dir, cfg.tempRoot)
		}
		assertEmptyDir(t, cfg.tempRoot)
	})
	t.Run("codegen fails", func(t *testing.T) {
		cfg := fixtureConfig(t, spec, catalog, list)
		cfg.codegen = (&fakeCodegen{err: errors.New("boom")}).run
		if err := generate(context.Background(), cfg); err == nil || !strings.Contains(err.Error(), "boom") {
			t.Fatalf("generate = %v, want the codegen error", err)
		}
		assertEmptyDir(t, cfg.tempRoot)
	})
	t.Run("codegen writes nothing", func(t *testing.T) {
		cfg := fixtureConfig(t, spec, catalog, list)
		cfg.codegen = func(context.Context, string, string, string) error { return nil }
		if err := generate(context.Background(), cfg); err == nil {
			t.Fatal("generate = nil, want an error for the missing output")
		}
		assertEmptyDir(t, cfg.tempRoot)
	})
}

// TestGenerate_NoPartialOutputOnError proves that a failing run leaves existing output alone,
// so a broken input can never leave a half-regenerated package behind.
func TestGenerate_NoPartialOutputOnError(t *testing.T) {
	cases := map[string]string{"codegen fails": "", "invalid list": "duplicate-entry"}
	for name, negative := range cases {
		t.Run(name, func(t *testing.T) {
			spec, catalog, list := fixturePaths(t, negative)
			cfg := fixtureConfig(t, spec, catalog, list)
			cfg.codegen = (&fakeCodegen{err: errors.New("boom")}).run
			old := map[string]string{
				filepath.Join(cfg.restDir, "rest.gen.go"):     "old rest",
				filepath.Join(cfg.routesDir, "routes.gen.go"): "old routes",
			}
			for path, content := range old {
				if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
					t.Fatal(err)
				}
				writeFile(t, path, content)
			}
			if err := generate(context.Background(), cfg); err == nil {
				t.Fatal("generate = nil, want an error")
			}
			for path, content := range old {
				if got := readText(t, path); got != content {
					t.Errorf("%s = %q after a failed run, want it unchanged", path, got)
				}
			}
			if _, err := os.Stat(filepath.Join(cfg.routesDir, "build.gen.go")); !os.IsNotExist(err) {
				t.Errorf("build.gen.go written by a failed run (stat: %v)", err)
			}
		})
	}
}

// decode parses YAML text into a generic value for comparisons.
func decode(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var v map[string]any
	if err := yaml.Unmarshal(data, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestGenerate_FilteredSpecKeepsAllComponents(t *testing.T) {
	data := []byte(readText(t, fixtureDir+"/openapi.yaml"))
	filtered, err := filterSpec(data, func(string) bool { return false })
	if err != nil {
		t.Fatal(err)
	}
	orig, got := decode(t, data), decode(t, filtered)
	if !reflect.DeepEqual(got["components"], orig["components"]) {
		t.Error("components of the filtered spec differ from the original")
	}
	for _, key := range []string{"openapi", "info", "servers", "security"} {
		if !reflect.DeepEqual(got[key], orig[key]) {
			t.Errorf("%s of the filtered spec differs from the original", key)
		}
	}
	if !bytes.Contains(filtered, []byte("# Fixture spec of the route generator")) {
		t.Error("the filtered spec lost the comments of the original")
	}
	if paths, ok := got["paths"].(map[string]any); !ok || len(paths) != 0 {
		t.Errorf("paths = %v, want an empty mapping for an empty enable list", got["paths"])
	}
}

func TestGenerate_FilteredSpecDropsWebhooksAndDisabledMethods(t *testing.T) {
	data := []byte(readText(t, fixtureDir+"/openapi.yaml"))
	filtered, err := filterSpec(data, func(id string) bool { return id == "getThing" })
	if err != nil {
		t.Fatal(err)
	}
	got := decode(t, filtered)
	if _, ok := got["webhooks"]; ok {
		t.Error("the filtered spec still has webhooks")
	}
	paths := got["paths"].(map[string]any)
	if _, ok := paths["/things"]; ok {
		t.Error("/things has no enabled method left and must be dropped")
	}
	item, ok := paths["/things/{thing_id}"].(map[string]any)
	if !ok {
		t.Fatal("/things/{thing_id} was dropped, but getThing is enabled")
	}
	if _, ok := item["patch"]; ok {
		t.Error("the disabled patch method of /things/{thing_id} survived")
	}
	if _, ok := item["get"]; !ok {
		t.Error("the enabled get method of /things/{thing_id} was dropped")
	}
	if !reflect.DeepEqual(got["components"], decode(t, data)["components"]) {
		t.Error("components changed")
	}
}

// TestGenerate_EmptyListRendersAnyServer covers the empty enable list that the real surface has
// today: the route files compile without any generated server symbol.
func TestGenerate_EmptyListRendersAnyServer(t *testing.T) {
	spec, catalog, list := fixturePaths(t, "")
	writeFile(t, list, "version: 1\nenabled: []\nlater:\n  - operation_id: createThing\n    etappe: M1\n"+
		"  - operation_id: getThing\n    etappe: M1\n  - operation_id: updateThing\n    etappe: M2\n    ticket: M2-01\n")
	cfg := fixtureConfig(t, spec, catalog, list)
	cfg.corePkg = ""
	fake := &fakeCodegen{}
	cfg.codegen = fake.run
	if err := generate(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	build := readText(t, filepath.Join(cfg.routesDir, "build.gen.go"))
	if !strings.Contains(build, "type Server = any") || strings.Contains(build, "import") {
		t.Errorf("build.gen.go for an empty list:\n%s", build)
	}
	routesFile := readText(t, filepath.Join(cfg.routesDir, "routes.gen.go"))
	if !strings.Contains(routesFile, "var operations = []Operation{}") {
		t.Errorf("routes.gen.go for an empty list:\n%s", routesFile)
	}
	if strings.Contains(string(fake.spec), "operationId") {
		t.Error("the filtered spec still holds operations")
	}
}

func TestRun_ExitCodes(t *testing.T) {
	spec, catalog, list := fixturePaths(t, "duplicate-entry")
	var stderr bytes.Buffer
	if code := run(context.Background(), []string{"-spec", spec}, &stderr); code != 2 {
		t.Errorf("missing flags: exit %d, want 2 (%s)", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "routesgen: flag -catalog is required\n") {
		t.Errorf("missing flags: stderr %q does not name the first missing flag -catalog", stderr.String())
	}
	stderr.Reset()
	if code := run(context.Background(), []string{"-unknown"}, &stderr); code != 2 {
		t.Errorf("unknown flag: exit %d, want 2", code)
	}
	stderr.Reset()
	out := t.TempDir()
	args := []string{"-spec", spec, "-catalog", catalog, "-list", list, "-codegen-config", codegenConfig,
		"-rest-dir", filepath.Join(out, "rest"), "-routes-dir", filepath.Join(out, "routes"), "-rest-pkg", fixtureRestPkg}
	if code := run(context.Background(), args, &stderr); code != 1 || !strings.Contains(stderr.String(), "getThing") {
		t.Errorf("invalid list: exit %d, stderr %q; want 1 naming getThing", code, stderr.String())
	}
}

// TestGenerate_WriteFailureLeavesOutputs: outputs are written to temporary files first and renamed
// only when every file is written, so a failing write leaves the old files and no temporary file.
func TestGenerate_WriteFailureLeavesOutputs(t *testing.T) {
	spec, catalog, list := fixturePaths(t, "")
	cfg := fixtureConfig(t, spec, catalog, list)
	cfg.codegen = (&fakeCodegen{}).run
	if err := os.MkdirAll(cfg.restDir, 0o750); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(cfg.restDir, "rest.gen.go"), "old rest")
	writeFile(t, cfg.routesDir, "a file where the routes directory belongs") // no temporary file fits
	if err := generate(context.Background(), cfg); err == nil {
		t.Fatal("generate = nil, want the write error")
	}
	if got := readText(t, filepath.Join(cfg.restDir, "rest.gen.go")); got != "old rest" {
		t.Errorf("rest.gen.go = %q after a failed write phase, want it unchanged", got)
	}
	entries, err := os.ReadDir(cfg.restDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("rest dir holds %d entries, want only rest.gen.go (no temporary file left)", len(entries))
	}
}

// TestGenerate_CancelledRunRemovesTempDir: an interrupted run (signal, cancelled context) stops the
// tool and still removes the temporary directory.
func TestGenerate_CancelledRunRemovesTempDir(t *testing.T) {
	spec, catalog, list := fixturePaths(t, "")
	cfg := fixtureConfig(t, spec, catalog, list)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := generate(ctx, cfg); err == nil {
		t.Fatal("generate = nil with a cancelled context")
	}
	assertEmptyDir(t, cfg.tempRoot)
}
