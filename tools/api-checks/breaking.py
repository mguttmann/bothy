# SPDX-License-Identifier: Apache-2.0
"""Breaking-change check of the API against the last publication (06 G6, ticket M7-11).

Compares docs/api/openapi.yaml of the working tree, including the gateway frame and event
schemas in its `components.schemas`, with the same file of the newest release tag
`server/v*` reachable from HEAD and fails on every breaking change of the table in
docs/spec/06-api.md section 13 (right column). Additive changes pass.

Base rule (06 13 item 7): the newest stable tag `server/vX.Y.Z` merged into HEAD; a
pre-release tag only when no stable tag exists; without any tag the check passes with a notice.
A break passes only when `info.version` raised its major against the base (06 13 items 4 and 5).

The direction of a schema (request, response or both) decides whether a new required field is a
break: paths give it for parameters, request bodies and responses, `webhooks` reversed (the
server sends the body), docs/api/asyncapi.yaml for gateway frames (`receive` is a request,
`send` a response, a reply the opposite). A schema reached by none of them counts in both
directions, so the check never misses a break because a direction is unknown.

Command (from anywhere; `--root` defaults to the repository of this file):

    breaking.py check [--root <dir>]   # exit 0 no break, a notice or an accepted major; 1 else

Every finding is one line `api-breaking: <06 rule>: <location>: <detail>`.
"""

from __future__ import annotations

import argparse
import os
import re
import subprocess
import sys
from collections.abc import Callable, Iterable, Iterator, Mapping
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

import yaml

REPO = Path(__file__).resolve().parents[2]
OPENAPI_PATH = "docs/api/openapi.yaml"
ASYNCAPI_PATH = "docs/api/asyncapi.yaml"
TAG_PREFIX = "server/v"
PREFIX = "api-breaking"

# The rows of the breaking column of the table in 06 section 13, by finding kind.
ROWS: Mapping[str, str] = {
    "removed": "06 13 row 1 remove or rename",
    "required": "06 13 row 2 newly required request field or parameter",
    "type": "06 13 row 3 type, format or meaning",
    "enum": "06 13 row 4 enum value removed",
    "status": "06 13 row 5 status code",
    "op-rule": "06 13 row 6 sort order, pagination or idempotency",
    "authz": "06 13 row 7 x-authz-action",
}

REQUEST = "request"
RESPONSE = "response"
BOTH = frozenset({REQUEST, RESPONSE})
METHODS = ("get", "put", "post", "delete", "options", "head", "patch", "trace")
# Query parameters that name the pagination kind of an operation (06 section 6).
PAGINATION = {
    "cursor": "cursor",
    "before_seq": "sequence",
    "after_seq": "sequence",
    "around_seq": "sequence",
    "after": "global sequence",
}
SCHEMA_REF = "#/components/schemas/"
_SEMVER = re.compile(
    r"^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)"
    r"(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+[0-9A-Za-z.-]+)?$"
)
# git exports these to hooks (the pre-push hook runs `make check`); without them git finds the
# repository from `--root`, as tools/ci/codegen_check.py does.
_GIT_ENV_DROP = ("GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR", "GIT_PREFIX")
_LOADER = getattr(yaml, "CSafeLoader", yaml.SafeLoader)


class BreakingError(Exception):
    """An input cannot be compared: unreadable YAML, a missing document or a bad version."""


@dataclass(frozen=True)
class Finding:
    """One breaking change.

    Attributes:
        row: a key of ROWS, the row of the 06 table the change breaks.
        location: where it is, `components/schemas/<name>/...` or `<METHOD> <path> ...`.
        detail: what changed, for example `property removed`.
    """

    row: str
    location: str
    detail: str

    def line(self) -> str:
        """Return the gate line, for example `api-breaking: 06 13 row 7 ...: GET /x: ...`."""
        return f"{PREFIX}: {ROWS[self.row]}: {self.location}: {self.detail}"


@dataclass(frozen=True)
class Result:
    """Outcome of a comparison.

    Attributes:
        findings: the breaking changes, in the order found.
        base_version: `info.version` of the base document.
        revision_version: `info.version` of the compared document.
        accepted: True when the revision raised the major version, so the breaks are allowed.
    """

    findings: list[Finding]
    base_version: str
    revision_version: str
    accepted: bool


