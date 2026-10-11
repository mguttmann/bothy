// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

// fixtureGoMod is the name under which testdata stores go.mod files (docs/DECISIONS.md M1-05/5): a
// tracked go.mod would make the fixture a Go module of the repository for the Makefile, OSV-Scanner
// and the license gate.
const fixtureGoMod = "go.mod.fixture"

// copyTree copies the files below src into dst and renames every go.mod.fixture to go.mod.
func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	tree := os.DirFS(src)
	if err := os.CopyFS(dst, tree); err != nil {
		t.Fatalf("copy %s to %s: %v", src, dst, err)
	}
	root, err := os.OpenRoot(dst)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	err = fs.WalkDir(tree, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.Name() != fixtureGoMod {
			return err
		}
		return root.Rename(name, path.Join(path.Dir(name), "go.mod"))
	})
	if err != nil {
		t.Fatalf("rename %s in %s: %v", fixtureGoMod, dst, err)
	}
}

// skeleton copies testdata/skeleton and the overlay of one case (a directory below testdata, empty
// for none) into a fresh directory and returns its path.
func skeleton(t *testing.T, overlay string) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	copyTree(t, filepath.Join("testdata", "skeleton"), dir)
	if overlay != "" {
		copyTree(t, filepath.Join("testdata", filepath.FromSlash(overlay)), dir)
	}
	return dir
}

// lint generates the config of the module in dir with this generator and runs the pinned
// go-arch-lint on it.
func lint(t *testing.T, dir string) (code int, output string) {
	t.Helper()
	bin := os.Getenv("GO_ARCH_LINT")
	if bin == "" {
		t.Fatal("set GO_ARCH_LINT to the pinned go-arch-lint (make test does this)")
	}
	r := mustRead(t, filepath.Join(dir, "architecture", "modules.yaml"))
	if problems, err := checkDirectories(r, dir); err != nil || len(problems) > 0 {
		t.Fatalf("exclude entries of the fixture hit other directories (%v):\n%s", err, formatProblems(problems))
	}
	if err := os.WriteFile(filepath.Join(dir, ".go-arch-lint.yml"), mustGenerate(t, r), 0o600); err != nil {
		t.Fatal(err)
	}

	//nolint:gosec // G204: the binary is the pinned go-arch-lint that make test names in GO_ARCH_LINT; running it is the test.
	cmd := exec.CommandContext(t.Context(), bin, "check", "--project-path", dir,
		"--arch-file", ".go-arch-lint.yml", "--output-color=false")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOPROXY=off", "GOTOOLCHAIN=local", "GOWORK=off")
	out, err := cmd.CombinedOutput()
	t.Logf("go-arch-lint output:\n%s", out)
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		return 0, string(out)
	case errors.As(err, &exitErr):
		return exitErr.ExitCode(), string(out)
	default:
		t.Fatalf("run %s: %v\n%s", bin, err, out)
		return -1, ""
	}
}

// archLint builds the skeleton with the overlay of one case and runs go-arch-lint on it.
func archLint(t *testing.T, overlay string) (code int, output string) {
	t.Helper()
	return lint(t, skeleton(t, overlay))
}

func assertPasses(t *testing.T, overlay string) {
	t.Helper()
	code, out := archLint(t, overlay)
	if code != 0 || !strings.Contains(out, "No warnings found") {
		t.Fatalf("go-arch-lint exit code %d, want 0 and \"No warnings found\":\n%s", code, out)
	}
}

// assertRejects expects exit code 1 and an output that names the offending package and the
// broken rule, so that a failure for another reason does not count.
func assertRejects(t *testing.T, overlay, pkg, rule string) {
	t.Helper()
	code, out := archLint(t, overlay)
	if code != 1 {
		t.Fatalf("go-arch-lint exit code %d, want 1:\n%s", code, out)
	}
	for _, want := range []string{pkg, rule} {
		if !strings.Contains(out, want) {
			t.Fatalf("go-arch-lint output lacks %q:\n%s", want, out)
		}
	}
}

func TestArchLintSkeletonClean(t *testing.T) {
	t.Parallel()
	assertPasses(t, "")
}

func TestArchLintDomainImportsDriver(t *testing.T) {
	t.Parallel()
	assertRejects(t, "violations/domain-imports-driver", "internal/chat/domain",
		"chat-domain shouldn't depend on github.com/jackc/pgx/v5")
}

func TestArchLintAppImportsForeignPort(t *testing.T) {
	t.Parallel()
	assertRejects(t, "violations/app-imports-foreign-port", "internal/chat/app",
		"chat-app shouldn't depend on example.com/skeleton/internal/messaging")
}

