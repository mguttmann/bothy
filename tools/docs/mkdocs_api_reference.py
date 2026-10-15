# SPDX-License-Identifier: Apache-2.0
"""MkDocs hook: render the REST and gateway references from the normative API documents.

`docs/api/openapi.yaml` and `docs/api/asyncapi.yaml` are the one source the routes and clients
are generated from (ADR 0013). This hook turns them into three static Markdown pages at build
time, so readers need no YAML and the site needs no browser script and no foreign server
(brief M7-19, rulings M7-19/1 to M7-19/3):

- `api/reference/rest.md`: every operation of `paths` grouped by tag, every webhook event,
  security schemes, shared responses and headers;
- `api/reference/schemas.md`: every schema of `components.schemas`;
- `api/reference/gateway.md`: servers, channels with close codes, operations and messages
  of the gateway; message bodies link to the schema page.

The pages exist only in the build (`File.generated`); a committed copy under
`docs/api/reference/` fails the build, so a stale reference cannot be checked in. A `$ref`
that does not resolve or two equal anchors fail the build too. Text from the documents is
Markdown; `<` outside code spans is escaped, so the documents cannot inject HTML.

The pages use the Markdown extensions `attr_list`, `tables`, `admonition`, `pymdownx.details`
and `pymdownx.superfences`, all enabled in mkdocs.yml. Registered in mkdocs.yml under `hooks:`.
"""

from __future__ import annotations

import json
import re
from collections.abc import Iterable, Iterator
from pathlib import Path
from typing import Any

import yaml
from mkdocs.config.defaults import MkDocsConfig
from mkdocs.exceptions import PluginError
from mkdocs.structure.files import File, Files

REST_PAGE = "api/reference/rest.md"
SCHEMA_PAGE = "api/reference/schemas.md"
GATEWAY_PAGE = "api/reference/gateway.md"
PAGES = (REST_PAGE, SCHEMA_PAGE, GATEWAY_PAGE)

# Link target of schema references from the REST and gateway pages (same directory).
_SCHEMAS = "schemas.md"
_METHODS = ("get", "put", "post", "delete", "options", "head", "patch", "trace")
_CODE_SPAN = re.compile(r"(`[^`]*`)")
_SCHEMA_REF = re.compile(r"#/components/schemas/(?P<name>[^/]+)$")
_MESSAGE_REF = re.compile(r"^#/components/messages/(?P<name>[^/]+)$")
_CHANNEL_REF = re.compile(r"^#/channels/(?P<name>[^/]+)$")

Node = dict[str, Any]


def text(value: object) -> str:
    """Return document text as Markdown with `<` escaped outside code spans.

    Args:
        value: A description or summary from the documents (None gives "").

    Example: `text("a <b> `<c>`")` returns ``a &lt;b> `<c>` ``.
    """
    if value is None:
        return ""
    parts = _CODE_SPAN.split(str(value))
    # A code span that a YAML block scalar wraps over two lines is not a code span to
    # Python-Markdown, so its `<` would become raw HTML: join such a span into one line.
    return "".join(
        re.sub(r"\s*\n\s*", " ", p) if p.startswith("`") else p.replace("<", "&lt;") for p in parts
    ).strip()


def cell(value: object) -> str:
    """Return document text fit for one Markdown table cell (no pipes, no line breaks)."""
    # Python-Markdown's tables keep `|` inside code spans, where a backslash would be literal.
    parts = _CODE_SPAN.split(text(value))
    out = "".join(p if p.startswith("`") else p.replace("|", "\\|") for p in parts)
    return out.replace("\n\n", "<br><br>").replace("\n", " ")


def code(value: object) -> str:
    """Return `value` as an inline code span; non-strings are written as compact JSON."""
    raw = value if isinstance(value, str) else json.dumps(value, ensure_ascii=False)
    return f"`{raw}`"


def _anchor_key(name: str) -> str:
    """Return a name usable inside an HTML id written with attr_list (dots become hyphens)."""
    return re.sub(r"[^A-Za-z0-9_-]", "-", name)


