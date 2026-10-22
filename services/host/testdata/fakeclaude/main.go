// SPDX-License-Identifier: Apache-2.0

// Command fakeclaude behaves like "claude auth status" for the tests of HD6 (brief M2-42 design
// note 9): it prints the fixture named by the file fake-auth in the checked CLAUDE_CONFIG_DIR (or,
// without it, in $HOME/.claude), exits like the CLI (0 logged in, 1 not), and appends one line with
// its own name to the start log whose path the test sets at build time:
//
//	go build -ldflags "-X main.startLog=<path>" -o <dir>/claude ./testdata/fakeclaude
//
// A second build named node serves as interpreter and as the wrong node of TM-T27. M2-45 adds the
// interactive mode (any arguments other than "auth status" and "login": it logs its arguments,
// working directory, the names of its environment variables, its CLAUDE_CONFIG_DIR and the SHA-256
// of ANTHROPIC_API_KEY, prints a line and exits on "exit" or the end of input) and the login mode
// (writes fake-auth with claude.ai into its configuration folder, on Linux and Windows also a
// .credentials.json). It never calls a model and never reads a login.
package main

import (
	"bufio"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
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
	dir := configDir()
	switch {
	case len(os.Args) == 2 && os.Args[1] == "login":
		login(dir)
		return
	case len(os.Args) < 3 || os.Args[1] != "auth" || os.Args[2] != "status":
		interactive()
		return
	}
	name := "none"
	// A token in the environment counts as a login of the kind oauth_token unless fake-auth names
	// another (24 7.4: the variable is a subscription login).
	if os.Getenv("CLAUDE_CODE_OAUTH_TOKEN") != "" { // nosemgrep: bothy-go-no-model-credentials -- the fake claude only tests whether the variable is set, never its value
		name = "oauth_token"
	}
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

// configDir is CLAUDE_CONFIG_DIR, or the default folder .claude in the home.
func configDir() string {
	dir := os.Getenv("CLAUDE_CONFIG_DIR") // nosemgrep: bothy-go-no-model-credentials -- the fake claude of the tests reads only its fixture folder, never a credential
	if dir == "" {
		home := os.Getenv("HOME")
		if home == "" {
			home = os.Getenv("USERPROFILE")
		}
		dir = filepath.Join(home, ".claude")
	}
	return dir
}

// login is the login mode: fake-auth names claude.ai; on Linux and Windows a .credentials.json
// lies beside it, where the real agent keeps a subscription login; on macOS the real agent keeps it
// in the keychain, so no file is written (24 7.4).
func login(dir string) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		os.Exit(1)
	}
	if err := os.WriteFile(filepath.Join(dir, "fake-auth"), []byte("claude.ai\n"), 0o600); err != nil {
		os.Exit(1)
	}
	if runtime.GOOS != "darwin" {
		if err := os.WriteFile(filepath.Join(dir, ".credentials.json"), []byte(`{"fake":"example"}`), 0o600); err != nil {
			os.Exit(1)
		}
	}
	_, _ = os.Stdout.WriteString("Login successful\n")
}

// interactive is the interactive mode: one line in the start log, a prompt line, then it echoes
// lines until "exit" or the end of input.
func interactive() {
	if startLog != "" {
		names := []string{}
		for _, kv := range os.Environ() {
			if k, _, ok := strings.Cut(kv, "="); ok {
				names = append(names, k)
			}
		}
		sort.Strings(names)
		cwd, _ := os.Getwd()
		sum := sha256.Sum256([]byte(os.Getenv("ANTHROPIC_API_KEY"))) // nosemgrep: bothy-go-no-model-credentials -- only the hash of the test key goes into the log
		args, _ := json.Marshal(os.Args[1:])
		line, _ := json.Marshal(map[string]string{"mode": "interactive", "args": string(args), "cwd": cwd, "env": strings.Join(names, ","),
			"config_dir": os.Getenv("CLAUDE_CONFIG_DIR"), "api_key_sha256": hex.EncodeToString(sum[:])}) // nosemgrep: bothy-go-no-model-credentials -- the folder only
		appendLog(string(line))
	}
	_, _ = os.Stdout.WriteString("fakeclaude interactive\n")
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		if strings.TrimSpace(sc.Text()) == "exit" {
			return
		}
		_, _ = os.Stdout.WriteString("fakeclaude: " + sc.Text() + "\n")
	}
}

func appendLog(line string) {
	f, err := os.OpenFile(startLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o666) //nolint:gosec // the log of the test, shared by all session users
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()
	_, _ = f.WriteString(line + "\n")
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
