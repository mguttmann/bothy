# SPDX-License-Identifier: Apache-2.0
"""Tests of the check `gdpr-export` (docs/spec/20-privacy.md 5.2, brief M7-06).

Every negative case copies the real schemas, their README and 06-api.md into `tmp_path`, changes
one thing and expects the named finding; the repository itself must stay clean.
"""

from __future__ import annotations

import json
import shutil
from pathlib import Path

import gdpr_export
import pytest

REPO = Path(__file__).resolve().parents[3]


@pytest.fixture
def tree(tmp_path: Path) -> Path:
    """A repository root with copies of the schemas and the API chapter."""
    root = tmp_path / "repo"
    shutil.copytree(REPO / gdpr_export.SCHEMA_DIR, root / gdpr_export.SCHEMA_DIR)
    (root / gdpr_export.SPEC_PATH).parent.mkdir(parents=True)
    shutil.copy2(REPO / gdpr_export.SPEC_PATH, root / gdpr_export.SPEC_PATH)
    return root


def test_repository_clean() -> None:
    """The checkout itself has no finding."""
    assert gdpr_export.check(REPO) == []


def test_invalid_schema(tree: Path) -> None:
    """A schema that breaks the 2020-12 meta-schema is a `schema` finding."""
    path = tree / gdpr_export.SCHEMA_DIR / "lines" / "audit.schema.json"
    doc = json.loads(path.read_text(encoding="utf-8"))
    doc["type"] = 7
    path.write_text(json.dumps(doc), encoding="utf-8")
    assert any(": schema: " in f and "audit.schema.json" in f for f in gdpr_export.check(tree))


def test_missing_ref(tree: Path) -> None:
    """A file schema whose section file is gone is a `ref` finding."""
    (tree / gdpr_export.SCHEMA_DIR / "sections" / "devices.sessions.schema.json").unlink()
    assert any(": ref: " in f and "devices.schema.json" in f for f in gdpr_export.check(tree))


def test_unreferenced_section(tree: Path) -> None:
    """A section file that its file schema does not name is a `section` finding."""
    src = tree / gdpr_export.SCHEMA_DIR / "sections" / "devices.sessions.schema.json"
    shutil.copy2(src, src.with_name("devices.extra.schema.json"))
    assert any(": section: " in f and "devices.extra" in f for f in gdpr_export.check(tree))


def test_readme_and_spec(tree: Path) -> None:
    """A line schema missing from the README and a chapter without the link are findings."""
    readme = tree / gdpr_export.README_PATH
    readme.write_text(
        readme.read_text(encoding="utf-8").replace("lines/runs.schema.json", "x"), encoding="utf-8"
    )
    spec = tree / gdpr_export.SPEC_PATH
    spec.write_text(
        spec.read_text(encoding="utf-8").replace("../api/gdpr-export/v1/", "x/"), encoding="utf-8"
    )
    found = gdpr_export.check(tree)
    assert any(": readme: " in f and "runs.schema.json" in f for f in found)
    assert any(": spec: " in f for f in found)