class Anchors:
    """Ids already used on one page; a second use of an id fails the build."""

    def __init__(self) -> None:
        """Start a page with no ids used."""
        self._seen: set[str] = set()

    def add(self, anchor: str) -> str:
        """Register `anchor` and return it. Raises PluginError if it is taken."""
        if anchor in self._seen:
            raise PluginError(f"api reference: duplicate anchor {anchor}")
        self._seen.add(anchor)
        return anchor


class Document:
    """One API document with local `$ref` resolution (JSON pointers starting with `#/`)."""

    def __init__(self, data: Node, name: str) -> None:
        """Wrap the parsed document `data`; `name` labels it in error messages."""
        self.data = data
        self.name = name

    def resolve(self, ref: str) -> Any:
        """Return the node `ref` points to. Raises PluginError if it does not exist."""
        if not ref.startswith("#/"):
            raise PluginError(f"api reference: {self.name}: unsupported $ref {ref}")
        node: Any = self.data
        for raw in ref[2:].split("/"):
            key = raw.replace("~1", "/").replace("~0", "~")
            if not isinstance(node, dict) or key not in node:
                raise PluginError(f"api reference: {self.name}: unresolvable $ref {ref}")
            node = node[key]
        return node

    def deref(self, node: Node) -> tuple[Node, list[str]]:
        """Follow `$ref` chains of `node`; return the target and the refs passed, in order."""
        refs: list[str] = []
        while isinstance(node, dict) and "$ref" in node:
            refs.append(node["$ref"])
            node = self.resolve(node["$ref"])
        return node, refs


# Schemas ---------------------------------------------------------------------------------


def schema_link(ref: str, prefix: str) -> str:
    """Return a Markdown link to the schema `ref` names, on the schema page `prefix`."""
    match = _SCHEMA_REF.search(ref)
    if match is None:
        return code(ref)
    name = match.group("name")
    return f"[{name}]({prefix}#schema-{name})"


def type_md(schema: Node, prefix: str) -> str:
    """Return a short Markdown type of `schema`, with links to named schemas.

    Args:
        schema: A schema object of the documents.
        prefix: Path of the schema page relative to the current page ("" on that page).

    Example: an array of `$ref: '#/components/schemas/Thread'` gives
    `array of [Thread](schemas.md#schema-Thread)`.
    """
    if "$ref" in schema:
        return schema_link(schema["$ref"], prefix)
    for key, word in (("allOf", " and "), ("oneOf", " or "), ("anyOf", " or ")):
        if key in schema:
            return word.join(type_md(part, prefix) for part in schema[key])
    kind = schema.get("type")
    if isinstance(kind, list):
        kind = " or ".join(kind)
    if kind == "array":
        return "array of " + type_md(schema.get("items", {}), prefix)
    if kind == "object" or (kind is None and "properties" in schema):
        extra = schema.get("additionalProperties")
        if isinstance(extra, dict) and not schema.get("properties"):
            return "map of " + type_md(extra, prefix)
        return "object"
    if "const" in schema:
        return "constant " + code(schema["const"])
    if kind is None:
        return "any"
    return f"{kind} ({schema['format']})" if "format" in schema else str(kind)


_CONSTRAINTS = (
    ("minLength", "min length"),
    ("maxLength", "max length"),
    ("minimum", "minimum"),
    ("maximum", "maximum"),
    ("exclusiveMinimum", "exclusive minimum"),
    ("exclusiveMaximum", "exclusive maximum"),
    ("minItems", "min items"),
    ("maxItems", "max items"),
    ("uniqueItems", "unique items"),
)


