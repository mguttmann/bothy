# Connect your own agent

This guide connects an agent of your own to an instance through the public API, in the ten steps
of [chapter 11 section 13](../spec/11-extensions.md#13-guide-connecting-your-own-agent-in-ten-steps).
It is written for a developer who builds an adapter: the program between the instance and an
agent runtime. The adapter uses only the public API through the Python SDK; there is no private
channel for agents of the project itself ([chapter 11 section 1](../spec/11-extensions.md#1-principles)).

The example adapter lives in `sdk/python/examples/guide/`. Every file of it appears on this page,
and every request, command and output line below is executed by a test against a fresh server
(see "How the repository checks this guide"). Its runtime is a small set of fixed rules instead
of a model, so following the guide costs nothing.

## Placeholders

Values in braces stand for what your instance and your adapter use. A placeholder captured from
an answer or an output line (for example `{agent_id}`) means the value shown there; later blocks
use it.

| Placeholder | Meaning |
|---|---|
| `{server_url}` | The address of the instance, for example `https://chat.example.com`. |
| `{adapter_dir}` | A folder that only the adapter's user may read (`mkdir -m 700`). The key, the token and the adapter's state live there. |
| `{alex_id}` | The user ID of Alex, a person with the level `trigger` and the capabilities `approve_actions` and `upload` in the space Support. |
| `{sam_id}` | The user ID of Sam, a person with the level `write` in the space Support. |
| `{new_uuid}` | A new UUIDv7 at each use, for example for `client_msg_id`. |
| `{expires_at}` | The expiry of the agent token, an RFC 3339 time in UTC within `auth.api_token_max_ttl`. |

Requests are shown as HTTP. The client adds the authentication of the caller (a DPoP-bound
session token), the DPoP proof and an `Idempotency-Key` where the API asks for one. The owner's
requests are administrative: the owner confirms them with a fresh step-up, as the admin screens
of the app and the console do before an admin action. Those screens call the same operations, so
the owner can follow every owner step there as well.

## Prerequisites

An instance with an owner (see [the installation with Compose](../operations/install-compose.md))
and two people, Alex and Sam. The owner creates the space Support with the forum channel
Tickets and gives Alex the level `trigger` with the capabilities `approve_actions` and `upload`
(Alex may start runs, approve actions and share files) and Sam the level `write` (Sam may post,
but not start a run).

```http title="Owner: createSpace"
POST /api/v1/spaces HTTP/1.1
Content-Type: application/json

{"name": "Support"}
```

```json title="Response 201"
{"id": "{space_id}", "name": "Support"}
```

```http title="Owner: createChannel"
POST /api/v1/spaces/{space_id}/channels HTTP/1.1
Content-Type: application/json

{"kind": "forum", "name": "Tickets"}
```

```json title="Response 201"
{"id": "{channel_id}", "kind": "forum", "name": "Tickets"}
```

```http title="Owner: createGrant"
POST /api/v1/grants HTTP/1.1
Content-Type: application/json

{
  "subject_kind": "user",
  "subject_id": "{alex_id}",
  "target_kind": "space",
  "target_id": "{space_id}",
  "level": "trigger",
  "capabilities": ["approve_actions", "upload"],
  "reason": "Alex works the support tickets"
}
```

```http title="Owner: createGrant"
POST /api/v1/grants HTTP/1.1
Content-Type: application/json

{
  "subject_kind": "user",
  "subject_id": "{sam_id}",
  "target_kind": "space",
  "target_id": "{space_id}",
  "level": "write",
  "reason": "Sam reports tickets"
}
```

The owner reads the instance ID once and hands it to the developer, who configures it in the
adapter (step 4).

```http title="Owner: getDiscovery"
GET /api/v1/instance/discovery HTTP/1.1
```

```json title="Response 200"
{"instance_id": "{instance_id}"}
```

You need [uv](https://docs.astral.sh/uv/) and a checkout of the repository; every command below
runs in its root.

## Step 1: Create the agent

The owner creates the agent: a principal of kind agent with a handle, a display name and the
`adapter_kind` of the adapter. An own adapter is `custom`.

```http title="Owner: createAgent"
POST /api/v1/agents HTTP/1.1
Content-Type: application/json

{
  "handle": "lotse",
  "display_name": "Lotse",
  "adapter_kind": "custom",
  "description": "Answers support tickets"
}
```

```json title="Response 201"
{
  "id": "{agent_id}",
  "handle": "lotse",
  "adapter_kind": "custom",
  "billing_mode": "organization",
  "tool_class": "chat"
}
```

The backend policy starts as `organization` with the tool class `chat`: the runs bill the
organization, and the runtime loads no tools beyond chat. `personal` needs `setAgentBillingMode`
with a credential holder and is bound by rule I1 of
[chapter 10](../spec/10-backend-policy.md).

## Step 2: Create a token

The agent token is bound to a key of the adapter (DPoP): a copied token is worthless without the
key. The binding is required while `agent` is in `auth.dpop_required_for`, which is the default
([chapter 08 section 12](../spec/08-identity.md)). First the adapter creates its key:

```python title="sdk/python/examples/guide/keygen.py"
# SPDX-License-Identifier: Apache-2.0
"""Step 2: create the adapter's key and print its thumbprint for the owner.

The owner binds the agent token to this key (``dpop_jkt``), so a copied token is worthless
without the key file. The key never leaves the adapter's folder.
"""

from __future__ import annotations

import argparse
from pathlib import Path

from connect import KEY_FILE

from bothy_sdk.auth import FileDpopKey


def main(argv: list[str] | None = None) -> None:
    """Write ``adapter.key`` (mode ``0600``, never overwritten) and print ``dpop_jkt``.

    Args:
        argv: The command line without the program name.
    """
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--adapter-dir", type=Path, required=True)
    args = parser.parse_args(argv)
    args.adapter_dir.mkdir(mode=0o700, parents=True, exist_ok=True)
    try:
        key = FileDpopKey.generate(args.adapter_dir / KEY_FILE)
    except FileExistsError:
        parser.exit(1, f"{args.adapter_dir / KEY_FILE} exists already; keeping it\n")
    print(f"dpop_jkt: {key.jkt}", flush=True)


if __name__ == "__main__":
    main()
```

On the first `uv run` uv installs the locked SDK; step 4 explains the installation.

```bash title="Shell: create the adapter key"
PYTHONPATH=sdk/python/src uv run --project sdk/python --locked python sdk/python/examples/guide/keygen.py --adapter-dir {adapter_dir}
```

```text title="Output"
dpop_jkt: {dpop_jkt}
```

The owner creates the token with this thumbprint. The answer shows the token once; store it in
`{adapter_dir}/agent.token` with mode `0600` and nowhere else, never in a command line, an
environment variable of a shared shell or a log.

```http title="Owner: createToken"
POST /api/v1/principals/{agent_id}/tokens HTTP/1.1
Content-Type: application/json

{"name": "Guide adapter", "dpop_jkt": "{dpop_jkt}", "expires_at": "{expires_at}"}
```

```json title="Response 201"
{"secret": "{secret:agent.token}", "token": {"name": "Guide adapter", "dpop_jkt": "{dpop_jkt}"}}
```

## Step 3: Grants and operator

The agent needs `write` on the places where it answers; the capability `upload` lets it send
files. The owner grants both on the channel:

```http title="Owner: createGrant"
POST /api/v1/grants HTTP/1.1
Content-Type: application/json

{
  "subject_kind": "agent",
  "subject_id": "{agent_id}",
  "target_kind": "channel",
  "target_id": "{channel_id}",
  "level": "write",
  "capabilities": ["upload"],
  "reason": "Lotse answers in Tickets"
}
```

Then the owner makes the agent the operator of the channel: the agent that answers there by
default. The operator has `write` without a grant ([chapter 09 section 2.3](../spec/09-agents.md));
the grant above adds `upload`. The change needs the channel's version in `If-Match`
([chapter 06 section 8](../spec/06-api.md)):

```http title="Owner: getChannel"
GET /api/v1/channels/{channel_id} HTTP/1.1
```

```json title="Response 200"
{"channel": {"id": "{channel_id}", "version": "{channel_version}"}}
```

```http title="Owner: setChannelOperator"
PUT /api/v1/channels/{channel_id}/operator HTTP/1.1
Content-Type: application/json
If-Match: "{channel_version}"

{"mode": "agent", "agent_id": "{agent_id}", "reason": "Lotse answers the tickets"}
```

```json title="Response 200"
{"id": "{channel_id}", "operator": {"agent_id": "{agent_id}"}}
```

The channel switches decide when the operator answers ([chapter 09 section 3.3](../spec/09-agents.md)).
In a forum every post is a thread; with `thread_require_mention` off, the agent answers in a
post without being mentioned. `PUT` replaces all switches, so the owner reads them first:

```http title="Owner: getChannelBehavior"
GET /api/v1/channels/{channel_id}/behavior HTTP/1.1
```

```json title="Response 200"
{"settings_version": "{settings_version}", "history_backfill_limit": "{history_backfill_limit}"}
```

```http title="Owner: putChannelBehavior"
PUT /api/v1/channels/{channel_id}/behavior HTTP/1.1
Content-Type: application/json
If-Match: "{settings_version}"

{
  "require_mention": true,
  "thread_require_mention": false,
  "free_response": false,
  "auto_thread": true,
  "allow_mentioned_agents": true,
  "history_backfill_limit": "{history_backfill_limit}",
  "prompt": "Answer support tickets politely.",
  "skills": []
}
```

```json title="Response 200"
{"thread_require_mention": false}
```

## Step 4: Install the SDK and connect

Until the SDK packages are published, the SDK comes from a checkout of this repository: uv
installs the locked environment of `sdk/python`, and `PYTHONPATH=sdk/python/src` in front of
each command makes the SDK importable from the checkout.

```bash title="Shell: install the SDK"
uv sync --project sdk/python --locked --quiet
```

The adapter is one program in small modules, one per step of this guide. `agent.py` reads the
command line and connects the SDK's adapter with the handler of the example; `connect.py`
configures the gateway connection with a consumer. The SDK reconnects with backoff and
acknowledges an event (`ack`) only after it stored the event with its token in the inbox.

```python title="sdk/python/examples/guide/agent.py"
# SPDX-License-Identifier: Apache-2.0
"""Example adapter of the guide "Connect your own agent" (docs/guides/connect-agent.md).

Start it from the repository root with the values the owner handed over; step 4 of the guide
shows the command. It prints one line per action the server confirmed, ``Step <n>: <text>``,
where ``<n>`` is the step of the guide that explains it, and never a message text, a prompt, a
token or a key. The SDK logs to stderr under the same rule. Ctrl+C ends it cleanly.
"""

from __future__ import annotations

import argparse
import asyncio
import logging
import signal
import sys
from collections.abc import Sequence
from pathlib import Path
from uuid import UUID

import commands
import connect
import heartbeat
import interactive
import invocation
import run as run_step
import runtime
from connect import report

from bothy_sdk.adapter import (
    ActualBilling,
    Adapter,
    AgentHandler,
    Invocation,
    RuntimeStatus,
    StartOptions,
)
from bothy_sdk.commands import AutocompleteRequest, ReconcileResult
from bothy_sdk.generated.models import CommandChoice, CommandOptionValue
from bothy_sdk.runs import RunHandle


class GuideAgent(AgentHandler):
    """The runtime side of the example; each method hands over to the module of its step.

    Args:
        stop_after_accept: Stop the process right after the first accepted run (step 10).
    """

    def __init__(self, *, stop_after_accept: bool = False) -> None:
        """Create the handler; `attach` connects it to its adapter."""
        self.stop_after_accept = stop_after_accept
        self.adapter: Adapter | None = None
        self.recovered: set[UUID] = set()
        self.runs: dict[UUID, runtime.Work] = {}
        self.reported = False
        self.ready = asyncio.Event()  # the start lines of steps 4 to 6 are printed

    def attach(self, adapter: Adapter, recovered: set[UUID]) -> None:
        """Remember the adapter and the runs a restart takes over from the inbox.

        Args:
            adapter: The adapter that runs this handler.
            recovered: Runs accepted by an earlier process but not started.
        """
        self.adapter = adapter
        self.recovered = recovered

    # Steps 4 to 6: the start lines, once per process.

    async def on_ready(self) -> None:
        """Print the connection and send the first heartbeat (steps 4 and 5)."""
        if self.reported or self.adapter is None:
            return
        await connect.announce(self.adapter)
        await heartbeat.report_once(self.adapter)
        self.reported = True

    def runtime_status(self) -> RuntimeStatus:
        """Return the billing part of every heartbeat (step 5)."""
        return heartbeat.STATUS

    async def on_commands_reconciled(self, result: ReconcileResult) -> None:
        """Print the registered commands once (step 6)."""
        if self.reported and not self.ready.is_set():
            report(6, "commands registered: " + ", ".join(c.name for c in commands.COMMANDS))
            self.ready.set()

    async def on_autocomplete(self, request: AutocompleteRequest) -> Sequence[CommandChoice]:
        """Suggest topics from the fixed list (step 6)."""
        return commands.suggest(request)

    async def on_autocomplete_sent(
        self, request: AutocompleteRequest, choices: Sequence[CommandChoice]
    ) -> None:
        """Print that the suggestions went out (step 6)."""
        report(6, f"sent {len(choices)} suggestion(s) for option {request.option}")

    # Step 7: accept, compare, load the context.

    def actual_billing(self, inv: Invocation) -> ActualBilling:
        """Return what the runtime really uses (step 7)."""
        return invocation.actual_billing()

    async def prepare(self, inv: Invocation, run: RunHandle) -> StartOptions:
        """Check the run against the runtime and load its context; no runtime yet (step 7)."""
        await self.ready.wait()
        if run.run_id in self.recovered:
            report(10, f"resuming run {run.run_id} from the inbox")
        else:
            mode = invocation.actual_billing().mode
            report(7, f"accepted run {run.run_id}: token checked, actual mode {mode}")
        if self.stop_after_accept and self.adapter is not None:
            report(10, f"stopped after accepting run {run.run_id}")
            await self.adapter.stop()
            await asyncio.Event().wait()  # cancelled by the stop; the run stays in the inbox
        invocation.check_actual(inv)
        command = getattr(inv.payload, "command_name", None)
        if inv.claims.trigger_kind == "command" and isinstance(command, str):
            # A model command has no triggering message, and the server defines no context for
            # it yet (09 4.6): the runtime works from the command and its options.
            report(7, f"run {run.run_id} comes from the command {command}")
            topic = next((str(o.value) for o in _options(inv) if o.name == "topic"), None)
            self.runs[run.run_id] = runtime.Work(runtime.decide("", command), topic=topic)
            return StartOptions(model=runtime.MODEL)
        rendered, marks = await invocation.load_context(run)
        report(7, f"context of run {run.run_id}: " + ", ".join(marks))
        rule = runtime.decide(rendered.trigger)
        self.runs[run.run_id] = runtime.Work(rule, channel_id=inv.claims.channel_id)
        return StartOptions(model=runtime.MODEL)

    # Steps 8 and 9: run, become interactive.

    async def execute(self, inv: Invocation, run: RunHandle) -> None:
        """Run the started run (step 8), interactive parts in step 9."""
        work = self.runs[run.run_id]
        report(8, f"started run {run.run_id}")
        await run_step.show_progress(run, "Reading the thread")
        if work.rule in interactive.RULES:
            await interactive.begin(work, run)
        elif work.rule is runtime.Rule.SUMMARIZE:
            await run_step.answer(work, run, runtime.summary(work.topic))
        else:
            await run_step.answer(work, run, runtime.ANSWER)

    async def on_followup(self, inv: Invocation, run: RunHandle) -> None:
        """A follow-up or the answer to a question (step 9)."""
        work = self.runs.get(run.run_id)
        if work is not None:
            await interactive.on_followup(work, inv, run)

    async def on_interaction(self, inv: Invocation, run: RunHandle) -> None:
        """A click on the approval card (step 9)."""
        work = self.runs.get(run.run_id)
        if work is not None:
            await interactive.on_interaction(work, inv, run)

    async def on_command(self, inv: Invocation, run: RunHandle | None) -> None:
        """A control command (step 9)."""
        await interactive.on_command(inv, run)

    async def on_cancel(self, run: RunHandle, reason: str) -> None:
        """A stop request for a started run (step 9)."""
        work = self.runs.get(run.run_id)
        if work is not None and work.rule is runtime.Rule.LONG:
            interactive.stop(work, reason)
            return
        await run.finish("cancelled", reason=reason)
        report(9, f"stopped run {run.run_id}: {reason}")


def _options(inv: Invocation) -> list[CommandOptionValue]:
    options = getattr(inv.payload, "options", None)
    return [o for o in options if isinstance(o, CommandOptionValue)] if options else []


def parse(argv: Sequence[str] | None = None) -> argparse.Namespace:
    """Read the command line.

    Args:
        argv: The arguments without the program name.

    Returns:
        The parsed arguments.
    """
    parser = argparse.ArgumentParser(description="Example adapter of the guide.")
    parser.add_argument("--server-url", required=True)
    parser.add_argument("--instance-id", type=UUID, required=True)
    parser.add_argument("--agent-id", type=UUID, required=True)
    parser.add_argument("--adapter-dir", type=Path, required=True)
    parser.add_argument("--consumer", default="default")
    parser.add_argument("--stop-after", choices=["accept"])
    return parser.parse_args(argv)


async def serve(args: argparse.Namespace) -> int:
    """Run the adapter until Ctrl+C or, with ``--stop-after accept``, the first acceptance.

    Args:
        args: The parsed command line.

    Returns:
        The exit code: 0 after a clean stop.
    """
    config = connect.adapter_config(
        server_url=args.server_url,
        instance_id=args.instance_id,
        agent_id=args.agent_id,
        adapter_dir=args.adapter_dir,
        consumer=args.consumer,
        capabilities=heartbeat.CAPABILITIES,
        commands=commands.COMMANDS,
    )
    handler = GuideAgent(stop_after_accept=args.stop_after == "accept")
    adapter = Adapter(config, handler)
    handler.attach(adapter, await connect.accepted_runs(adapter))
    interrupted = asyncio.Event()
    stopping: set[asyncio.Task[None]] = set()

    def on_sigint() -> None:
        interrupted.set()
        task = asyncio.ensure_future(adapter.stop())
        stopping.add(task)
        task.add_done_callback(stopping.discard)

    loop = asyncio.get_running_loop()
    loop.add_signal_handler(signal.SIGINT, on_sigint)
    reason = await adapter.run()
    # A Ctrl+C can arrive twice (the terminal and uv both pass it on); the adapter has stopped,
    # so a late one must not interrupt the exit.
    loop.remove_signal_handler(signal.SIGINT)
    signal.signal(signal.SIGINT, signal.SIG_IGN)
    if interrupted.is_set():
        report(10, "stopped")
        return 0
    if handler.stop_after_accept and handler.ready.is_set():
        return 0
    print(f"adapter stopped: close code {reason.code}", file=sys.stderr)
    return 1


def main() -> None:
    """Entry point."""
    logging.basicConfig(
        level=logging.WARNING, stream=sys.stderr, format="%(levelname)s %(message)s"
    )
    sys.exit(asyncio.run(serve(parse())))


if __name__ == "__main__":
    main()
```

```python title="sdk/python/examples/guide/connect.py"
# SPDX-License-Identifier: Apache-2.0
"""Step 4: connect to the gateway as a consumer.

The SDK holds the connection: it identifies with the agent token and a DPoP proof of the
adapter key, reconnects with backoff, stores every event with its token in the inbox before it
acknowledges it (``ack``) and keeps the consumer's cursor on the server. The instance ID and the
agent ID come from the owner once; the SDK checks every invocation token against them.
"""

from __future__ import annotations

import stat
from collections.abc import Mapping, Sequence
from pathlib import Path
from typing import Any
from uuid import UUID

from bothy_sdk.adapter import Adapter, AdapterConfig
from bothy_sdk.auth import Credentials, FileDpopKey, Secret
from bothy_sdk.commands import CommandSpec
from bothy_sdk.generated.api.me import get_me
from bothy_sdk.generated.models import Me
from bothy_sdk.inbox import EntryState

ADAPTER_NAME = "guide-example"
ADAPTER_VERSION = "1.0.0"
TOKEN_FILE = "agent.token"  # noqa: S105  # nosec B105 - a file name, not a secret
KEY_FILE = "adapter.key"


def report(step: int, text: str) -> None:
    """Print one confirmed action as ``Step <n>: <text>``.

    Never pass a message text, a prompt, a token or a person's name: stdout is a log.

    Args:
        step: The step of the guide that explains the action.
        text: What happened.
    """
    print(f"Step {step}: {text}", flush=True)


def credentials(adapter_dir: Path) -> Credentials:
    """Load the agent token and the adapter key, refusing files others can read.

    Args:
        adapter_dir: The adapter's folder.

    Returns:
        The token bound to the key (DPoP).

    Raises:
        SystemExit: A file is readable or writable by group or others.
    """
    for name in (TOKEN_FILE, KEY_FILE):
        path = adapter_dir / name
        if path.stat().st_mode & (stat.S_IRWXG | stat.S_IRWXO):
            raise SystemExit(f"{path} must be accessible by the adapter only (chmod 600 {path})")
    return Credentials(
        Secret.from_file(adapter_dir / TOKEN_FILE), FileDpopKey.load(adapter_dir / KEY_FILE)
    )


def adapter_config(  # noqa: PLR0913 - the settings of the command line
    *,
    server_url: str,
    instance_id: UUID,
    agent_id: UUID,
    adapter_dir: Path,
    consumer: str,
    capabilities: Mapping[str, Any],
    commands: Sequence[CommandSpec],
) -> AdapterConfig:
    """Configure the SDK's adapter for the gateway.

    Args:
        server_url: The address of the instance.
        instance_id: The instance ID from the owner.
        agent_id: The agent ID from the owner.
        adapter_dir: The adapter's folder; ``state/`` keeps the inbox and the outbox.
        consumer: The consumer name; each consumer has its own cursor.
        capabilities: The heartbeat capabilities (step 5).
        commands: The declared commands (step 6).

    Returns:
        The configuration.
    """
    state = adapter_dir / "state"
    state.mkdir(mode=0o700, exist_ok=True)
    return AdapterConfig(
        base_url=f"{server_url.rstrip('/')}/api/v1",
        instance_id=instance_id,
        agent_id=agent_id,
        credentials=credentials(adapter_dir),
        consumer=consumer,
        transport="gateway",
        state_dir=state,
        adapter_name=ADAPTER_NAME,
        adapter_version=ADAPTER_VERSION,
        runtime_name="guide-rules",
        capabilities=capabilities,
        commands=commands,
    )


async def accepted_runs(adapter: Adapter) -> set[UUID]:
    """Return the runs the inbox holds accepted but not started (a restart resumes them).

    Args:
        adapter: The adapter before `Adapter.run`.

    Returns:
        Their run IDs.
    """
    entries = await adapter.inbox.in_states(EntryState.ACCEPTED)
    return {e.run_id for e in entries if e.run_id is not None}


async def announce(adapter: Adapter) -> None:
    """Print that the adapter is connected, with the agent's handle (``getMe``).

    Args:
        adapter: The connected adapter.
    """
    with adapter.client.unbound():
        me = await adapter.client.call_as(Me, get_me.asyncio_detailed)
    report(4, f"connected as {me.principal.handle} with consumer {adapter.config.consumer}")
```

For TypeScript the SDK `@bothy/sdk` offers the same modules; see
[its README](../../sdk/typescript/README.md) and the reference linked there.

The developer takes the instance ID and the agent ID once from the owner and passes them to the
adapter, never read from the server at start: the SDK checks `iss` and `aud` of every invocation
token against them ([chapter 09 section 4.5](../spec/09-agents.md), point 4). Start the adapter
in its own terminal:

```bash title="Adapter: start"
PYTHONPATH=sdk/python/src uv run --project sdk/python --locked python sdk/python/examples/guide/agent.py --server-url {server_url} --instance-id {instance_id} --agent-id {agent_id} --adapter-dir {adapter_dir} --consumer default
```

```text title="Output"
Step 4: connected as lotse with consumer default
```

## Step 5: Report adapter features

The heartbeat reports what the adapter can do (`capabilities`), the billing mode its runtime
really uses and where the runtime's key comes from, never the key itself.

```python title="sdk/python/examples/guide/heartbeat.py"
# SPDX-License-Identifier: Apache-2.0
"""Step 5: report what the adapter can do and how it bills.

The SDK sends this report with every gateway heartbeat. Right after connecting, the example
sends it once with ``sendHeartbeat`` as well, because that answer confirms the report and shows
the presence the server derived from it.
"""

from __future__ import annotations

from connect import report

from bothy_sdk.adapter import Adapter, RuntimeStatus
from bothy_sdk.generated.api.agent_runtime import send_heartbeat
from bothy_sdk.generated.models import HeartbeatAck

CAPABILITIES = {"followups": "steer", "components": True, "files_out": True}
"""What the adapter handles (09 2.2): follow-ups steer the running run, it sends cards and files."""

STATUS = RuntimeStatus(
    state="online",
    reported_billing_mode="organization",
    billing_modes_supported=("organization",),
    # Where the runtime's key comes from, never the key itself: the example uses none.
    billing_evidence={"key_type": "none"},
)
"""The billing part of every heartbeat."""


async def report_once(adapter: Adapter) -> None:
    """Send the heartbeat now and print the presence the server answered.

    Args:
        adapter: The connected adapter.
    """
    with adapter.client.unbound():
        ack = await adapter.client.call_as(
            HeartbeatAck, send_heartbeat.asyncio_detailed, body=adapter.heartbeat()
        )
    presence = ack.presence
    report(
        5,
        f"heartbeat accepted: {presence.state}, mode {presence.reported_billing_mode}, "
        f"tool class {presence.reported_tool_class}",
    )
```

The SDK sends this report with every gateway heartbeat; right after connecting, the example sends
it once with `sendHeartbeat` and prints the presence the server answered.

```text title="Output"
Step 5: heartbeat accepted: online, mode organization, tool class chat
```

The owner sees the presence in the agent:

```http title="Owner: getAgent"
GET /api/v1/agents/{agent_id} HTTP/1.1
```

```json title="Response 200"
{"id": "{agent_id}", "presence": {"state": "online", "reported_tool_class": "chat"}}
```

## Step 6: Register commands

A control command (`version`) is answered by the adapter at once, also during a run, and never
reaches the runtime. A model command (`summarize`) starts a run like a message. Its option
`topic` offers suggestions while a person types.

```python title="sdk/python/examples/guide/commands.py"
# SPDX-License-Identifier: Apache-2.0
"""Step 6: the adapter's slash commands.

The SDK makes the agent's registered commands equal to this list after every connect: it lists
the commands in the channels of ``listServedChannels`` and registers, updates or removes the
agent's own ones. Suggestions come from a fixed list: typing must never cost a model call.
"""

from __future__ import annotations

from bothy_sdk.commands import AutocompleteRequest, CommandSpec
from bothy_sdk.generated.models import CommandChoice, CommandOption, CommandOptionType

TOPICS = ("billing", "login", "shipping")
"""The suggestions for the option ``topic``."""

COMMANDS = (
    CommandSpec(
        "summarize",
        "Summarize the thread",
        options=(
            CommandOption(
                name="topic",
                description="What the summary is about",
                type_=CommandOptionType.STRING,
                required=False,
                autocomplete=True,
            ),
        ),
    ),
    CommandSpec("version", "Show the adapter version", control=True),
)
"""``summarize`` starts a run (model command); ``version`` is answered at once (control)."""


def suggest(request: AutocompleteRequest) -> list[CommandChoice]:
    """Return the topics that start with what the person typed.

    Args:
        request: The autocomplete request.

    Returns:
        The matching topics.
    """
    typed = request.value.casefold()
    return [CommandChoice(name=t, value=t) for t in TOPICS if t.startswith(typed)]
```

```text title="Output"
Step 6: commands registered: summarize, version
```

Alex sees both commands in the channel:

```http title="Alex: listChannelCommands"
GET /api/v1/channels/{channel_id}/commands HTTP/1.1
```

```json title="Response 200"
{
  "items": [
    {"name": "summarize", "control": false, "id": "{summarize_command_id}"},
    {"name": "version", "control": true, "id": "{version_command_id}"}
  ]
}
```

Suggestions need a thread: the server asks the adapter for them with an invocation token of
the thread the person types in, and a forum channel has no main thread. Step 8 shows them in a
post.

## Step 7: Receive the invocation

```python title="sdk/python/examples/guide/invocation.py"
# SPDX-License-Identifier: Apache-2.0
"""Step 7: receive an invocation.

The SDK checks the invocation token of every event (signature, issuer, audience, expiry, replay,
the run and thread it names) and stores the event in the inbox before it acknowledges it. Only
then does the adapter compare what its runtime really uses with what the token says (I3): a
mismatch ends the run without a start. The context comes with a trust mark per message; text of
people without the right to trigger reaches the runtime only inside marked data blocks.
"""

from __future__ import annotations

from bothy_sdk.adapter import ActualBilling, Invocation, RunRefused
from bothy_sdk.context import RenderedContext, render_context
from bothy_sdk.generated.models import BillingEvidence, BillingEvidenceKeyType
from bothy_sdk.invocation import Actual, TokenVerdict, compare_actual
from bothy_sdk.runs import RunHandle

ACTUAL_MODE = "organization"
"""How the example's runtime bills: on the organization, without an own key."""

ACTUAL_TOOL_CLASS = "chat"
"""The tools the example's runtime has loaded: none beyond chat."""


def actual_billing() -> ActualBilling:
    """Return what the runtime really uses (I3).

    Returns:
        The actual mode with its evidence.
    """
    return ActualBilling(ACTUAL_MODE, BillingEvidence(key_type=BillingEvidenceKeyType.NONE))


def check_actual(inv: Invocation) -> None:
    """Compare what the runtime uses with the token (I3, chapter 10 section 7).

    The billing mode must equal the configured one; the runtime's tools must not exceed the
    configured tool class.

    Args:
        inv: The checked invocation.

    Raises:
        RunRefused: With ``billing_mode_mismatch`` or ``tool_class_mismatch``; the run ends
            ``failed`` and never starts.
    """
    verdict = compare_actual(inv.claims, Actual(ACTUAL_MODE, ACTUAL_TOOL_CLASS))
    if verdict is TokenVerdict.BILLING_MODE_MISMATCH:
        raise RunRefused("billing_mode_mismatch")
    if verdict is TokenVerdict.TOOL_CLASS_MISMATCH:
        raise RunRefused("tool_class_mismatch")


async def load_context(run: RunHandle) -> tuple[RenderedContext, list[str]]:
    """Fetch the run context and render it for the runtime.

    Args:
        run: The run, not started yet.

    Returns:
        The rendered context and the trust marks in thread order.
    """
    ctx = await run.load_context()
    messages = sorted(ctx.thread_context, key=lambda m: m.thread_seq)
    marks = [str(m.trust) for m in messages] + [str(ctx.trigger_message.trust)]
    return render_context(ctx), marks
```

Sam opens a post in Tickets. Sam has no `trigger`, so nothing runs:

```http title="Sam: createThread"
POST /api/v1/channels/{channel_id}/threads HTTP/1.1
Content-Type: application/json

{
  "title": "Invoice export fails",
  "first_message": {
    "client_msg_id": "{new_uuid}",
    "content_md": "The invoice export stops at 50 percent since this morning."
  }
}
```

```json title="Response 201"
{"thread": {"id": "{thread_id}"}}
```

Alex asks Lotse for help in the post:

```http title="Alex: createMessage"
POST /api/v1/threads/{thread_id}/messages HTTP/1.1
Content-Type: application/json

{"client_msg_id": "{new_uuid}", "content_md": "Lotse, can you help Sam with the export?"}
```

The adapter accepts the run after the SDK's token check and its own comparison, then loads the
context. Sam's message is marked `unverified`: Sam may not trigger, so the runtime gets the text
only inside a marked data block. Alex's message is the trigger.

```text title="Output"
Step 7: accepted run {run_id}: token checked, actual mode organization
Step 7: context of run {run_id}: unverified, trigger
```

## Step 8: Manage the run

```python title="sdk/python/examples/guide/runtime.py"
# SPDX-License-Identifier: Apache-2.0
"""The example runtime: fixed rules on the trigger text stand in for a model.

A real adapter hands the rendered context to its model at this point. The guide's runtime only
looks for a few words in the trigger text, so following the guide costs nothing and its output
is the same on every run. It never repeats a text it reads.
"""

from __future__ import annotations

import asyncio
from dataclasses import dataclass, field
from enum import StrEnum
from uuid import UUID

from bothy_sdk.generated.models import CostReport

MODEL = "guide-rules"
"""The model name the example reports in ``startRun`` and in its usage."""

TOKENS_IN, TOKENS_OUT = 120, 40
"""The usage the example reports per run: fixed, because there is no model."""

ANSWER = "Thanks for the details. I have checked the ticket and will take care of it."
"""The answer when no other rule applies."""

FILE_NAME = "summary.txt"
FILE_TEXT = "Ticket summary: one open ticket, waiting for a reply.\n"
"""The text file the rule ``file`` answers with."""


class Rule(StrEnum):
    """What the runtime does with a run."""

    FOLLOWUP = "follow-up"
    QUESTION = "question"
    APPROVAL = "approval"
    LONG = "long"
    FILE = "file"
    SUMMARIZE = "summarize"
    ANSWER = "answer"


# Case-insensitive, first match wins.
_WORDS = (Rule.FOLLOWUP, Rule.QUESTION, Rule.APPROVAL, Rule.LONG, Rule.FILE)


def decide(trigger_text: str, command: str | None = None) -> Rule:
    """Pick the rule for a run.

    Args:
        trigger_text: The text of the triggering message (empty for a command).
        command: The name of the model command that started the run, if any.

    Returns:
        The rule.

    Examples:
        >>> decide("Please ask me a QUESTION first")
        <Rule.QUESTION: 'question'>
        >>> decide("", command="summarize")
        <Rule.SUMMARIZE: 'summarize'>
    """
    text = trigger_text.casefold()
    for rule in _WORDS:
        if rule.value in text:
            return rule
    if command == Rule.SUMMARIZE.value:
        return Rule.SUMMARIZE
    return Rule.ANSWER


def summary(topic: str | None) -> str:
    """The answer of the model command ``summarize``.

    Args:
        topic: The value of the option ``topic``, if given.

    Returns:
        A short summary that names the topic.
    """
    return f"Summary for topic {topic or 'all'}: one open ticket, waiting for a reply."


def usage() -> CostReport:
    """The usage of one run, reported with ``finishRun``.

    No ``amount_micros``: the server prices the tokens with its price table.

    Returns:
        One report for the example's runtime.
    """
    return CostReport(
        provider="example", model=MODEL, currency="EUR", tokens_in=TOKENS_IN, tokens_out=TOKENS_OUT
    )


@dataclass
class Work:
    """The runtime's state of one run.

    Attributes:
        rule: What the run does.
        channel_id: The run's channel (uploads belong to a channel).
        topic: The option ``topic`` of a model command.
        card_message_id: The message of a question or approval card.
        stopped: Set when the run is asked to stop.
        stop_reason: Why it stops.
        followup: Set when a follow-up was absorbed.
    """

    rule: Rule
    channel_id: UUID | None = None
    topic: str | None = None
    card_message_id: UUID | None = None
    stopped: asyncio.Event = field(default_factory=asyncio.Event)
    stop_reason: str = "user_cancelled"
    followup: asyncio.Event = field(default_factory=asyncio.Event)
```

```python title="sdk/python/examples/guide/run.py"
# SPDX-License-Identifier: Apache-2.0
"""Step 8: manage a run.

The SDK starts the run with the newest invocation token and calls `execute` only after ``200``.
Every request inside carries that token, so the run can do no more than the person who
triggered it (07 SR9). The live box shows a status line, one step, a text excerpt and metrics;
the answer goes out through the outbox, which never sends it twice; ``finishRun`` reports the
usage.
"""

from __future__ import annotations

from connect import report
from runtime import TOKENS_IN, Work, usage

from bothy_sdk.adapter import RuntimeFailure
from bothy_sdk.outbox import OutboxState
from bothy_sdk.runs import RunHandle


async def show_progress(run: RunHandle, status_line: str) -> None:
    """Fill the live box and wait until the server stored it.

    Args:
        run: The started run.
        status_line: The status line to show.
    """
    step = run.patch.add_step("status", "read-thread", title="Read the thread")
    run.patch.set_status_line(status_line)
    run.patch.set_last_text_excerpt("Checked the ticket.")
    run.patch.set_metric("tokens_in", TOKENS_IN)
    run.patch.update_step(step, status="completed")
    await run.patch.flush()
    report(8, f"live box updated for run {run.run_id}")


async def answer(work: Work, run: RunHandle, text: str) -> None:
    """Send the answer and finish the run with its usage.

    Args:
        work: The runtime's state of the run.
        run: The started run.
        text: The answer.

    Raises:
        RuntimeFailure: The answer was not stored; the run ends ``failed``.
    """
    sent = await run.send(text)
    if sent.state is not OutboxState.SENT:
        raise RuntimeFailure("the answer was not stored")
    report(8, f"answer sent in run {run.run_id}")
    await finish(work, run)


async def finish(work: Work, run: RunHandle) -> None:
    """End the run ``completed`` with a summary and its usage (``finishRun``).

    Args:
        work: The runtime's state of the run.
        run: The started run.
    """
    await run.finish("completed", summary=f"Answered ({work.rule})", costs=[usage()])
    report(8, f"finished run {run.run_id}: completed, usage reported")
```

The SDK starts the run with `startRun` and calls `execute` only after `200`. Every request of the
run carries its invocation token, so the run can do no more than Alex may (rule SR9 of
[chapter 07](../spec/07-permissions.md)).

```text title="Output"
Step 8: started run {run_id}
Step 8: live box updated for run {run_id}
Step 8: answer sent in run {run_id}
Step 8: finished run {run_id}: completed, usage reported
```

```http title="Alex: getRun"
GET /api/v1/runs/{run_id} HTTP/1.1
```

```json title="Response 200"
{
  "state": "completed",
  "billing_mode_used": "organization",
  "summary": "Answered (answer)"
}
```

Alex now types the model command in the post. While Alex types a topic, the server asks the
adapter for suggestions (step 6):

```http title="Alex: autocompleteCommand"
POST /api/v1/channels/{channel_id}/commands/{summarize_command_id}/autocomplete HTTP/1.1
Content-Type: application/json

{"thread_id": "{thread_id}", "option": "topic", "value": "bi"}
```

```json title="Response 200"
{"choices": [{"name": "billing", "value": "billing"}]}
```

```text title="Output"
Step 6: sent 1 suggestion(s) for option topic
```

The model command starts a run as well; the adapter receives it as an invocation with the
command and its options. A run from a command has no triggering message, and the server defines
no context for it yet ([chapter 09 section 4.6](../spec/09-agents.md)), so the runtime works from
the command and its options:

```http title="Alex: invokeCommand"
POST /api/v1/channels/{channel_id}/commands/{summarize_command_id}/invoke HTTP/1.1
Content-Type: application/json

{"thread_id": "{thread_id}", "options": [{"name": "topic", "value": "billing"}]}
```

```json title="Response 202"
{"accepted": true, "run_id": "{command_run_id}"}
```

```text title="Output"
Step 7: accepted run {command_run_id}: token checked, actual mode organization
Step 7: run {command_run_id} comes from the command summarize
Step 8: started run {command_run_id}
Step 8: live box updated for run {command_run_id}
Step 8: answer sent in run {command_run_id}
Step 8: finished run {command_run_id}: completed, usage reported
```

```http title="Alex: getRun"
GET /api/v1/runs/{command_run_id} HTTP/1.1
```

```json title="Response 200"
{"trigger_kind": "command", "state": "completed"}
```

## Step 9: Become interactive

```python title="sdk/python/examples/guide/interactive.py"
# SPDX-License-Identifier: Apache-2.0
"""Step 9: follow-ups, stop, questions, approvals and control commands.

A message to a running run arrives as a follow-up (``steer``): the example marks it
``delivered``, then ``absorbed``. A question or an approval card puts the run into
``waiting_input``; the SDK resumes it with the token of the answer, a message or a click. A stop
request reaches `stop`; the run ends ``cancelled`` with the request's reason. A control command
is answered at once, also while a run is busy.
"""

from __future__ import annotations

import base64
import hashlib
from typing import Any, BinaryIO, cast
from uuid import UUID

from connect import ADAPTER_VERSION, report
from run import answer, finish
from runtime import ANSWER, FILE_NAME, FILE_TEXT, Rule, Work

from bothy_sdk.adapter import Invocation, RuntimeFailure
from bothy_sdk.components import SelectOption, approval_card, question_card
from bothy_sdk.generated.api.agent_runtime import respond_to_interaction
from bothy_sdk.generated.api.files import complete_upload, create_upload, upload_part
from bothy_sdk.generated.api.interactions import update_component
from bothy_sdk.generated.api.messages import get_message
from bothy_sdk.generated.models import (
    ComponentUpdate,
    InteractionRespond,
    InteractionRespondState,
    InteractionResponse,
    JsonObject,
    Message,
    UploadComplete,
    UploadCreate,
    UploadPartResult,
    UploadSession,
)
from bothy_sdk.generated.types import File
from bothy_sdk.outbox import OutboxState
from bothy_sdk.runs import RunHandle

RULES = (Rule.FOLLOWUP, Rule.QUESTION, Rule.APPROVAL, Rule.LONG, Rule.FILE)
"""The rules that make a run interactive."""


async def begin(work: Work, run: RunHandle) -> None:
    """Start the interactive part of a run; returns when the run waits or has ended.

    Args:
        work: The runtime's state of the run.
        run: The started run.
    """
    if work.rule is Rule.FOLLOWUP:
        run.patch.set_status_line("Waiting for a follow-up")
        await run.patch.flush()
        report(9, f"run {run.run_id} waits for a follow-up")
        await work.followup.wait()
        await answer(work, run, ANSWER)
    elif work.rule is Rule.QUESTION:
        card = question_card(
            "guide.question", [SelectOption("Yes", "yes"), SelectOption("No", "no")]
        )
        await _ask(work, run, "Is the ticket about your own account?", card, "an answer")
    elif work.rule is Rule.APPROVAL:
        await _ask(work, run, "May I reset the export job?", approval_card(), "an approval")
    elif work.rule is Rule.FILE:
        await send_file(work, run)
    else:
        run.patch.set_status_line("Working until stopped")
        await run.patch.flush()
        report(9, f"run {run.run_id} keeps working until it is stopped")
        await work.stopped.wait()
        await run.finish("cancelled", reason=work.stop_reason)
        report(9, f"stopped run {run.run_id}: {work.stop_reason}")


async def _ask(work: Work, run: RunHandle, text: str, card: list[Any], what: str) -> None:
    """Send a card, then let the run wait for a person (``awaitRunInput``)."""
    sent = await run.send(text, kind="card", components=card)
    work.card_message_id = sent.message_id
    if work.rule is Rule.APPROVAL and sent.created is not None:
        once = next(c.id for c in sent.created.components if c.custom_id == "approval.once")
        report(9, f"approval card sent in run {run.run_id}: allow once is {once}")
    else:
        report(9, f"question card sent in run {run.run_id}")
    await run.await_input(f"Waiting for {what}")
    report(9, f"run {run.run_id} waits for {what}")


async def on_followup(work: Work, inv: Invocation, run: RunHandle) -> None:
    """Handle a follow-up: steer the running run, or take it as the answer to a question.

    Args:
        work: The runtime's state of the run.
        inv: The ``agent.followup`` event.
        run: The run; the SDK resumed it when it was waiting.
    """
    # Marked absorbed, the message is handled; the server starts no follow-on run for it (09 7).
    message_id: UUID = inv.payload.message_id
    run.patch.followup_delivered(message_id)
    await run.patch.flush()
    run.patch.followup_absorbed(message_id)
    await run.patch.flush()
    if work.rule is Rule.QUESTION:
        report(9, f"answer received, run {run.run_id} resumed")
        await answer(work, run, ANSWER)
        return
    report(9, f"follow-up absorbed in run {run.run_id}")
    work.followup.set()


async def on_interaction(work: Work, inv: Invocation, run: RunHandle) -> None:
    """Handle a click on the approval card; the SDK resumed the run with its token.

    Args:
        work: The runtime's state of the run.
        inv: The ``interaction.created`` event.
        run: The resumed run.
    """
    interaction = inv.payload.interaction
    decision = str(interaction.custom_id).removeprefix("approval.")
    label = f"Decision: {decision}"
    await run.client.call(
        respond_to_interaction.asyncio_detailed,
        interaction_id=interaction.id,
        body=InteractionRespond(
            state=InteractionRespondState.ACKNOWLEDGED, response=InteractionResponse(text=label)
        ),
    )
    await _show_decision(run, work.card_message_id, interaction.component_id, label)
    report(9, f"decision {decision} in run {run.run_id}, run resumed")
    await answer(work, run, "Thanks, the export job will be reset once.")


async def _show_decision(
    run: RunHandle, message_id: UUID | None, component_id: UUID, label: str
) -> None:
    """Show the decision on the clicked button (``updateComponent``)."""
    if message_id is None:
        return
    message = await run.client.call_as(Message, get_message.asyncio_detailed, message_id=message_id)
    component = next(c for c in message.components if c.id == component_id)
    definition = {**component.definition.to_dict(), "label": label}
    await run.client.call(
        update_component.asyncio_detailed,
        component_id=component_id,
        # The clicked button stays used; only its label shows the decision (09 11.4).
        body=ComponentUpdate(definition=JsonObject.from_dict(definition)),
        if_match=f'"{component.version}"',
    )


async def on_command(inv: Invocation, run: RunHandle | None) -> None:
    """Answer the control command ``version`` as a message of the agent.

    Args:
        inv: The ``command.invoked`` event.
        run: The agent's active run in the thread, if any.
    """
    if inv.payload.command_name != "version" or run is None:
        return
    await run.send(f"Guide example adapter {ADAPTER_VERSION}")
    report(9, f"answered control command version in run {run.run_id}")


async def send_file(work: Work, run: RunHandle) -> None:
    """Answer with a text file: upload it to the channel, then send it as an attachment.

    The upload is the generated core's ``createUpload``, ``uploadPart`` and ``completeUpload``;
    the server scans the file before anyone can download it.

    Args:
        work: The runtime's state of the run.
        run: The started run.

    Raises:
        RuntimeFailure: The run has no channel, or the message was not stored.
    """
    if work.channel_id is None:
        raise RuntimeFailure("the run names no channel")
    content = FILE_TEXT.encode()
    digest = hashlib.sha256(content)
    session = await run.client.call_as(
        UploadSession,
        create_upload.asyncio_detailed,
        channel_id=work.channel_id,
        body=UploadCreate(
            name=FILE_NAME,
            size_bytes=len(content),
            sha256=digest.hexdigest(),
            declared_mime_type="text/plain",
        ),
    )
    part = await run.client.call_as(
        UploadPartResult,
        upload_part.asyncio_detailed,
        file_id=session.file_id,
        part_number=1,
        # The async client sends the bytes as they are; the generated type names a file object.
        body=File(payload=cast(BinaryIO, content)),
        content_digest=f"sha-256=:{base64.b64encode(digest.digest()).decode()}:",
    )
    await run.client.call(
        complete_upload.asyncio_detailed,
        file_id=session.file_id,
        body=UploadComplete(parts=[part]),
    )
    sent = await run.send("Here is the summary as a file.", attachment_file_ids=[session.file_id])
    if sent.state is not OutboxState.SENT:
        raise RuntimeFailure("the message with the file was not stored")
    report(9, f"file sent in run {run.run_id} as message {sent.message_id}")
    await finish(work, run)


def stop(work: Work, reason: str) -> None:
    """Ask the runtime of a run to stop (``run.cancel_requested``).

    Args:
        work: The runtime's state of the run.
        reason: The request's reason, for example ``user_cancelled``.
    """
    work.stop_reason = reason
    work.stopped.set()
```

**Follow-ups.** Alex opens a post that asks Lotse to wait for a follow-up:

```http title="Alex: createThread"
POST /api/v1/channels/{channel_id}/threads HTTP/1.1
Content-Type: application/json

{
  "title": "Login loop",
  "first_message": {"client_msg_id": "{new_uuid}", "content_md": "Lotse, wait for my follow-up please."}
}
```

```json title="Response 201"
{"thread": {"id": "{followup_thread_id}"}}
```

```text title="Output"
Step 7: accepted run {followup_run_id}: token checked, actual mode organization
Step 7: context of run {followup_run_id}: trigger
Step 8: started run {followup_run_id}
Step 8: live box updated for run {followup_run_id}
Step 9: run {followup_run_id} waits for a follow-up
```

```http title="Alex: createMessage"
POST /api/v1/threads/{followup_thread_id}/messages HTTP/1.1
Content-Type: application/json

{"client_msg_id": "{new_uuid}", "content_md": "It happens only in the mobile app."}
```

```text title="Output"
Step 9: follow-up absorbed in run {followup_run_id}
Step 8: answer sent in run {followup_run_id}
Step 8: finished run {followup_run_id}: completed, usage reported
```

**Questions.** A run that needs an answer posts a card and waits (`awaitRunInput`); a message of
the person is the answer as well ([chapter 09 section 7](../spec/09-agents.md)):

```http title="Alex: createThread"
POST /api/v1/channels/{channel_id}/threads HTTP/1.1
Content-Type: application/json

{
  "title": "Account locked",
  "first_message": {"client_msg_id": "{new_uuid}", "content_md": "Lotse, ask me a question first."}
}
```

```json title="Response 201"
{"thread": {"id": "{question_thread_id}"}}
```

```text title="Output"
Step 7: accepted run {question_run_id}: token checked, actual mode organization
Step 7: context of run {question_run_id}: trigger
Step 8: started run {question_run_id}
Step 8: live box updated for run {question_run_id}
Step 9: question card sent in run {question_run_id}
Step 9: run {question_run_id} waits for an answer
```

```http title="Alex: createMessage"
POST /api/v1/threads/{question_thread_id}/messages HTTP/1.1
Content-Type: application/json

{"client_msg_id": "{new_uuid}", "content_md": "Yes, it is my own account."}
```

```text title="Output"
Step 9: answer received, run {question_run_id} resumed
Step 8: answer sent in run {question_run_id}
Step 8: finished run {question_run_id}: completed, usage reported
```

**Approvals.** Before an action with consequences the run asks with the approval card of
[chapter 09 section 11.5](../spec/09-agents.md). Only the person who started the run (or an
owner) with `approve_actions` may allow it:

```http title="Alex: createThread"
POST /api/v1/channels/{channel_id}/threads HTTP/1.1
Content-Type: application/json

{
  "title": "Stuck export job",
  "first_message": {"client_msg_id": "{new_uuid}", "content_md": "Lotse, reset the export job after my approval."}
}
```

```json title="Response 201"
{"thread": {"id": "{approval_thread_id}"}}
```

```text title="Output"
Step 7: accepted run {approval_run_id}: token checked, actual mode organization
Step 7: context of run {approval_run_id}: trigger
Step 8: started run {approval_run_id}
Step 8: live box updated for run {approval_run_id}
Step 9: approval card sent in run {approval_run_id}: allow once is {once_component_id}
Step 9: run {approval_run_id} waits for an approval
```

```http title="Alex: createInteraction"
POST /api/v1/components/{once_component_id}/interactions HTTP/1.1
Content-Type: application/json

{"client_interaction_id": "{new_uuid}"}
```

```json title="Response 201"
{"component_id": "{once_component_id}", "custom_id": "approval.once"}
```

```text title="Output"
Step 9: decision once in run {approval_run_id}, run resumed
Step 8: answer sent in run {approval_run_id}
Step 8: finished run {approval_run_id}: completed, usage reported
```

**Stop.** A long run works until a person stops it. Meanwhile the control command still reaches
the adapter:

```http title="Alex: createThread"
POST /api/v1/channels/{channel_id}/threads HTTP/1.1
Content-Type: application/json

{
  "title": "Nightly import",
  "first_message": {"client_msg_id": "{new_uuid}", "content_md": "Lotse, watch the long import tonight."}
}
```

```json title="Response 201"
{"thread": {"id": "{long_thread_id}"}}
```

```text title="Output"
Step 7: accepted run {long_run_id}: token checked, actual mode organization
Step 7: context of run {long_run_id}: trigger
Step 8: started run {long_run_id}
Step 8: live box updated for run {long_run_id}
Step 9: run {long_run_id} keeps working until it is stopped
```

```http title="Alex: invokeCommand"
POST /api/v1/channels/{channel_id}/commands/{version_command_id}/invoke HTTP/1.1
Content-Type: application/json

{"thread_id": "{long_thread_id}"}
```

```json title="Response 202"
{"accepted": true}
```

```text title="Output"
Step 9: answered control command version in run {long_run_id}
```

```http title="Alex: cancelRun"
POST /api/v1/runs/{long_run_id}/cancel HTTP/1.1
Content-Type: application/json

{"reason": "Not needed tonight"}
```

```json title="Response 202"
{"id": "{long_run_id}"}
```

```text title="Output"
Step 9: stopped run {long_run_id}: user_cancelled
```

```http title="Alex: getRun"
GET /api/v1/runs/{long_run_id} HTTP/1.1
```

```json title="Response 200"
{"state": "cancelled", "state_reason": "user_cancelled"}
```

**Files.** The adapter answers with a file: it uploads a small text file to the channel
(`createUpload`, `uploadPart`, `completeUpload` of the generated core; the agent's grant carries
`upload`) and sends it with `attachment_file_ids`. The heartbeat of step 5 announces
`files_out`. The server scans the file before anyone can download it. A run acts with the rights
of the person who triggered it, so Alex's `upload` lets the run upload (SR9).

```http title="Alex: createThread"
POST /api/v1/channels/{channel_id}/threads HTTP/1.1
Content-Type: application/json

{
  "title": "Weekly summary",
  "first_message": {"client_msg_id": "{new_uuid}", "content_md": "Lotse, send me the summary as a file."}
}
```

```json title="Response 201"
{"thread": {"id": "{file_thread_id}"}}
```

```text title="Output"
Step 7: accepted run {file_run_id}: token checked, actual mode organization
Step 7: context of run {file_run_id}: trigger
Step 8: started run {file_run_id}
Step 8: live box updated for run {file_run_id}
Step 9: file sent in run {file_run_id} as message {file_message_id}
Step 8: finished run {file_run_id}: completed, usage reported
```

```http title="Alex: getMessage"
GET /api/v1/messages/{file_message_id} HTTP/1.1
```

```json title="Response 200"
{"attachments": [{"file": {"original_name": "summary.txt", "mime_type": "text/plain"}}]}
```

## Step 10: Test and operate

The example's tests run the adapter against the SDK's FakeServer, in process and without a
model: the scenarios AG-08, AG-09, AG-11, AG-12 and AG-28 of
[chapter 09 section 16.2](../spec/09-agents.md), the rules of the runtime, the key file, the
refusal of readable secret files, the start lines and an output without message text or secrets.

```python title="sdk/python/examples/guide/tests/test_agent.py"
# SPDX-License-Identifier: Apache-2.0
"""The example adapter against the SDK's FakeServer: in process, on 127.0.0.1, no model.

Scenarios of docs/spec/09-agents.md 16.2 on the adapter side (AG-08, AG-09, AG-11, AG-12,
AG-28), the runtime's rules, the key file, the refusal of readable secret files, an output
without message text or secrets, and the order of the start lines.
"""

from __future__ import annotations

import asyncio
import contextlib
import inspect
import random
import re
import stat
import time
from collections.abc import AsyncIterator, Callable
from pathlib import Path
from typing import Any
from uuid import UUID

import commands
import connect
import heartbeat
import keygen
import pytest
from agent import GuideAgent
from runtime import Rule, decide

from bothy_sdk.adapter import Adapter, AdapterConfig, Invocation, StartOptions
from bothy_sdk.client import Client
from bothy_sdk.gateway import Gateway
from bothy_sdk.generated.api.agent_runtime import patch_run
from bothy_sdk.generated.models import JsonPatchOperation, JsonPatchOperationOp, RunPatch
from bothy_sdk.inbox import InboxEntry
from bothy_sdk.runs import RunHandle
from bothy_sdk.sources import GatewaySource
from bothy_sdk.testing import FakeServer, fingerprint

# quality.test.patience: the only upper bound of a wait; it ends a test that would hang.
PATIENCE = 180.0
CANARY = "canary-5e1f-in-every-text"


async def until(fake: FakeServer, predicate: Callable[[], Any], what: str) -> None:
    """Wait for a condition while the FakeServer's clock moves on."""
    deadline = time.monotonic() + PATIENCE
    while True:
        result = predicate()
        if inspect.isawaitable(result):
            result = await result
        if result:
            return
        assert time.monotonic() < deadline, f"waited for {what}"
        await fake.pass_time(0.05)
        await asyncio.sleep(0.005)


@pytest.fixture
async def fake() -> AsyncIterator[FakeServer]:
    async with FakeServer() as server:
        yield server


def config(fake: FakeServer, consumer: str) -> AdapterConfig:
    """The configuration of connect.adapter_config, with the FakeServer's credentials."""
    return AdapterConfig(
        base_url=fake.base_url,
        instance_id=fake.instance_id,
        agent_id=fake.agent_id,
        credentials=fake.credentials,
        consumer=consumer,
        settings=fake.sdk_settings,
        capabilities=heartbeat.CAPABILITIES,
        commands=commands.COMMANDS,
    )


@contextlib.asynccontextmanager
async def running(
    fake: FakeServer, handler: GuideAgent, consumer: str, rng: random.Random | None = None
) -> AsyncIterator[Adapter]:
    """Run the example's handler until the start lines are out; stop it at the end."""
    adapter = Adapter(config(fake, consumer), handler, clock=fake.clock)
    if rng is not None:
        gateway = Gateway(
            fake.gateway_url,
            fake.credentials,
            consumer=consumer,
            agent_id=fake.agent_id,
            settings=fake.sdk_settings,
            clock=fake.clock,
            rng=rng,
            heartbeat=adapter.heartbeat,
            nonce_sink=adapter.client.auth.remember_nonce,
        )
        adapter.source = GatewaySource(gateway)
    handler.attach(adapter, await connect.accepted_runs(adapter))
    task = asyncio.ensure_future(adapter.run())
    try:
        await until(fake, handler.ready.is_set, "the start lines")
        yield adapter
    finally:
        await adapter.stop()
        with contextlib.suppress(Exception):
            await asyncio.wait_for(task, PATIENCE)


class Gated(GuideAgent):
    """Holds every run in ``prepare`` until the test opens the gate."""

    def __init__(self) -> None:
        super().__init__()
        self.gate = asyncio.Event()
        self.prepared = 0
        self.reconciled = 0

    async def prepare(self, inv: Invocation, run: RunHandle) -> StartOptions:
        self.prepared += 1
        await self.gate.wait()
        return await super().prepare(inv, run)

    async def on_commands_reconciled(self, result: Any) -> None:
        self.reconciled += 1
        await super().on_commands_reconciled(result)


def starts(fake: FakeServer, run_id: UUID) -> list[Any]:
    return [r for r in fake.requests if r.operation_id == "startRun" and str(run_id) in r.path]


async def entry_of(adapter: Adapter, run_id: UUID) -> InboxEntry:
    entries = await adapter.inbox.for_run(run_id)
    return entries[0]


@pytest.mark.anyio
@pytest.mark.parametrize(
    "kind", ["missing", "bad_signature", "alg_hs256", "aud_other_agent", "typ_access"]
)
async def test_ag08_invalid_token_starts_nothing(
    fake: FakeServer, kind: str, capsys: pytest.CaptureFixture[str]
) -> None:
    async with running(fake, GuideAgent(), f"ag08-{kind}"):
        triggered = await fake.trigger("Lotse, please help", forged=kind)
        await until(fake, lambda: fake.run_state(triggered.run_id).state == "failed", "the end")
    view = fake.run_state(triggered.run_id)
    assert view.state_reason == "invalid_invocation_token"
    assert view.starts == 0
    assert "Step 7: accepted" not in capsys.readouterr().out


@pytest.mark.anyio
async def test_ag09_double_delivery_starts_once(fake: FakeServer) -> None:
    handler = Gated()
    async with running(fake, handler, "ag09") as adapter:
        triggered = await fake.trigger("Lotse, please help")
        await until(fake, lambda: handler.prepared == 1, "prepare")
        first = await entry_of(adapter, triggered.run_id)
        assert first.seq is not None
        # The server delivers the acknowledged event again with a new token (09 4.3).
        await fake.restart("ag09", first.seq - 1)

        async def replaced() -> bool:
            entry = await entry_of(adapter, triggered.run_id)
            return entry.deliveries >= 2 and entry.token != first.token

        await until(fake, replaced, "the newer token in the inbox")
        newest = await entry_of(adapter, triggered.run_id)
        handler.gate.set()
        await until(fake, lambda: fake.run_state(triggered.run_id).state == "completed", "end")
    assert handler.prepared == 1
    started = starts(fake, triggered.run_id)
    assert len(started) == 1
    assert newest.token is not None
    assert started[0].json["invocation_token"] == fingerprint(newest.token)


@pytest.mark.anyio
async def test_ag11_billing_mode_mismatch_starts_no_runtime(
    capsys: pytest.CaptureFixture[str],
) -> None:
    async with FakeServer(billing_mode="personal") as personal:
        async with running(personal, GuideAgent(), "ag11"):
            triggered = await personal.trigger("Lotse, please help")
            await until(
                personal, lambda: personal.run_state(triggered.run_id).state == "failed", "end"
            )
        view = personal.run_state(triggered.run_id)
    assert view.state_reason == "billing_mode_mismatch"
    assert view.starts == 0
    assert "Step 8: started" not in capsys.readouterr().out


@pytest.mark.anyio
async def test_ag12_patch_conflict_is_rebased(
    fake: FakeServer, capsys: pytest.CaptureFixture[str]
) -> None:
    async with running(fake, GuideAgent(), "ag12"):
        triggered = await fake.trigger("Lotse, wait for my follow-up")
        run_id = triggered.run_id
        await until(fake, lambda: "waits for a follow-up" in capsys.readouterr().out, "waiting")
        # Someone else changes the run first: the example's next patch gets 409.
        other = Client(
            fake.base_url, fake.credentials, settings=fake.sdk_settings, clock=fake.clock
        )
        try:
            await other.call(
                patch_run.asyncio_detailed,
                run_id=run_id,
                body=RunPatch(
                    base_version=fake.run_state(run_id).version,
                    ops=[
                        JsonPatchOperation(
                            op=JsonPatchOperationOp.REPLACE,
                            path="/status_line",
                            value="external change",
                        )
                    ],
                ),
            )
        finally:
            await other.aclose()
        message_id = await fake.deliver_followup(run_id, "It happens on mobile only")
        await until(fake, lambda: fake.run_state(run_id).state == "completed", "the end")
    assert fake.followup_state(run_id, message_id) == "absorbed"
    assert fake.run(run_id)["status_line"] == "external change"


@pytest.mark.anyio
async def test_ag28_expired_token_is_renewed(fake: FakeServer) -> None:
    handler = Gated()
    async with running(fake, handler, "ag28"):
        triggered = await fake.trigger("Lotse, please help")
        await until(fake, lambda: handler.prepared == 1, "prepare")
        await fake.pass_time(
            fake.timings["agent.invocation_token_ttl"] + fake.timings["auth.clock_leeway"] + 1.0
        )
        handler.gate.set()
        await until(fake, lambda: fake.run_state(triggered.run_id).state == "completed", "end")
    renewals = [r for r in fake.requests if r.operation_id == "renewInvocationToken"]
    assert len(renewals) == 1
    assert fake.run_state(triggered.run_id).starts == 1
    assert len(starts(fake, triggered.run_id)) == 1


def test_runtime_rules() -> None:
    assert decide("Please wait for my FOLLOW-UP") is Rule.FOLLOWUP
    assert decide("ask me a question") is Rule.QUESTION
    assert decide("after my approval") is Rule.APPROVAL
    assert decide("a long import") is Rule.LONG
    assert decide("send the log as a FILE") is Rule.FILE
    assert decide("", command="summarize") is Rule.SUMMARIZE
    assert decide("Hello") is Rule.ANSWER
    # First match wins, in the order of the rules.
    assert decide("a long question after a follow-up") is Rule.FOLLOWUP
    assert decide("long approval question") is Rule.QUESTION
    assert decide("long approval") is Rule.APPROVAL
    assert decide("long summary", command="summarize") is Rule.LONG


def test_keygen_writes_private_key_file(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    keygen.main(["--adapter-dir", str(tmp_path)])
    printed = capsys.readouterr().out
    match = re.fullmatch(r"dpop_jkt: ([A-Za-z0-9_-]{43})\n", printed)
    assert match is not None
    key = tmp_path / connect.KEY_FILE
    assert stat.S_IMODE(key.stat().st_mode) == 0o600
    before = key.read_bytes()
    with pytest.raises(SystemExit) as exit_info:
        keygen.main(["--adapter-dir", str(tmp_path)])
    assert exit_info.value.code == 1
    assert key.read_bytes() == before


def _secret_files(tmp_path: Path) -> None:
    keygen.main(["--adapter-dir", str(tmp_path)])
    token = tmp_path / connect.TOKEN_FILE
    token.write_text(CANARY, encoding="utf-8")
    token.chmod(0o600)


@pytest.mark.parametrize("name", [connect.TOKEN_FILE, connect.KEY_FILE])
def test_refuses_readable_secret_files(tmp_path: Path, name: str) -> None:
    _secret_files(tmp_path)
    (tmp_path / name).chmod(0o640)
    with pytest.raises(SystemExit) as exit_info:
        connect.credentials(tmp_path)
    message = str(exit_info.value.code)
    assert name in message
    assert CANARY not in message


def test_adapter_config_reads_the_adapter_dir(tmp_path: Path) -> None:
    _secret_files(tmp_path)
    instance, agent = UUID(int=1), UUID(int=2)
    cfg = connect.adapter_config(
        server_url="https://chat.example.com/",
        instance_id=instance,
        agent_id=agent,
        adapter_dir=tmp_path,
        consumer="default",
        capabilities={},
        commands=(),
    )
    assert cfg.base_url == "https://chat.example.com/api/v1"
    assert cfg.transport == "gateway"
    assert cfg.state_dir == tmp_path / "state"
    assert stat.S_IMODE((tmp_path / "state").stat().st_mode) == 0o700
    assert cfg.credentials.dpop is not None


@pytest.mark.anyio
async def test_output_has_no_content_or_secret(
    fake: FakeServer, capsys: pytest.CaptureFixture[str], caplog: pytest.LogCaptureFixture
) -> None:
    caplog.set_level("DEBUG")
    async with running(fake, GuideAgent(), "canary"):
        triggered = await fake.trigger(f"Lotse, {CANARY}")
        await until(fake, lambda: fake.run_state(triggered.run_id).state == "completed", "end")
        message_id = await fake.deliver_followup(triggered.run_id, CANARY)
        assert message_id is not None
    captured = capsys.readouterr()
    for text in (captured.out, captured.err, caplog.text):
        assert CANARY not in text
        assert fake.credentials.api_token.reveal() not in text
        assert re.search(r"eyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+", text) is None
    assert f"Step 8: finished run {triggered.run_id}: completed, usage reported" in captured.out


class EarlyBeat(random.Random):
    """The first gateway heartbeat right after ``ready``, before the reconciliation ends."""

    def random(self) -> float:
        return 0.001


START_LINES = [
    "Step 4: connected as lotse with consumer {consumer}",
    "Step 5: heartbeat accepted: online, mode organization, tool class chat",
    "Step 6: commands registered: summarize, version",
]


@pytest.mark.anyio
@pytest.mark.parametrize("seed", [None, 1, 2, 3])
async def test_start_lines_order(
    fake: FakeServer, seed: int | None, capsys: pytest.CaptureFixture[str]
) -> None:
    consumer = f"order-{seed or 'early'}"
    rng = EarlyBeat() if seed is None else random.Random(seed)  # noqa: S311 - jitter, no secret
    handler = Gated()
    async with running(fake, handler, consumer, rng=rng):
        # Further gateway heartbeats, then a reconnect with a new ready and a new reconciliation.
        await fake.pass_time(fake.timings.get("sync.heartbeat_interval", 30.0) * 3)
        await fake.close_connections(4013)
        await until(fake, lambda: handler.reconciled >= 2, "the reconciliation after reconnect")
    lines = capsys.readouterr().out.splitlines()
    assert lines == [line.format(consumer=consumer) for line in START_LINES]


def lines_of(capsys: pytest.CaptureFixture[str], keep: list[str]) -> Callable[[], list[str]]:
    """A reader of the stdout lines so far; ``keep`` collects what each call took."""

    def read() -> list[str]:
        keep.extend(capsys.readouterr().out.splitlines())
        return keep

    return read


@pytest.mark.anyio
async def test_question_resumes_with_the_answer(
    fake: FakeServer, capsys: pytest.CaptureFixture[str]
) -> None:
    out: list[str] = []
    read = lines_of(capsys, out)
    async with running(fake, GuideAgent(), "question"):
        triggered = await fake.trigger("Lotse, ask me a question first")
        run_id = triggered.run_id
        await until(fake, lambda: f"Step 9: run {run_id} waits for an answer" in read(), "wait")
        message_id = await fake.deliver_followup(run_id, "Yes, my own account")
        await until(fake, lambda: fake.run_state(run_id).state == "completed", "the end")
    read()
    assert f"Step 9: question card sent in run {run_id}" in out
    assert f"Step 9: answer received, run {run_id} resumed" in out
    assert fake.followup_state(run_id, message_id) == "absorbed"


@pytest.mark.anyio
async def test_approval_resumes_with_the_click(
    fake: FakeServer, capsys: pytest.CaptureFixture[str]
) -> None:
    out: list[str] = []
    read = lines_of(capsys, out)
    async with running(fake, GuideAgent(), "approval"):
        triggered = await fake.trigger("Lotse, reset it after my approval")
        run_id = triggered.run_id
        await until(fake, lambda: f"Step 9: run {run_id} waits for an approval" in read(), "wait")
        assert await fake.click(run_id, "approval.once") is not None
        await until(fake, lambda: fake.run_state(run_id).state == "completed", "the end")
    read()
    card = next(line for line in out if line.startswith("Step 9: approval card sent"))
    component_id = UUID(card.rsplit(" ", 1)[1])
    assert fake.component(component_id)["definition"]["label"] == "Decision: once"
    assert f"Step 9: decision once in run {run_id}, run resumed" in out


@pytest.mark.anyio
async def test_long_run_answers_control_commands_and_stops(
    fake: FakeServer, capsys: pytest.CaptureFixture[str]
) -> None:
    out: list[str] = []
    read = lines_of(capsys, out)
    async with running(fake, GuideAgent(), "long"):
        triggered = await fake.trigger("Lotse, watch the long import")
        run_id = triggered.run_id
        await until(
            fake,
            lambda: f"Step 9: run {run_id} keeps working until it is stopped" in read(),
            "work",
        )
        await fake.invoke_command("version", run_id=run_id)
        await until(
            fake,
            lambda: f"Step 9: answered control command version in run {run_id}" in read(),
            "the answer",
        )
        await fake.cancel(run_id)
        await until(fake, lambda: fake.run_state(run_id).state == "cancelled", "the stop")
    read()
    assert f"Step 9: stopped run {run_id}: user_cancelled" in out
    assert fake.run_state(run_id).state_reason == "user_cancelled"
```

```bash title="Shell: run the example's tests"
uv run --project sdk/python --locked pytest sdk/python/examples/guide/tests -q -p no:cacheprovider
```

**Logs.** The adapter prints run, thread and component IDs, never a message text, a prompt, a
token or a key; the SDK logs to stderr under the same rule.

**Restart.** The adapter stores an accepted event in its inbox before it acknowledges it, so a
restart between acceptance and start loses nothing: the next process starts the run from the
inbox, exactly once. Stop the adapter with Ctrl+C:

```bash title="Adapter: stop"
# Ctrl+C in the adapter's terminal
```

```text title="Output"
Step 10: stopped
```

To see the restart, start it with `--stop-after accept`: it stops by itself right after it
accepted the next run, before the start.

```bash title="Adapter: start"
PYTHONPATH=sdk/python/src uv run --project sdk/python --locked python sdk/python/examples/guide/agent.py --server-url {server_url} --instance-id {instance_id} --agent-id {agent_id} --adapter-dir {adapter_dir} --consumer default --stop-after accept
```

```text title="Output"
Step 4: connected as lotse with consumer default
Step 5: heartbeat accepted: online, mode organization, tool class chat
Step 6: commands registered: summarize, version
```

```http title="Alex: createThread"
POST /api/v1/channels/{channel_id}/threads HTTP/1.1
Content-Type: application/json

{
  "title": "Password reset mail",
  "first_message": {"client_msg_id": "{new_uuid}", "content_md": "Lotse, the reset mail does not arrive."}
}
```

```json title="Response 201"
{"thread": {"id": "{restart_thread_id}"}}
```

```text title="Output"
Step 7: accepted run {restart_run_id}: token checked, actual mode organization
Step 10: stopped after accepting run {restart_run_id}
```

Start it again as in step 4: the run comes from the inbox.

```bash title="Adapter: start"
PYTHONPATH=sdk/python/src uv run --project sdk/python --locked python sdk/python/examples/guide/agent.py --server-url {server_url} --instance-id {instance_id} --agent-id {agent_id} --adapter-dir {adapter_dir} --consumer default
```

```text title="Output"
Step 4: connected as lotse with consumer default
Step 5: heartbeat accepted: online, mode organization, tool class chat
Step 6: commands registered: summarize, version
Step 10: resuming run {restart_run_id} from the inbox
Step 7: context of run {restart_run_id}: trigger
Step 8: started run {restart_run_id}
Step 8: live box updated for run {restart_run_id}
Step 8: answer sent in run {restart_run_id}
Step 8: finished run {restart_run_id}: completed, usage reported
```

```http title="Alex: listThreadRuns"
GET /api/v1/threads/{restart_thread_id}/runs HTTP/1.1
```

```json title="Response 200"
{"items": [{"id": "{restart_run_id}", "state": "completed"}]}
```

**Versions.** Pin the SDK exactly in your adapter's `pyproject.toml`. Until the packages are
published, the SDK comes from a checkout of this repository, as in step 4; the pin names the
version of that checkout.

```toml title="pyproject.toml (excerpt)"
[project]
name = "my-adapter"
dependencies = [
  "bothy-sdk==0.0.0",
]
```

The compatibility matrix in [sdk/README.md](../../sdk/README.md) lists which SDK versions work
with which server versions. Finally stop the adapter:

```bash title="Adapter: stop"
# Ctrl+C in the adapter's terminal
```

```text title="Output"
Step 10: stopped
```

## How the repository checks this guide

The Go test `TestGuide` in `services/server/test/guide` reads this page, starts a fresh server,
and runs every block in order: requests as the owner, Alex or Sam, shell commands, and the
example adapter as a background process. It compares every answer with the JSON shown (a subset,
with placeholders that capture or compare values), every output line with the adapter's stdout,
every shown file with the file in `sdk/python/examples/guide/` and the pin with
`sdk/python/VERSION`, and fails at the first block that differs from the text. Run it with
`cd services/server && go test ./test/guide/ -run TestGuide`; the grammar of the blocks is
described in the README of that package.

## Next

- [Chapter 09: agents](../spec/09-agents.md), the contract between the server and an adapter.
- [Chapter 11: extensions](../spec/11-extensions.md), the SDK design and versioning.
- [SDK compatibility matrix](../../sdk/README.md).
- [Python SDK reference](../sdk/python.md).
- [TypeScript SDK README](../../sdk/typescript/README.md).
