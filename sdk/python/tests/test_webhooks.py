# SPDX-License-Identifier: Apache-2.0
"""The webhook verifier (docs/api/webhooks.md 3, docs/spec/11-extensions.md 7 and 9).

Header and timestamp rules, signature entries, parsers, the decoded envelope, the trust list
cache and a seeded fuzz. Keys come from fixed seeds, secrets carry ``EXAMPLE``; no request
leaves the process and no test reads the wall clock.
"""

from __future__ import annotations

import asyncio
import base64
import doctest
import json
import random
from collections.abc import Callable
from datetime import UTC, datetime
from typing import Any

import httpx
import pytest
from conftest import BASE_URL
from cryptography.hazmat.primitives.asymmetric import ec
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PublicKey
from cryptography.hazmat.primitives.asymmetric.x25519 import X25519PrivateKey
from cryptography.hazmat.primitives.serialization import Encoding, PublicFormat

import bothy_sdk
import bothy_sdk.testing.webhooks as testing_webhooks
from bothy_sdk import webhooks
from bothy_sdk.auth import Credentials, Secret
from bothy_sdk.client import Client
from bothy_sdk.errors import ProblemError
from bothy_sdk.generated.client import Client as CoreClient
from bothy_sdk.generated.models import CommandInvokedEvent
from bothy_sdk.settings import SdkSettings
from bothy_sdk.testing import FakeClock, sign_webhook, webhook_public_key
from bothy_sdk.tolerance import UnknownEvent
from bothy_sdk.webhooks import (
    VerifiedWebhook,
    WebhookConfigError,
    WebhookKeyCache,
    WebhookVerificationError,
    parse_webhook_public_key,
    parse_webhook_secret,
    verify_webhook,
)

# The prefix is assembled at run time so no literal matches the secret-scanning format.
WHSEC = "_".join(["whsec", ""])

SEED_A = bytes([0x11]) * 32
SEED_B = bytes([0x22]) * 32
SECRET = f"{WHSEC}EXAMPLEsecretOfTheWebhookTest123"
OTHER_SECRET = f"{WHSEC}EXAMPLEanotherSecretOfTheTest456"
ID = "0192f4a0-7b1c-757c-87c8-267db4b2c89f"
TS = 1_760_346_000
BODY = b'{"id":"0192f4a0-7b1c-757c-87c8-267db4b2c89f","type":"message.created"}'
TOLERANCE = SdkSettings().webhook_receiver_tolerance
SYNC_EPOCH = "0192f4a0-7b1c-7a3e-8c41-5d2e9b7f1a60"


def clock_at(seconds: int = TS) -> FakeClock:
    return FakeClock(datetime.fromtimestamp(seconds, tz=UTC))


def signed(
    body: bytes | str = BODY,
    *,
    webhook_id: str = ID,
    timestamp: int = TS,
    seeds: list[bytes] | None = None,
    secrets: list[str] | None = None,
) -> dict[str, str]:
    return {
        "webhook-id": webhook_id,
        "webhook-timestamp": str(timestamp),
        "webhook-signature": sign_webhook(
            webhook_id=webhook_id,
            timestamp=timestamp,
            body=body,
            seeds=[SEED_A] if seeds is None else seeds,
            secrets=[SECRET] if secrets is None else secrets,
        ),
    }


def verify(**overrides: Any) -> VerifiedWebhook:
    options: dict[str, Any] = {
        "body": BODY,
        "headers": signed(),
        "keys": [webhook_public_key(SEED_A)],
        "secrets": [SECRET],
        "clock": clock_at(),
    }
    options.update(overrides)
    body = options.pop("body")
    headers = options.pop("headers")
    return verify_webhook(body, headers, **options)


def failure(**overrides: Any) -> WebhookVerificationError:
    with pytest.raises(WebhookVerificationError) as info:
        verify(**overrides)
    return info.value


# Headers


@pytest.mark.parametrize("name", ["webhook-id", "webhook-timestamp", "webhook-signature"])
def test_missing_header(name: str) -> None:
    headers = signed()
    del headers[name]
    assert failure(headers=headers).code == "missing_header"
    assert failure(headers=httpx.Headers(headers)).code == "missing_header"