def notes(schema: Node) -> list[str]:
    """Return the value rules of `schema` (enum, constraints, default, example) as phrases."""
    out: list[str] = []
    values = schema.get("enum")
    if values is None and isinstance(schema.get("items"), dict):
        values = schema["items"].get("enum")
    if values is not None:
        out.append("Values: " + ", ".join(code(v) for v in values) + ".")
    rules = [f"{label} {schema[key]}" for key, label in _CONSTRAINTS if key in schema]
    if "pattern" in schema:
        rules.append("pattern " + code(schema["pattern"]))
    if rules:
        joined = "; ".join(rules)
        out.append(joined[0].upper() + joined[1:] + ".")
    if "default" in schema:
        out.append("Default: " + code(schema["default"]) + ".")
    if schema.get("examples"):
        out.append("Example: " + code(schema["examples"][0]) + ".")
    if schema.get("deprecated"):
        out.append("Deprecated.")
    return out


def property_rows(schema: Node, prefix: str, path: str = "") -> Iterator[str]:
    """Yield one table row per property of `schema`, nested inline objects as `a.b`/`a[].b`."""
    required = set(schema.get("required", []))
    for name, prop in schema.get("properties", {}).items():
        full = path + name
        desc = " ".join([text(prop.get("description")), *notes(prop)]).strip()
        req = "yes" if name in required else "no"
        yield f"| `{full}` | {type_md(prop, prefix)} | {req} | {cell(desc)} |"
        if "properties" in prop:
            yield from property_rows(prop, prefix, full + ".")
        items = prop.get("items")
        if prop.get("type") == "array" and isinstance(items, dict) and "properties" in items:
            yield from property_rows(items, prefix, full + "[].")


def schema_body(schema: Node, prefix: str) -> list[str]:
    """Return the Markdown blocks that describe one schema (type, values, properties)."""
    blocks: list[str] = []
    if schema.get("description"):
        blocks.append(text(schema["description"]))
    parts = [schema]
    if "allOf" in schema:
        refs = [p for p in schema["allOf"] if "$ref" in p]
        if refs:
            blocks.append("All of: " + ", ".join(type_md(p, prefix) for p in refs))
        parts = [p for p in schema["allOf"] if "$ref" not in p]
    elif "oneOf" in schema or "anyOf" in schema:
        options = schema.get("oneOf") or schema.get("anyOf") or []
        blocks.append("One of: " + ", ".join(type_md(p, prefix) for p in options))
    elif not schema.get("properties"):
        blocks.append("Type: " + type_md(schema, prefix))
    if "discriminator" in schema:
        blocks.append("Discriminator: " + code(schema["discriminator"]["propertyName"]))
    rows = [row for part in parts for row in property_rows(part, prefix)]
    if rows:
        head = "| Property | Type | Required | Description |\n|---|---|---|---|"
        blocks.append("\n".join([head, *rows]))
    for part in parts:
        extra = part.get("additionalProperties")
        if extra is False:
            blocks.append("No other properties are allowed.")
        elif isinstance(extra, dict) and part.get("properties"):
            blocks.append("Other properties: " + type_md(extra, prefix))
    phrases = notes(schema)
    if phrases:
        blocks.append(" ".join(phrases))
    return blocks


def render_schemas(openapi: Node) -> str:
    """Return the schema page: every schema of `components.schemas` with its anchor."""
    anchors = Anchors()
    out = [
        "# Schemas",
        "Generated at build time from `docs/api/openapi.yaml`; the gateway uses the same "
        "schemas. Do not edit this page: change the document.",
    ]
    for name, schema in openapi.get("components", {}).get("schemas", {}).items():
        out.append(f"### {name} {{#{anchors.add('schema-' + name)}}}")
        out.extend(schema_body(schema, ""))
    return "\n\n".join(out) + "\n"


# REST ------------------------------------------------------------------------------------


def _example_blocks(media: Node) -> list[str]:
    """Return collapsible JSON blocks for the examples of one media type object."""
    examples = dict(media.get("examples", {}))
    if "example" in media:
        examples["example"] = {"value": media["example"]}
    blocks = []
    for key, example in examples.items():
        title = str(example.get("summary") or key).replace('"', "'")
        body = json.dumps(example.get("value"), indent=2, ensure_ascii=False)
        lines = "\n".join("    " + line for line in ["```json", *body.splitlines(), "```"])
        blocks.append(f'??? example "{title}"\n\n{lines}')
    return blocks


