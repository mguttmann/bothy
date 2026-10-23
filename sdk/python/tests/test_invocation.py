# SPDX-License-Identifier: Apache-2.0
"""The invocation check of 09 4.5 against the cases of M2-06's reference verifier.

The cases and verdicts are those of ``TestParseInvocationToken_HeaderRules``,
``TestReferenceVerifier_RejectsAlgAndTyp``, ``TestReferenceVerifier_Points`` and
``TestReferenceVerifier_JWKSCache`` (services/server/internal/runs); the SDK verifier must agree
with the reference on each of them. Tokens are never printed: case names identify a failure.
"""

from __future__ import annotations

import dataclasses
import json
from collections.abc import Awaitable, Callable
from datetime import UTC, datetime
from types import SimpleNamespace
from typing import Any
from uuid import UUID

import pytest
from _tokens import (
    TYP,
    Issuer,
    Jwks,
    b64,
    header,
    payload_of,
    public_bytes,
    sign_ed,
    sign_hs256_with_public_key,
    unb64,
)
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey

from bothy_sdk.auth import Secret
from bothy_sdk.generated.models import (
    AgentFollowupPayload,
    AgentInvocationPayload,
    AgentTriggerers,
    CommandAutocompleteRequestedPayload,
    CommandInvokedPayload,
    InteractionCreatedPayload,
)
from bothy_sdk.ids import uuid7
from bothy_sdk.inbox import MemoryInboxStore
from bothy_sdk.invocation import (
    TOKEN_EVENT_TYPES,
    Actual,
    Expectation,
    InvocationVerifier,
    TokenVerdict,
    compare_actual,
    expectation_for,
)
from bothy_sdk.settings import SdkSettings
from bothy_sdk.testing import FakeClock

# The values of the reference tests: agent.invocation_token_ttl, adapter.invocation_token_leeway,
# adapter.jwks_refresh_min_interval and the max-age of the JWKS (agent.jwks_max_age).
TTL = 60
LEEWAY = 30
REFRESH = 300
MAX_AGE = 1200

SETTINGS = SdkSettings(invocation_token_leeway=LEEWAY, jwks_refresh_min_interval=REFRESH)
ACTUAL = Actual(billing_mode="organization", tool_class="scoped")


@pytest.fixture
def anyio_backend() -> str:
    return "asyncio"


class Env:
    """A clock, an issuer, a JWKS fetcher and a verifier of the issuer's agent."""

    def __init__(
        self,
        *,
        max_age: float | None = MAX_AGE,
        allowlist: Callable[[], Awaitable[AgentTriggerers]] | None = None,
    ) -> None:
        self.clock = FakeClock(datetime(2026, 10, 20, 9, 0, tzinfo=UTC))
        self.issuer = Issuer(self.clock.now, uuid7(), uuid7(), TTL)
        self.jwks = Jwks(self.issuer, max_age)
        self.store = MemoryInboxStore()
        self.verifier = InvocationVerifier(
            instance_id=self.issuer.instance_id,
            agent_id=self.issuer.agent_id,
            jwks=self.jwks,
            replay=self.store,
            settings=SETTINGS,
            clock=self.clock,
            allowlist=allowlist,
        )

    def full(self) -> Expectation:
        g = self.issuer.grant
        return Expectation(
            thread_id=UUID(g["thread_id"]),
            run_id=UUID(g["run_id"]),
            trigger_claim="message_id",
            trigger_id=UUID(g["message_id"]),
        )

    async def verdict(
        self, token: Secret | str, expect: Expectation | None = None, actual: Actual = ACTUAL
    ) -> TokenVerdict:
        secret = token if isinstance(token, Secret) else Secret(token)
        check = await self.verifier.check(secret, expect or self.full(), actual)
        return check.verdict


