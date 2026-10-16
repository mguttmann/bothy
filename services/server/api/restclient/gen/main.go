// SPDX-License-Identifier: Apache-2.0

// Command gen generates the API client package api/restclient from docs/api/openapi.yaml with
// the oapi-codegen pinned in go.mod (06 "Codegenerierung", docs/DECISIONS.md D-M8-10-1). Unlike
// api/routes/gen it keeps every operation of the contract: a client of the public API calls
// operations the server of an older release does not serve yet. The output of oapi-codegen is cut
// at top-level declarations into restclient.gen.go, restclient_02.gen.go and so on, each below
// repo.max_file_kb (api/internal/gosplit); parts of an earlier run that this run does not write
// are removed.
//
// Exit codes: 0 success, 1 failed generation, 2 usage error.
//
// Example (from services/server/api):
//
//	go run ./restclient/gen -spec ../../../docs/api/openapi.yaml -config restclient/oapi-codegen.yaml -out restclient
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"github.com/OWNER/bothy/services/server/api/internal/gosplit"
)

// maxFileBytes is repo.max_file_kb (docs/spec/23-qualitaet.md) in bytes: the hook
// check-added-large-files refuses larger files.
const maxFileBytes = 1024 * 1024

// output is the file name oapi-codegen writes (key output of the configuration) and the name of
// the first part.
const output = "restclient.gen.go"

// partName matches the names of the parts after the first.
var partName = regexp.MustCompile(`^restclient_[0-9]{2}\.gen\.go$`)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Stderr, runOapiCodegen)
	stop()
	os.Exit(code)
}

// codegenFunc runs oapi-codegen with configPath on specPath in dir.
type codegenFunc func(ctx context.Context, configPath, specPath, dir string) error

// run parses args, generates and writes the parts, and returns the exit code; errors go to stderr.
func run(ctx context.Context, args []string, stderr io.Writer, codegen codegenFunc) int {
	fs := flag.NewFlagSet("restclientgen", flag.ContinueOnError)
	fs.SetOutput(stderr)
	spec := fs.String("spec", "", "path of docs/api/openapi.yaml")
	config := fs.String("config", "", "oapi-codegen configuration (output: "+output+")")
	out := fs.String("out", "", "directory of the package restclient")
	if err := fs.Parse(args); err != nil || *spec == "" || *config == "" || *out == "" || fs.NArg() > 0 {
		_, _ = fmt.Fprintln(stderr, "usage: gen -spec FILE -config FILE -out DIR")
		return 2
	}
	if err := generate(ctx, *spec, *config, *out, codegen); err != nil {
		_, _ = fmt.Fprintf(stderr, "restclientgen: %v\n", err)
		return 1
	}
	return 0
}

// generate runs oapi-codegen in a temporary directory, splits its output and writes the parts to
// out, removing parts of an earlier run that are not written again. The temporary directory is
// removed on every path.
func generate(ctx context.Context, spec, config, out string, codegen codegenFunc) (err error) {
	specPath, err := filepath.Abs(spec)
	if err != nil {
		return err
	}
	configPath, err := filepath.Abs(config)
	if err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", "restclientgen-")
	if err != nil {
		return fmt.Errorf("temporary directory: %w", err)
	}
	defer func() { err = errors.Join(err, os.RemoveAll(tmp)) }()
	if err := codegen(ctx, configPath, specPath, tmp); err != nil {
		return err
	}
	src, err := os.ReadFile(filepath.Clean(filepath.Join(tmp, output)))
	if err != nil {
		return fmt.Errorf("oapi-codegen wrote no %s: %w", output, err)
	}
	parts, err := gosplit.Chunks(markDefaultClients(src), maxFileBytes)
	if err != nil {
		return err
	}
	return writeParts(out, parts)
}

// defaultClient matches the line on which oapi-codegen builds a default http.Client when the
// caller passes no doer.
var defaultClient = regexp.MustCompile(`(?m)^(\t+[A-Za-z_.]+ = &http\.Client\{\})$`)

// markDefaultClients appends the nosemgrep marker of bothy-go-egress-only to every default
// http.Client of the generated code (INT-W34/3): the import tool always passes its egress doer
// (D-M8-10-8), so the default is never built there.
func markDefaultClients(src []byte) []byte {
	return defaultClient.ReplaceAll(src, []byte("$1 // nosemgrep: bothy-go-egress-only -- default of the generated client; bothy import passes its egress doer (D-M8-10-8)"))
}

// fileName returns the file name of part i (0-based).
func fileName(i int) string {
	if i == 0 {
		return output
	}
	return fmt.Sprintf("restclient_%02d.gen.go", i+1)
}

// writeParts writes the parts and removes stale parts.
func writeParts(out string, parts [][]byte) error {
	if len(parts) > 99 {
		return errors.New("more parts than two-digit names allow")
	}
	keep := map[string]bool{}
	for i, p := range parts {
		name := fileName(i)
		keep[name] = true
		old, err := os.ReadFile(filepath.Clean(filepath.Join(out, name)))
		if err == nil && bytes.Equal(old, p) {
			continue
		}
		if err := os.WriteFile(filepath.Join(out, name), p, 0o600); err != nil { // #nosec G703 -- out is the -out flag of go:generate, name a fixed pattern
			return err
		}
	}
	entries, err := os.ReadDir(out)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if partName.MatchString(e.Name()) && !keep[e.Name()] {
			if err := os.Remove(filepath.Join(out, e.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

// runOapiCodegen runs the oapi-codegen pinned as a tool in go.mod in dir: `go tool -n` resolves
// it from the current directory, which go generate sets to a directory of the module.
func runOapiCodegen(ctx context.Context, configPath, specPath, dir string) error {
	var diag bytes.Buffer
	resolve := exec.CommandContext(ctx, "go", "tool", "-n", "oapi-codegen")
	resolve.Stderr = &diag
	bin, err := resolve.Output()
	if err != nil {
		return fmt.Errorf("go tool -n oapi-codegen: %w\n%s", err, diag.String())
	}
	cmd := exec.CommandContext(ctx, strings.TrimSpace(string(bin)), "-config", configPath, specPath) //nolint:gosec // the binary is the tool pinned in go.mod, as resolved by go tool
	cmd.Dir = dir
	cmd.Stdout, cmd.Stderr = &diag, &diag
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("oapi-codegen: %w\n%s", err, diag.String())
	}
	return nil
}
