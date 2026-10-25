# SPDX-License-Identifier: Apache-2.0
r"""The test adapter of the portability test (docs/spec/16-operations.md 11.1 step 3, brief M7-14).

An agent adapter on the Python SDK whose runtime is the SDK's test double
(`bothy_sdk.testing.doubles.RecordingHandler`): no model runs (D-W-125). Every run it receives
answers with ``pong`` and ends; the SDK checks each invocation token, accepts the event into its
inbox before the acknowledgement and starts the run once (09 4.5 to 4.9).

The probe (``bothy-smoke portability``) creates the agent and its DPoP-bound token and writes
``agent.json``, ``token`` and ``dpop.pem`` into the adapter directory (the secrets with mode
``0600``). This program reads them, connects as the agent and writes ``ready`` into the same
directory once its first poll has returned, so the probe sends the trigger only after the consumer
exists (a new consumer starts at ``head_seq``, 04 15.1).

The adapter talks to the public URL of the installation (``https://chat.example.com``), which
the DPoP proofs must name, through the published port on 127.0.0.1: a transport of this file
sends every request to ``--origin`` instead, like the probe does with its base and public URL.
It uses REST polling (``pollAgentEvents``) instead of the gateway, so this one transport carries
every request of the adapter.

Logs go to stderr at level WARNING and never carry a token or content (SDK rule).

Usage:
  uv run --project sdk/python --locked python tests/portability/adapter.py \\
      --dir <adapter directory> --origin http://127.0.0.1:<port> --public-url https://chat.example.com
Exit codes: 0 stopped by SIGTERM or SIGINT, 1 the adapter stopped on its own, 2 bad input.
"""

from __future__ import annotations

import argparse
import asyncio
import json
import logging
import signal
import sys
from collections.abc import AsyncIterator
from pathlib import Path
from typing import Any
from urllib.parse import urlsplit
from uuid import UUID

import httpx
from bothy_sdk.adapter import Adapter, AdapterConfig, Invocation
from bothy_sdk.auth import Credentials, FileDpopKey, Secret
from bothy_sdk.runs import RunHandle
from bothy_sdk.settings import SdkSettings
from bothy_sdk.sources import CursorAdvance, Delivery, EventSource
from bothy_sdk.testing.doubles import RecordingHandler, RecordingSource

# The reply of every run; the probe waits for it (services/server/test/smoke AgentReply).
REPLY = "pong"
# Interval of pollAgentEvents after an empty page: short, so the run starts soon after the
# trigger; far below the rate limit of the agent runtime (sdk.poll_interval).
POLL_INTERVAL_S = 0.5


class EchoDouble(RecordingHandler):
    """The SDK's recording test double that answers every run with `REPLY`."""

    async def execute(self, inv: Invocation, run: RunHandle) -> None:
        """Record the call, then send the reply of the run."""
        await super().execute(inv, run)
        await run.send(REPLY)


class ReadySource(RecordingSource):
    """The SDK's recording source double that writes `ready_file` after the first poll.

    The ready event of the source comes before its first ``pollAgentEvents``, and only that poll
    creates the consumer, which starts at ``head_seq`` (04 15.1): a trigger sent between the two
    would never reach this consumer. The first cursor of a poll proves the consumer exists.

    Args:
        inner: The polling source of the adapter.
        ready_file: The file the probe waits for.
    """

    def __init__(self, inner: EventSource, ready_file: Path) -> None:
        """Wrap ``inner``; see the class docstring."""
        super().__init__(inner)
        self.ready_file = ready_file

    async def __aiter__(self) -> AsyncIterator[Delivery]:
        """Yield the deliveries; write the ready file after the first poll's cursor."""
        async for delivery in super().__aiter__():
            yield delivery
            if isinstance(delivery, CursorAdvance) and not self.ready_file.exists():
                self.ready_file.write_text("ready\n", encoding="utf-8")


