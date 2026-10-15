// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// codegenFunc runs oapi-codegen with the configuration file on the spec file in the working
// directory dir, where oapi-codegen writes the file the configuration names under output. Tests
// replace it; the generator uses runOapiCodegen.
type codegenFunc func(ctx context.Context, configPath, specPath, dir string) error

// config is one generator run.
type config struct {
	spec, catalog, list string // inputs: openapi.yaml, actions.yaml, operations.yaml
	asyncapi            string // asyncapi.yaml for the gateway protocol versions; "" for none
	codegenConfig       string // api/rest/oapi-codegen.yaml
	restDir             string // output directory of rest.gen.go
	routesDir           string // output directory of routes.gen.go, catalog.gen.go, build.gen.go
	restPkg             string // import path of the package in restDir
	corePkg             string // import path of api/routes; "" when routesDir is api/routes itself
	tempRoot            string // parent of the temporary directory; "" is os.TempDir
	codegen             codegenFunc
	diag                io.Writer // receives the output of oapi-codegen
}

// generate validates the inputs, writes the filtered spec into a temporary directory, runs
// oapi-codegen on it and renders the route files. It writes the outputs only after every step
// succeeded and through writeAll, so a failing run leaves the checked-in files as they are, and
// it removes the temporary directory on every path, also when ctx is cancelled.
func generate(ctx context.Context, c config) (err error) {
	in, err := loadInputs(c.spec, c.catalog, c.list)
	if err != nil {
		return err
	}
	if c.asyncapi != "" {
		if in.gatewayVersions, err = readGatewayVersions(c.asyncapi); err != nil {
			return err
		}
	}
	filtered, err := filterSpec(in.specData, func(id string) bool { return in.enabled[id] })
	if err != nil {
		return err
	}
	var enabled []opMeta
	for _, op := range in.ops {
		if in.enabled[op.ID] {
			enabled = append(enabled, op)
		}
	}
	if err := checkFiltered(filtered, enabled); err != nil {
		return err
	}
	routeFiles, err := renderRoutes(in, c.corePkg, c.restPkg)
	if err != nil {
		return err
	}
	rest, err := runInTempDir(ctx, c, filtered)
	if err != nil {
		return err
	}
	// openapi.gen.yaml is the contract byte for byte, next to the server generated from it, so the
	// request validator of platform/httpserver embeds exactly what the generator read
	// (make codegen-check holds the copy).
	outputs := map[string][]byte{
		filepath.Join(c.restDir, "rest.gen.go"):      rest,
		filepath.Join(c.restDir, "openapi.gen.yaml"): in.specData,
	}
	for name, src := range routeFiles {
		outputs[filepath.Join(c.routesDir, name)] = src
	}
	return writeAll(outputs)
}

// writeAll writes every output to a temporary file next to its target and renames the files only
// after all of them are written, so a failing write leaves the old outputs as they are; each
// rename replaces one file atomically. Temporary files are removed on every error.
func writeAll(outputs map[string][]byte) error {
	paths := slices.Sorted(maps.Keys(outputs))
	temps := map[string]string{}
	cleanup := func() {
		for _, tmp := range temps {
			_ = os.Remove(tmp)
		}
	}
	for _, path := range paths {
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0o750); err != nil {
			cleanup()
			return err
		}
		f, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-")
		if err != nil {
			cleanup()
			return err
		}
		temps[path] = f.Name()
		_, werr := f.Write(outputs[path])
		if err := errors.Join(werr, f.Close()); err != nil {
			cleanup()
			return err
		}
	}
	for _, path := range paths {
		if err := os.Rename(temps[path], path); err != nil {
			cleanup()
			return err
		}
		delete(temps, path)
	}
	return nil
}

