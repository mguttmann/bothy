# SPDX-License-Identifier: Apache-2.0
"""Tests for the MkDocs hook that renders the REST and gateway references from docs/api/."""

from __future__ import annotations

import shutil
from pathlib import Path
from typing import Any

import external_assets_check
import mkdocs_api_reference as hook
import pytest
import yaml
from mkdocs.commands.build import build
from mkdocs.config import load_config
from mkdocs.exceptions import PluginError
from mkdocs.structure.files import File, Files

REPO = Path(__file__).resolve().parents[3]
API = REPO / "docs" / "api"
HOOK = Path(hook.__file__).resolve()


def _load(name: str) -> dict[str, Any]:
    data: dict[str, Any] = yaml.safe_load((API / name).read_text(encoding="utf-8"))
    return data


def _operation_ids(openapi: dict[str, Any]) -> list[str]:
    ids = []
    for group in ("paths", "webhooks"):
        for item in openapi.get(group, {}).values():
            ids.extend(op["operationId"] for op in item.values() if isinstance(op, dict))
    return [i for i in ids if isinstance(i, str)]


def _mini_openapi() -> dict[str, Any]:
    return {
        "openapi": "3.1.0",
        "info": {"title": "Chat", "version": "1.0.0", "description": "Example `<api>` text."},
        "servers": [{"url": "/api/v1", "description": "Relative."}],
        "security": [{"bearerAuth": []}],
        "tags": [{"name": "Threads", "description": "Thread operations."}],
        "paths": {
            "/threads/{thread_id}": {
                "parameters": [{"$ref": "#/components/parameters/ThreadId"}],
                "get": {
                    "operationId": "getThread",
                    "summary": "Read a thread",
                    "tags": ["Threads"],
                    "x-authz-action": "thread.read",
                    "parameters": [
                        {
                            "name": "fields",
                            "in": "query",
                            "description": "A | B",
                            "schema": {"type": "array", "items": {"type": "string"}},
                        }
                    ],
                    "responses": {
                        "200": {
                            "description": "OK.",
                            "headers": {"ETag": {"$ref": "#/components/headers/ETag"}},
                            "content": {
                                "application/json": {
                                    "schema": {"$ref": "#/components/schemas/Thread"},
                                    "examples": {"default": {"summary": "One", "value": {"a": 1}}},
                                }
                            },
                        },
                        "404": {"$ref": "#/components/responses/NotFound"},
                    },
                },
                "post": {
                    "operationId": "postUntagged",
                    "summary": "Untagged",
                    "security": [],
                    "requestBody": {
                        "required": True,
                        "content": {
                            "application/json": {"schema": {"$ref": "#/components/schemas/Thread"}}
                        },
                    },
                    "responses": {"204": {"description": "Done."}},
                },
            }
        },
        "webhooks": {
            "thread.created": {
                "post": {
                    "operationId": "onThreadCreatedEvent",
                    "summary": "Event",
                    "tags": ["Webhook events"],
                    "requestBody": {
                        "content": {
                            "application/json": {"schema": {"$ref": "#/components/schemas/Thread"}}
                        }
                    },
                    "responses": {"204": {"description": "Received."}},
                }
            }
        },
        "components": {
            "securitySchemes": {"bearerAuth": {"type": "http", "scheme": "bearer"}},
            "parameters": {
                "ThreadId": {
                    "name": "thread_id",
                    "in": "path",
                    "required": True,
                    "schema": {"type": "string", "format": "uuid"},
                }
            },
            "headers": {"ETag": {"description": "Version.", "schema": {"type": "string"}}},
            "responses": {
                "NotFound": {
                    "description": "Problem codes: `not_found`.",
                    "content": {
                        "application/problem+json": {
                            "schema": {"$ref": "#/components/schemas/Problem"}
                        }
                    },
                }
            },
            "schemas": {
                "Problem": {"type": "object", "properties": {"code": {"type": "string"}}},
                "State": {"type": "string", "enum": ["open", "closed"]},
                "Thread": {
                    "type": "object",
                    "description": "A thread.",
                    "required": ["id"],
                    "additionalProperties": False,
                    "properties": {
                        "id": {"type": "string", "format": "uuid", "examples": ["x"]},
                        "state": {"$ref": "#/components/schemas/State"},
                        "tags": {
                            "type": "array",
                            "maxItems": 5,
                            "items": {"type": "object", "properties": {"name": {"type": "string"}}},
                        },
                        "labels": {"type": "object", "additionalProperties": {"type": "integer"}},
                    },
                },
                "ThreadEvent": {
                    "description": "Event.",
                    "allOf": [
                        {"$ref": "#/components/schemas/Problem"},
                        {"type": "object", "properties": {"thread": {"type": "string"}}},
                    ],
                },
                "Any": {
                    "oneOf": [
                        {"$ref": "#/components/schemas/Thread"},
                        {"$ref": "#/components/schemas/Problem"},
                    ],
                    "discriminator": {"propertyName": "kind"},
                },
            },
        },
    }


