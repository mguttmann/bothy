# SPDX-License-Identifier: Apache-2.0
"""Check `client-routes`: every HTTP operation the Python clients call is served (HERMES-E2E).

The Python SDK (`sdk/python/src/bothy_sdk`, without its generated core) and the Python adapters
under `plugins/` call the server through the generated operation modules of the SDK
(`bothy_sdk.generated.api.<tag>.<operation>`). An operation they import must exist in
docs/api/openapi.yaml with the method and path of its generated module, stand under `enabled` in
the server's enable list services/server/api/operations.yaml and be registered in the generated
route table services/server/api/routes/routes.gen.go. The first contract break of the Hermes
adapter in live use was exactly such a call (`getAgentTriggerers`, documented but not enabled, so
the server answered 404; SERVER-AGENT-TRIGGERERS).

Findings, one line each, `client-routes: <sub-check>: <subject>: <detail>`, the subject being the
source file and line of the import:

- `module`: the imported name has no generated module (the SDK was not regenerated);
- `openapi`: the method and path of the generated module are no operation of openapi.yaml, or
  their operationId does not fit the module name;
- `enabled`: the operation stands under `later` (or nowhere) in operations.yaml;
- `router`: the operation is missing from routes.gen.go, or registered with another method or
  path.

Example: `check(Path("."))` returns `[]` for a clean checkout.
"""

from __future__ import annotations

import ast
import re
from collections.abc import Iterator
from dataclasses import dataclass
from pathlib import Path

import yaml

RULE = "client-routes"
"""Name of the rule in `api_checks.py check --rule`."""
OPENAPI_PATH = Path("docs/api/openapi.yaml")
"""The REST contract."""
ENABLE_LIST = Path("services/server/api/operations.yaml")
"""The operations the server generates and registers (`enabled`)."""
ROUTE_TABLE = Path("services/server/api/routes/routes.gen.go")
"""The generated route table of the server."""
GENERATED = Path("sdk/python/src/bothy_sdk/generated/api")
"""The generated operation modules of the Python SDK."""
API_PREFIX = "/api/v1"
"""The base path of every operation in the route table."""
GENERATED_PACKAGE = "bothy_sdk.generated.api"
"""The import prefix of a generated operation module."""

_METHODS = ("get", "put", "post", "patch", "delete")
_ROUTE = re.compile(
    r'ID:\s+"([A-Za-z0-9]+)",\s*\n\s*Method:\s+"([A-Z]+)",\s*\n\s*Path:\s+"([^"]+)"'
)
_SNAKE = re.compile(r"(?<!^)(?=[A-Z])")


@dataclass(frozen=True)
class Call:
    """One generated operation a client imports."""

    tag: str
    name: str
    where: str


def _finding(sub: str, subject: str, detail: str) -> str:
    """Return one finding line."""
    return f"{RULE}: {sub}: {subject}: {detail}"


def client_sources(root: Path) -> list[Path]:
    """The Python sources of the clients: the SDK (no generated core) and every plugin's code.

    Tests and the generated core are left out; a plugin's own package lies under `src/` or at the
    plugin's top level.
    """
    sources = [
        p
        for p in sorted((root / "sdk/python/src/bothy_sdk").rglob("*.py"))
        if "generated" not in p.relative_to(root / "sdk/python/src/bothy_sdk").parts
    ]
    for plugin in sorted((root / "plugins").glob("*/pyproject.toml")):
        base = plugin.parent
        for path in sorted(base.rglob("*.py")):
            parts = path.relative_to(base).parts
            if parts[0] in ("tests", "typings", ".venv", "node_modules") or "tests" in parts:
                continue
            sources.append(path)
    return sources


def _imports(tree: ast.AST, rel: str) -> Iterator[Call]:
    """The generated operation modules a source imports, with the line of each import."""
    for node in ast.walk(tree):
        if isinstance(node, ast.Import):
            # import bothy_sdk.generated.api.<tag>.<op> (an `import` of a tag module alone names
            # no operation; its attribute uses are not followed).
            for alias in node.names:
                parts = alias.name.split(".")
                if alias.name.startswith(GENERATED_PACKAGE + ".") and len(parts) > 4:
                    yield Call(parts[3], ".".join(parts[4:]), f"{rel}:{node.lineno}")
            continue
        if not isinstance(node, ast.ImportFrom) or node.module is None:
            continue
        module = node.module
        if module.startswith(GENERATED_PACKAGE + "."):
            tag = module[len(GENERATED_PACKAGE) + 1 :]
            if "." in tag:  # from bothy_sdk.generated.api.<tag>.<op> import ...
                tag, name = tag.split(".", 1)
                yield Call(tag, name, f"{rel}:{node.lineno}")
                continue
            for alias in node.names:
                yield Call(tag, alias.name, f"{rel}:{node.lineno}")
        elif module == GENERATED_PACKAGE:
            for alias in node.names:
                yield Call(alias.name, "*", f"{rel}:{node.lineno}")