def bad_headers(kid: str) -> dict[str, str]:
    """The header cases of TestParseInvocationToken_HeaderRules and RejectsAlgAndTyp."""
    h = {
        "typ missing": header(kid, typ=None),
        "kid empty": header("", typ=TYP),
        "kid missing": json.dumps({"alg": "EdDSA", "typ": TYP}),
        "crit": json.dumps({"alg": "EdDSA", "kid": kid, "typ": TYP, "crit": ["exp"]}),
        "alg missing": header(kid, alg=None),
        "kid number": json.dumps({"alg": "EdDSA", "kid": 1, "typ": TYP}),
        "duplicate alg": '{"alg":"none","alg":"EdDSA","kid":"' + kid + '","typ":"' + TYP + '"}',
        "duplicate typ": '{"alg":"EdDSA","kid":"' + kid + '","typ":"JWT","typ":"' + TYP + '"}',
        "header not an object": "[1]",
        "header trailing data": header(kid) + "{}",
    }
    for alg in ("none", "HS256", "ES256", "RS256", "Ed25519", "eddsa"):
        h["alg " + alg] = header(kid, alg=alg)
    for typ in (
        "JWT",
        "at+jwt",
        "terminal-session+jwt",
        "application/agent-invocation+jwt",
        "Agent-Invocation+JWT",
        "agent-invocation+jwt ",
    ):
        h["typ " + typ] = header(kid, typ=typ)
    return h


def invalid_cases(env: Env, valid: str) -> tuple[dict[str, str], dict[str, Expectation]]:
    """The INVALID cases of the reference tests: tokens and, where it differs, the expectation."""
    issuer = env.issuer
    key = issuer.keys[issuer.active]
    head, body, sig = valid.split(".")
    payload = unb64(body)

    # Points 1 and 2 and the token form: every variant is INVALID.
    invalid: dict[str, str] = {
        name: sign_ed(h, payload, key) for name, h in bad_headers(issuer.active).items()
    }
    invalid["HS256 with the public key"] = sign_hs256_with_public_key(
        header(issuer.active, alg="HS256"), payload, key
    )
    invalid["none with an empty signature"] = (
        b64(header(issuer.active, alg="none").encode()) + "." + body + "."
    )
    invalid["padded signature"] = f"{head}.{body}.{sig}="
    invalid["padded payload"] = f"{head}.{body}=.{sig}"
    invalid["character outside base64url"] = f"{head}.{body}.{sig}+"
    invalid["two segments"] = f"{head}.{body}"
    invalid["four segments"] = f"{valid}.{sig}"
    invalid["empty header segment"] = f".{body}.{sig}"
    invalid["signature of another key"] = sign_ed(
        header(issuer.active), payload, Ed25519PrivateKey.generate()
    )
    # Payload shape (decodeClaims of the reference).
    for name, change in {
        "aud array": lambda m: m.update(aud=[m["aud"]]),
        "iss missing": lambda m: m.pop("iss"),
        "sub empty": lambda m: m.update(sub=""),
        "permission_version missing": lambda m: m.pop("permission_version"),
        "iat string": lambda m: m.update(iat="1"),
        "exp fraction": lambda m: m.update(exp=1.5),
        "iat boolean": lambda m: m.update(iat=True),
        "unknown tool_class": lambda m: m.update(tool_class="root"),
        "run_id null": lambda m: m.update(run_id=None),
        "message_id number": lambda m: m.update(message_id=5),
    }.items():
        invalid[name] = issuer.resign(valid, change)
    invalid["duplicate payload member"] = sign_ed(
        header(issuer.active), payload[:-1] + b',"aud":"' + str(uuid7()).encode() + b'"}', key
    )
    invalid["payload trailing data"] = sign_ed(header(issuer.active), payload + b"{}", key)

    # Points 4 and 6, and an iat in the future (point 5).
    full = env.full()
    expect_for: dict[str, Expectation] = {}
    invalid["foreign iss"] = issuer.resign(valid, lambda m: m.update(iss=str(uuid7())))
    invalid["foreign aud"] = issuer.resign(valid, lambda m: m.update(aud=str(uuid7())))
    invalid["upper-case aud"] = issuer.resign(valid, lambda m: m.update(aud=m["aud"].upper()))
    changed = payload.replace(b'"level":"trigger"', b'"level":"triggex"')
    invalid["changed payload byte"] = f"{head}.{b64(changed)}.{sig}"
    for name, member in (
        ("run_id", "run_id"),
        ("thread_id", "thread_id"),
        ("triggering object", "trigger_id"),
    ):
        invalid[name] = issuer.raw()
        expect_for[name] = dataclasses.replace(full, **{member: uuid7()})
    invalid["iat in the future"] = issuer.resign(
        valid, lambda m: m.update(iat=m["iat"] + LEEWAY + 1, exp=m["iat"] + LEEWAY + 1 + TTL)
    )
    invalid["exp not after iat"] = issuer.resign(valid, lambda m: m.update(exp=m["iat"]))
    return invalid, expect_for


