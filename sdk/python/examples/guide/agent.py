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
        if work is not None:
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
