# SPDX-License-Identifier: Apache-2.0
"""Test driver of the Fake-Agent against the FakeServer: requests with the FakeServer's clock as
``now`` (brief M2-31, Tests "Unit, Python")."""

from __future__ import annotations

import base64
from typing import Any

from bothy_sdk.testing.control import Request
from bothy_sdk.testing.fake_agent import FakeAgent
from bothy_sdk.testing.fakeserver import FakeServer


def _b64(n: int) -> str:
    return base64.urlsafe_b64encode(n.to_bytes(32, "big")).rstrip(b"=").decode()


def private_jwk(fake: FakeServer) -> dict[str, str]:
    """The private JWK of the FakeServer's DPoP key (what the conductor reads from the harness)."""
    key = fake.credentials.dpop._key  # type: ignore[union-attr]
    numbers = key.private_numbers()
    pub = numbers.public_numbers
    return {
        "kty": "EC",
        "crv": "P-256",
        "x": _b64(pub.x),
        "y": _b64(pub.y),
        "d": _b64(numbers.private_value),
    }


class Driver:
    """Sends requests to one FakeAgent with the FakeServer's clock as ``now``."""

    def __init__(self, fake: FakeServer) -> None:
        self.fake = fake
        self.agent = FakeAgent()
        self.n = 0

    async def op(self, op: str, **args: Any) -> dict[str, Any]:
        self.n += 1
        reply = await self.agent.handle(Request(self.n, op, args, self.fake.clock.now()))
        if not reply.ok:
            return {
                "error": reply.error_kind.value if reply.error_kind else "",
                "detail": reply.error_detail,
            }
        return reply.result

    async def hello(
        self, *, runtime: dict[str, Any] | None = None, token: str | None = None
    ) -> None:
        rt = {"billing_mode": "organization", "tool_class": "scoped", "model": "model-test"}
        rt.update(runtime or {})
        result = await self.op(
            "hello",
            base_url=self.fake.base_url,
            gateway_url="/api/v1/gateway",
            instance_id=str(self.fake.instance_id),
            agent_id=str(self.fake.agent_id),
            consumer="default",
            transport="rest",
            credentials={
                "token": token or self.fake.credentials.api_token.reveal(),
                "dpop_private_jwk": private_jwk(self.fake),
            },
            runtime=rt,
            settings={
                "sdk.retry_backoff_base": "10ms",
                "sdk.retry_backoff_cap": "50ms",
                "adapter.invocation_token_leeway": "5s",
            },
        )
        assert result == {}, result
        assert "head_seq" in await self.op("connect")

    async def close(self) -> None:
        await self.agent.close()