def _content_cell(content: Node) -> str:
    """Return `media type: schema` pairs of a content map for one table cell."""
    return "<br>".join(
        f"`{mt}`: {type_md(m.get('schema', {}), _SCHEMAS)}" for mt, m in content.items()
    )


def _extensions(node: Node) -> list[str]:
    """Return list items for the `x-` extensions of a node."""
    return [f"- `{k}`: {code(v)}" for k, v in node.items() if k.startswith("x-")]


def _security(requirements: list[Node]) -> str:
    """Return the alternatives of a security requirement list as Markdown."""
    if not requirements:
        return "none"
    options = [" and ".join(f"[{name}](#security-{name})" for name in req) for req in requirements]
    return " or ".join(options)


class _Rest:
    """Renders the REST page of one OpenAPI document."""

    def __init__(self, openapi: Node) -> None:
        self.doc = Document(openapi, "openapi.yaml")
        self.api = openapi
        self.anchors = Anchors()

    def parameters(self, raw: Iterable[Node]) -> list[str]:
        rows = []
        for item in raw:
            param, _ = self.doc.deref(item)
            schema = param.get("schema", {})
            desc = " ".join([text(param.get("description")), *notes(schema)]).strip()
            req = "yes" if param.get("required") else "no"
            rows.append(
                f"| `{param['name']}` | {param['in']} | {type_md(schema, _SCHEMAS)} "
                f"| {req} | {cell(desc)} |"
            )
        if not rows:
            return []
        head = "| Name | In | Type | Required | Description |\n|---|---|---|---|---|"
        return ["Parameters:", "\n".join([head, *rows])]

    def headers_cell(self, headers: Node) -> str:
        names = []
        for name, header in headers.items():
            if "$ref" in header:
                self.doc.deref(header)
                ref_name = header["$ref"].rsplit("/", 1)[-1]
                names.append(f"[{name}](#header-{ref_name})")
            else:
                names.append(code(name))
        return ", ".join(names)

    def responses(self, responses: Node) -> list[str]:
        rows, examples = [], []
        for status, item in responses.items():
            response, refs = self.doc.deref(item)
            desc = cell(response.get("description"))
            if refs:
                name = refs[0].rsplit("/", 1)[-1]
                desc = f"[{name}](#response-{name}): {desc}"
            else:
                for media in response.get("content", {}).values():
                    examples.extend(_example_blocks(media))
            content = _content_cell(response.get("content", {}))
            headers = self.headers_cell(response.get("headers", {}))
            rows.append(f"| {status} | {desc} | {content} | {headers} |")
        head = "| Status | Description | Content | Headers |\n|---|---|---|---|"
        return ["Responses:", "\n".join([head, *rows]), *examples]

    def operation(self, method: str, where: str, op: Node, shared: list[Node]) -> list[str]:
        op_id = op["operationId"]
        out = [f"### {op_id} {{#{self.anchors.add('op-' + op_id.lower())}}}"]
        out.append(f"`{method.upper()}` {where}")
        if op.get("summary"):
            out.append(f"**{text(op['summary'])}**")
        if op.get("description"):
            out.append(text(op["description"]))
        meta = []
        if "security" in op:
            meta.append("- Security: " + _security(op["security"]))
        else:
            meta.append("- Security: " + _security(self.api.get("security", [])) + " (default)")
        if op.get("deprecated"):
            meta.append("- Deprecated")
        meta.extend(_extensions(op))
        out.append("\n".join(meta))
        merged: dict[tuple[str, str], Node] = {}
        for item in [*shared, *op.get("parameters", [])]:
            param, _ = self.doc.deref(item)
            merged[(param["name"], param["in"])] = item
        out.extend(self.parameters(merged.values()))
        if "requestBody" in op:
            body, _ = self.doc.deref(op["requestBody"])
            label = "Request body (required):" if body.get("required") else "Request body:"
            out.append(label + " " + _content_cell(body.get("content", {})))
            if body.get("description"):
                out.append(text(body["description"]))
            for media in body.get("content", {}).values():
                out.extend(_example_blocks(media))
        out.extend(self.responses(op.get("responses", {})))
        return out

    def render(self) -> str:
        info = self.api.get("info", {})
        out = [
            "# REST reference",
            f"Generated at build time from `docs/api/openapi.yaml` ({text(info.get('title'))}, "
            f"version {code(info.get('version'))}). Do not edit this page: change the document. "
            "Body schemas: [Schemas](schemas.md); webhook delivery, signatures and retries: "
            "[webhooks](../webhooks.md).",
        ]
        for key in ("summary", "description"):
            if info.get(key):
                out.append(text(info[key]))
        if "license" in info:
            out.append("License: " + text(info["license"].get("name")))
        servers = self.api.get("servers", [])
        if servers:
            rows = [f"| `{s['url']}` | {cell(s.get('description'))} |" for s in servers]
            out.extend(["## Servers", "\n".join(["| URL | Description |\n|---|---|", *rows])])
        out.extend(self.paths())
        out.extend(self.webhooks())
        out.extend(self.components())
        return "\n\n".join(out) + "\n"

    def paths(self) -> list[str]:
        groups: dict[str, list[list[str]]] = {t["name"]: [] for t in self.api.get("tags", [])}
        other: list[list[str]] = []
        for path, item in self.api.get("paths", {}).items():
            shared = item.get("parameters", [])
            for method in _METHODS:
                if method in item:
                    op = item[method]
                    block = self.operation(method, f"`{path}`", op, shared)
                    tags = op.get("tags") or []
                    groups.setdefault(tags[0], []).append(block) if tags else other.append(block)
        descriptions = {t["name"]: t.get("description") for t in self.api.get("tags", [])}
        out: list[str] = []
        for tag, blocks in groups.items():
            if not blocks:
                continue
            out.append(f"## {tag}")
            if descriptions.get(tag):
                out.append(text(descriptions[tag]))
            out.extend(line for block in blocks for line in block)
        if other:
            out.append("## Other operations")
            out.extend(line for block in other for line in block)
        return out

    def webhooks(self) -> list[str]:
        hooks = self.api.get("webhooks", {})
        if not hooks:
            return []
        out = [
            "## Webhook events",
            "Requests the server sends to subscribed endpoints. Delivery, the signature "
            "headers, retries and the receiver checks: [webhooks](../webhooks.md); the "
            "envelope fields are those of the event schemas below.",
        ]
        for name, item in hooks.items():
            for method in _METHODS:
                if method in item:
                    out.extend(self.operation(method, f"webhook `{name}`", item[method], []))
        return out

    def components(self) -> list[str]:
        comps = self.api.get("components", {})
        out: list[str] = []
        schemes = comps.get("securitySchemes", {})
        if schemes:
            out.append("## Security schemes")
            for name, scheme in schemes.items():
                out.append(f"### {name} {{#{self.anchors.add('security-' + name)}}}")
                fields = [f"- `{k}`: {code(v)}" for k, v in scheme.items() if k != "description"]
                out.append("\n".join(fields))
                if scheme.get("description"):
                    out.append(text(scheme["description"]))
        responses = comps.get("responses", {})
        if responses:
            out.append("## Shared responses")
            for name, response in responses.items():
                out.append(f"### {name} {{#{self.anchors.add('response-' + name)}}}")
                out.append(text(response.get("description")))
                out.extend(_extensions(response) and ["\n".join(_extensions(response))])
                if response.get("content"):
                    out.append("Content: " + _content_cell(response["content"]))
                if response.get("headers"):
                    out.append("Headers: " + self.headers_cell(response["headers"]))
                for media in response.get("content", {}).values():
                    out.extend(_example_blocks(media))
        headers = comps.get("headers", {})
        if headers:
            out.append("## Shared headers")
            for name, header in headers.items():
                out.append(f"### {name} {{#{self.anchors.add('header-' + name)}}}")
                out.append("Type: " + type_md(header.get("schema", {}), _SCHEMAS))
                if header.get("description"):
                    out.append(text(header["description"]))
        return out


