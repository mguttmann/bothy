# SPDX-License-Identifier: Apache-2.0
"""The actual billing mode (09 17.8; Design note 6) and the tool class (09 17.11; note 7)."""

from __future__ import annotations

import logging
from collections.abc import Callable

import pytest

from bothy_hermes_adapter.actual_mode import UNKNOWN, determine
from bothy_hermes_adapter.runtime import ResolvedCredential
from bothy_hermes_adapter.tool_class import TOOLSET_CLASSES, classify

CANARY = "sk-ant-api03-canary-" + "w" * 20


def _raise() -> ResolvedCredential:
    raise RuntimeError(CANARY)


@pytest.mark.parametrize(
    ("resolve", "mode", "key_type"),
    [
        (lambda: ResolvedCredential("anthropic", "api_key"), "organization", "api_key"),
        (lambda: ResolvedCredential("anthropic", "oauth"), "personal", "oauth"),
        (lambda: ResolvedCredential("custom", "api_key"), "organization", "api_key"),
        (lambda: ResolvedCredential(None, "unknown"), None, "unknown"),
        (lambda: ResolvedCredential("bedrock", "unknown"), None, "unknown"),
        (lambda: ResolvedCredential("openai-codex", "oauth"), None, "unknown"),
        (lambda: ResolvedCredential("anthropic", "something"), None, "unknown"),
        (lambda: ResolvedCredential("Bad Provider!", "api_key"), None, "unknown"),
        (lambda: {"provider": "anthropic", "key_type": "api_key"}, None, "unknown"),
        (_raise, None, "unknown"),
    ],
    ids=[
        "anthropic-api-key",
        "anthropic-oauth",
        "other-provider-api-key",
        "no-credential",
        "cloud-credentials",
        "oauth-of-another-provider",
        "unknown-key-type",
        "unknown-provider-form",
        "unknown-answer",
        "exception",
    ],
)
def test_actual_mode_table(
    resolve: Callable[[], object],
    mode: str | None,
    key_type: str,
    caplog: pytest.LogCaptureFixture,
) -> None:
    with caplog.at_level(logging.DEBUG):
        actual = determine(resolve)
    assert (actual.mode, actual.key_type) == (mode, key_type)
    assert actual.evidence["method"] == "resolved_credentials"
    assert actual.sdk_mode == (mode or "unknown")
    if mode is None and actual.provider is None:
        assert "provider" not in actual.evidence
    assert CANARY not in repr(actual) + str(actual.evidence) + caplog.text


def test_unknown_is_fail_closed() -> None:
    assert UNKNOWN.mode is None
    assert UNKNOWN.evidence == {"key_type": "unknown", "method": "resolved_credentials"}


def test_tool_class_classification() -> None:
    assert classify(frozenset()) == "chat"
    assert classify(frozenset({"clarify", "todo"})) == "chat"
    assert classify(frozenset({"todo", "terminal"})) == "host"
    assert classify(frozenset({"some-mcp-server"})) == "host"
    assert classify(frozenset({"todo", "web"})) == "host"
    assert all(cls in ("chat", "scoped", "host") for cls, _ in TOOLSET_CLASSES.values())
    assert all(reason for _, reason in TOOLSET_CLASSES.values())


def test_tool_class_scoped_rank(monkeypatch: pytest.MonkeyPatch) -> None:
    table = dict(TOOLSET_CLASSES)
    table["scoped-example"] = ("scoped", "named tools with their own minimal credentials")
    monkeypatch.setattr("bothy_hermes_adapter.tool_class.TOOLSET_CLASSES", table)
    assert classify(frozenset({"todo", "scoped-example"})) == "scoped"
    assert classify(frozenset({"scoped-example", "terminal"})) == "host"