func TestArchLintAppImportsForeignApp(t *testing.T) {
	t.Parallel()
	assertRejects(t, "violations/app-imports-foreign-app", "internal/messaging/app",
		"messaging-app shouldn't depend on example.com/skeleton/internal/chat/app")
}

func TestArchLintPackageWithoutComponent(t *testing.T) {
	t.Parallel()
	assertRejects(t, "violations/package-without-component", "internal/chat/util",
		"not attached to any component")
}

func TestArchLintTestSupport(t *testing.T) {
	t.Parallel()
	t.Run("testsupport uses adapter", func(t *testing.T) {
		t.Parallel()
		assertPasses(t, "violations/testsupport-uses-adapter")
	})
	t.Run("adapter uses testsupport", func(t *testing.T) {
		t.Parallel()
		assertRejects(t, "violations/adapter-uses-testsupport", "internal/chat/adapters",
			"chat-adapters shouldn't depend on example.com/skeleton/test/harness")
	})
}

// TestArchLintChapter02Layout builds the layout that docs/spec/02-architektur.md "Code-Layout"
// prescribes, with two or more packages in one component: adapters/postgres with sqlcgen, the
// test helpers platform/pg/pgtest and adapters/chattest, sub-packages of domain, app and wire, a
// package below cmd, api/routes calling api/rest and two kernel-core packages. go-arch-lint lets
// a component import its own packages only if it names itself in mayDependOn.
func TestArchLintChapter02Layout(t *testing.T) {
	t.Parallel()
	assertPasses(t, "layout")
}

// The rules stay as narrow as before: naming the own component opens no other edge.
func TestArchLintLayersStayClosed(t *testing.T) {
	t.Parallel()
	cases := []struct{ overlay, pkg, rule string }{
		{"violations/domain-imports-own-adapter", "internal/chat/domain",
			"chat-domain shouldn't depend on example.com/skeleton/internal/chat/adapters/clamav"},
		{"violations/domain-imports-own-app", "internal/chat/domain",
			"chat-domain shouldn't depend on example.com/skeleton/internal/chat/app/commands"},
		{"violations/port-imports-own-domain", "internal/chat/expose.go",
			"Component chat shouldn't depend on example.com/skeleton/internal/chat/domain"},
		{"violations/app-imports-foreign-app-subpackage", "internal/messaging/app",
			"messaging-app shouldn't depend on example.com/skeleton/internal/chat/app/commands"},
		{"violations/kernel-port-imports-kernel-port", "internal/kernel/txn",
			"kernel-ports shouldn't depend on example.com/skeleton/internal/kernel/actor"},
	}
	for _, tc := range cases {
		t.Run(path.Base(tc.overlay), func(t *testing.T) {
			t.Parallel()
			assertRejects(t, tc.overlay, tc.pkg, tc.rule)
		})
	}
}

// TestArchLintVendorPatternIsLiteral holds a vendor group to the libraries it names. go-arch-lint
// builds a regular expression from each pattern and escapes only "." and "/", so archgen must
// refuse every pattern that is no Go import path before it reaches the config.
func TestArchLintVendorPatternIsLiteral(t *testing.T) {
	t.Parallel()
	const overlay = "violations/adapter-imports-foreign-vendor"
	t.Run("library outside the group", func(t *testing.T) {
		t.Parallel()
		assertRejects(t, overlay, "internal/chat/adapters/postgres",
			"chat-adapters shouldn't depend on example.org/other")
	})
	for _, pattern := range []string{"github.com/jackc/pgx/v5|", "[^/]+/**", "github.com/**"} {
		t.Run(pattern, func(t *testing.T) {
			t.Parallel()
			dir := skeleton(t, overlay)
			root, err := os.OpenRoot(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = root.Close() }()
			const name = "architecture/modules.yaml"
			data, err := root.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			const group = "pgx: [github.com/jackc/pgx/v5, github.com/jackc/pgx/v5/**]"
			if !strings.Contains(string(data), group) {
				t.Fatalf("%s lacks %q", name, group)
			}
			text := strings.Replace(string(data), group, fmt.Sprintf("pgx: [%q]", pattern), 1)
			if err := root.WriteFile(name, []byte(text), 0o600); err != nil {
				t.Fatal(err)
			}
			reg := filepath.Join(dir, filepath.FromSlash(name))
			code, _, stderr := runArchgen("-register", reg, "-out", filepath.Join(dir, ".go-arch-lint.yml"))
			if code == 0 {
				_, out := lint(t, dir)
				t.Fatalf("archgen accepted the pattern %q; go-arch-lint then reports:\n%s", pattern, out)
			}
			if code != 1 || !strings.Contains(stderr, "vendors.pgx[0]: invalid import path") {
				t.Fatalf("exit code %d, stderr %q; want 1 and the problem at vendors.pgx[0]", code, stderr)
			}
		})
	}
}
