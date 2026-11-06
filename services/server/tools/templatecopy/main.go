// SPDX-License-Identifier: Apache-2.0

// Command templatecopy copies the shipped agent templates (templates/agents/<slug>/ at the
// repository root) into the server module, where internal/agents/adapters/templates embeds them: Go embeds
// only files below its module (docs/spec/26-builtin-agents.md 7.2). Every template folder is
// copied byte for byte; files at the root of the source (its README) stay out, and the output
// folder is replaced, so a removed template leaves no stale copy. A symbolic link is refused, as
// the template gate refuses it (26 7.1).
//
// It runs through go generate (directive in internal/agents/adapters/templates/doc.go), and make
// codegen-check holds the copy equal to the source:
//
//	go generate ./internal/agents/adapters/templates/
//
// Exit codes: 0 written, 1 I/O error or a link, 2 usage error.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

func main() { os.Exit(run(os.Args[1:], os.Stderr)) }

// run parses args (-src, the folder of the templates; -out, the folder to replace) and copies.
func run(args []string, stderr io.Writer) int {
	flags := flag.NewFlagSet("templatecopy", flag.ContinueOnError)
	flags.SetOutput(stderr)
	src := flags.String("src", "", "folder of the shipped templates")
	out := flags.String("out", "", "folder to replace with the copy")
	if err := flags.Parse(args); err != nil || *src == "" || *out == "" {
		_, _ = fmt.Fprintln(stderr, "usage: templatecopy -src templates/agents -out agents")
		return 2
	}
	if err := Copy(*src, *out); err != nil {
		_, _ = fmt.Fprintln(stderr, "templatecopy:", err)
		return 1
	}
	return 0
}

// Copy replaces out with a copy of every folder directly below src and everything inside it.
// Errors: an unreadable source, a symbolic link or another file that is not a regular file, and
// write errors.
func Copy(src, out string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(out); err != nil {
		return err
	}
	if err := os.MkdirAll(out, 0o750); err != nil {
		return err
	}
	for _, e := range entries {
		if e.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("%s is a symbolic link", e.Name())
		}
		if !e.IsDir() {
			continue
		}
		if err := copyTree(filepath.Join(src, e.Name()), filepath.Join(out, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

// copyTree copies the folder from to to, regular files and folders only.
func copyTree(from, to string) error {
	return filepath.WalkDir(from, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		target := filepath.Join(to, rel)
		switch {
		case d.IsDir():
			return os.MkdirAll(target, 0o750)
		case !d.Type().IsRegular():
			return errors.New(filepath.ToSlash(filepath.Join(filepath.Base(from), rel)) + " is not a regular file")
		}
		data, err := os.ReadFile(path) //nolint:gosec // a path of the walk below the source folder
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o600) //nolint:gosec // target is the walked path below the output folder
	})
}
