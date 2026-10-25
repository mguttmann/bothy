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