def test_headers_are_case_insensitive() -> None:
    h = signed()
    headers = {
        "Webhook-Id": h["webhook-id"],
        "WEBHOOK-TIMESTAMP": h["webhook-timestamp"],
        "Webhook-Signature": h["webhook-signature"],
    }
    assert verify(headers=headers).id == ID
    assert verify(headers=httpx.Headers(headers)).id == ID


def test_entries_differing_in_case_are_joined() -> None:
    h = signed(seeds=[SEED_A], secrets=[])
    headers = {
        "webhook-id": ID,
        "webhook-timestamp": str(TS),
        "webhook-signature": "v1,QUJD",
        "Webhook-Signature": h["webhook-signature"],
    }
    # Joined with ", ": the v1a entry follows a space and still verifies.
    assert verify(headers=headers, secrets=[]).scheme == "v1a"
    two_ids = {**h, "Webhook-Id": ID}
    assert failure(headers=two_ids).code == "signature_mismatch"


@pytest.mark.parametrize(
    "value",
    [
        "abc",
        "1.5",
        "",
        "-1",
        "+1760346000",
        " 1760346000",
        "1760346000\n",
        "1e9",
        "9" * 16,
        "١٧٦٠٣٤٦٠٠٠",
    ],
)
def test_invalid_timestamp(value: str) -> None:
    headers = {**signed(), "webhook-timestamp": value}
    assert failure(headers=headers).code == "invalid_timestamp"


# Time window


def test_tolerance_bounds_are_accepted() -> None:
    assert TOLERANCE == 300.0
    for ts in (TS - int(TOLERANCE), TS + int(TOLERANCE)):
        assert verify(headers=signed(timestamp=ts)).timestamp == ts


def test_one_second_beyond_the_tolerance_is_rejected() -> None:
    for ts in (TS - int(TOLERANCE) - 1, TS + int(TOLERANCE) + 1):
        assert failure(headers=signed(timestamp=ts)).code == "timestamp_out_of_tolerance"


def test_tolerance_argument_and_default_clock(monkeypatch: pytest.MonkeyPatch) -> None:
    headers = signed(timestamp=TS + 10)
    assert failure(headers=headers, tolerance=5).code == "timestamp_out_of_tolerance"
    assert verify(headers=headers, tolerance=10).id == ID
    # Without a clock the system clock is used; it is replaced so no test reads the wall time.
    monkeypatch.setattr(webhooks, "SystemClock", clock_at)
    verified = verify_webhook(BODY, signed(), keys=[webhook_public_key(SEED_A)])
    assert verified.timestamp == TS


def test_leading_zeros_are_signed_in_canonical_form() -> None:
    headers = {**signed(), "webhook-timestamp": f"0{TS}"}
    assert verify(headers=headers).timestamp == TS


# Signatures


def test_no_trusted_key() -> None:
    assert failure(keys=[], secrets=[]).code == "no_trusted_key"
    with pytest.raises(WebhookVerificationError) as info:
        verify_webhook(BODY, signed(), clock=clock_at())
    assert info.value.code == "no_trusted_key"


def test_header_and_timestamp_are_checked_before_the_trust_list() -> None:
    headers = signed()
    del headers["webhook-id"]
    assert failure(headers=headers, keys=[], secrets=[]).code == "missing_header"
    late = signed(timestamp=TS + 10_000)
    assert failure(headers=late, keys=[], secrets=[]).code == "timestamp_out_of_tolerance"


def test_valid_entry_among_spaces_unknown_scheme_and_bad_base64() -> None:
    entries = signed()["webhook-signature"]
    header = f"v2,QUJD   v1a,@@not-base64@@ v1,%%%  {entries}   v1a, v1,QUJD== nocomma "
    headers = {**signed(), "webhook-signature": header}
    assert verify(headers=headers).id == ID
    assert verify(headers=headers, secrets=[]).scheme == "v1a"
    assert verify(headers=headers, keys=[]).scheme == "v1"


def test_scheme_of_the_first_valid_entry() -> None:
    assert verify().scheme == "v1a"
    assert verify(headers=signed(seeds=[])).scheme == "v1"


