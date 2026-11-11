# SPDX-License-Identifier: Apache-2.0
"""Check `viz-fixtures`: every fixture of bothy_viz is valid against its OpenAPI schema (VZ-T11).

docs/spec/25-visualization.md 7.2 item 2: the fixtures of `packages/bothy_viz/fixtures/` are
checked in the gate against the schemas of docs/api/openapi.yaml, so a schema change fails until
the fixtures follow it. A file `<schema>.<state>.json` names its schema before the first dot.

Findings, one line each, `viz-fixtures: <sub-check>: <subject>: <detail>`:

- `schema`: the file names no schema of `components.schemas`;
- `valid`: the file is no JSON or breaks its schema (first error with its JSON pointer);
- `readme`: a file is missing from the table "Fixtures" of the fixtures README, or a row names
  a file that does not exist;
- `coverage`: a schema of the table has no fixture file;
- `content`: a `StatsSeries.<query>.<state>.json` carries what the server never sends. The schema
  leaves `metrics` open (a map of names to numbers), so the rule reads the rest from the
  description of the operation of `<query>` (`usage`, `runs`, `errors`): a metric name the
  description does not list (`push_failure_rate` without its provider), a rate (`<what>_rate`,
  `push_failure_rate_<provider>`) outside 0 to 1 (the server sends shares, never percent) and
  `active_people` in a bucket that is no week or month.

The schema resolves with its references inside the whole document (OpenAPI 3.1 schemas are
JSON Schema 2020-12) through the validator `jsonschema` of the pinned tools environment.

Example: `check(Path("."))` returns `[]` for a clean checkout.
"""

from __future__ import annotations

import functools
import json
import re
from pathlib import Path
from typing import Any

import jsonschema  # type: ignore[import-untyped]
import yaml

RULE = "viz-fixtures"
"""Name of the rule in `api_checks.py check --rule`."""
FIXTURES_DIR = Path("packages/bothy_viz/fixtures")
"""Fixture files of bothy_viz."""
README_PATH = FIXTURES_DIR / "README.md"
"""README with the table "Fixtures": which view reads which fixture."""
OPENAPI_PATH = Path("docs/api/openapi.yaml")
"""The REST contract with the schemas."""
TABLE_HEADING = "## Fixtures"
"""Heading of the table in the README."""

SERIES_OPERATIONS = {"usage": "getUsageStats", "runs": "getRunStats", "errors": "getErrorStats"}
"""The operation whose description lists the metric names of `StatsSeries.<query>.*.json`."""
PEOPLE_METRIC = "active_people"
"""Counted only per week or month (description of `getUsageStats`)."""
PEOPLE_GRANULARITIES = ("week", "month")
"""The bucket sizes that carry `PEOPLE_METRIC`."""

_LOADER = getattr(yaml, "CSafeLoader", yaml.SafeLoader)
_CODE = re.compile(r"`([^`]+)`")
# A count by a code is no rate, also when the code reads like one (`failed_reason_rate_limited`).
_RATE = re.compile(r"^(?!failed_reason_|messages_).*_rate$|^push_failure_rate_")
_PLACEHOLDER = re.compile(r"<[^>]+>")
_ROW = re.compile(r"^\|\s*`([^`]+\.json)`\s*\|\s*`([^`]+)`\s*\|")


def _finding(sub: str, subject: str, detail: str) -> str:
    return f"{RULE}: {sub}: {subject}: {detail}"


def listed_fixtures(root: Path) -> dict[str, tuple[str, int]]:
    """The rows of the table "Fixtures": file name to (schema, line number).

    Raises:
        ValueError: the README has no such table.
    """
    lines = (root / README_PATH).read_text(encoding="utf-8").splitlines()
    try:
        start = lines.index(TABLE_HEADING)
    except ValueError:
        raise ValueError(f'{README_PATH} has no table "{TABLE_HEADING[3:]}"') from None
    rows: dict[str, tuple[str, int]] = {}
    for number, line in enumerate(lines[start + 1 :], start=start + 2):
        if line.startswith("## "):
            break
        match = _ROW.match(line)
        if match is not None:
            rows[match.group(1)] = (match.group(2), number)
    if not rows:
        raise ValueError(f'{README_PATH} has no table "{TABLE_HEADING[3:]}"')
    return rows


def _pointer(path: Any) -> str:
    return "/" + "/".join(str(part) for part in path)