# ---- helpers ------------------------------------------------------------------------------


def _map(value: Any) -> Mapping[str, Any]:
    return value if isinstance(value, Mapping) else {}


def _list(value: Any) -> list[Any]:
    return value if isinstance(value, list) else []


def _resolve(doc: Mapping[str, Any], node: Any) -> Any:
    """Follow local `$ref`s of `node` inside `doc` (non-schema components such as parameters)."""
    seen: set[str] = set()
    while isinstance(node, Mapping) and isinstance(node.get("$ref"), str):
        ref = node["$ref"]
        if not ref.startswith("#/") or ref.startswith(SCHEMA_REF) or ref in seen:
            return node
        seen.add(ref)
        target: Any = doc
        for part in ref[2:].split("/"):
            target = _map(target).get(part.replace("~1", "/").replace("~0", "~"))
        node = target
    return node


@dataclass
class _Collector:
    findings: list[Finding] = field(default_factory=list)

    def add(self, row: str, location: str, detail: str) -> None:
        finding = Finding(row, location, detail)
        if finding not in self.findings:
            self.findings.append(finding)


# ---- directions of the named schemas ------------------------------------------------------


def _schema_directions(
    docs: Iterable[Mapping[str, Any]], asyncapi_docs: Iterable[Mapping[str, Any]]
) -> dict[str, frozenset[str]]:
    """Return, per name in `components.schemas`, the directions in which the schema travels."""
    docs = list(docs)
    found: dict[str, set[str]] = {}
    visited: set[tuple[int, str, str]] = set()

    def walk(doc: Mapping[str, Any], node: Any, direction: str) -> None:
        stack = [node]
        while stack:
            item = stack.pop()
            if isinstance(item, list):
                stack.extend(item)
                continue
            if not isinstance(item, Mapping):
                continue
            ref = item.get("$ref")
            if isinstance(ref, str) and ref.startswith("#/"):
                key = (id(doc), ref, direction)
                if key in visited:
                    continue
                visited.add(key)
                if ref.startswith(SCHEMA_REF):
                    name = ref[len(SCHEMA_REF) :]
                    found.setdefault(name, set()).add(direction)
                    stack.append(_map(_map(doc.get("components")).get("schemas")).get(name))
                else:
                    stack.append(_resolve(doc, item))
                continue
            stack.extend(item.values())

    def operation(doc: Mapping[str, Any], item: Any, server_sends: bool) -> None:
        sent, answered = (RESPONSE, REQUEST) if server_sends else (REQUEST, RESPONSE)
        item = _map(item)
        walk(doc, item.get("parameters"), sent)
        for method in METHODS:
            op = _map(item.get(method))
            walk(doc, op.get("parameters"), sent)
            walk(doc, op.get("requestBody"), sent)
            walk(doc, op.get("responses"), answered)

    for doc in docs:
        for item in _map(doc.get("paths")).values():
            operation(doc, item, server_sends=False)
        for item in _map(doc.get("webhooks")).values():
            operation(doc, item, server_sends=True)
    for asyncapi in asyncapi_docs:
        for raw_op in _map(asyncapi.get("operations")).values():
            op = _map(raw_op)
            to_server = op.get("action") == "receive"
            for name in _asyncapi_schema_refs(asyncapi, op.get("messages")):
                for doc in docs:
                    walk(doc, {"$ref": SCHEMA_REF + name}, REQUEST if to_server else RESPONSE)
            for name in _asyncapi_schema_refs(asyncapi, _map(op.get("reply")).get("messages")):
                for doc in docs:
                    walk(doc, {"$ref": SCHEMA_REF + name}, RESPONSE if to_server else REQUEST)
    names = {n for doc in docs for n in _map(_map(doc.get("components")).get("schemas"))}
    return {n: frozenset(found[n]) if n in found else BOTH for n in names}