def render_rest(openapi: Node) -> str:
    """Return the REST page for an OpenAPI 3.1 document.

    Raises:
        PluginError: a `$ref` does not resolve or two anchors are equal.
    """
    return _Rest(openapi).render()


# Gateway ---------------------------------------------------------------------------------


class _Gateway:
    """Renders the gateway page of one AsyncAPI 3 document."""

    def __init__(self, asyncapi: Node) -> None:
        self.doc = Document(asyncapi, "asyncapi.yaml")
        self.api = asyncapi
        self.anchors = Anchors()

    def message_link(self, item: Node) -> str:
        _, refs = self.doc.deref(item)
        names = [m.group("name") for r in refs if (m := _MESSAGE_REF.match(r))]
        name = names[-1] if names else refs[-1].rsplit("/", 1)[-1]
        return f"[{name}](#message-{_anchor_key(name)})"

    def channel_link(self, item: Node) -> str:
        channel, refs = self.doc.deref(item)
        match = _CHANNEL_REF.match(refs[-1]) if refs else None
        key = match.group("name") if match else ""
        return f"[{text(channel.get('title') or key)}](#channel-{_anchor_key(key)})"

    def render(self) -> str:
        info = self.api.get("info", {})
        out = [
            "# Gateway reference",
            f"Generated at build time from `docs/api/asyncapi.yaml` ({text(info.get('title'))}, "
            f"version {code(info.get('version'))}). Do not edit this page: change the document. "
            "Body schemas: [Schemas](schemas.md).",
        ]
        if info.get("description"):
            out.append(text(info["description"]))
        meta = _extensions(info)
        if self.api.get("defaultContentType"):
            meta.insert(0, "- Default content type: " + code(self.api["defaultContentType"]))
        if meta:
            out.append("\n".join(meta))
        servers = self.api.get("servers", {})
        if servers:
            head = "| Server | Host | Path | Protocol | Description |\n|---|---|---|---|---|"
            rows = [
                f"| `{name}` | `{s.get('host', '')}` | `{s.get('pathname', '')}` "
                f"| {s.get('protocol', '')} | {cell(s.get('description'))} |"
                for name, s in servers.items()
            ]
            out.extend(["## Servers", "\n".join([head, *rows])])
        out.extend(self.channels())
        out.extend(self.operations())
        out.extend(self.messages())
        return "\n\n".join(out) + "\n"

    def channels(self) -> list[str]:
        out = ["## Channels"]
        for key, channel in self.api.get("channels", {}).items():
            anchor = self.anchors.add("channel-" + _anchor_key(key))
            out.append(f"### {text(channel.get('title') or key)} {{#{anchor}}}")
            out.append(f"Address: `{channel.get('address')}`")
            if channel.get("description"):
                out.append(text(channel["description"]))
            links = [self.message_link(m) for m in channel.get("messages", {}).values()]
            if links:
                out.append("Messages: " + ", ".join(links))
            codes = channel.get("x-close-codes", [])
            if codes:
                columns = list(dict.fromkeys(k for c in codes for k in c))
                head = "| " + " | ".join(c.replace("_", " ").capitalize() for c in columns)
                rows = [
                    "| " + " | ".join(cell(c.get(k, "")) for k in columns) + " |" for c in codes
                ]
                rule = "|" + "---|" * len(columns)
                out.extend(["Close codes:", "\n".join([head + " |", rule, *rows])])
            other = {
                k: v for k, v in channel.items() if k.startswith("x-") and k != "x-close-codes"
            }
            if other:
                out.append("\n".join(_extensions(other)))
            if channel.get("bindings"):
                body = json.dumps(channel["bindings"], indent=2, ensure_ascii=False)
                lines = "\n".join("    " + line for line in ["```json", *body.splitlines(), "```"])
                out.append(f'??? info "Bindings"\n\n{lines}')
        return out

    def operations(self) -> list[str]:
        out = ["## Operations"]
        for key, op in self.api.get("operations", {}).items():
            out.append(f"### {key} {{#{self.anchors.add('operation-' + _anchor_key(key))}}}")
            line = f"Action: `{op.get('action')}`"
            if "channel" in op:
                line += " · Channel: " + self.channel_link(op["channel"])
            out.append(line)
            for field in ("summary", "description"):
                if op.get(field):
                    out.append(text(op[field]))
            messages = [self.message_link(m) for m in op.get("messages", [])]
            if messages:
                out.append("Messages: " + ", ".join(messages))
            reply = op.get("reply", {})
            replies = [self.message_link(m) for m in reply.get("messages", [])]
            if replies:
                out.append("Reply: " + ", ".join(replies))
            meta = _extensions(op)
            if meta:
                out.append("\n".join(meta))
        return out

    def messages(self) -> list[str]:
        out = ["## Messages"]
        for key, message in self.api.get("components", {}).get("messages", {}).items():
            out.append(f"### {key} {{#{self.anchors.add('message-' + _anchor_key(key))}}}")
            line = "Name: " + code(message.get("name", key))
            if message.get("title") and message["title"] != message.get("name"):
                line += " · Title: " + text(message["title"])
            out.append(line)
            for field in ("summary", "description"):
                if message.get(field):
                    out.append(text(message[field]))
            payload = message.get("payload")
            if isinstance(payload, dict):
                if "$ref" in payload or not payload.get("properties"):
                    out.append("Payload: " + type_md(payload, _SCHEMAS))
                else:
                    out.append("Payload:")
                    out.extend(schema_body(payload, _SCHEMAS))
            meta = _extensions(message)
            if meta:
                out.append("\n".join(meta))
        return out