def _mini_asyncapi() -> dict[str, Any]:
    return {
        "asyncapi": "3.0.0",
        "info": {"title": "Gateway", "version": "0.1.0", "x-protocol-versions": [1]},
        "servers": {"instance": {"host": "chat.example.com", "protocol": "wss"}},
        "channels": {
            "gateway": {
                "address": "/gateway",
                "title": "Gateway",
                "description": "One connection.",
                "messages": {
                    "op.identify": {"$ref": "#/components/messages/op.identify"},
                    "op.ready": {"$ref": "#/components/messages/op.ready"},
                },
                "x-close-codes": [{"code": 4000, "name": "unknown_error", "client_action": "x"}],
            }
        },
        "operations": {
            "receive.identify": {
                "action": "receive",
                "channel": {"$ref": "#/channels/gateway"},
                "summary": "Authenticate.",
                "messages": [{"$ref": "#/channels/gateway/messages/op.identify"}],
                "x-authz-action": "gateway.connect",
                "reply": {"messages": [{"$ref": "#/channels/gateway/messages/op.ready"}]},
            }
        },
        "components": {
            "messages": {
                "op.identify": {
                    "name": "identify",
                    "summary": "Authenticate.",
                    "payload": {
                        "type": "object",
                        "required": ["op", "d"],
                        "properties": {
                            "op": {"type": "string", "enum": ["identify"]},
                            "d": {"$ref": "./openapi.yaml#/components/schemas/Thread"},
                        },
                    },
                },
                "op.ready": {"name": "ready", "payload": {"type": "object"}},
            }
        },
    }


def test_rest_page_holds_every_operation_with_its_anchor() -> None:
    page = hook.render_rest(_mini_openapi())
    assert page.startswith("# REST reference\n")
    for op_id in ("getThread", "postUntagged", "onThreadCreatedEvent"):
        assert f"{{#op-{op_id.lower()}}}" in page
    assert "`GET` `/threads/{thread_id}`" in page
    assert "| `thread_id` | path |" in page
    assert "A \\| B" in page
    assert "[Thread](schemas.md#schema-Thread)" in page
    assert "[NotFound](#response-NotFound)" in page
    assert "[ETag](#header-ETag)" in page
    assert "`x-authz-action`: `thread.read`" in page
    assert "Security: none" in page
    assert "## Other operations" in page
    assert "[webhooks](../webhooks.md)" in page
    assert '??? example "One"' in page
    assert "Example `<api>` text" in page


def test_text_outside_code_spans_is_escaped() -> None:
    assert hook.text("a <b> `<c>` d") == "a &lt;b> `<c>` d"
    assert hook.text("`{x:\n  <body>}` <y>") == "`{x: <body>}` &lt;y>"


def test_pipe_is_escaped_only_outside_code_spans() -> None:
    # Python-Markdown tables keep `|` inside code spans; a backslash there would be literal.
    assert hook.cell("a | b `^(x|y)$`") == "a \\| b `^(x|y)$`"


def test_schema_page_renders_types_and_nested_properties() -> None:
    page = hook.render_schemas(_mini_openapi())
    assert "### Thread {#schema-Thread}" in page
    assert "| `id` | string (uuid) | yes |" in page
    assert "| `state` | [State](#schema-State) | no |" in page
    assert "| `tags[].name` | string | no |" in page
    assert "array of object" in page
    assert "map of integer" in page
    assert "Example: `x`" in page
    assert "Max items 5." in page
    assert "No other properties are allowed." in page
    assert "Values: `open`, `closed`" in page
    assert "All of: [Problem](#schema-Problem)" in page
    assert "| `thread` | string | no |" in page
    assert "One of: [Thread](#schema-Thread), [Problem](#schema-Problem)" in page
    assert "Discriminator: `kind`" in page