def _asyncapi_schema_refs(asyncapi: Mapping[str, Any], node: Any) -> Iterator[str]:
    """Yield the openapi.yaml schema names that messages of an AsyncAPI operation point to."""
    marker = "openapi.yaml" + SCHEMA_REF
    seen: set[str] = set()
    stack = [node]
    while stack:
        item = stack.pop()
        if isinstance(item, list):
            stack.extend(item)
        elif isinstance(item, Mapping):
            ref = item.get("$ref")
            if isinstance(ref, str) and marker in ref:
                yield ref.split(marker, 1)[1]
            elif isinstance(ref, str) and ref.startswith("#/") and ref not in seen:
                seen.add(ref)
                target: Any = asyncapi
                for part in ref[2:].split("/"):
                    target = _map(target).get(part.replace("~1", "/").replace("~0", "~"))
                stack.append(target)
            else:
                stack.extend(item.values())


# ---- schemas ------------------------------------------------------------------------------


def _compare_schema(
    base: Any, rev: Any, loc: str, directions: frozenset[str], out: _Collector
) -> None:
    """Compare one schema node; named schemas behind a `$ref` are compared on their own."""
    if not isinstance(base, Mapping) or not isinstance(rev, Mapping):
        if base != rev and isinstance(base, Mapping) != isinstance(rev, Mapping):
            out.add("type", loc, "schema replaced")
        return
    if "$ref" in base or "$ref" in rev:
        if base.get("$ref") != rev.get("$ref"):
            out.add("type", loc, f"$ref {base.get('$ref')} -> {rev.get('$ref')}")
        return
    for key in ("type", "format", "const", "default"):
        if base.get(key) != rev.get(key):
            out.add("type", loc, f"{key} {base.get(key)!r} -> {rev.get(key)!r}")
    if "enum" in base and "enum" in rev:
        kept = _list(rev["enum"])
        for value in _list(base["enum"]):
            if value not in kept:
                out.add("enum", loc, f"enum value {value!r} removed")
    _compare_properties(base, rev, loc, directions, out)
    for key in ("items", "additionalProperties"):
        b, r = base.get(key), rev.get(key)
        if isinstance(b, Mapping) or isinstance(r, Mapping):
            _compare_schema(b, r, f"{loc}/{key}", directions, out)
    for key in ("oneOf", "anyOf", "allOf"):
        _compare_variants(
            _list(base.get(key)), _list(rev.get(key)), f"{loc}/{key}", key, directions, out
        )
    _compare_discriminator(
        _map(base.get("discriminator")), _map(rev.get("discriminator")), loc, out
    )
    _compare_problem_codes(base, rev, loc, out)


def _compare_properties(
    base: Mapping[str, Any],
    rev: Mapping[str, Any],
    loc: str,
    directions: frozenset[str],
    out: _Collector,
) -> None:
    props_b, props_r = _map(base.get("properties")), _map(rev.get("properties"))
    req_b, req_r = set(_list(base.get("required"))), set(_list(rev.get("required")))
    for name, schema in props_b.items():
        at = f"{loc}/properties/{name}"
        if name not in props_r:
            out.add("removed", at, "property removed")
            continue
        _compare_schema(schema, props_r[name], at, directions, out)
        if name in req_b and name not in req_r and RESPONSE in directions:
            out.add("removed", at, "response property no longer required (may be absent)")
        if name not in req_b and name in req_r and REQUEST in directions:
            out.add("required", at, "request property became required")
    for name in props_r:
        if name not in props_b and name in req_r and REQUEST in directions:
            out.add("required", f"{loc}/properties/{name}", "new required request property")


def _compare_variants(
    base: list[Any],
    rev: list[Any],
    loc: str,
    key: str,
    directions: frozenset[str],
    out: _Collector,
) -> None:
    if not base:
        return
    refs_b = [_map(v).get("$ref") for v in base]
    refs_r = [_map(v).get("$ref") for v in rev]
    if key != "allOf" and all(refs_b) and all(refs_r):
        for ref in refs_b:
            if ref not in refs_r:
                out.add("removed", loc, f"variant {ref} removed")
        return
    if len(base) != len(rev):
        out.add("type", loc, f"{len(base)} -> {len(rev)} subschemas")
        return
    for index, (b, r) in enumerate(zip(base, rev, strict=True)):
        _compare_schema(b, r, f"{loc}/{index}", directions, out)


