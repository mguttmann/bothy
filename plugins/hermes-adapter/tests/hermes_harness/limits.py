# SPDX-License-Identifier: Apache-2.0
"""Named deadlines of the recording harness.

These bound a test, not product behaviour, so they are module constants and no configuration
keys (docs/DECISIONS.md, rulings of M2-32). Code and documentation name the constant, never
its value.
"""

from __future__ import annotations

from typing import Final

#: Upper bound for the checkout and the install of the pinned version (network downloads).
PROVISION_DEADLINE_S: Final = 1800.0
#: Upper bound from the gateway process start to the probe's ``connect`` call and readiness.
GATEWAY_START_DEADLINE_S: Final = 240.0
#: Upper bound for one scenario step (a command result, an awaited call, a latch release).
STEP_DEADLINE_S: Final = 120.0
#: Grace between ``SIGTERM`` and ``SIGKILL`` when the gateway process group is stopped.
STOP_GRACE_S: Final = 20.0
#: Time the harness waits for further calls after a scenario's last step, so late calls of
#: the same turn (final edits, completion hooks) are part of the recording.
SETTLE_S: Final = 3.0
#: Interval in which the probe asks the control channel for the next command.
COMMAND_POLL_S: Final = 0.05
#: Pause between the text chunks of a paced answer of the fake model.
CHUNK_PAUSE_S: Final = 1.0
#: Number of scenarios that run at the same time, each with its own gateway process.
PARALLEL_SCENARIOS: Final = 4
