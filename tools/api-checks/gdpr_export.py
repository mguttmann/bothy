# SPDX-License-Identifier: Apache-2.0
"""Check `gdpr-export`: the JSON Schemas of the export format of privacy requests (brief M7-06).

docs/spec/20-privacy.md 5.2 and docs/spec/06-api.md section 10 describe the export package as one
JSON Schema (draft 2020-12) per file under `docs/api/gdpr-export/v1/`: `<file>.schema.json` for
the JSON object files with one `$ref` per section to `sections/<file>.<section>.schema.json`, and
`lines/<file>.schema.json` for one line of each JSON Lines file. The server test DP-T03 validates a
real package against them; this check keeps the schema set itself sound without a server.

Findings, one line each, `gdpr-export: <sub-check>: <subject>: <detail>`:

- `json`: a schema file is no JSON;
- `schema`: a file is no valid JSON Schema 2020-12 (its `$schema` must name that dialect);
- `ref`: a `$ref` names a file that does not exist;
- `section`: a section file is not referenced by the schema of its file, or a file schema
  references a section file that does not exist;
- `readme`: a file schema or line schema is missing from the table of the README;
- `spec`: 06-api.md does not link the directory.

Example: `check(Path("."))` returns `[]` for a clean checkout.
"""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any

import jsonschema  # type: ignore[import-untyped]

RULE = "gdpr-export"
"""Name of the rule in `api_checks.py check --rule`."""
SCHEMA_DIR = Path("docs/api/gdpr-export/v1")
"""The schemas of version 1 of the export format."""
README_PATH = SCHEMA_DIR / "README.md"
"""README with the table of the files of the archive."""
SPEC_PATH = Path("docs/spec/06-api.md")
"""The API chapter that must link the schemas."""
DIALECT = "https://json-schema.org/draft/2020-12/schema"
"""The one dialect of the schemas."""


def _finding(sub: str, subject: str, detail: str) -> str:
    """Return one finding line."""
    return f"{RULE}: {sub}: {subject}: {detail}"


def _refs(node: Any) -> list[str]:
    """Return every `$ref` value below node."""
    out: list[str] = []
    if isinstance(node, dict):
        for key, value in node.items():
            if key == "$ref" and isinstance(value, str):
                out.append(value)
            else:
                out += _refs(value)
    elif isinstance(node, list):
        for value in node:
            out += _refs(value)
    return out


def _load(root: Path, rel: Path, found: list[str]) -> Any:
    """Load the schema at rel as JSON and check it against its dialect; None when it fails."""
    try:
        doc = json.loads((root / rel).read_text(encoding="utf-8"))
    except (OSError, ValueError) as err:
        found.append(_finding("json", rel.as_posix(), str(err).splitlines()[0]))
        return None
    if not isinstance(doc, dict) or doc.get("$schema") != DIALECT:
        found.append(_finding("schema", rel.as_posix(), f"$schema must be {DIALECT}"))
        return None
    try:
        jsonschema.Draft202012Validator.check_schema(doc)
    except jsonschema.SchemaError as err:
        found.append(_finding("schema", rel.as_posix(), err.message))
        return None
    return doc


def check(root: Path) -> list[str]:
    """Run the rule against the checkout `root`; return the findings, empty when clean."""
    found: list[str] = []
    base = root / SCHEMA_DIR
    files = sorted(p.relative_to(root) for p in base.rglob("*.schema.json"))
    if not files:
        return [_finding("schema", SCHEMA_DIR.as_posix(), "no schema files")]
    docs: dict[Path, Any] = {}
    for rel in files:
        doc = _load(root, rel, found)
        if doc is None:
            continue
        docs[rel] = doc
        for ref in _refs(doc):
            target = ref.split("#", 1)[0]
            if target and not (root / rel).parent.joinpath(target).is_file():
                found.append(_finding("ref", rel.as_posix(), f"{ref} does not exist"))
    referenced: set[Path] = set()
    for rel, doc in docs.items():
        if rel.parent == SCHEMA_DIR:
            for ref in _refs(doc):
                referenced.add((root / rel).parent.joinpath(ref.split("#", 1)[0]).resolve())
    for rel in files:
        if rel.parent.name == "sections" and (root / rel).resolve() not in referenced:
            stem = rel.name.split(".", 1)[0]
            found.append(
                _finding("section", rel.as_posix(), f"not referenced by {stem}.schema.json")
            )
    readme = (
        (root / README_PATH).read_text(encoding="utf-8") if (root / README_PATH).is_file() else ""
    )
    for rel in files:
        if rel.parent.name != "sections" and rel.relative_to(SCHEMA_DIR).as_posix() not in readme:
            found.append(_finding("readme", rel.as_posix(), f"missing from {README_PATH}"))
    spec = (root / SPEC_PATH).read_text(encoding="utf-8") if (root / SPEC_PATH).is_file() else ""
    if "../api/gdpr-export/v1/" not in spec:
        found.append(_finding("spec", SPEC_PATH.as_posix(), f"does not link {SCHEMA_DIR}"))
    return found