@pytest.mark.anyio
async def test_verifier_agrees_with_reference_cases() -> None:
    env = Env()
    issuer = env.issuer
    valid = issuer.raw()
    invalid, expect_for = invalid_cases(env, valid)
    for name, tok in invalid.items():
        got = await env.verdict(tok, expect_for.get(name))
        assert got is TokenVerdict.INVALID, name

    # The jti store is unchanged by the rejections: the valid token passes once, then REPLAY.
    assert await env.verdict(valid) is TokenVerdict.VALID
    assert await env.verdict(valid) is TokenVerdict.REPLAY

    # Point 8: a differing and an unknown actual mode never match.
    for mode in ("personal", "unknown"):
        got = await env.verdict(issuer.token(), actual=Actual(mode, "scoped"))
        assert got is TokenVerdict.BILLING_MODE_MISMATCH, mode
    # Point 10: scoped above chat is a mismatch, chat under scoped passes.
    got = await env.verdict(issuer.token(), actual=Actual("organization", "host"))
    assert got is TokenVerdict.TOOL_CLASS_MISMATCH
    chat = issuer.token(tool_class="chat")
    got = await env.verdict(chat, actual=Actual("organization", "scoped"))
    assert got is TokenVerdict.TOOL_CLASS_MISMATCH
    assert await env.verdict(issuer.token(), actual=Actual("organization", "chat")) is (
        TokenVerdict.VALID
    )
    # An unknown actual tool class never passes.
    got = await env.verdict(issuer.token(), actual=Actual("organization", "unknown"))
    assert got is TokenVerdict.TOOL_CLASS_MISMATCH

    # Point 5 after 8: expired with a billing mismatch gives the mismatch; expired alone is
    # EXPIRED and carries the claims; the last second before exp plus the leeway passes.
    old = issuer.token()
    await env.clock.advance(TTL + LEEWAY)
    got = await env.verdict(old, actual=Actual("personal", "scoped"))
    assert got is TokenVerdict.BILLING_MODE_MISMATCH
    expired = await env.verifier.check(old, env.full(), ACTUAL)
    assert expired.verdict is TokenVerdict.EXPIRED
    assert expired.claims is not None
    assert expired.claims.jti == payload_of(old.reveal())["jti"]
    env2 = Env()
    last = env2.issuer.token()
    await env2.clock.advance(TTL + LEEWAY - 1)
    assert await env2.verdict(last) is TokenVerdict.VALID

    # Point 7: the same valid token twice.
    tok = issuer.token()
    first = await env.verifier.check(tok, env.full(), ACTUAL)
    assert first.verdict is TokenVerdict.VALID
    assert first.claims is not None
    assert first.claims.run_id == UUID(issuer.grant["run_id"])
    assert first.claims.permission_version == 4
    assert first.claims.exp - first.claims.iat == TTL
    assert await env.verdict(tok) is TokenVerdict.REPLAY

    # A token without run passes against an expectation without run (autocomplete).
    command_id = uuid7()
    auto = issuer.token(
        trigger_kind="autocomplete", run_id=None, message_id=None, command_id=str(command_id)
    )
    expect = Expectation(
        thread_id=UUID(issuer.grant["thread_id"]),
        run_id=None,
        trigger_claim="command_id",
        trigger_id=command_id,
    )
    assert await env.verdict(auto, expect) is TokenVerdict.VALID