def test_every_key_and_every_secret_is_tried() -> None:
    headers = signed(seeds=[SEED_B], secrets=[OTHER_SECRET])
    keys = [webhook_public_key(SEED_A), webhook_public_key(SEED_B)]
    assert verify(headers=headers, keys=keys, secrets=[]).scheme == "v1a"
    assert verify(headers=headers, keys=[], secrets=[SECRET, OTHER_SECRET]).scheme == "v1"
    assert failure(headers=headers).code == "signature_mismatch"


def test_v1_entry_of_another_length_does_not_raise() -> None:
    long_entry = base64.b64encode(bytes([1]) * 64).decode()
    headers = {**signed(), "webhook-signature": f"v1,QUJD v1,{long_entry}"}
    assert failure(headers=headers).code == "signature_mismatch"
    short_v1a = {**signed(), "webhook-signature": "v1a,QUJD"}
    assert failure(headers=short_v1a).code == "signature_mismatch"


def test_empty_signature_header() -> None:
    assert failure(headers={**signed(), "webhook-signature": ""}).code == "signature_mismatch"


def test_reserialized_body_fails() -> None:
    body = b'{ "id": "0192f4a0-7b1c-757c-87c8-267db4b2c89f", "type": "message.created" }'
    headers = signed(body)
    assert verify(body=body, headers=headers).id == ID
    reserialized = json.dumps(json.loads(body), separators=(",", ":")).encode()
    assert failure(body=reserialized, headers=headers).code == "signature_mismatch"


def test_body_bytes_are_signed_as_utf8() -> None:
    body = '{"type":"message.created","payload":{"content_md":"Café \U0001f600"}}'
    assert verify(body=body.encode(), headers=signed(body)).id == ID


def test_a_key_is_never_taken_from_the_request() -> None:
    forged = signed(seeds=[SEED_B], secrets=[])
    assert failure(headers=forged, secrets=[]).code == "signature_mismatch"


def test_a_key_object_of_another_type_counts_as_no_match() -> None:
    other = X25519PrivateKey.from_private_bytes(bytes(32)).public_key()
    keys: list[Any] = [other, webhook_public_key(SEED_A)]
    assert verify(keys=keys, secrets=[]).scheme == "v1a"


# Parsers


def _spki(public_key: Any) -> str:
    der = public_key.public_bytes(Encoding.DER, PublicFormat.SubjectPublicKeyInfo)
    return base64.b64encode(der).decode()


def test_parse_webhook_public_key_reads_the_whpk_form() -> None:
    key = parse_webhook_public_key(webhook_public_key(SEED_A))
    assert isinstance(key, Ed25519PublicKey)


def test_parse_webhook_public_key_rejects_malformed_values() -> None:
    encoded = webhook_public_key(SEED_A).removeprefix("whpk_")
    der = base64.b64decode(encoded)
    p256 = _spki(ec.derive_private_key(1, ec.SECP256R1()).public_key())
    x25519 = _spki(X25519PrivateKey.from_private_bytes(bytes(32)).public_key())
    for value in [
        WHSEC + encoded,
        "WHPK_" + encoded,
        encoded,
        "whpk_",
        "whpk_@@@@",
        "whpk_" + encoded.rstrip("="),
        # The URL alphabet: "-" and "_" in place of "+" and "/".
        "whpk_-" + encoded[1:],
        "whpk_" + encoded[:-2] + "_=",
        "whpk_" + encoded + "\n",
        "whpk_" + base64.b64encode(der[:20]).decode(),
        "whpk_" + p256,
        "whpk_" + x25519,
    ]:
        with pytest.raises(WebhookConfigError):
            parse_webhook_public_key(value)


def test_parse_webhook_secret() -> None:
    assert base64.b64encode(parse_webhook_secret(SECRET)).decode() == SECRET.removeprefix(WHSEC)
    for value in [
        "",
        WHSEC,
        "EXAMPLE",
        "whsek_EXAMPLEsecretOfTheWebhookTest123",
        WHSEC + "EXAMPLE",
        WHSEC + "EXAM PLEsecretOfTheWebhookTest12",
        WHSEC + "EXAMPLEsecretOfTheWebhookTest123\n",
        WHSEC + "EXAMPLEsecretOfTheWebhookTest123=",
        WHSEC + "EXAMPLE-_",
    ]:
        with pytest.raises(WebhookConfigError):
            parse_webhook_secret(value)
    assert issubclass(WebhookConfigError, ValueError)


