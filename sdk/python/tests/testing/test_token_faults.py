# SPDX-License-Identifier: Apache-2.0
"""The forged token variants of the Fake-Agent (brief M2-31 Design note 5 e)."""

from __future__ import annotations

from datetime import UTC, datetime
from uuid import UUID

import pytest
from _tokens import Issuer, Jwks

from bothy_sdk.auth import Secret
from bothy_sdk.ids import uuid7
from bothy_sdk.inbox import MemoryInboxStore
from bothy_sdk.invocation import Actual, Expectation, InvocationVerifier, TokenVerdict
from bothy_sdk.settings import SdkSettings
from bothy_sdk.testing import FakeClock
from bothy_sdk.testing.token_faults import VARIANTS, forge

pytestmark = pytest.mark.anyio


@pytest.fixture
def anyio_backend() -> str:
    return "asyncio"


def _env() -> tuple[Issuer, InvocationVerifier]:
    clock = FakeClock(datetime(2030, 1, 1, 9, 0, tzinfo=UTC))
    issuer = Issuer(clock.now, uuid7(), uuid7(), 60)
    verifier = InvocationVerifier(
        instance_id=issuer.instance_id,
        agent_id=issuer.agent_id,
        jwks=Jwks(issuer, 1200),
        replay=MemoryInboxStore(),
        settings=SdkSettings(invocation_token_leeway=5),
        clock=clock,
    )
    return issuer, verifier


def _expect(issuer: Issuer) -> Expectation:
    g = issuer.grant
    return Expectation(UUID(g["thread_id"]), UUID(g["run_id"]), "message_id", UUID(g["message_id"]))


@pytest.mark.parametrize("variant", sorted(VARIANTS))
async def test_every_variant_is_refused_as_invalid(variant: str) -> None:
    issuer, verifier = _env()
    original = issuer.token()
    text = original.reveal()
    forged = forge(variant, original)
    check = await verifier.check(forged, _expect(issuer), Actual("organization", "scoped"))
    assert check.verdict is TokenVerdict.INVALID
    assert original.reveal() == text
    assert forged.reveal() != text


async def test_the_original_token_stays_valid() -> None:
    issuer, verifier = _env()
    token = issuer.token()
    for variant in VARIANTS:
        forge(variant, token)
    check = await verifier.check(token, _expect(issuer), Actual("organization", "scoped"))
    assert check.verdict is TokenVerdict.VALID


def test_unknown_variant_and_malformed_token() -> None:
    issuer, _ = _env()
    with pytest.raises(KeyError):
        forge("swap_kid", issuer.token())
    with pytest.raises(ValueError, match="compact JWS"):
        forge("bad_signature", Secret("not-a-token"))