def test_gateway_page_holds_every_message_and_operation() -> None:
    page = hook.render_gateway(_mini_asyncapi())
    assert page.startswith("# Gateway reference\n")
    assert "{#message-op-identify}" in page
    assert "Name: `identify`" in page
    assert "{#message-op-ready}" in page
    assert "{#operation-receive-identify}" in page
    assert "[op.identify](#message-op-identify)" in page
    assert "Reply: [op.ready](#message-op-ready)" in page
    assert "[Thread](schemas.md#schema-Thread)" in page
    assert "| 4000 | unknown_error | x |" in page
    assert "Channel: [Gateway](#channel-gateway)" in page


def test_unresolvable_reference_fails() -> None:
    spec = _mini_openapi()
    spec["paths"]["/threads/{thread_id}"]["parameters"] = [{"$ref": "#/components/parameters/X"}]
    with pytest.raises(PluginError, match="components/parameters/X"):
        hook.render_rest(spec)


def test_anchor_collision_fails() -> None:
    spec = _mini_asyncapi()
    spec["components"]["messages"]["op-identify"] = {"name": "other"}
    with pytest.raises(PluginError, match="message-op-identify"):
        hook.render_gateway(spec)


def _config(tmp_path: Path) -> Any:
    docs = tmp_path / "docs"
    (docs / "api").mkdir(parents=True)
    (docs / "index.md").write_text("# Start\n")
    (docs / "api" / "webhooks.md").write_text("# Webhooks\n")
    shutil.copy(API / "openapi.yaml", docs / "api" / "openapi.yaml")
    shutil.copy(API / "asyncapi.yaml", docs / "api" / "asyncapi.yaml")
    cfg = tmp_path / "mkdocs.yml"
    cfg.write_text(
        yaml.safe_dump(
            {
                "site_name": "t",
                "docs_dir": str(docs),
                "site_dir": str(tmp_path / "site"),
                "theme": {"name": "material", "font": False},
                "hooks": [str(HOOK)],
                "markdown_extensions": [
                    "admonition",
                    "attr_list",
                    "tables",
                    "pymdownx.details",
                    "pymdownx.superfences",
                    {"toc": {"permalink": True}},
                ],
                "validation": {"links": {"anchors": "warn"}},
                "nav": [
                    {"Start": "index.md"},
                    {"REST": hook.REST_PAGE},
                    {"Schemas": hook.SCHEMA_PAGE},
                    {"Gateway": hook.GATEWAY_PAGE},
                    {"Webhooks": "api/webhooks.md"},
                ],
            }
        )
    )
    return load_config(str(cfg), strict=True)


def test_built_site_holds_every_operation_and_message(tmp_path: Path) -> None:
    config = _config(tmp_path)
    build(config)
    site = tmp_path / "site"
    rest = (site / "api" / "reference" / "rest" / "index.html").read_text(encoding="utf-8")
    gateway = (site / "api" / "reference" / "gateway" / "index.html").read_text(encoding="utf-8")
    schemas = (site / "api" / "reference" / "schemas" / "index.html").read_text(encoding="utf-8")
    openapi, asyncapi = _load("openapi.yaml"), _load("asyncapi.yaml")
    missing = [i for i in _operation_ids(openapi) if f'id="op-{i.lower()}"' not in rest]
    assert missing == []
    assert all(f">{i}<" in rest for i in _operation_ids(openapi))
    for key, message in asyncapi["components"]["messages"].items():
        assert key in gateway
        assert f"<code>{message['name']}</code>" in gateway
    assert all(f'id="schema-{n}"' in schemas for n in openapi["components"]["schemas"])
    assert external_assets_check.scan(site) == []


def test_committed_copy_of_a_generated_page_fails(tmp_path: Path) -> None:
    config = _config(tmp_path)
    stale = tmp_path / "docs" / "api" / "reference" / "rest.md"
    stale.parent.mkdir()
    stale.write_text("# old\n")
    files = Files([File("api/reference/rest.md", config.docs_dir, config.site_dir, False)])
    with pytest.raises(PluginError, match="generated"):
        hook.on_files(files, config=config)


def test_missing_document_fails(tmp_path: Path) -> None:
    config = _config(tmp_path)
    (tmp_path / "docs" / "api" / "asyncapi.yaml").unlink()
    with pytest.raises(PluginError, match=r"asyncapi\.yaml"):
        hook.on_files(Files([]), config=config)


def test_site_config_registers_the_hook_and_lists_the_pages() -> None:
    text = (REPO / "mkdocs.yml").read_text(encoding="utf-8")
    assert "tools/docs/mkdocs_api_reference.py" in text
    for page in (hook.REST_PAGE, hook.SCHEMA_PAGE, hook.GATEWAY_PAGE):
        assert page in text