def _compare_discriminator(
    base: Mapping[str, Any], rev: Mapping[str, Any], loc: str, out: _Collector
) -> None:
    if not base:
        return
    if base.get("propertyName") != rev.get("propertyName"):
        out.add("type", f"{loc}/discriminator", "propertyName changed")
    mapping_r = _map(rev.get("mapping"))
    for value, target in _map(base.get("mapping")).items():
        at = f"{loc}/discriminator/mapping/{value}"
        if value not in mapping_r:
            out.add("removed", at, "discriminator value removed")
        elif mapping_r[value] != target:
            out.add("type", at, f"{target} -> {mapping_r[value]}")


def _compare_problem_codes(
    base: Mapping[str, Any], rev: Mapping[str, Any], loc: str, out: _Collector
) -> None:
    codes_r = {_map(c).get("code"): _map(c) for c in _list(rev.get("x-problem-codes"))}
    for entry in _list(base.get("x-problem-codes")):
        code = _map(entry).get("code")
        at = f"{loc}/x-problem-codes/{code}"
        if code not in codes_r:
            out.add("removed", at, "problem code removed")
        elif codes_r[code].get("status") != _map(entry).get("status"):
            out.add("status", at, f"status {entry.get('status')} -> {codes_r[code].get('status')}")


# ---- operations ---------------------------------------------------------------------------


@dataclass(frozen=True)
class _Op:
    label: str
    op: Mapping[str, Any]
    params: Mapping[tuple[str, str], Mapping[str, Any]]
    server_sends: bool


def _operations(doc: Mapping[str, Any]) -> dict[str, _Op]:
    ops: dict[str, _Op] = {}
    for group, server_sends in (("paths", False), ("webhooks", True)):
        for name, raw_item in _map(doc.get(group)).items():
            item = _map(raw_item)
            for method in METHODS:
                if method not in item:
                    continue
                op = _map(item[method])
                params: dict[tuple[str, str], Mapping[str, Any]] = {}
                for raw in [*_list(item.get("parameters")), *_list(op.get("parameters"))]:
                    param = _map(_resolve(doc, raw))
                    where, pname = str(param.get("in")), str(param.get("name"))
                    params[(where, pname.lower() if where == "header" else pname)] = param
                where = "webhook " if server_sends else ""
                label = f"{method.upper()} {where}{name}"
                ops[label] = _Op(label, op, params, server_sends)
    return ops


def _pagination(doc: Mapping[str, Any], op: _Op) -> str:
    kinds: set[str] = set()
    for (where, name), param in op.params.items():
        if where != "query" or name not in PAGINATION:
            continue
        # `after` is also a time filter (`GET /search`); only an integer one is the global
        # sequence of 06 section 6 item 5.
        if name == "after" and _named(doc, param.get("schema")).get("type") != "integer":
            continue
        kinds.add(PAGINATION[name])
    return " and ".join(sorted(kinds)) or "none"


def _named(doc: Mapping[str, Any], schema: Any) -> Mapping[str, Any]:
    """Return `schema`, or the named schema its `$ref` points to (one step)."""
    node = _map(schema)
    ref = node.get("$ref")
    if isinstance(ref, str) and ref.startswith(SCHEMA_REF):
        node = _map(_map(_map(doc.get("components")).get("schemas")).get(ref[len(SCHEMA_REF) :]))
    return node


def _schema_default(doc: Mapping[str, Any], schema: Any) -> Any:
    return _named(doc, schema).get("default")


def _compare_operation(
    docs: tuple[Mapping[str, Any], Mapping[str, Any]], base: _Op, rev: _Op, out: _Collector
) -> None:
    label = base.label
    sent = frozenset({RESPONSE if base.server_sends else REQUEST})
    answered = frozenset({REQUEST if base.server_sends else RESPONSE})
    if "operationId" in base.op and base.op.get("operationId") != rev.op.get("operationId"):
        out.add(
            "removed",
            label,
            f"operationId {base.op.get('operationId')} -> {rev.op.get('operationId')}",
        )
    if base.op.get("x-authz-action") != rev.op.get("x-authz-action"):
        out.add(
            "authz",
            label,
            f"x-authz-action {base.op.get('x-authz-action')} -> {rev.op.get('x-authz-action')}",
        )
    if base.op.get("x-idempotency-exempt") != rev.op.get("x-idempotency-exempt"):
        out.add(
            "op-rule",
            label,
            f"x-idempotency-exempt {base.op.get('x-idempotency-exempt')} "
            f"-> {rev.op.get('x-idempotency-exempt')}",
        )
    before, after = _pagination(docs[0], base), _pagination(docs[1], rev)
    if not base.server_sends and before != after:
        out.add("op-rule", label, f"pagination {before} -> {after}")
    _compare_parameters(docs, base, rev, sent, out)
    _compare_request_body(docs, base, rev, sent, out)
    _compare_responses(docs, base, rev, answered, out)