def test_config_errors_never_repeat_the_value() -> None:
    for parse in (parse_webhook_secret, parse_webhook_public_key):
        for value in (WHSEC + "EXAMPLEnot base64 at all", "whpk_EXAMPLEnot base64"):
            with pytest.raises(WebhookConfigError) as info:
                parse(value)
            assert "EXAMPLE" not in str(info.value)


def test_verify_webhook_raises_config_errors_for_configured_values() -> None:
    with pytest.raises(WebhookConfigError):
        verify(keys=["whpk_nope"])
    with pytest.raises(WebhookConfigError):
        verify(secrets=[WHSEC])
    with pytest.raises(WebhookConfigError):
        verify(secrets=[b""])
    assert verify(keys=[], secrets=[parse_webhook_secret(SECRET)]).scheme == "v1"


# sign_webhook


def test_sign_webhook_rejects_a_seed_of_another_length() -> None:
    with pytest.raises(ValueError, match="32 bytes"):
        webhook_public_key(bytes(31))
    with pytest.raises(ValueError, match="32 bytes"):
        sign_webhook(webhook_id=ID, timestamp=TS, body=BODY, seeds=[bytes(33)])


def test_sign_webhook_with_raw_secret_bytes() -> None:
    raw = parse_webhook_secret(SECRET)
    from_raw = sign_webhook(webhook_id=ID, timestamp=TS, body=BODY, secrets=[raw])
    assert from_raw == sign_webhook(webhook_id=ID, timestamp=TS, body=BODY, secrets=[SECRET])
    assert sign_webhook(webhook_id=ID, timestamp=TS, body=BODY) == ""
    assert bothy_sdk.testing.sign_webhook is testing_webhooks.sign_webhook


# Decoded envelope

INVOKED: dict[str, Any] = {
    "id": "0192f4a0-7b1c-7ff0-88b6-7d3b051173dd",
    "seq": 5301,
    "schema_version": 1,
    "occurred_at": "2026-10-10T09:30:00.000Z",
    "permission_version": 418,
    "scope": {
        "space_id": "0192f4a0-7b1c-7881-8522-0a58634abc14",
        "channel_id": "0192f4a0-7b1c-72d4-8bae-7661d2b2f203",
    },
    "resource": {"kind": "command", "id": "0192f4a0-7b1c-7beb-80fc-f336f4fe79aa"},
    "sync_epoch": SYNC_EPOCH,
    "type": "command.invoked",
    "payload": {
        "invocation_id": "0192f4a0-7b1c-712e-8ccb-17c85f8d11c9",
        "command_id": "0192f4a0-7b1c-7beb-80fc-f336f4fe79aa",
        "command_name": "escalate",
        "control": False,
        "channel_id": "0192f4a0-7b1c-72d4-8bae-7661d2b2f203",
        "invoked_by": {
            "id": "0192f4a0-7b1c-7f89-8d6b-6960453241bc",
            "kind": "user",
            "handle": "alex",
            "display_name": "Alex",
        },
        "options": [{"name": "ticket", "value": "4711"}],
    },
}


def _verified(body: bytes) -> VerifiedWebhook:
    return verify(body=body, headers=signed(body))


def test_known_event_is_decoded() -> None:
    verified = _verified(json.dumps(INVOKED).encode())
    assert isinstance(verified.envelope, CommandInvokedEvent)
    assert verified.envelope.payload.command_name == "escalate"
    assert verified.sync_epoch == SYNC_EPOCH


def test_null_members_are_dropped() -> None:
    with_nulls = {**INVOKED, "truncated": None, "extra": [None, 1]}
    verified = _verified(json.dumps(with_nulls).encode())
    assert isinstance(verified.envelope, CommandInvokedEvent)
    body = b'{"type":"future.example","gone":null,"list":[null],"nested":{"x":null}}'
    envelope = _verified(body).envelope
    assert isinstance(envelope, UnknownEvent)
    assert dict(envelope.raw) == {"type": "future.example", "list": [None], "nested": {}}
    # The last of duplicate names wins before nulls are dropped.
    assert _verified(b'{"type":"future.example","a":1,"a":null}').envelope.raw == {
        "type": "future.example"
    }