@functools.lru_cache(maxsize=4)
def _openapi(text: str) -> Any:
    """The parsed document; cached by its text, because parsing it dominates the check.

    Composed by the C loader when available and constructed by the safe loader, the same
    result as `yaml.safe_load` in a fraction of the time (as `tools/catalog` does).
    """
    node = yaml.compose(text, Loader=_LOADER)
    return None if node is None else yaml.SafeLoader("").construct_document(node)


def check(root: Path) -> list[str]:
    """Run the rule against the checkout `root`; return the findings, empty when clean."""
    document = _openapi((root / OPENAPI_PATH).read_text(encoding="utf-8"))
    schemas: dict[str, Any] = document["components"]["schemas"]
    try:
        listed = listed_fixtures(root)
    except ValueError as err:
        return [_finding("readme", README_PATH.as_posix(), str(err))]
    found: list[str] = []
    files = sorted(p.name for p in (root / FIXTURES_DIR).glob("*.json"))
    for name in files:
        schema = name.split(".", 1)[0]
        if schema not in schemas:
            found.append(_finding("schema", name, f"no schema {schema} in {OPENAPI_PATH}"))
        else:
            invalid = _validate(root / FIXTURES_DIR / name, schema, document)
            found += invalid or _content(root / FIXTURES_DIR / name, document)
        if name not in listed:
            found.append(_finding("readme", name, f"not listed in {README_PATH}"))
    for name, (schema, _line) in sorted(listed.items()):
        if name not in files:
            found.append(_finding("readme", name, f"listed in {README_PATH} but missing"))
        if not any(f.split(".", 1)[0] == schema for f in files):
            finding = _finding("coverage", schema, "no fixture file")
            if finding not in found:
                found.append(finding)
    return found


def _validate(path: Path, schema: str, document: dict[str, Any]) -> list[str]:
    try:
        instance = json.loads(path.read_text(encoding="utf-8"))
    except json.JSONDecodeError as err:
        return [_finding("valid", path.name, f"not JSON: {err}")]
    root_schema = {"$ref": f"#/components/schemas/{schema}", "components": document["components"]}
    validator = jsonschema.Draft202012Validator(
        root_schema, format_checker=jsonschema.Draft202012Validator.FORMAT_CHECKER
    )
    errors = sorted(validator.iter_errors(instance), key=lambda e: list(e.absolute_path))
    if not errors:
        return []
    first = errors[0]
    return [_finding("valid", path.name, f"{_pointer(first.absolute_path)}: {first.message}")]


def metric_names(document: dict[str, Any], operation: str) -> re.Pattern[str]:
    """The metric names the description of `operation` lists, as one pattern.

    Every name in backticks counts; a part in angle brackets stands for any code
    (`push_failure_rate_<provider>`, `failed_reason_<code>`).

    Raises:
        ValueError: the document has no such operation.
    """
    for item in document["paths"].values():
        for method in item.values():
            if isinstance(method, dict) and method.get("operationId") == operation:
                names = _CODE.findall(method["description"])
                patterns = (_PLACEHOLDER.sub("[a-z0-9_]+", re.escape(n)) for n in names)
                return re.compile("|".join(patterns))
    raise ValueError(f"no operation {operation} in {OPENAPI_PATH}")


def _content(path: Path, document: dict[str, Any]) -> list[str]:
    """The `content` findings of a valid `StatsSeries.<query>.<state>.json`; no other has any."""
    parts = path.name.split(".")
    if parts[0] != "StatsSeries" or len(parts) != 4 or parts[1] not in SERIES_OPERATIONS:
        return []
    operation = SERIES_OPERATIONS[parts[1]]
    known = metric_names(document, operation)
    series = json.loads(path.read_text(encoding="utf-8"))
    granularity = series["granularity"]
    found: list[str] = []
    for index, bucket in enumerate(series["buckets"]):
        entries = [
            (f"/buckets/{index}/metrics/{name}", name, value)
            for name, value in bucket["metrics"].items()
        ]
        entries += [
            (f"/buckets/{index}/below_min_group/{at}", name, None)
            for at, name in enumerate(bucket.get("below_min_group", []))
        ]
        for pointer, name, value in entries:
            if known.fullmatch(name) is None:
                detail = f"not a metric of {operation} in {OPENAPI_PATH}"
            elif name == PEOPLE_METRIC and granularity not in PEOPLE_GRANULARITIES:
                sizes = " or ".join(PEOPLE_GRANULARITIES)
                detail = f"only with granularity {sizes}, not {granularity}"
            elif value is not None and _RATE.search(name) and not 0 <= value <= 1:
                detail = f"{value} is no share between 0 and 1"
            else:
                continue
            found.append(_finding("content", path.name, f"{pointer}: {detail}"))
    return found