def _compare_parameters(
    docs: tuple[Mapping[str, Any], Mapping[str, Any]],
    base: _Op,
    rev: _Op,
    sent: frozenset[str],
    out: _Collector,
) -> None:
    for (where, name), param in base.params.items():
        at = f"{base.label} parameter {where} {name}"
        other = rev.params.get((where, name))
        if other is None:
            out.add("removed", at, "parameter removed")
            continue
        if not param.get("required") and other.get("required") and REQUEST in sent:
            out.add("required", at, "parameter became required")
        _compare_schema(param.get("schema"), other.get("schema"), at, sent, out)
        before = _schema_default(docs[0], param.get("schema"))
        after = _schema_default(docs[1], other.get("schema"))
        if name == "sort" and before != after:
            out.add("op-rule", at, f"default sort order {before!r} -> {after!r}")
    for (where, name), param in rev.params.items():
        if (where, name) not in base.params and param.get("required") and REQUEST in sent:
            out.add("required", f"{base.label} parameter {where} {name}", "new required parameter")


def _compare_content(
    base: Any, rev: Any, at: str, directions: frozenset[str], out: _Collector
) -> None:
    content_r = _map(rev)
    for media, entry in _map(base).items():
        if media not in content_r:
            out.add("type", f"{at} {media}", "media type removed")
            continue
        _compare_schema(
            _map(entry).get("schema"),
            _map(content_r[media]).get("schema"),
            f"{at} {media}",
            directions,
            out,
        )


def _compare_request_body(
    docs: tuple[Mapping[str, Any], Mapping[str, Any]],
    base: _Op,
    rev: _Op,
    sent: frozenset[str],
    out: _Collector,
) -> None:
    at = f"{base.label} request body"
    body_b = _map(_resolve(docs[0], base.op.get("requestBody")))
    body_r = _map(_resolve(docs[1], rev.op.get("requestBody")))
    if body_b and not body_r:
        out.add("removed", at, "request body removed")
        return
    if body_r.get("required") and not body_b.get("required") and REQUEST in sent:
        out.add("required", at, "request body became required")
    _compare_content(body_b.get("content"), body_r.get("content"), at, sent, out)


def _compare_responses(
    docs: tuple[Mapping[str, Any], Mapping[str, Any]],
    base: _Op,
    rev: _Op,
    answered: frozenset[str],
    out: _Collector,
) -> None:
    responses_r = _map(rev.op.get("responses"))
    for code, raw in _map(base.op.get("responses")).items():
        at = f"{base.label} response {code}"
        if code not in responses_r:
            out.add("status", at, "response status removed")
            continue
        resp_b = _map(_resolve(docs[0], raw))
        resp_r = _map(_resolve(docs[1], responses_r[code]))
        headers_r = {str(k).lower(): v for k, v in _map(resp_r.get("headers")).items()}
        for header, raw_header in _map(resp_b.get("headers")).items():
            if str(header).lower() not in headers_r:
                out.add("removed", f"{at} header {header}", "response header removed")
                continue
            _compare_schema(
                _map(_resolve(docs[0], raw_header)).get("schema"),
                _map(_resolve(docs[1], headers_r[str(header).lower()])).get("schema"),
                f"{at} header {header}",
                answered,
                out,
            )
        _compare_content(resp_b.get("content"), resp_r.get("content"), at, answered, out)


# ---- entry points -------------------------------------------------------------------------