// runInTempDir writes the filtered spec into a fresh temporary directory, runs oapi-codegen there
// and returns the generated package. The directory is removed when it returns, also on error.
func runInTempDir(ctx context.Context, c config, filtered []byte) (_ []byte, err error) {
	output, err := codegenOutput(c.codegenConfig)
	if err != nil {
		return nil, err
	}
	configPath, err := filepath.Abs(c.codegenConfig)
	if err != nil {
		return nil, err
	}
	tmp, err := os.MkdirTemp(c.tempRoot, "routesgen-")
	if err != nil {
		return nil, fmt.Errorf("temporary directory: %w", err)
	}
	defer func() {
		err = errors.Join(err, os.RemoveAll(tmp))
	}()
	specPath := filepath.Join(tmp, "openapi.yaml")
	if err := os.WriteFile(specPath, filtered, 0o600); err != nil {
		return nil, err
	}
	codegen := c.codegen
	if codegen == nil {
		codegen = func(ctx context.Context, configPath, specPath, dir string) error {
			return runOapiCodegen(ctx, c.diag, configPath, specPath, dir)
		}
	}
	if err := codegen(ctx, configPath, specPath, tmp); err != nil {
		return nil, err
	}
	rest, err := os.ReadFile(filepath.Clean(filepath.Join(tmp, output)))
	if err != nil {
		return nil, fmt.Errorf("oapi-codegen wrote no %s: %w", output, err)
	}
	return rest, nil
}

// codegenOutput returns the file name the oapi-codegen configuration writes to (key output). It
// must be a bare file name: oapi-codegen v2 resolves it against its working directory and
// ignores -o when a configuration file is given, so the generator runs it in the temporary
// directory and reads the file from there.
func codegenOutput(configPath string) (string, error) {
	doc, _, err := readYAML(configPath)
	if err != nil {
		return "", fmt.Errorf("%s: %w", configPath, err)
	}
	name := scalar(value(doc.Content[0], "output"))
	if name == "" || name != filepath.Base(name) || name == "." || name == ".." {
		return "", fmt.Errorf("%s: output must be a bare file name, got %q", configPath, name)
	}
	return name, nil
}

// goDirective matches the go directive of a go.mod file.
var goDirective = regexp.MustCompile(`(?m)^go[ \t]+(\S+)[ \t]*$`)

// runOapiCodegen runs the oapi-codegen pinned as a tool in go.mod (v2.8.0) in dir. `go tool -n`
// resolves the tool from the current directory, which go generate sets to a directory of the
// module, and prints the path of the built binary; the binary then runs in dir. dir gets a
// go.mod stub with the module's go directive, because oapi-codegen checks that directive (Go
// 1.22 patterns for std-http) in its working directory.
func runOapiCodegen(ctx context.Context, diag io.Writer, configPath, specPath, dir string) error {
	var output bytes.Buffer
	if err := writeGoModStub(ctx, dir); err != nil {
		return err
	}
	resolve := exec.CommandContext(ctx, "go", "tool", "-n", "oapi-codegen") // NOSONAR: the go of go generate; see M1-17/33.
	resolve.Stderr = &output
	bin, err := resolve.Output()
	if err != nil {
		return fmt.Errorf("go tool -n oapi-codegen: %w\n%s", err, output.String())
	}
	cmd := exec.CommandContext(ctx, strings.TrimSpace(string(bin)), "-config", configPath, specPath) //nolint:gosec // the binary is the tool pinned in go.mod, as resolved by go tool
	cmd.Dir = dir
	cmd.Stdout, cmd.Stderr = &output, &output
	err = cmd.Run()
	if diag != nil && output.Len() > 0 {
		_, _ = diag.Write(output.Bytes())
	}
	if err != nil {
		return fmt.Errorf("oapi-codegen: %w\n%s", err, output.String())
	}
	return nil
}

// writeGoModStub writes dir/go.mod with the go directive of the current module (go env GOMOD).
func writeGoModStub(ctx context.Context, dir string) error {
	path, err := exec.CommandContext(ctx, "go", "env", "GOMOD").Output() // NOSONAR: the go of go generate; see M1-17/33.
	if err != nil {
		return fmt.Errorf("go env GOMOD: %w", err)
	}
	mod, err := os.ReadFile(filepath.Clean(strings.TrimSpace(string(path))))
	if err != nil {
		return fmt.Errorf("read the module's go.mod: %w", err)
	}
	m := goDirective.FindSubmatch(mod)
	if m == nil {
		return fmt.Errorf("the module's go.mod has no go directive")
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	stub := fmt.Sprintf("module routesgen.invalid\n\ngo %s\n", m[1])
	return errors.Join(root.WriteFile("go.mod", []byte(stub), 0o600), root.Close())
}
