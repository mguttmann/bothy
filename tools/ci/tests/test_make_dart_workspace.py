# SPDX-License-Identifier: Apache-2.0
"""The Dart workspace lines of the root Makefile (brief M4-01, Design notes 2 to 6).

Each test runs a copy of the root Makefile in a temporary git repository that tracks exactly
the files of the case (the component discovery reads tracked files only). Fake `uv`, `flutter`
and `dart` executables come first on PATH and record their calls; the fake `uv` runs a Python
script of the copy for real when the case copied it (tools/ci/lcov_filter.py), so the coverage
sum is the real one. `MAKEFLAGS`, `MAKELEVEL` and the configuration the Makefile reads with `?=`
are dropped, so the outer `make test` cannot leak into the runs.

The last test needs the real Flutter SDK: it copies one member into a temporary workspace and
proves that a public symbol without a doc comment fails `flutter analyze --fatal-infos`.
"""

from __future__ import annotations

import json
import os
import re
import shutil
import subprocess
import sys
from pathlib import Path

import pytest

REPO = Path(__file__).resolve().parents[3]
_DROPPED = {"MAKEFLAGS", "MAKELEVEL", "MFLAGS", "COMPONENTS", "COVERAGE_DART_MIN", "GO_MODS"}

_FAKE = """#!{python}
import json, os, subprocess, sys
name = os.path.basename(sys.argv[0])
args = sys.argv[1:]
with open(os.environ["FAKE_LOG"], "a", encoding="utf-8") as fh:
    fh.write(json.dumps({{"tool": name, "args": args, "cwd": os.getcwd()}}) + "\\n")
if name == "uv" and "python" in args:
    script = args[args.index("python") + 1:]
    if script and os.path.isfile(script[0]):
        sys.exit(subprocess.run([sys.executable, *script]).returncode)
if name == "flutter" and args[:1] == ["test"] and os.environ.get("FAKE_LCOV"):
    os.makedirs("coverage", exist_ok=True)
    with open("coverage/lcov.info", "w", encoding="utf-8") as fh:
        fh.write(os.environ["FAKE_LCOV"])
sys.exit(0)
"""

ROOT_PUBSPEC = "name: bothy_workspace\nworkspace:\n  - packages/alpha\n  - packages/beta\n"


class Repo:
    """A temporary repository with a copy of the root Makefile and recording fakes."""

    def __init__(self, root: Path) -> None:
        self.root = root
        self.bin = root.parent / "bin"
        self.log = root.parent / "calls.jsonl"
        root.mkdir()
        shutil.copy(REPO / "Makefile", root / "Makefile")
        (root / "tools").mkdir()
        fake = _FAKE.format(python=sys.executable)
        for name in ("uv", "flutter", "dart"):
            path = self.bin / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(fake, encoding="utf-8")
            path.chmod(0o755)

    def write(self, rel: str, text: str = "") -> Path:
        path = self.root / rel
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text, encoding="utf-8")
        return path

    def workspace(self, *, tests: bool = True) -> None:
        """Two members alpha and beta with a library and (by default) a test each."""
        self.write("pubspec.yaml", ROOT_PUBSPEC)
        self.write("pubspec.lock", "packages: {}\n")
        for name in ("alpha", "beta"):
            self.write(f"packages/{name}/pubspec.yaml", f"name: {name}\nresolution: workspace\n")
            self.write(f"packages/{name}/lib/{name}.dart", "library;\n")
            if tests:
                self.write(f"packages/{name}/test/{name}_test.dart", "void main() {}\n")

    def commit(self) -> None:
        for args in (
            ["init", "-q", "-b", "main"],
            ["config", "core.hooksPath", "/dev/null"],
            ["add", "."],
            ["-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-qm", "x"],
        ):
            subprocess.run(["git", "-C", str(self.root), *args], check=True, capture_output=True)

    def environ(self, **env: str) -> dict[str, str]:
        environ = {k: v for k, v in os.environ.items() if k not in _DROPPED}
        environ["PATH"] = f"{self.bin}{os.pathsep}{environ['PATH']}"
        environ.update(FAKE_LOG=str(self.log), **env)
        return environ

    def make(self, *args: str, **env: str) -> subprocess.CompletedProcess[str]:
        return subprocess.run(
            ["make", "-s", "-C", str(self.root), *args],
            env=self.environ(**env),
            capture_output=True,
            text=True,
            check=False,
            timeout=120,
        )

    def recipe(self, target: str, marker: str) -> subprocess.CompletedProcess[str]:
        """Run the shell command of `target` that contains `marker`, as `make -n` prints it."""
        printed = self.make("-n", target, "COMPONENTS=apps").stdout
        commands: list[str] = []
        for line in printed.splitlines():
            if commands and (commands[-1].endswith("\\") or line.startswith("\t")):
                commands[-1] += "\n" + line
            else:
                commands.append(line)
        [command] = [c for c in commands if marker in c]
        return subprocess.run(
            ["bash", "-c", command],
            cwd=self.root,
            env=self.environ(),
            capture_output=True,
            text=True,
            check=False,
            timeout=120,
        )

    def calls(self, tool: str) -> list[dict[str, object]]:
        if not self.log.exists():
            return []
        entries = [json.loads(line) for line in self.log.read_text().splitlines()]
        return [e for e in entries if e["tool"] == tool]

    def python_calls(self) -> list[list[str]]:
        """The `uv run ... python <script> <args>` calls, as the script and its arguments."""
        return [
            [str(a) for a in c["args"][c["args"].index("python") + 1 :]]  # type: ignore[union-attr]
            for c in self.calls("uv")
            if "python" in c["args"]  # type: ignore[operator]
        ]