def test_unknown_type_returns_unknown_event() -> None:
    body = f'{{"id":"x","seq":7,"type":"future.example","sync_epoch":"{SYNC_EPOCH}"}}'.encode()
    verified = _verified(body)
    assert isinstance(verified.envelope, UnknownEvent)
    assert (verified.envelope.type, verified.envelope.seq) == ("future.example", 7)
    assert verified.sync_epoch == SYNC_EPOCH


@pytest.mark.parametrize(
    "body",
    [b"not json", b"\xff\xfe{", b"[1,2]", b'"text"', b"5", b"null", b"NaN", b'{"a":Infinity}'],
)
def test_body_that_is_no_json_object(body: bytes) -> None:
    verified = _verified(body)
    assert verified.envelope == UnknownEvent(type="", seq=None, raw={})
    assert verified.sync_epoch is None


def test_byte_order_mark_is_skipped() -> None:
    verified = _verified(b'\xef\xbb\xbf{"type":"future.example"}')
    assert isinstance(verified.envelope, UnknownEvent)
    assert verified.envelope.type == "future.example"


@pytest.mark.parametrize(
    "event",
    [
        {"type": "command.invoked", "seq": 9},
        {**INVOKED, "payload": "not an object"},
        {**INVOKED, "payload": {**INVOKED["payload"], "options": 5}},
        {**INVOKED, "seq": "many", "occurred_at": 3},
        {"type": "message.created", "truncated": None, "payload": {"message": None}},
    ],
)
def test_known_type_that_does_not_decode_is_unknown(event: dict[str, Any]) -> None:
    verified = _verified(json.dumps(event).encode())
    envelope = verified.envelope
    assert isinstance(envelope, UnknownEvent)
    assert envelope.type == event["type"]
    assert envelope.seq == (event["seq"] if isinstance(event.get("seq"), int) else None)
    assert "type" in envelope.raw


def test_sync_epoch_that_is_no_string_is_ignored() -> None:
    assert _verified(b'{"type":"message.created","sync_epoch":5}').sync_epoch is None


def test_verified_webhook_is_frozen_and_hashable_without_envelope() -> None:
    verified = _verified(b'{"type":"future.example"}')
    with pytest.raises(AttributeError):
        verified.id = "other"  # type: ignore[misc]
    assert hash(verified) == hash(_verified(b'{"type":"future.example"}'))


# WebhookKeyCache

SEED_C = bytes([0x33]) * 32
KEYS: dict[str, Any] = {
    "keys": [
        {"kid": "whk-2026-11", "public_key": webhook_public_key(SEED_B), "state": "next"},
        {"kid": "whk-2026-10", "public_key": webhook_public_key(SEED_A), "state": "active"},
        {
            "kid": "whk-2026-09",
            "public_key": webhook_public_key(SEED_C),
            "state": "retiring",
            "not_after": "2026-11-01T00:00:00.000Z",
        },
    ]
}


class KeyServer:
    """A MockTransport handler answering ``getWebhookKeys``; records every request."""

    def __init__(self, cache_control: str | None, body: Any = None, status: int = 200) -> None:
        self.cache_control = cache_control
        self.body = KEYS if body is None else body
        self.status = status
        self.requests: list[httpx.Request] = []

    def __call__(self, request: httpx.Request) -> httpx.Response:
        self.requests.append(request)
        headers = {"content-type": "application/json"}
        if self.cache_control is not None:
            headers["cache-control"] = self.cache_control
        content = self.body if isinstance(self.body, bytes) else json.dumps(self.body).encode()
        return httpx.Response(self.status, content=content, headers=headers)

    def client(self) -> CoreClient:
        return CoreClient(base_url=BASE_URL, httpx_args={"transport": httpx.MockTransport(self)})


def _public_bytes(key: Ed25519PublicKey) -> bytes:
    return key.public_bytes(Encoding.Raw, PublicFormat.Raw)


