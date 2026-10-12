// SPDX-License-Identifier: Apache-2.0

// Command catalogen generates the closed action catalogue of the server from
// docs/spec/actions.yaml (ADR 0049), so that the catalogue file stays the only source of actions,
// event types, WebSocket ops, terminal channel ops and resource kinds (05 rules 3 to 5, ADR 0017).
//
// It writes two files:
//
//   - internal/kernel/authorize/catalog_ids_gen.go: the identifier types Action, EventType, Op,
//     ResourceKind and ChannelOp with one constant per entry and the Parse and All functions;
//   - internal/authz/domain/catalog_gen.go: the metadata of every entry as domain values.
//
// It runs through go generate (directive in internal/kernel/authorize/doc.go):
//
//	go generate ./internal/kernel/authorize/...
//
// A catalogue that breaks a normalisation rule writes nothing; every problem goes to stderr with
// the entry and the field. Exit codes: 0 written, 1 invalid catalogue or I/O error, 2 usage error.
package main

import (
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/OWNER/bothy/services/server/tools/catalogen/internal/gen"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// Exit codes of run.
const (
	exitOK      = 0
	exitFailure = 1
	exitUsage   = 2
)

// run is main without the process: it parses args, generates both files and returns the exit code.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("catalogen", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	catalog := fs.String("catalog", "docs/spec/actions.yaml", "action catalogue to read")
	ids := fs.String("ids", "catalog_ids_gen.go", "identifier file to write (package authorize)")
	domain := fs.String("domain", "catalog_gen.go", "metadata file to write (package domain)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			usage(stdout, fs)
			return exitOK
		}
		say(stderr, "catalogen: %v\n", err)
		usage(stderr, fs)
		return exitUsage
	}
	if fs.NArg() > 0 {
		say(stderr, "catalogen: unexpected argument %q\n", fs.Arg(0))
		usage(stderr, fs)
		return exitUsage
	}
	if err := generate(*catalog, *ids, *domain); err != nil {
		say(stderr, "%v\n", err)
		return exitFailure
	}
	return exitOK
}

// generate renders both files before it writes either, so an invalid catalogue or a failed write
// changes nothing.
func generate(catalogPath, idsPath, domainPath string) error {
	data, err := os.ReadFile(catalogPath) //nolint:gosec // G304: the path is a flag of a repository tool
	if err != nil {
		return wrap(err)
	}
	c, err := gen.Parse(data)
	if err != nil {
		return err
	}
	reserved, err := declaredNames(filepath.Dir(idsPath), filepath.Base(idsPath))
	if err != nil {
		return err
	}
	if err := gen.CheckReserved(c, reserved); err != nil {
		return err
	}
	idsSrc, err := gen.RenderIDs(c)
	if err != nil {
		return err
	}
	domainSrc, err := gen.RenderDomain(c)
	if err != nil {
		return err
	}
	return writeBoth(idsPath, idsSrc, domainPath, domainSrc)
}

// writeBoth writes both files through temporary files next to them and renames them only when
// both were written; when the second rename fails, the first file gets its previous content back,
// so a failed write leaves the previous files unchanged.
func writeBoth(pathA string, a []byte, pathB string, b []byte) error {
	tmpA, err := writeTemp(pathA, a)
	if err != nil {
		return err
	}
	tmpB, err := writeTemp(pathB, b)
	if err != nil {
		_ = os.Remove(tmpA)
		return err
	}
	old, readErr := os.ReadFile(pathA) //nolint:gosec // G304: the output path is a flag of a repository tool
	if err := os.Rename(tmpA, pathA); err != nil {
		_ = os.Remove(tmpA)
		_ = os.Remove(tmpB)
		return wrap(err)
	}
	if err := os.Rename(tmpB, pathB); err != nil {
		_ = os.Remove(tmpB)
		return errors.Join(wrap(err), restore(pathA, old, readErr))
	}
	return nil
}

// restore puts back the previous content of path after a failed second rename: old when the
// file existed (readErr nil), nothing when it did not.
func restore(path string, old []byte, readErr error) error {
	if errors.Is(readErr, fs.ErrNotExist) {
		return removeQuiet(path)
	}
	if readErr != nil {
		return wrap(readErr)
	}
	tmp, err := writeTemp(path, old)
	if err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return wrap(err)
	}
	return nil
}

func removeQuiet(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return wrap(err)
	}
	return nil
}

// wrap prefixes err with the tool name.
func wrap(err error) error { return fmt.Errorf("catalogen: %w", err) }

// writeTemp writes data to a new temporary file in the directory of path and returns its name.
func writeTemp(path string, data []byte) (string, error) {
	f, err := os.CreateTemp(filepath.Dir(path), ".catalogen-*")
	if err != nil {
		return "", wrap(err)
	}
	name := f.Name()
	_, werr := f.Write(data)
	cerr := f.Close()
	if err := errors.Join(werr, cerr, os.Chmod(name, 0o644)); err != nil { //nolint:gosec // G302: generated source, readable like any source file
		_ = os.Remove(name)
		return "", wrap(err)
	}
	return name, nil
}

// declaredNames returns the top-level names declared in the Go files of dir other than skip,
// generated files (*_gen.go) and tests; a generated constant must not redeclare one of them.
func declaredNames(dir, skip string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, wrap(err)
	}
	var names []string
	fset := token.NewFileSet()
	for _, e := range entries {
		if !handWritten(e, skip) {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, wrap(err)
		}
		for _, d := range f.Decls {
			names = append(names, declNames(d)...)
		}
	}
	return names, nil
}

// handWritten reports a Go source file that is neither skip, generated nor a test.
func handWritten(e fs.DirEntry, skip string) bool {
	n := e.Name()
	return !e.IsDir() && n != skip && strings.HasSuffix(n, ".go") && !strings.HasSuffix(n, "_test.go") &&
		!strings.HasSuffix(n, "_gen.go")
}

// declNames returns the package-level names one declaration introduces (methods introduce none).
func declNames(d ast.Decl) []string {
	var names []string
	switch d := d.(type) {
	case *ast.FuncDecl:
		if d.Recv == nil {
			names = append(names, d.Name.Name)
		}
	case *ast.GenDecl:
		for _, spec := range d.Specs {
			switch sp := spec.(type) {
			case *ast.TypeSpec:
				names = append(names, sp.Name.Name)
			case *ast.ValueSpec:
				for _, id := range sp.Names {
					names = append(names, id.Name)
				}
			}
		}
	}
	return names
}

func usage(w io.Writer, fs *flag.FlagSet) {
	say(w, "usage: catalogen [-catalog file] [-ids file] [-domain file]\n")
	fs.SetOutput(w)
	fs.PrintDefaults()
	fs.SetOutput(io.Discard)
}

// say writes to w; a failed write to stdout or stderr has nowhere to be reported.
func say(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}