def compare(
    base: Mapping[str, Any],
    revision: Mapping[str, Any],
    *,
    asyncapi_docs: Iterable[Mapping[str, Any]] = (),
) -> list[Finding]:
    """Return the breaking changes from `base` to `revision` (rows of 06 13, right column).

    Args:
        base: the published OpenAPI document, parsed.
        revision: the document to check, parsed.
        asyncapi_docs: parsed AsyncAPI documents that give gateway frames their direction.

    Returns:
        The findings in the order found, each once; empty when the change is additive.

    Example: removing `Thing.size` gives
    `[Finding("removed", "components/schemas/Thing/properties/size", "property removed")]`.
    """
    out = _Collector()
    directions = _schema_directions((base, revision), asyncapi_docs)
    schemas_b = _map(_map(base.get("components")).get("schemas"))
    schemas_r = _map(_map(revision.get("components")).get("schemas"))
    ops_b, ops_r = _operations(base), _operations(revision)
    for label, op in ops_b.items():
        if label not in ops_r:
            out.add("removed", label, "operation removed")
        else:
            _compare_operation((base, revision), op, ops_r[label], out)
    responses_r = _map(_map(revision.get("components")).get("responses"))
    for name, resp in _map(_map(base.get("components")).get("responses")).items():
        before, after = _map(resp).get("x-status"), _map(responses_r.get(name)).get("x-status")
        if name in responses_r and before != after:
            out.add("status", f"components/responses/{name}", f"x-status {before} -> {after}")
    for name, schema in schemas_b.items():
        at = f"components/schemas/{name}"
        if name not in schemas_r:
            out.add("removed", at, "schema removed")
        else:
            _compare_schema(schema, schemas_r[name], at, directions.get(name, BOTH), out)
    return out.findings


def _major(doc: Mapping[str, Any], which: str) -> tuple[str, int]:
    version = str(_map(doc.get("info")).get("version"))
    match = _SEMVER.match(version)
    if not match:
        raise BreakingError(f"info.version {version!r} of the {which} document is not SemVer")
    return version, int(match.group(1))


def evaluate(
    base: Mapping[str, Any],
    revision: Mapping[str, Any],
    *,
    asyncapi_docs: Iterable[Mapping[str, Any]] = (),
) -> Result:
    """Compare two documents and apply the major version exception (06 13 items 4 and 5).

    Args:
        base: the published OpenAPI document, parsed.
        revision: the document to check, parsed.
        asyncapi_docs: parsed AsyncAPI documents that give gateway frames their direction.

    Returns:
        The findings and whether the revision's `info.version` raised the major version.

    Raises:
        BreakingError: an `info.version` is not SemVer.
    """
    base_version, base_major = _major(base, "base")
    revision_version, revision_major = _major(revision, "compared")
    findings = compare(base, revision, asyncapi_docs=asyncapi_docs)
    return Result(findings, base_version, revision_version, revision_major > base_major)


def _precedence(version: str) -> tuple[Any, ...]:
    match = _SEMVER.match(version)
    if not match:
        raise BreakingError(f"{version!r} is not SemVer")
    core = tuple(int(match.group(i)) for i in (1, 2, 3))
    if match.group(4) is None:
        return (*core, 1, ())
    ids = tuple((0, int(p), "") if p.isdigit() else (1, 0, p) for p in match.group(4).split("."))
    return (*core, 0, ids)


def select_base_tag(tags: Iterable[str]) -> str | None:
    """Return the base tag: the newest stable `server/vX.Y.Z`, else the newest pre-release.

    Args:
        tags: tag names; names that are not `server/v<SemVer>` are ignored.

    Returns:
        The tag name, or None when no tag of the server exists.

    Example: `select_base_tag(["server/v0.1.0", "server/v0.2.0-beta.1"])` is `"server/v0.1.0"`.
    """
    versions = {t: t[len(TAG_PREFIX) :] for t in tags if t.startswith(TAG_PREFIX)}
    versions = {t: v for t, v in versions.items() if _SEMVER.match(v)}
    stable = [t for t, v in versions.items() if _SEMVER.match(v).group(4) is None]  # type: ignore[union-attr]
    pool = stable or list(versions)
    if not pool:
        return None
    return max(pool, key=lambda t: _precedence(versions[t]))


