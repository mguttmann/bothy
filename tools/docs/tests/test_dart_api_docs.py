# SPDX-License-Identifier: Apache-2.0
"""Tests of tools/docs/dart_api_docs.py, the Dart API pages of the site (brief M4-01, note 5).

A fake `flutter` and `dart` first on PATH record their calls; the fake `dart doc` writes an
index below its `--output` folder, prints what FAKE_DART_OUTPUT says and exits with
FAKE_DART_EXIT.
"""

from __future__ import annotations

import json
import os
import re
import sys
from pathlib import Path

import dart_api_docs
import pytest
import yaml

REPO = Path(__file__).resolve().parents[3]
RULES = REPO / "tools" / "arch-check" / "rules.yaml"

FAKE = """#!{python}
import json, os, sys
name = os.path.basename(sys.argv[0])
args = sys.argv[1:]
with open(os.environ["FAKE_LOG"], "a", encoding="utf-8") as fh:
    fh.write(json.dumps({{"tool": name, "args": args, "cwd": os.getcwd()}}) + "\\n")
if name == "dart" and args[:1] == ["doc"]:
    out = args[args.index("--output") + 1]
    os.makedirs(out, exist_ok=True)
    with open(os.path.join(out, "index.html"), "w", encoding="utf-8") as fh:
        fh.write("<html></html>")
    print(os.environ.get("FAKE_DART_OUTPUT", "Found 0 warnings and 0 errors."))
    sys.exit(int(os.environ.get("FAKE_DART_EXIT", "0")))
sys.exit(0)
"""

RULES_TEXT = """members:
  - {name: client, path: apps/client, kind: app, edges: [domain]}
  - {name: domain, path: packages/domain, kind: dart, edges: []}
  - {name: auth, path: packages/auth, kind: dart, edges: [domain]}
"""


@pytest.fixture
def repo(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> Path:
    root = tmp_path / "repo"
    for rel in ("apps/client", "packages/domain", "packages/auth"):
        (root / rel).mkdir(parents=True)
        (root / rel / "pubspec.yaml").write_text("name: x\n", encoding="utf-8")
    (root / "rules.yaml").write_text(RULES_TEXT, encoding="utf-8")
    bin_dir = tmp_path / "bin"
    bin_dir.mkdir()
    for name in ("flutter", "dart"):
        tool = bin_dir / name
        tool.write_text(FAKE.format(python=sys.executable), encoding="utf-8")
        tool.chmod(0o755)
    monkeypatch.setenv("PATH", f"{bin_dir}{os.pathsep}{os.environ['PATH']}")
    monkeypatch.setenv("FAKE_LOG", str(tmp_path / "calls.jsonl"))
    return root


def _calls(repo: Path) -> list[dict[str, object]]:
    log = repo.parent / "calls.jsonl"
    return [json.loads(line) for line in log.read_text(encoding="utf-8").splitlines()]


def _run(repo: Path) -> int:
    return dart_api_docs.main(
        ["--root", str(repo), "--rules", str(repo / "rules.yaml"), "--out", "docs/api/dart"]
    )


def test_documents_every_package_but_not_the_apps(repo: Path) -> None:
    assert _run(repo) == 0
    out = (repo / "docs" / "api" / "dart").resolve()
    assert _calls(repo) == [
        {
            "tool": "flutter",
            "args": ["pub", "get", "--enforce-lockfile"],
            "cwd": str(repo.resolve()),
        },
        {
            "tool": "dart",
            "args": ["doc", "--output", str(out / "domain")],
            "cwd": str((repo / "packages" / "domain").resolve()),
        },
        {
            "tool": "dart",
            "args": ["doc", "--output", str(out / "auth")],
            "cwd": str((repo / "packages" / "auth").resolve()),
        },
    ]
    assert (out / "domain" / "index.html").is_file()


def test_warning_line_fails(
    repo: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    monkeypatch.setenv("FAKE_DART_OUTPUT", "  warning: unresolved doc reference [Foo]")
    assert _run(repo) == 1
    err = capsys.readouterr().err
    assert "dart-api-docs: packages/domain: dart doc printed a warning or an error" in err
    # The first failing package stops the run.
    assert [c["cwd"] for c in _calls(repo) if c["tool"] == "dart"] == [
        str((repo / "packages" / "domain").resolve())
    ]


def test_error_line_fails(repo: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("FAKE_DART_OUTPUT", "error: could not resolve package")
    assert _run(repo) == 1


def test_nonzero_exit_fails(
    repo: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    monkeypatch.setenv("FAKE_DART_EXIT", "1")
    assert _run(repo) == 1
    assert "dart-api-docs: packages/domain: dart doc failed with exit code 1" in (
        capsys.readouterr().err
    )


def test_writes_only_below_out(repo: Path) -> None:
    before = set(repo.rglob("*"))
    stale = repo / "docs" / "api" / "dart" / "domain" / "old.html"
    stale.parent.mkdir(parents=True)
    stale.write_text("old", encoding="utf-8")
    assert _run(repo) == 0
    after = set(repo.rglob("*")) - before
    out = repo / "docs" / "api" / "dart"
    assert after, "nothing was written"
    assert all(p == out or out in p.parents or p in out.parents for p in after), after
    # A page of an earlier build does not survive the new one.
    assert not stale.exists()


def test_out_outside_root_is_rejected(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    code = dart_api_docs.main(
        ["--root", str(repo), "--rules", str(repo / "rules.yaml"), "--out", "../elsewhere"]
    )
    assert code == 2
    assert "must lie inside the repository" in capsys.readouterr().err


def test_page_lists_every_package() -> None:
    """docs/reference/dart-api.md links the pages of exactly the packages of rules.yaml."""
    rules = yaml.safe_load(RULES.read_text(encoding="utf-8"))
    packages = [m["name"] for m in rules["members"] if m["kind"] != "app"]
    page = (REPO / "docs" / "reference" / "dart-api.md").read_text(encoding="utf-8")
    linked = re.findall(r"\]\(\.\./api/dart/([a-z0-9_]+)/index\.html\)", page)
    assert linked == packages