@pytest.fixture
def repo(tmp_path: Path) -> Repo:
    return Repo(tmp_path / "repo")


def test_arch_runs_dart_check_when_members_exist(repo: Repo) -> None:
    repo.workspace()
    repo.commit()
    result = repo.make("arch", "COMPONENTS=apps")
    assert result.returncode == 0, result.stdout + result.stderr
    assert "SKIP  arch " in result.stdout  # the Go part: no Go module
    assert repo.python_calls() == [["tools/arch-check/arch_check.py"]]


def test_arch_skips_dart_without_members(repo: Repo) -> None:
    repo.write("README.md", "x\n")
    repo.commit()
    result = repo.make("arch", "COMPONENTS=apps")
    assert result.returncode == 0, result.stdout + result.stderr
    assert re.search(r"SKIP\s+arch-dart\s+no Dart package selected", result.stdout)
    assert repo.python_calls() == []
    repo.workspace()
    repo.commit()
    server = repo.make("arch", "COMPONENTS=server")
    assert re.search(r"SKIP\s+arch-dart\s+no Dart package selected", server.stdout)
    assert repo.python_calls() == []


def test_codegen_check_runs_l10n_check_in_members_with_l10n_yaml(repo: Repo) -> None:
    repo.workspace()
    repo.write("packages/beta/l10n.yaml", "arb-dir: lib/l10n\noutput-dir: lib/src/generated\n")
    repo.commit()
    result = repo.make("codegen-check", "COMPONENTS=apps")
    assert result.returncode == 0, result.stdout + result.stderr
    assert "codegen-dart" not in result.stdout
    assert repo.python_calls() == [["tools/ci/l10n_check.py", "--dir", "packages/beta"]]


def test_codegen_check_stops_when_the_l10n_check_fails(repo: Repo) -> None:
    repo.workspace()
    for name in ("alpha", "beta"):
        repo.write(f"packages/{name}/l10n.yaml", "arb-dir: lib/l10n\n")
    repo.write("tools/ci/l10n_check.py", "import sys\nsys.exit(1)\n")
    repo.commit()
    result = repo.make("codegen-check", "COMPONENTS=apps")
    assert result.returncode != 0
    assert repo.python_calls() == [["tools/ci/l10n_check.py", "--dir", "packages/alpha"]]


def test_codegen_check_skips_dart_without_l10n_or_members(repo: Repo) -> None:
    repo.workspace()
    repo.commit()
    result = repo.make("codegen-check", "COMPONENTS=apps")
    assert result.returncode == 0, result.stdout + result.stderr
    assert re.search(r"SKIP\s+codegen-dart\s+no Dart member with l10n.yaml", result.stdout)
    server = repo.make("codegen-check", "COMPONENTS=server")
    assert re.search(r"SKIP\s+codegen-dart\s+no Dart package selected", server.stdout)
    assert repo.python_calls() == []


def test_lint_runs_one_root_analyze(repo: Repo) -> None:
    repo.workspace()
    repo.commit()
    result = repo.recipe("lint", "flutter analyze")
    assert result.returncode == 0, result.stdout + result.stderr
    root = str(repo.root.resolve())
    assert [(c["args"], c["cwd"]) for c in repo.calls("flutter")] == [
        (["pub", "get", "--enforce-lockfile"], root),
        (["analyze", "--fatal-infos"], root),
    ]


def test_lint_runs_custom_lint_in_members_that_declare_it(repo: Repo) -> None:
    repo.workspace()
    repo.write(
        "packages/beta/pubspec.yaml",
        "name: beta\nresolution: workspace\ndev_dependencies:\n  custom_lint: any\n",
    )
    repo.commit()
    result = repo.recipe("lint", "flutter analyze")
    assert result.returncode == 0, result.stdout + result.stderr
    assert [(c["args"], c["cwd"]) for c in repo.calls("dart")] == [
        (["run", "custom_lint"], str((repo.root / "packages" / "beta").resolve()))
    ]