@pytest.mark.anyio
async def test_replay_ends_after_exp_plus_leeway() -> None:
    env = Env()
    tok = env.issuer.token()
    assert await env.verdict(tok) is TokenVerdict.VALID
    # The jti is remembered until exp plus the leeway; afterwards the token is expired anyway.
    await env.clock.advance(TTL + LEEWAY - 1)
    assert await env.verdict(tok) is TokenVerdict.REPLAY
    await env.clock.advance(1)
    assert await env.verdict(tok) is TokenVerdict.EXPIRED


def test_compare_actual_points_8_and_10() -> None:
    env = Env()
    claims = SimpleNamespace(billing_mode="organization", tool_class="scoped")
    assert compare_actual(claims, ACTUAL) is TokenVerdict.VALID
    assert compare_actual(claims, Actual("unknown", "chat")) is (TokenVerdict.BILLING_MODE_MISMATCH)
    assert compare_actual(claims, Actual("organization", "host")) is (
        TokenVerdict.TOOL_CLASS_MISMATCH
    )
    assert env.verifier.fetches == 0


@pytest.mark.anyio
async def test_jwks_cache_and_forced_reload_throttle() -> None:
    env = Env()
    v = env.verifier
    assert await env.verdict(env.issuer.token()) is TokenVerdict.VALID
    assert (v.fetches, v.forced_reloads) == (1, 0)
    await env.clock.advance(MAX_AGE - 1)
    assert await env.verdict(env.issuer.token()) is TokenVerdict.VALID
    assert v.fetches == 1
    await env.clock.advance(1)
    assert await env.verdict(env.issuer.token()) is TokenVerdict.VALID
    assert (v.fetches, v.forced_reloads) == (2, 0)

    # An unknown kid: exactly one forced reload, then UNKNOWN_KEY without claims.
    stranger = Ed25519PrivateKey.generate()
    payload = json.dumps(env.issuer.claims()).encode()
    unknown = sign_ed(header("unknown-1"), payload, stranger)
    check = await v.check(Secret(unknown), env.full(), ACTUAL)
    assert (check.verdict, check.claims) == (TokenVerdict.UNKNOWN_KEY, None)
    assert (v.fetches, v.forced_reloads) == (3, 1)
    # A second unknown kid within adapter.jwks_refresh_min_interval fetches nothing.
    await env.clock.advance(REFRESH - 1)
    other = sign_ed(header("unknown-2"), payload, stranger)
    assert await env.verdict(other) is TokenVerdict.UNKNOWN_KEY
    assert (v.fetches, v.forced_reloads) == (3, 1)
    # After the interval a kid published meanwhile is found by one forced reload.
    await env.clock.advance(1)
    env.issuer.add_key()
    assert await env.verdict(env.issuer.token()) is TokenVerdict.VALID
    assert (v.fetches, v.forced_reloads) == (4, 2)

    # refresh() fetches now and counts as a fetch, not as a forced reload.
    await v.refresh()
    assert (v.fetches, v.forced_reloads) == (5, 2)


@pytest.mark.anyio
async def test_jwks_without_max_age_fetched_on_next_use() -> None:
    env = Env(max_age=None)
    assert await env.verdict(env.issuer.token()) is TokenVerdict.VALID
    assert await env.verdict(env.issuer.token()) is TokenVerdict.VALID
    assert (env.verifier.fetches, env.verifier.forced_reloads) == (2, 0)


@pytest.mark.anyio
async def test_jwks_unusable_keys_skipped() -> None:
    env = Env()
    issuer = env.issuer
    hidden = issuer.add_key(active=False, published=False)
    x = b64(public_bytes(issuer.keys[hidden]))
    base = {"kty": "OKP", "crv": "Ed25519", "kid": hidden, "x": x}
    env.jwks.extra = [
        {**base, "kty": "EC"},
        {**base, "crv": "Ed448"},
        {**base, "alg": "ES256"},
        {**base, "use": "enc"},
        {**base, "x": x + "AA"},
        {**base, "x": x[:-1] + "="},
    ]
    tok = issuer.raw(kid=hidden)
    assert await env.verdict(tok) is TokenVerdict.UNKNOWN_KEY
    # The same key with alg and use left out is usable.
    env.jwks.extra = [base]
    await env.clock.advance(REFRESH)
    assert await env.verdict(issuer.raw(kid=hidden)) is TokenVerdict.VALID