@pytest.mark.anyio
async def test_cache_fetches_once_without_a_token_and_trusts_three_states() -> None:
    server = KeyServer("public, max-age=1200")
    cache = WebhookKeyCache(server.client(), clock=FakeClock())
    first, second = await asyncio.gather(cache.keys(), cache.keys())
    assert len(first) == 3
    assert [_public_bytes(k) for k in first] == [_public_bytes(k) for k in second]
    await cache.keys()
    assert [r.url.path for r in server.requests] == ["/api/v1/instance/webhook-keys"]
    assert "authorization" not in server.requests[0].headers
    for seed in (SEED_A, SEED_B, SEED_C):
        headers = signed(seeds=[seed], secrets=[])
        assert verify(headers=headers, keys=first, secrets=[]).scheme == "v1a"


@pytest.mark.anyio
async def test_cache_refreshes_only_after_max_age() -> None:
    server = KeyServer("public, max-age=1200")
    clock = FakeClock()
    cache = WebhookKeyCache(server.client(), clock=clock)
    await cache.keys()
    await clock.advance(1199.999)
    await cache.keys()
    assert len(server.requests) == 1
    await clock.advance(0.001)
    await cache.keys()
    assert len(server.requests) == 2


@pytest.mark.anyio
async def test_cache_without_cache_control_uses_jwks_max_age() -> None:
    server = KeyServer(None)
    clock = FakeClock()
    settings = SdkSettings(webhook_keys_max_age=50.0, jwks_refresh_min_interval=1.0)
    cache = WebhookKeyCache(server.client(), clock=clock, settings=settings)
    await cache.keys()
    await clock.advance(49.999)
    await cache.keys()
    assert len(server.requests) == 1
    await clock.advance(0.001)
    await cache.keys()
    assert len(server.requests) == 2


@pytest.mark.anyio
async def test_cache_default_age_is_agent_jwks_max_age() -> None:
    server = KeyServer("no-store")
    clock = FakeClock()
    cache = WebhookKeyCache(server.client(), clock=clock)
    await cache.keys()
    await clock.advance(SdkSettings().webhook_keys_max_age - 0.001)
    await cache.keys()
    assert len(server.requests) == 1
    await clock.advance(0.001)
    await cache.keys()
    assert len(server.requests) == 2


@pytest.mark.anyio
@pytest.mark.parametrize("header", ["max-age=0", 'max-age="10"', "max-age=abc"])
async def test_cache_max_age_below_the_min_interval_waits_that_interval(header: str) -> None:
    server = KeyServer(header)
    clock = FakeClock()
    settings = SdkSettings(webhook_keys_max_age=500.0)
    cache = WebhookKeyCache(server.client(), clock=clock, settings=settings)
    await cache.keys()
    await cache.keys()
    assert len(server.requests) == 1
    # A max-age that is no integer counts as absent: then webhook_keys_max_age applies.
    wait = 500.0 if header.endswith("abc") else settings.jwks_refresh_min_interval
    await clock.advance(wait - 0.001)
    await cache.keys()
    assert len(server.requests) == 1
    await clock.advance(0.001)
    await cache.keys()
    assert len(server.requests) == 2


@pytest.mark.anyio
async def test_cache_never_refreshes_after_a_failed_verification() -> None:
    server = KeyServer("public, max-age=1200")
    cache = WebhookKeyCache(server.client(), clock=FakeClock())
    for _ in range(5):
        forged = signed(seeds=[bytes([0x44]) * 32], secrets=[])
        keys = await cache.keys()
        assert failure(headers=forged, keys=keys, secrets=[]).code == "signature_mismatch"
    assert len(server.requests) == 1


@pytest.mark.anyio
async def test_cache_skips_unreadable_keys_and_unknown_states() -> None:
    body = {
        "keys": [
            *KEYS["keys"],
            {"kid": "bad", "public_key": "whpk_nope", "state": "active"},
            {"kid": "number", "public_key": 5, "state": "active"},
            {"kid": "future", "public_key": webhook_public_key(SEED_A), "state": "revoked"},
        ]
    }
    cache = WebhookKeyCache(KeyServer("max-age=60", body).client(), clock=FakeClock())
    assert len(await cache.keys()) == 3


PROBLEM_503: dict[str, Any] = {
    "type": "about:blank",
    "title": "unavailable",
    "status": 503,
    "code": "unavailable",
    "correlation_id": "0192f4a0-7b1c-7000-8a00-000000000009",
}


