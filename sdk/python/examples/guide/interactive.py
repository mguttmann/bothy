# SPDX-License-Identifier: Apache-2.0
"""Step 9: follow-ups, stop, questions, approvals and control commands.

A message to a running run arrives as a follow-up (``steer``): the example marks it
``delivered``, then ``absorbed``. A question or an approval card puts the run into
``waiting_input``; the SDK resumes it with the token of the answer, a message or a click. A stop
request reaches `stop`; the run ends ``cancelled`` with the request's reason. A control command
is answered at once, also while a run is busy.
"""

from __future__ import annotations

import asyncio
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
        if await _followup_or_stop(work):
            await answer(work, run, ANSWER)
        else:
            await _stopped(work, run)
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
        await _stopped(work, run)


async def _followup_or_stop(work: Work) -> bool:
    """Wait for a follow-up or a stop request; True for the follow-up."""
    waits = {
        asyncio.ensure_future(work.followup.wait()),
        asyncio.ensure_future(work.stopped.wait()),
    }
    _, pending = await asyncio.wait(waits, return_when=asyncio.FIRST_COMPLETED)
    for task in pending:
        task.cancel()
    return not work.stopped.is_set()


async def _stopped(work: Work, run: RunHandle) -> None:
    """End a run ``cancelled`` with the reason of the stop request (09 8)."""
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

    A run that waits for a follow-up or works until stopped ends ``cancelled``; a short run
    finishes its answer first.

    Args:
        work: The runtime's state of the run.
        reason: The request's reason, for example ``user_cancelled``.
    """
    work.stop_reason = reason
    work.stopped.set()
