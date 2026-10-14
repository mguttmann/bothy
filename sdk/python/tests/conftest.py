# SPDX-License-Identifier: Apache-2.0
"""Shared helpers of the SDK tests: the spec, the shared fixtures and a fake transport.

REST calls never leave the process: the generated client gets an `httpx.MockTransport` that
answers every request with one fixture, on the base URL `https://example.com/api/v1`.
"""

from __future__ import annotations

import json
from collections.abc import Callable, Mapping
from functools import cache
from pathlib import Path
from typing import Any

import httpx
import pytest
import yaml

from bothy_sdk.generated.client import Client

REPO = Path(__file__).resolve().parents[3]
SPEC_PATH = REPO / "docs" / "api" / "openapi.yaml"
FIXTURES = REPO / "sdk" / "testdata" / "tolerance"
BASE_URL = "https://example.com/api/v1"

_Loader = getattr(yaml, "CSafeLoader", yaml.SafeLoader)


@cache
def spec() -> dict[str, Any]:
    """docs/api/openapi.yaml, parsed once per test run."""
    with SPEC_PATH.open(encoding="utf-8") as fh:
        loaded: dict[str, Any] = yaml.load(fh, Loader=_Loader)  # noqa: S506 (safe loader)
    return loaded


def schema(name: str) -> dict[str, Any]:
    """The schema `name` of `components.schemas`."""
    found: dict[str, Any] = spec()["components"]["schemas"][name]
    return found


def fixture(name: str) -> dict[str, Any]:
    """The shared fixture `sdk/testdata/tolerance/<name>.json`."""
    loaded: dict[str, Any] = json.loads((FIXTURES / f"{name}.json").read_text(encoding="utf-8"))
    return loaded


@pytest.fixture
def answering() -> Callable[[Mapping[str, Any]], tuple[Client, list[httpx.Request]]]:
    """Factory: a generated client whose transport answers every request with `body` (200)."""

    def make(body: Mapping[str, Any]) -> tuple[Client, list[httpx.Request]]:
        seen: list[httpx.Request] = []

        def handler(request: httpx.Request) -> httpx.Response:
            seen.append(request)
            return httpx.Response(200, json=body)

        client = Client(base_url=BASE_URL, httpx_args={"transport": httpx.MockTransport(handler)})
        return client, seen

    return make