def _git(root: Path, *args: str) -> subprocess.CompletedProcess[str]:
    env = {k: v for k, v in os.environ.items() if k not in _GIT_ENV_DROP}
    return subprocess.run(
        ["git", "-C", str(root), *args],
        capture_output=True,
        text=True,
        env=env,
        check=False,
    )


def _load_text(text: str, what: str) -> Mapping[str, Any]:
    try:
        doc = yaml.load(text, Loader=_LOADER)  # noqa: S506  # nosec B506 - a safe loader
    except yaml.YAMLError as exc:
        raise BreakingError(f"{what} is not valid YAML ({exc.__class__.__name__})") from exc
    if not isinstance(doc, Mapping):
        raise BreakingError(f"{what} is not a mapping")
    return doc


def _published_tags(root: Path) -> list[str]:
    if _git(root, "rev-parse", "--verify", "--quiet", "HEAD").returncode != 0:
        return []
    listed = _git(root, "tag", "--list", "--merged", "HEAD", f"{TAG_PREFIX}*")
    if listed.returncode != 0:
        raise BreakingError(f"git tag failed: {listed.stderr.strip()}")
    return listed.stdout.split()


def check(root: Path, out: Callable[[str], None] = print) -> int:
    """Run the gate on the repository at `root`.

    Args:
        root: repository root with docs/api/openapi.yaml in its working tree.
        out: receives each output line.

    Returns:
        0 without a break, without a published version or with an accepted major step;
        1 on a break or an input that cannot be compared.
    """
    try:
        tag = select_base_tag(_published_tags(root))
        if tag is None:
            out(f"{PREFIX}: no published version (no tag {TAG_PREFIX}*); nothing to compare")
            return 0
        shown = _git(root, "show", f"{tag}:{OPENAPI_PATH}")
        if shown.returncode != 0:
            raise BreakingError(f"{tag} has no {OPENAPI_PATH}")
        base = _load_text(shown.stdout, f"{tag}:{OPENAPI_PATH}")
        asyncapi_docs = []
        base_async = _git(root, "show", f"{tag}:{ASYNCAPI_PATH}")
        if base_async.returncode == 0:
            asyncapi_docs.append(_load_text(base_async.stdout, f"{tag}:{ASYNCAPI_PATH}"))
        current_async = root / ASYNCAPI_PATH
        if current_async.is_file():
            asyncapi_docs.append(_load_text(current_async.read_text("utf-8"), ASYNCAPI_PATH))
        current = root / OPENAPI_PATH
        if not current.is_file():
            raise BreakingError(f"{OPENAPI_PATH} is missing")
        revision = _load_text(current.read_text("utf-8"), OPENAPI_PATH)
        result = evaluate(base, revision, asyncapi_docs=asyncapi_docs)
    except (BreakingError, OSError, UnicodeDecodeError) as exc:
        out(f"{PREFIX}: source: {exc}")
        return 1
    if not result.findings:
        out(f"{PREFIX}: {OPENAPI_PATH} has no breaking change against {tag}")
        return 0
    count = len(result.findings)
    noun = "breaking change" if count == 1 else "breaking changes"
    step = f"info.version {result.base_version} -> {result.revision_version}"
    if result.accepted:
        for finding in result.findings:
            out(f"{PREFIX}: accepted ({step}): {finding.line()[len(PREFIX) + 2 :]}")
        out(f"{PREFIX}: {count} {noun} against {tag} accepted by the major version step")
        return 0
    for finding in result.findings:
        out(finding.line())
    out(
        f"{PREFIX}: {count} {noun} against {tag} ({step}); a break needs a new major version "
        "of info.version (docs/spec/06-api.md section 13 items 4 and 5)"
    )
    return 1


def main(argv: list[str] | None = None) -> int:
    """Command line: `breaking.py check [--root <dir>]`; returns the exit code of `check`."""
    parser = argparse.ArgumentParser(description="API breaking-change check (06 G6).")
    sub = parser.add_subparsers(dest="cmd", required=True)
    p_check = sub.add_parser("check", help="compare with the last published version")
    p_check.add_argument("--root", type=Path, default=REPO)
    args = parser.parse_args(argv)
    return check(args.root)


if __name__ == "__main__":
    sys.exit(main())