def test_test_runs_each_member_after_one_root_resolution(repo: Repo) -> None:
    repo.workspace()
    repo.commit()
    result = repo.make("test", "COMPONENTS=apps")
    assert result.returncode == 0, result.stdout + result.stderr
    assert [(c["args"], Path(str(c["cwd"])).name) for c in repo.calls("flutter")] == [
        (["pub", "get", "--enforce-lockfile"], "repo"),
        (["test", "--coverage"], "alpha"),
        (["test", "--coverage"], "beta"),
    ]


def test_member_without_tests_fails(repo: Repo) -> None:
    repo.workspace()
    shutil.rmtree(repo.root / "packages" / "beta" / "test")
    repo.commit()
    result = repo.make("test", "COMPONENTS=apps")
    assert result.returncode == 2
    assert "packages/beta: no test/ directory (docs/spec/23-qualitaet.md, 3)" in result.stderr


def test_coverage_ignores_generated_files(repo: Repo) -> None:
    repo.workspace()
    repo.write("tools/ci/lcov_filter.py", (REPO / "tools/ci/lcov_filter.py").read_text())
    repo.commit()
    lcov = (
        "SF:lib/src/model.g.dart\nLF:40\nLH:0\nend_of_record\n"
        "SF:lib/src/generated/api.dart\nLF:40\nLH:0\nend_of_record\n"
        "SF:lib/alpha.dart\nLF:10\nLH:9\nend_of_record\n"
    )
    result = repo.make("coverage", "COMPONENTS=apps", FAKE_LCOV=lcov)
    assert result.returncode == 0, result.stdout + result.stderr
    low = repo.make("coverage", "COMPONENTS=apps", FAKE_LCOV=lcov.replace("LH:9", "LH:5"))
    assert low.returncode == 2
    assert "packages/alpha: coverage 50.0% < 80%" in low.stdout + low.stderr


def test_coverage_fails_closed_without_totals(repo: Repo) -> None:
    # Without the filter script the fake uv prints nothing: the gate must not pass on that.
    repo.workspace()
    repo.commit()
    result = repo.make("coverage", "COMPONENTS=apps", FAKE_LCOV="SF:lib/a.dart\nLF:10\nLH:1\n")
    assert result.returncode == 2
    assert "coverage: tools/ci/lcov_filter.py printed no totals for packages/alpha" in result.stderr


def test_coverage_without_lcov_fails(repo: Repo) -> None:
    repo.workspace()
    repo.write("tools/ci/lcov_filter.py", (REPO / "tools/ci/lcov_filter.py").read_text())
    repo.commit()
    result = repo.make("coverage", "COMPONENTS=apps")
    assert result.returncode == 2
    assert "packages/alpha: coverage/lcov.info is missing; make test writes it" in result.stderr


def test_fmt_check_skips_generated_files(repo: Repo) -> None:
    repo.workspace()
    repo.write("packages/alpha/lib/src/model.g.dart", "// generated\n")
    repo.write("packages/alpha/lib/src/generated/api.dart", "// generated\n")
    repo.write("packages/beta/lib/src/x.freezed.dart", "// generated\n")
    repo.commit()
    result = repo.recipe("fmt-check", "dart format")
    assert result.returncode == 0, result.stdout + result.stderr
    [call] = repo.calls("dart")
    assert call["args"] == [
        "format",
        "--output=none",
        "--set-exit-if-changed",
        "packages/alpha/lib/alpha.dart",
        "packages/alpha/test/alpha_test.dart",
        "packages/beta/lib/beta.dart",
        "packages/beta/test/beta_test.dart",
    ]


def test_docs_runs_dart_api_docs(repo: Repo) -> None:
    repo.workspace()
    repo.commit()
    result = repo.make("docs", "COMPONENTS=apps")
    assert result.returncode == 0, result.stdout + result.stderr
    scripts = [c[0] for c in repo.python_calls()]
    # After the site build the docs gate of M7-19 checks it for assets of another origin.
    assert scripts == [
        "tools/docs/adr_index.py",
        "tools/docs/dart_api_docs.py",
        "tools/docs/external_assets_check.py",
    ]
    assert repo.python_calls()[1] == ["tools/docs/dart_api_docs.py", "--out", "docs/api/dart"]
    uv = [c["args"] for c in repo.calls("uv")]
    assert any("mkdocs" in args for args in uv)  # type: ignore[operator]
    # The site links into the generated pages, so the docs gate builds them for every
    # component selection, not only for `apps` (docs/DECISIONS.md D-M4-01-6).
    server = repo.make("docs", "COMPONENTS=server")
    assert server.returncode == 0, server.stdout + server.stderr
    assert [c[0] for c in repo.python_calls()].count("tools/docs/dart_api_docs.py") == 2


