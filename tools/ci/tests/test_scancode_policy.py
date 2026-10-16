# SPDX-License-Identifier: Apache-2.0
"""Tests for the ScanCode release gate (tools/ci/scancode_policy.py, licenses.md section 4)."""

from __future__ import annotations

import json
from pathlib import Path

import pytest
import scancode_policy as sp

REPO = Path(__file__).resolve().parents[3]
ALLOWED = frozenset({"Apache-2.0", "MIT", "BSD-3-Clause"})


@pytest.mark.parametrize(
    ("expression", "ok"),
    [
        ("Apache-2.0", True),
        ("MIT AND Apache-2.0", True),
        ("MIT AND GPL-2.0-only", False),
        ("GPL-2.0-only OR MIT", True),
        ("(GPL-2.0-only OR LGPL-2.1-only) AND MIT", False),
        ("(GPL-2.0-only OR BSD-3-Clause) AND (MIT OR AGPL-3.0-only)", True),
        ("Apache-2.0 WITH LLVM-exception", True),
        ("GPL-2.0-only WITH Classpath-exception-2.0", False),
        ("LicenseRef-scancode-unknown-license-reference", False),
        ("mit", False),
    ],
)
def test_expression_rule(expression: str, ok: bool) -> None:
    assert sp.allowed_expression(expression, ALLOWED) is ok


@pytest.mark.parametrize("bad", ["MIT AND", "(MIT", "MIT OR OR Apache-2.0", "MIT)", ""])
def test_malformed_expression_is_not_allowed(bad: str) -> None:
    assert sp.allowed_expression(bad, ALLOWED) is False


def report(tmp_path: Path, files: list[dict[str, object]]) -> Path:
    path = tmp_path / "scancode.json"
    path.write_text(json.dumps({"headers": [{"tool_name": "scancode-toolkit"}], "files": files}))
    return path


def test_findings_skip_allowed_and_excepted_files(tmp_path: Path) -> None:
    path = report(
        tmp_path,
        [
            {"path": "a.go", "type": "file", "detected_license_expression_spdx": "Apache-2.0"},
            {
                "path": "docs/x.md",
                "type": "file",
                "detected_license_expression_spdx": "GPL-3.0-only",
            },
            {"path": "b.py", "type": "file", "detected_license_expression_spdx": "AGPL-3.0-only"},
            {"path": "c.py", "type": "file", "detected_license_expression_spdx": None},
            {"path": "d", "type": "directory", "detected_license_expression_spdx": "GPL-3.0-only"},
            {"path": "e.go", "type": "file", "scan_errors": ["timeout"]},
        ],
    )
    exceptions = [sp.Waiver("docs/*", "documentation names licenses")]
    findings = sp.findings(json.loads(path.read_text()), ALLOWED, exceptions)
    assert [(f.path, f.problem) for f in findings] == [
        ("b.py", "license AGPL-3.0-only"),
        ("e.go", "scan error: timeout"),
    ]


def test_exceptions_file_needs_a_reason(tmp_path: Path) -> None:
    good = tmp_path / "ok.txt"
    good.write_text("# comment\n\ndocs/*.md | the documentation names licenses\n")
    assert sp.load_exceptions(good) == [sp.Waiver("docs/*.md", "the documentation names licenses")]
    bad = tmp_path / "bad.txt"
    bad.write_text("docs/*.md\n")
    with pytest.raises(sp.PolicyError, match="line 1"):
        sp.load_exceptions(bad)


def test_report_without_files_is_an_error(tmp_path: Path) -> None:
    path = tmp_path / "r.json"
    path.write_text("{}")
    with pytest.raises(sp.PolicyError, match="not a ScanCode report"):
        sp.load_report(path)


def test_cli(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    allowed = tmp_path / "allowed.txt"
    allowed.write_text("# list\nMIT\n")
    exceptions = tmp_path / "exceptions.txt"
    exceptions.write_text("vendor/* | third-party test data, not shipped\n")
    clean = report(
        tmp_path,
        [
            {"path": "a.py", "type": "file", "detected_license_expression_spdx": "MIT"},
            {
                "path": "vendor/x",
                "type": "file",
                "detected_license_expression_spdx": "GPL-2.0-only",
            },
        ],
    )
    args = ["--allowed", str(allowed), "--exceptions", str(exceptions), "--report"]
    assert sp.main([*args, str(clean)]) == 0
    assert "scancode-policy: 2 files" in capsys.readouterr().out
    dirty = report(
        tmp_path,
        [{"path": "a.py", "type": "file", "detected_license_expression_spdx": "SSPL-1.0"}],
    )
    assert sp.main([*args, str(dirty)]) == 1
    assert "a.py: license SSPL-1.0" in capsys.readouterr().out
    assert sp.main([*args, str(tmp_path / "missing.json")]) == 2


def test_repository_exceptions_file_is_valid() -> None:
    entries = sp.load_exceptions(REPO / "quality" / "licenses" / "scancode-exceptions.txt")
    assert entries, "the exceptions file lists the reviewed paths"
    assert all(e.reason for e in entries)