@pytest.mark.anyio
async def test_cache_failed_fetch_raises_and_is_retried() -> None:
    server = KeyServer("max-age=60", PROBLEM_503, status=503)
    cache = WebhookKeyCache(server.client(), clock=FakeClock())
    with pytest.raises(ProblemError) as info:
        await cache.keys()
    assert info.value.status == 503
    server.body, server.status = KEYS, 200
    assert len(await cache.keys()) == 3
    assert len(server.requests) == 2


@pytest.mark.anyio
@pytest.mark.parametrize(
    ("status", "body"),
    [
        (503, b"EXAMPLE plain text"),
        (200, b'{"keys":[{"state":"active"}]}'),
        (200, b"[]"),
        (203, json.dumps({**PROBLEM_503, "title": "EXAMPLE", "status": 203}).encode()),
    ],
)
async def test_cache_unreadable_answer_raises_without_its_body(status: int, body: bytes) -> None:
    server = KeyServer(None, body, status=status)
    cache = WebhookKeyCache(server.client(), clock=FakeClock())
    with pytest.raises((RuntimeError, ProblemError)) as info:
        await cache.keys()
    assert "EXAMPLE" not in str(info.value)
    assert "getWebhookKeys" in str(info.value) or isinstance(info.value, ProblemError)
    server.body, server.status = KEYS, 200
    assert len(await cache.keys()) == 3


@pytest.mark.anyio
async def test_cache_concurrent_failure_reaches_every_caller_once() -> None:
    server = KeyServer(None, b"EXAMPLE", status=500)
    cache = WebhookKeyCache(server.client(), clock=FakeClock())
    results = await asyncio.gather(cache.keys(), cache.keys(), return_exceptions=True)
    assert all(isinstance(r, RuntimeError) for r in results)
    assert len(server.requests) == 1


@pytest.mark.anyio
async def test_cache_over_the_hand_layer_client_reads_the_response_headers() -> None:
    server = KeyServer("max-age=120")
    clock = FakeClock()
    client = Client(
        BASE_URL,
        Credentials(Secret("EXAMPLE-token")),
        clock=clock,
        transport=httpx.MockTransport(server),
    )
    async with client:
        cache = WebhookKeyCache(client, clock=clock)
        assert len(await cache.keys()) == 3
        await clock.advance(119.999)
        await cache.keys()
        assert len(server.requests) == 1
        await clock.advance(0.001)
        await cache.keys()
        assert len(server.requests) == 2


@pytest.mark.anyio
async def test_cache_over_the_hand_layer_client_raises_problem_errors() -> None:
    problem = {"type": "about:blank", "title": "forbidden", "status": 403, "code": "forbidden"}
    server = KeyServer(None, problem, status=403)
    client = Client(
        BASE_URL, Credentials(Secret("EXAMPLE-token")), transport=httpx.MockTransport(server)
    )
    async with client:
        with pytest.raises(ProblemError) as info:
            await WebhookKeyCache(client, clock=FakeClock()).keys()
    assert info.value.status == 403


# Seeded fuzz: the verifier only raises WebhookVerificationError, and no message contains the
# body, a signature or a secret.

FUZZ_SEED = 0x5EED2030
ALPHABET = list(
    "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/=,. -_\t:%@ü\u0000"
)
PIECES = ["v1a,", "v1,", "v2,", " ", ",", "=", "==", "QUJD", "v1a,QUJD", "v1,"]


