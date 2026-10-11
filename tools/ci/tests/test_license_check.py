# SPDX-License-Identifier: Apache-2.0
"""Tests for the repository license gate (LICENSE text and SPDX headers)."""

from __future__ import annotations

import re
from pathlib import Path

import license_check as lc
import pytest

REPO = Path(__file__).resolve().parents[3]


def test_repository_license_is_canonical_apache_2() -> None:
    assert lc.license_findings(REPO / "LICENSE") == []


def test_modified_license_is_reported(tmp_path: Path) -> None:
    f = tmp_path / "LICENSE"
    f.write_text((REPO / "LICENSE").read_text() + "extra\n")
    assert lc.license_findings(f) == [f"{f}: not the canonical Apache-2.0 text"]


@pytest.mark.parametrize("name", ["a.go", "b.py", "c.dart", "d.ts", "e.sh", "f.kt", "g.swift"])
def test_source_without_header_is_reported(tmp_path: Path, name: str) -> None:
    (tmp_path / name).write_text("package main\n")
    assert lc.header_findings(tmp_path, [name]) == [
        f"{name}: missing 'SPDX-License-Identifier: Apache-2.0'"
    ]


def test_header_within_first_lines_passes(tmp_path: Path) -> None:
    (tmp_path / "a.py").write_text(
        "#!/usr/bin/env python3\n# SPDX-License-Identifier: Apache-2.0\n"
    )
    (tmp_path / "b.go").write_text("// SPDX-License-Identifier: Apache-2.0\npackage b\n")
    assert lc.header_findings(tmp_path, ["a.py", "b.go"]) == []


def test_header_too_late_is_reported(tmp_path: Path) -> None:
    body = "\n" * lc.HEADER_SEARCH_LINES + "# SPDX-License-Identifier: Apache-2.0\n"
    (tmp_path / "a.py").write_text(body)
    assert len(lc.header_findings(tmp_path, ["a.py"])) == 1


def test_other_license_identifier_is_reported(tmp_path: Path) -> None:
    (tmp_path / "a.py").write_text("# SPDX-License-Identifier: MIT\n")
    assert len(lc.header_findings(tmp_path, ["a.py"])) == 1


@pytest.mark.parametrize(
    "name",
    [
        "README.md",
        "x.yaml",
        "apps/client/lib/model.g.dart",
        "apps/client/lib/model.freezed.dart",
        "services/server/internal/api/api.gen.go",
        "sdk/typescript/src/generated/schema.ts",
        "tools/node/node_modules/x/index.js",
    ],
)
def test_non_source_and_generated_files_are_skipped(tmp_path: Path, name: str) -> None:
    path = tmp_path / name
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text("no header\n")
    assert lc.header_findings(tmp_path, [name]) == []


def test_repository_sources_have_headers() -> None:
    assert lc.main(["--repo", str(REPO)]) == 0


def test_allowed_licenses_doc_matches_list() -> None:
    listed = {
        line.strip()
        for line in (REPO / "quality" / "licenses" / "allowed.txt").read_text().splitlines()
        if line.strip() and not line.startswith("#")
    }
    doc = (REPO / "docs" / "quality" / "licenses.md").read_text(encoding="utf-8")
    table = doc.split("## Erlaubte Lizenzen", 1)[1].split("\n## ", 1)[0]
    documented = set(re.findall(r"^\| `([^`]+)` \|", table, re.MULTILINE))
    assert documented == listed