def render_gateway(asyncapi: Node) -> str:
    """Return the gateway page for an AsyncAPI 3 document.

    Raises:
        PluginError: a `$ref` does not resolve or two anchors are equal.
    """
    return _Gateway(asyncapi).render()


# Hook ------------------------------------------------------------------------------------


def _read(path: Path) -> Node:
    """Load one API document. Raises PluginError if it is missing or not a mapping."""
    try:
        data = yaml.safe_load(path.read_text(encoding="utf-8"))
    except FileNotFoundError as exc:
        raise PluginError(f"api reference: {path} not found") from exc
    if not isinstance(data, dict):
        raise PluginError(f"api reference: {path} is not a mapping")
    return data


def on_files(files: Files, config: MkDocsConfig) -> Files:
    """Add the generated reference pages to the build (MkDocs event `on_files`).

    Args:
        files: The files MkDocs found in docs_dir.
        config: The site configuration; the documents are read from `<docs_dir>/api/`.

    Returns:
        `files` with the three generated pages.

    Raises:
        PluginError: a page of `PAGES` is committed under docs_dir, a document is missing, a
            `$ref` does not resolve or two anchors are equal.
    """
    for page in PAGES:
        if files.get_file_from_path(page) is not None:
            raise PluginError(
                f"api reference: {page} is generated at build time by "
                "tools/docs/mkdocs_api_reference.py; remove the committed copy"
            )
    api = Path(config.docs_dir) / "api"
    openapi = _read(api / "openapi.yaml")
    asyncapi = _read(api / "asyncapi.yaml")
    rendered = {
        REST_PAGE: render_rest(openapi),
        SCHEMA_PAGE: render_schemas(openapi),
        GATEWAY_PAGE: render_gateway(asyncapi),
    }
    for uri, content in rendered.items():
        files.append(File.generated(config, uri, content=content))
    return files
