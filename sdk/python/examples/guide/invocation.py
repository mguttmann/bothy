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
