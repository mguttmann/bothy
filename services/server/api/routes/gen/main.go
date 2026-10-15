// SPDX-License-Identifier: Apache-2.0

// Command gen generates the REST layer of the server from the API contract (ADR 0013, plan
// decision P-04). It validates docs/api/openapi.yaml, docs/spec/actions.yaml and the enable list
// api/operations.yaml, writes a copy of the spec that keeps only the enabled operations into a
// temporary directory, runs the oapi-codegen pinned in go.mod on it, and writes:
//
//   - <rest-dir>/rest.gen.go: models of every schema, std-http and strict server of the enabled
//     operations (oapi-codegen),
//   - <routes-dir>/routes.gen.go, catalog.gen.go, build.gen.go: the operation table, the action
//     catalogue and the glue for api/routes,
//   - <routes-dir>/meta.gen.go: the spec version, the gateway protocol versions of -asyncapi and
//     the problem catalogue x-problem-codes (M1-46).
//
// Every problem of the inputs is reported, one per line with file and operation ID or entry;
// nothing is written then. api/generate.go runs it for the real surface and for the fixture of
// its tests; services/server/api/README.md explains the flags.
//
// Exit codes: 0 success, 1 invalid inputs or failed generation, 2 usage error.
//
// Example (from services/server/api):
//
//	go run ./routes/gen -spec ../../../docs/api/openapi.yaml -catalog ../../../docs/spec/actions.yaml \
//	  -list operations.yaml -codegen-config rest/oapi-codegen.yaml -rest-dir rest -routes-dir routes \
//	  -rest-pkg github.com/OWNER/bothy/services/server/api/rest
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// An interrupt cancels the context: oapi-codegen is stopped and the deferred removal of the
	// temporary directory runs before the process exits.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Stderr)
	stop()
	os.Exit(code)
}

// run parses the flags in args, runs the generator and returns the exit code; problems and
// usage go to stderr.
func run(ctx context.Context, args []string, stderr io.Writer) int {
	fs := flag.NewFlagSet("routesgen", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var c config
	// Each flag is named once; the required ones are checked in this order after parsing.
	flags := []struct {
		value    *string
		name     string
		usage    string
		required bool
	}{
		{&c.spec, "spec", "OpenAPI document (docs/api/openapi.yaml)", true},
		{&c.catalog, "catalog", "action catalogue (docs/spec/actions.yaml)", true},
		{&c.list, "list", "enable list (api/operations.yaml)", true},
		{&c.codegenConfig, "codegen-config", "oapi-codegen configuration (api/rest/oapi-codegen.yaml)", true},
		{&c.restDir, "rest-dir", "output directory of rest.gen.go", true},
		{&c.routesDir, "routes-dir", "output directory of the route files", true},
		{&c.restPkg, "rest-pkg", "import path of the package in -rest-dir", true},
		{&c.corePkg, "core-pkg", "import path of api/routes; empty when -routes-dir is api/routes", false},
		{&c.asyncapi, "asyncapi", "AsyncAPI document (docs/api/asyncapi.yaml) for the gateway protocol versions; empty for none", false},
	}
	for _, f := range flags {
		fs.StringVar(f.value, f.name, "", f.usage)
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	for _, f := range flags {
		if f.required && *f.value == "" {
			_, _ = fmt.Fprintf(stderr, "routesgen: flag -%s is required\n", f.name)
			fs.Usage()
			return 2
		}
	}
	c.diag = stderr
	if err := generate(ctx, c); err != nil {
		var problems *problemsError
		if errors.As(err, &problems) {
			_, _ = fmt.Fprintf(stderr, "routesgen: %d problem(s):\n%s\n", len(problems.items), problems.Error())
		} else {
			_, _ = fmt.Fprintf(stderr, "routesgen: %v\n", err)
		}
		return 1
	}
	return 0
}