def _payload(cls: Any, data: dict[str, Any]) -> SimpleNamespace:
    return SimpleNamespace(payload=cls.from_dict(data))


def _principal() -> dict[str, Any]:
    return {"id": str(uuid7()), "kind": "user", "handle": "alex", "display_name": "Alex"}


def test_expectation_rows() -> None:
    run, thread, channel, obj = uuid7(), uuid7(), uuid7(), uuid7()
    base = {
        "run_id": str(run),
        "thread_id": str(thread),
        "channel_id": str(channel),
        "triggered_by": _principal(),
        "billing_mode": "organization",
    }
    rows = {
        "message": ({"trigger_kind": "message", "message_id": str(obj)}, "message_id", obj),
        "interaction": (
            {"trigger_kind": "interaction", "interaction_id": str(obj)},
            "interaction_id",
            obj,
        ),
        "message without id": ({"trigger_kind": "message"}, "message_id", None),
    }
    for name, (extra, claim, trigger) in rows.items():
        event = _payload(AgentInvocationPayload, {**base, **extra})
        got = expectation_for("agent.invocation", event)
        assert got == Expectation(thread, run, claim, trigger), name
    # A trigger kind this SDK does not know has no counterpart: fail closed.
    event = _payload(AgentInvocationPayload, {**base, "trigger_kind": "steer"})
    got = expectation_for("agent.invocation", event)
    assert (got.thread_id, got.run_id, got.trigger_id) == (thread, run, None)

    followup = _payload(
        AgentFollowupPayload,
        {"run_id": str(run), "thread_id": str(thread), "message_id": str(obj)},
    )
    assert expectation_for("agent.followup", followup) == Expectation(
        thread, run, "message_id", obj
    )

    interaction = {
        "id": str(obj),
        "component_id": str(uuid7()),
        "message_id": str(uuid7()),
        "principal_id": str(uuid7()),
        "client_interaction_id": str(uuid7()),
        "values": {},
        "state": "dispatched",
        "created_at": "2026-10-20T09:00:00.000Z",
        "run_id": str(run),
        "thread_id": str(thread),
    }
    created = _payload(InteractionCreatedPayload, {"interaction": interaction})
    assert expectation_for("interaction.created", created) == Expectation(
        thread, run, "interaction_id", obj
    )

    command = {
        "invocation_id": str(uuid7()),
        "command_id": str(obj),
        "command_name": "status",
        "control": False,
        "channel_id": str(channel),
        "invoked_by": _principal(),
        "options": [],
    }
    invoked = _payload(CommandInvokedPayload, command)
    assert expectation_for("command.invoked", invoked) == Expectation(None, None, "command_id", obj)
    invoked = _payload(
        CommandInvokedPayload, {**command, "run_id": str(run), "thread_id": str(thread)}
    )
    assert expectation_for("command.invoked", invoked) == Expectation(
        thread, run, "command_id", obj
    )

    auto = {
        "request_id": str(uuid7()),
        "command_id": str(obj),
        "channel_id": str(channel),
        "user_id": str(uuid7()),
        "option": "name",
        "value": "",
        "expires_at": "2026-10-20T09:00:03.000Z",
        "thread_id": str(thread),
    }
    requested = _payload(CommandAutocompleteRequestedPayload, auto)
    assert expectation_for("command.autocomplete_requested", requested) == Expectation(
        thread, None, "command_id", obj
    )

    assert {
        "agent.invocation",
        "agent.followup",
        "interaction.created",
        "command.invoked",
        "command.autocomplete_requested",
    } == TOKEN_EVENT_TYPES
    with pytest.raises(ValueError, match=r"run\.cancel_requested"):
        expectation_for("run.cancel_requested", followup)
    with pytest.raises(TypeError):
        expectation_for("agent.followup", created)


