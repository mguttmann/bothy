# SPDX-License-Identifier: Apache-2.0
"""How the pin is named in spec 09 17.10 and in the rulings (D-HERMES-PIN-1, D-HERMES-MAIN-3).

The pin follows the newest upstream main commit and names no release tag. The pin file names
the standing ruling ``FOLLOW_MAIN``, a row of the Hermes decision files holds it, and the 17.10
intro carries the sentence "Current pin: ..." with ``commit[:12]`` and ``recorded_on``, which the
daily workflow rewrites (``follow_main.py``).
"""

from __future__ import annotations

import re
from pathlib import Path

from follow_main import FOLLOW_MAIN, current_pin_sentence
from hermes_harness.pin import HermesPin

REPO = Path(__file__).resolve().parents[3]
# The Hermes decision files: only they may rule on the pin. The name prefix selects the component
# plugins in tools/ci/components.py, so a pull request that changes one runs these tests.
RULING_FILES = "docs/decisions.d/HERMES-"


def ruling_rows() -> dict[str, str]:
    """Every ruling table row of the Hermes decision files, by its ID."""
    files = sorted(REPO.glob(RULING_FILES + "*.md"))
    rows: dict[str, str] = {}
    for path in files:
        for line in path.read_text(encoding="utf-8").splitlines():
            match = re.match(r"\| (D-[A-Z0-9-]+-[0-9]+) \|", line)
            if match:
                rows[match.group(1)] = line
    return rows


def pin_reference_problems(
    pin: HermesPin, pin_text: str, intro: str, rows: dict[str, str]
) -> list[str]:
    """What the 17.10 intro, the pin file and the rulings miss for ``pin``; empty when none."""
    problems = []
    if pin.tag is not None:
        problems.append(f"the pin follows upstream main ({FOLLOW_MAIN}) and names no release tag")
    sentence = current_pin_sentence(pin.commit, pin.recorded_on)
    if sentence not in intro.splitlines():
        problems.append(f"17.10 intro lacks the line {sentence!r}")
    if FOLLOW_MAIN not in pin_text:
        problems.append(f"the pin file does not name {FOLLOW_MAIN}")
    if FOLLOW_MAIN not in rows:
        problems.append(f"no Hermes decision file holds the ruling {FOLLOW_MAIN}")
    return problems