def client_calls(root: Path) -> list[Call]:
    """Every generated operation the client sources import (doctest examples included)."""
    calls: list[Call] = []
    for path in client_sources(root):
        rel = path.relative_to(root).as_posix()
        tree = ast.parse(path.read_text(encoding="utf-8"), filename=rel)
        calls += [c for c in _imports(tree, rel) if c.name != "*"]
    return calls


def _request(module: Path) -> tuple[str, str] | None:
    """Method and path of a generated module (`_get_kwargs`), `None` when it has none."""
    tree = ast.parse(module.read_text(encoding="utf-8"))
    for node in ast.walk(tree):
        if not isinstance(node, ast.Dict):
            continue
        keys = [k.value if isinstance(k, ast.Constant) else None for k in node.keys]
        if "method" not in keys or "url" not in keys:
            continue
        method = node.values[keys.index("method")]
        url = node.values[keys.index("url")]
        if not isinstance(method, ast.Constant):
            return None
        return str(method.value).upper(), _template(url)
    return None


def _template(node: ast.expr) -> str:
    """The path template of a generated `url`: a text, `"...".format(...)` or an f-string."""
    if isinstance(node, ast.Constant):
        return str(node.value)
    if (
        isinstance(node, ast.Call)
        and isinstance(node.func, ast.Attribute)
        and node.func.attr == "format"
        and isinstance(node.func.value, ast.Constant)
    ):
        return str(node.func.value.value)
    out = ""
    if isinstance(node, ast.JoinedStr):
        for part in node.values:
            if isinstance(part, ast.Constant):
                out += str(part.value)
            elif isinstance(part, ast.FormattedValue):
                inner = part.value
                while isinstance(inner, ast.Call) and inner.args:
                    inner = inner.args[0]
                name = inner.id if isinstance(inner, ast.Name) else "param"
                out += "{" + name + "}"
    return out


def _shape(path: str) -> str:
    """A path with every parameter name replaced, so names need not agree."""
    return re.sub(r"\{[^}]+\}", "{}", path)


def _openapi(root: Path) -> dict[tuple[str, str], str]:
    """(method, path shape) to operationId of every operation of openapi.yaml."""
    doc = yaml.safe_load((root / OPENAPI_PATH).read_text(encoding="utf-8"))
    out: dict[tuple[str, str], str] = {}
    for path, item in (doc.get("paths") or {}).items():
        for method in _METHODS:
            op = (item or {}).get(method)
            if isinstance(op, dict) and isinstance(op.get("operationId"), str):
                out[(method.upper(), _shape(path))] = op["operationId"]
    return out


def _enabled(root: Path) -> set[str]:
    doc = yaml.safe_load((root / ENABLE_LIST).read_text(encoding="utf-8"))
    return {str(x) for x in (doc.get("enabled") or [])}


def _routes(root: Path) -> dict[str, tuple[str, str]]:
    text = (root / ROUTE_TABLE).read_text(encoding="utf-8")
    return {m[0]: (m[1], m[2]) for m in _ROUTE.findall(text)}


def _snake(operation_id: str) -> str:
    return _SNAKE.sub("_", operation_id).lower()


def check(root: Path) -> list[str]:
    """Run the rule against the checkout `root`; return the findings, empty when clean."""
    found: list[str] = []
    operations = _openapi(root)
    enabled = _enabled(root)
    routes = _routes(root)
    if not routes:
        return [_finding("router", ROUTE_TABLE.as_posix(), "no route found")]
    for call in client_calls(root):
        module = root / GENERATED / call.tag / f"{call.name}.py"
        request = _request(module) if module.is_file() else None
        if request is None:
            found.append(
                _finding("module", call.where, f"{call.tag}.{call.name} is no generated operation")
            )
            continue
        method, path = request
        op_id = operations.get((method, _shape(path)))
        if op_id is None or _snake(op_id) != call.name:
            found.append(
                _finding("openapi", call.where, f"{method} {path} ({call.name}) not in openapi")
            )
            continue
        if op_id not in enabled:
            found.append(
                _finding("enabled", call.where, f"{op_id} is not enabled in {ENABLE_LIST}")
            )
            continue
        route = routes.get(op_id)
        if route is None or route[0] != method or _shape(route[1]) != _shape(API_PREFIX + path):
            found.append(
                _finding("router", call.where, f"{op_id} {method} {path} is not registered")
            )
    return found