def test_docs_skips_dart_api_docs_without_members(repo: Repo) -> None:
    repo.write("README.md", "x\n")
    repo.commit()
    result = repo.make("docs")
    assert result.returncode == 0, result.stdout + result.stderr
    assert re.search(r"SKIP\s+dart-api-docs", result.stdout)
    assert "tools/docs/dart_api_docs.py" not in [c[0] for c in repo.python_calls()]


def test_linkcheck_builds_missing_dart_api_pages_first(repo: Repo) -> None:
    repo.workspace()
    repo.commit()
    result = repo.recipe("linkcheck", "dart_api_docs.py")
    assert result.returncode == 0, result.stdout + result.stderr
    assert repo.python_calls() == [["tools/docs/dart_api_docs.py", "--out", "docs/api/dart"]]
    (repo.root / "docs" / "api" / "dart").mkdir(parents=True)
    again = repo.recipe("linkcheck", "dart_api_docs.py")
    assert again.returncode == 0, again.stdout + again.stderr
    assert len(repo.python_calls()) == 1


_FETCH = """import os, sys
print(os.path.join(os.environ["FAKE_BIN"], sys.argv[2]))
"""


@pytest.mark.parametrize("target", ["licenses", "osv"])
def test_lock_gates_read_the_root_lock_once(repo: Repo, target: str) -> None:
    repo.workspace()
    repo.write("quality/licenses/allowed.txt", "MIT\n")
    repo.write("tools/bin/fetch_tools.py", _FETCH)
    osv = repo.bin / "osv-scanner"
    shutil.copy(repo.bin / "uv", osv)
    repo.commit()
    result = repo.make(target, "COMPONENTS=apps", FAKE_BIN=str(repo.bin))
    assert result.returncode == 0, result.stdout + result.stderr
    [call] = repo.calls("osv-scanner")
    args = [str(a) for a in call["args"]]  # type: ignore[union-attr]
    assert [args[i + 1] for i, a in enumerate(args) if a == "-L"] == ["pubspec.lock"]


def test_apps_platform_not_in_gates(repo: Repo) -> None:
    text = (REPO / "Makefile").read_text(encoding="utf-8")
    gates = re.search(r"^gates: (fmt-check[^\n]*(?:\\\n[^\n]*)*)", text, re.MULTILINE)
    assert gates
    assert "apps-platform" not in gates.group(1)
    assert "apps-platform" not in re.search(r"^check: ([^\n]*)", text, re.MULTILINE).group(1)  # type: ignore[union-attr]
    repo.workspace()
    repo.commit()
    result = repo.make("apps-platform")
    assert result.returncode == 0, result.stdout + result.stderr
    assert repo.python_calls() == [["tools/ci/apps_platform.py"]]


@pytest.mark.skipif(
    shutil.which("flutter") is None,
    reason="needs the Flutter SDK on PATH; the gate hosts and CI have it (brief M4-01)",
)
def test_public_symbol_without_doc_fails_lint(tmp_path: Path) -> None:
    ws = tmp_path / "ws"
    member = ws / "packages" / "domain"
    shutil.copytree(
        REPO / "packages" / "domain",
        member,
        ignore=shutil.ignore_patterns(".dart_tool", "build", "coverage"),
    )
    shutil.copy(REPO / "analysis_options.yaml", ws / "analysis_options.yaml")
    root = (REPO / "pubspec.yaml").read_text(encoding="utf-8")
    head = root.split("workspace:", 1)[0]
    (ws / "pubspec.yaml").write_text(head + "workspace:\n  - packages/domain\n", encoding="utf-8")
    library = member / "lib" / "domain.dart"
    environ = {k: v for k, v in os.environ.items() if k not in _DROPPED}

    def flutter(*args: str) -> subprocess.CompletedProcess[str]:
        return subprocess.run(
            ["flutter", *args],
            cwd=ws,
            env=environ,
            capture_output=True,
            text=True,
            check=False,
            timeout=600,
        )

    got = flutter("pub", "get", "--offline")
    assert got.returncode == 0, got.stdout + got.stderr
    clean = flutter("analyze", "--fatal-infos")
    assert clean.returncode == 0, clean.stdout + clean.stderr
    library.write_text(library.read_text(encoding="utf-8") + "\nint undocumented() => 1;\n")
    failing = flutter("analyze", "--fatal-infos")
    assert failing.returncode != 0
    assert "public_member_api_docs" in failing.stdout + failing.stderr