class OriginTransport(httpx.AsyncBaseTransport):
    """Sends every request to `origin` and keeps its path and query.

    The SDK signs each request for its URL on the public address before the transport sees it;
    only the destination changes, as when a reverse proxy forwards the public address.

    Args:
        origin: Scheme, host and port the requests go to (a loopback address).
    """

    def __init__(self, origin: str) -> None:
        """Create the transport; see the class docstring."""
        parts = urlsplit(origin)
        if parts.scheme not in ("http", "https") or not parts.hostname:
            raise ValueError("origin must be an http URL with a host")
        self.scheme, self.host, self.port = parts.scheme, parts.hostname, parts.port
        self.inner = httpx.AsyncHTTPTransport()

    async def handle_async_request(self, request: httpx.Request) -> httpx.Response:
        """Forward the request to the origin."""
        body = await request.aread()
        url = request.url.copy_with(scheme=self.scheme, host=self.host, port=self.port)
        # Host follows the destination, as the probe sends it (the DPoP proof names the public URL).
        headers = [(k, v) for k, v in request.headers.multi_items() if k.lower() != "host"]
        forwarded = httpx.Request(request.method, url, headers=headers, content=body)
        return await self.inner.handle_async_request(forwarded)

    async def aclose(self) -> None:
        """Close the inner transport."""
        await self.inner.aclose()


def load_config(directory: Path, public_url: str) -> AdapterConfig:
    """Build the adapter configuration from the files of the probe.

    Args:
        directory: The adapter directory with agent.json, token and dpop.pem.
        public_url: The public URL of the installation.

    Returns:
        The configuration with polling and a state directory below `directory`.

    Raises:
        ValueError: A file is missing or malformed.
        PermissionError: A secret file is readable by others.
    """
    agent: dict[str, Any] = json.loads((directory / "agent.json").read_text(encoding="utf-8"))
    state = directory / "state"
    state.mkdir(mode=0o700, exist_ok=True)
    return AdapterConfig(
        base_url=public_url.rstrip("/") + "/api/v1",
        instance_id=UUID(agent["instance_id"]),
        agent_id=UUID(agent["agent_id"]),
        credentials=Credentials(
            Secret.from_file(directory / "token"), FileDpopKey.load(directory / "dpop.pem")
        ),
        consumer="portability",
        transport="polling",
        state_dir=state,
        adapter_name="bothy-portability-adapter",
        runtime_name="sdk-test-double",
        settings=SdkSettings(poll_interval=POLL_INTERVAL_S),
    )


async def serve(config: AdapterConfig, handler: EchoDouble, origin: str, ready: Path) -> int:
    """Run the adapter until SIGTERM or SIGINT; return the exit code."""
    adapter = Adapter(config, handler, http_transport=OriginTransport(origin))
    adapter.source = ReadySource(await adapter.make_source(), ready)
    loop = asyncio.get_running_loop()
    stopped = asyncio.Event()
    stopping: set[asyncio.Task[None]] = set()

    def stop() -> None:
        stopped.set()
        stopping.add(loop.create_task(adapter.stop()))

    for sig in (signal.SIGTERM, signal.SIGINT):
        loop.add_signal_handler(sig, stop)
    reason = await adapter.run()
    if stopped.is_set():
        return 0
    print(f"adapter: stopped on its own ({reason})", file=sys.stderr)
    return 1


def main(argv: list[str] | None = None) -> int:
    """Command line entry point; see the module documentation."""
    parser = argparse.ArgumentParser(description="Test adapter of the portability test.")
    parser.add_argument("--dir", required=True, type=Path)
    parser.add_argument("--origin", required=True)
    parser.add_argument("--public-url", required=True)
    args = parser.parse_args(argv)
    logging.basicConfig(level=logging.WARNING, stream=sys.stderr)
    try:
        config = load_config(args.dir, args.public_url)
        OriginTransport(args.origin)
    except (OSError, ValueError, KeyError) as exc:
        print(f"adapter: {type(exc).__name__}", file=sys.stderr)
        return 2
    return asyncio.run(serve(config, EchoDouble(), args.origin, args.dir / "ready"))


if __name__ == "__main__":
    sys.exit(main())
