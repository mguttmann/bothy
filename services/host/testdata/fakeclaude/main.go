// SPDX-License-Identifier: Apache-2.0

// Command fakeclaude behaves like "claude auth status" for the tests of HD6 (brief M2-42 design
// note 9): it prints the fixture named by the file fake-auth in the checked CLAUDE_CONFIG_DIR (or,
// without it, in $HOME/.claude), exits like the CLI (0 logged in, 1 not), and appends one line with
// its own name to the start log whose path the test sets at build time:
//
//	go build -ldflags "-X main.startLog=<path>" -o <dir>/claude ./testdata/fakeclaude
//
// A second build named node serves as interpreter and as the wrong node of TM-T27. It never calls
// a model and never reads a login.
package main

import (
	"embed"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// startLog is set with -ldflags -X; empty means no log.
var startLog = ""

//go:embed fixtures/*.json fixtures/garbage.txt
var fixtures embed.FS

func main() {
	logStart()
	if filepath.Base(os.Args[0]) == "node" || filepath.Base(os.Args[0]) == "node.exe" {
		// As interpreter: run the script argument as if it were claude.
		if len(os.Args) > 1 {
			os.Args = os.Args[1:]
		}
	}
	if len(os.Args) < 3 || os.Args[1] != "auth" || os.Args[2] != "status" {
		os.Exit(2)
	}
	dir := os.Getenv("CLAUDE_CONFIG_DIR") // nosemgrep: bothy-go-no-model-credentials -- the fake claude of the tests reads only its fixture folder, never a credential
	if dir == "" {
		home := os.Getenv("HOME")
		if home == "" {
			home = os.Getenv("USERPROFILE")
		}
		dir = filepath.Join(home, ".claude")
	}
	name := "none"
	if b, err := os.ReadFile(filepath.Join(dir, "fake-auth")); err == nil {
		name = strings.TrimSpace(string(b))
	}
	switch name {
	case "hang":
		time.Sleep(time.Hour)
	case "garbage":
		b, _ := fixtures.ReadFile("fixtures/garbage.txt")
		_, _ = os.Stdout.Write(b)
		os.Exit(0)
	}
	b, err := fixtures.ReadFile("fixtures/" + name + ".json")
	if err != nil {
		os.Exit(3)
	}
	_, _ = os.Stdout.Write(b)
	if name == "none" {
		os.Exit(1)
	}
}

func logStart() {
	if startLog == "" {
		return
	}
	f, err := os.OpenFile(startLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o666) //nolint:gosec // the log of the test, shared by all session users
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()
	exe, _ := os.Executable()
	_, _ = f.WriteString(exe + "\n")
}
