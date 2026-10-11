// SPDX-License-Identifier: Apache-2.0

// Command archgen generates the go-arch-lint configuration of the server from the module
// register, so that the register stays the only source of module boundaries
// (docs/spec/02-architektur.md, "Modulregister als Quelle"; ADR 0050).
//
// It reads architecture/modules.yaml, rejects a register that breaks a rule of the format (with
// file, line, column and YAML path for every problem) or whose exclude entries would also hit
// other directories of the module (the directory of the output file), and writes
// .go-arch-lint.yml. Run it from
// services/server through go generate:
//
//	go generate ./tools/archgen
//
// or directly, with the flag defaults shown:
//
//	go run ./tools/archgen -register architecture/modules.yaml -out .go-arch-lint.yml
//
// Exit codes: 0 written, 1 invalid register or I/O error, 2 usage error.
package main

//go:generate go run . -register ../../architecture/modules.yaml -out ../../.go-arch-lint.yml

import (
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

// run is main without the process: it parses args, generates the config and returns the exit
// code. Problems and errors go to stderr; -h prints the usage to stdout.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("archgen", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	registerPath := fs.String("register", "architecture/modules.yaml", "module register to read")
	outPath := fs.String("out", ".go-arch-lint.yml", "go-arch-lint configuration to write")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			usage(stdout, fs)
			return exitOK
		}
		say(stderr, "archgen: %v\n", err)
		usage(stderr, fs)
		return exitUsage
	}
	if fs.NArg() > 0 {
		say(stderr, "archgen: unexpected argument %q\n", fs.Arg(0))
		usage(stderr, fs)
		return exitUsage
	}

	r, err := readRegister(*registerPath)
	if err != nil {
		say(stderr, "%v\n", err)
		return exitFailure
	}
	// The config lies in the module root, where go-arch-lint resolves the exclude entries.
	problems, err := checkDirectories(r, filepath.Dir(filepath.Clean(*outPath)))
	if err != nil {
		say(stderr, "archgen: read directories: %v\n", err)
		return exitFailure
	}
	if len(problems) > 0 {
		for _, p := range problems {
			say(stderr, "%s\n", p.format(*registerPath))
		}
		return exitFailure
	}
	data, err := generate(r)
	if err == nil {
		err = writeFile(*outPath, data)
	}
	if err != nil {
		say(stderr, "archgen: write %s: %v\n", *outPath, err)
		return exitFailure
	}
	return exitOK
}

// say writes a message for the user. A failing write to stdout or stderr leaves nothing to report
// it to, so its error is dropped on purpose.
func say(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

func usage(w io.Writer, fs *flag.FlagSet) {
	say(w, "Usage: archgen [-register file] [-out file]\n\n"+
		"Generates the go-arch-lint configuration from the module register.\n\nFlags:\n")
	fs.SetOutput(w)
	fs.PrintDefaults()
	fs.SetOutput(io.Discard)
}

// writeFile replaces path with data: it writes a temporary file beside path and renames it over
// path, so a run that stops halfway (a full disk, a signal) leaves the old file intact instead of
// a truncated config. The new file gets the usual permissions of a checked-in file (0666 before
// umask) through os.Create, unlike os.WriteFile and os.CreateTemp with their explicit modes.
func writeFile(path string, data []byte) (err error) {
	path = filepath.Clean(path)
	tmp := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+"."+rand.Text()+".tmp")
	f, err := os.Create(filepath.Clean(tmp))
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			// Best effort: the error worth reporting is the one that stopped the write.
			_ = os.Remove(tmp)
		}
	}()
	if _, err = f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