def _fuzz_round(rand: random.Random, round_no: int, valid: dict[str, str]) -> str:
    def text(limit: int) -> str:
        n = rand.randrange(limit) if limit else 0
        return "".join(
            rand.choice(PIECES) if rand.random() < 0.2 else rand.choice(ALPHABET) for _ in range(n)
        )

    marker = f"EXAMPLE-body-marker-{round_no}"
    choice = rand.random()
    if choice < 0.15:
        body = BODY
    elif choice < 0.55:
        as_text = rand.random() < 0.5
        body = (marker + text(40)).encode() if as_text else f'{{"type":"{marker}"}}'.encode()
    else:
        body = bytes(rand.randrange(256) for _ in range(rand.randrange(64)))
    headers: dict[str, str] = {}
    for name in ("webhook-id", "webhook-timestamp", "webhook-signature"):
        r = rand.random()
        if r < 0.1:
            continue
        if r < 0.6:
            headers[name] = valid[name]
        elif name == "webhook-timestamp" and r < 0.8:
            headers[name] = str(TS + int((rand.random() - 0.5) * 1000))
        else:
            headers[name] = text(200 if name == "webhook-signature" else 30)
    if rand.random() < 0.3:
        headers = {k.upper(): v for k, v in headers.items()}
    keys = [parse_webhook_public_key(webhook_public_key(SEED_A))] if rand.random() < 0.8 else []
    secrets: list[bytes | str] = [parse_webhook_secret(SECRET)] if rand.random() < 0.8 else []
    try:
        verify_webhook(body, headers, keys=keys, secrets=secrets, clock=clock_at())
    except WebhookVerificationError as err:
        message = str(err)
        assert marker not in message
        assert "EXAMPLE" not in message
        signature = headers.get("webhook-signature") or headers.get("WEBHOOK-SIGNATURE") or ""
        for entry in signature.split(" "):
            value = entry.partition(",")[2]
            assert len(value) < 4 or value not in message
        return err.code
    return "accepted"


def test_seeded_fuzz_gives_only_verification_errors() -> None:
    rand = random.Random(FUZZ_SEED)  # noqa: S311 - deterministic test input
    valid = signed()
    seen: set[str] = set()
    for round_no in range(2000):
        try:
            seen.add(_fuzz_round(rand, round_no, valid))
        except Exception as exc:  # pragma: no cover - only on a failure
            pytest.fail(f"fuzz seed {FUZZ_SEED:#x} round {round_no}: {type(exc).__name__} {exc}")
    # The fuzz reaches every outcome, so it exercises each step.
    assert seen == {
        "accepted",
        "invalid_timestamp",
        "missing_header",
        "no_trusted_key",
        "signature_mismatch",
        "timestamp_out_of_tolerance",
    }, f"fuzz seed {FUZZ_SEED:#x}"


def test_no_error_message_contains_the_body_or_a_secret() -> None:
    body = b'{"type":"message.created","secret":"EXAMPLE-very-private-body"}'
    headers = signed(body)
    signature_values = [e.partition(",")[2] for e in headers["webhook-signature"].split(" ")]
    no_id = {k: v for k, v in headers.items() if k != "webhook-id"}
    cases: list[Callable[[], VerifiedWebhook]] = [
        lambda: verify(body=body, headers=no_id),
        lambda: verify(body=body, headers={**headers, "webhook-timestamp": "EXAMPLE"}),
        lambda: verify(body=body, headers=signed(body, timestamp=TS + 100_000)),
        lambda: verify(body=body, headers=headers, keys=[], secrets=[]),
        lambda: verify(body=body + b" ", headers=headers),
        lambda: verify(
            body=body, headers=headers, secrets=[OTHER_SECRET], keys=[webhook_public_key(SEED_B)]
        ),
    ]
    codes: set[str] = set()
    forbidden = [
        "EXAMPLE",
        body.decode(),
        SECRET,
        SECRET.removeprefix(WHSEC),
        OTHER_SECRET.removeprefix(WHSEC),
        *signature_values,
    ]
    for case in cases:
        with pytest.raises(WebhookVerificationError) as info:
            case()
        codes.add(info.value.code)
        rendered = f"{info.value!s} {info.value!r} {info.value.args!r}"
        for value in forbidden:
            assert value not in rendered
    assert len(codes) == 5


def test_module_examples_run() -> None:
    for module in (webhooks, testing_webhooks):
        result = doctest.testmod(module, optionflags=doctest.ELLIPSIS)
        assert result.failed == 0, module.__name__
        assert result.attempted > 0, module.__name__


def test_public_exports() -> None:
    for name in (
        "VerifiedWebhook",
        "WebhookConfigError",
        "WebhookKeyCache",
        "WebhookVerificationError",
        "verify_webhook",
    ):
        assert getattr(bothy_sdk, name) is getattr(webhooks, name)
        assert name in bothy_sdk.__all__