@pytest.mark.anyio
async def test_command_trigger_without_payload_counterpart_invalid() -> None:
    env = Env()
    issuer = env.issuer
    command_invocation = uuid7()
    payload = AgentInvocationPayload.from_dict(
        {
            "run_id": issuer.grant["run_id"],
            "thread_id": issuer.grant["thread_id"],
            "channel_id": issuer.grant["channel_id"],
            "trigger_kind": "command",
            "triggered_by": _principal(),
            "billing_mode": "organization",
            "command_invocation_id": str(command_invocation),
        }
    )
    expect = expectation_for("agent.invocation", SimpleNamespace(payload=payload))
    assert (expect.trigger_claim, expect.trigger_id) == ("command_id", None)
    # Even a token whose command_id equals the payload's only command-like ID is refused.
    tok = issuer.token(trigger_kind="command", message_id=None, command_id=str(command_invocation))
    check = await env.verifier.check(tok, expect, ACTUAL)
    assert check.verdict is TokenVerdict.INVALID
    # Nothing was remembered: the same token against a complete expectation still passes.
    full = Expectation(expect.thread_id, expect.run_id, "command_id", command_invocation)
    assert await env.verdict(tok, full) is TokenVerdict.VALID


class Triggerers:
    """An in-test allow list loader that counts its calls."""

    def __init__(self, ids: list[UUID], version: int) -> None:
        self.ids = ids
        self.version = version
        self.calls = 0

    async def __call__(self) -> AgentTriggerers:
        self.calls += 1
        return AgentTriggerers(principal_ids=list(self.ids), permission_version=self.version)


@pytest.mark.anyio
async def test_allowlist_reload_and_refusal() -> None:
    alex, sam, robin = uuid7(), uuid7(), uuid7()
    loader = Triggerers([alex], 4)
    env = Env(allowlist=loader)
    issuer = env.issuer

    # Loaded on the first check; a listed sub passes.
    assert await env.verdict(issuer.token(sub=str(alex))) is TokenVerdict.VALID
    assert loader.calls == 1
    # The same permission_version: no reload; an unlisted sub is INVALID.
    assert await env.verdict(issuer.token(sub=str(sam))) is TokenVerdict.INVALID
    assert loader.calls == 1
    # A higher permission_version reloads first; the new list admits sam.
    loader.ids, loader.version = [alex, sam], 5
    assert await env.verdict(issuer.token(sub=str(sam), permission_version=5)) is (
        TokenVerdict.VALID
    )
    assert loader.calls == 2
    # Still missing after the reload: INVALID.
    loader.version = 6
    assert await env.verdict(issuer.token(sub=str(robin), permission_version=6)) is (
        TokenVerdict.INVALID
    )
    assert loader.calls == 3
    # access_changed() reloads before the next check, even without a higher version.
    loader.ids = [alex]
    env.verifier.access_changed()
    assert await env.verdict(issuer.token(sub=str(sam), permission_version=6)) is (
        TokenVerdict.INVALID
    )
    assert loader.calls == 4
    # The allow list comes after point 6 and before point 8: a refused sub with a billing
    # mismatch is INVALID, a token for another thread never loads the list.
    got = await env.verdict(issuer.token(sub=str(robin)), actual=Actual("personal", "scoped"))
    assert got is TokenVerdict.INVALID
    env.verifier.access_changed()
    wrong = dataclasses.replace(env.full(), thread_id=uuid7())
    assert await env.verdict(issuer.token(sub=str(alex)), wrong) is TokenVerdict.INVALID
    assert loader.calls == 4


@pytest.mark.anyio
async def test_no_token_text_in_repr_or_errors() -> None:
    env = Env()
    tok = env.issuer.token()
    check = await env.verifier.check(tok, env.full(), ACTUAL)
    text = repr(check) + str(check) + repr(env.verifier)
    for part in tok.reveal().split("."):
        assert part not in text
