# SPDX-License-Identifier: Apache-2.0
"""The verifier against the contract file of every verifier, docs/api/webhook-test-vectors.json.

docs/DECISIONS.md D-M2-26-12 makes the file the shared contract; M2-50 Design note 8 the Python
module its consumer. Derived secrets are built in memory and never written or printed.
"""

from __future__ import annotations

import base64
import json
from datetime import UTC, datetime
from typing import Any

import pytest
from conftest import REPO

from bothy_sdk.testing import FakeClock, sign_webhook
from bothy_sdk.webhooks import (
    WebhookVerificationError,
    parse_webhook_public_key,
    parse_webhook_secret,
    verify_webhook,
)

VECTORS: dict[str, Any] = json.loads(
    (REPO / "docs" / "api" / "webhook-test-vectors.json").read_text(encoding="utf-8")
)
SIGNATURES: list[dict[str, Any]] = VECTORS["signatures"]
OTHER_ID = "0192f4a0-7b1c-757c-87c8-267db4b2c8a0"


def _headers(case: dict[str, Any], **overrides: str) -> dict[str, str]:
    headers = {
        "webhook-id": case["webhook_id"],
        "webhook-timestamp": str(case["timestamp"]),
        "webhook-signature": case["signature_header"],
    }
    headers.update({k.replace("_", "-"): v for k, v in overrides.items()})
    return headers


def _clock(case: dict[str, Any]) -> FakeClock:
    return FakeClock(datetime.fromtimestamp(case["timestamp"], tz=UTC))


def _code(body: bytes, headers: dict[str, str], case: dict[str, Any]) -> str:
    try:
        verify_webhook(
            body,
            headers,
            keys=case["public_key"],
            secrets=case["hmac_secrets"],
            clock=_clock(case),
        )
    except WebhookVerificationError as err:
        return err.code
    return "accepted"


def _cases() -> list[Any]:
    return [pytest.param(c, id=c["name"]) for c in SIGNATURES]


def test_vectors_have_signature_and_derivation_cases() -> None:
    assert VECTORS["version"] == 1
    assert SIGNATURES
    assert VECTORS["derivation"]


@pytest.mark.parametrize("case", _cases())
def test_case_verifies_with_its_keys_and_secrets(case: dict[str, Any]) -> None:
    verified = verify_webhook(
        case["body"].encode(),
        _headers(case),
        keys=case["public_key"],
        secrets=case["hmac_secrets"],
        clock=_clock(case),
    )
    assert verified.id == case["webhook_id"]
    assert verified.timestamp == case["timestamp"]


@pytest.mark.parametrize("case", _cases())
def test_case_verifies_with_parsed_keys_and_secrets(case: dict[str, Any]) -> None:
    verified = verify_webhook(
        case["body"].encode(),
        _headers(case),
        keys=[parse_webhook_public_key(k) for k in case["public_key"]],
        secrets=[parse_webhook_secret(s) for s in case["hmac_secrets"]],
        clock=_clock(case),
    )
    assert verified.id == case["webhook_id"]


@pytest.mark.parametrize("case", _cases())
def test_case_verifies_with_each_scheme_alone(case: dict[str, Any]) -> None:
    entries = case["signature_header"].split(" ")
    body = case["body"].encode()
    if any(e.startswith("v1a,") for e in entries):
        for key in case["public_key"]:
            verified = verify_webhook(body, _headers(case), keys=[key], clock=_clock(case))
            assert verified.scheme == "v1a"
    if any(e.startswith("v1,") for e in entries):
        for secret in case["hmac_secrets"]:
            verified = verify_webhook(body, _headers(case), secrets=[secret], clock=_clock(case))
            assert verified.scheme == "v1"


@pytest.mark.parametrize("case", _cases())
def test_case_rejects_a_changed_byte(case: dict[str, Any]) -> None:
    body = case["body"].encode()
    for index in (0, len(body) // 2, len(body) - 1):
        changed = bytearray(body)
        changed[index] ^= 0x01
        assert _code(bytes(changed), _headers(case), case) == "signature_mismatch"


@pytest.mark.parametrize("case", _cases())
def test_case_rejects_another_id_and_timestamp(case: dict[str, Any]) -> None:
    body = case["body"].encode()
    assert _code(body, _headers(case, webhook_id=OTHER_ID), case) == "signature_mismatch"
    later = str(case["timestamp"] + 1)
    assert _code(body, _headers(case, webhook_timestamp=later), case) == "signature_mismatch"


@pytest.mark.parametrize("case", _cases())
def test_sign_webhook_reproduces_the_header(case: dict[str, Any]) -> None:
    header = sign_webhook(
        webhook_id=case["webhook_id"],
        timestamp=case["timestamp"],
        body=case["body"],
        seeds=[base64.b64decode(s) for s in case["ed25519_seed_base64"]],
        secrets=case["hmac_secrets"],
    )
    assert header == case["signature_header"]


def test_derived_secrets_parse_to_their_bytes() -> None:
    for derivation in VECTORS["derivation"]:
        raw = bytes.fromhex(derivation["derived_hex"])
        parsed = parse_webhook_secret("whsec_" + base64.b64encode(raw).decode())
        # Compared as hex, which the file holds anyway; the whsec_ form is never printed.
        assert parsed.hex() == derivation["derived_hex"]
