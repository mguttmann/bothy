# SPDX-License-Identifier: Apache-2.0
"""How the pin is named in spec 09 17.10 and in the rulings (D-HERMES-MAIN-3).

A pin with a release tag names its tag and ``commit[:12]`` in the 17.10 intro. A pin without a
tag is allowed only by a ruling that names its full commit: the pin file names that ruling, and
the intro names ``commit[:12]`` and the same ruling.
"""

from __future__ import annotations

import re
from pathlib import Path

from hermes_harness.pin import HermesPin

REPO = Path(__file__).resolve().parents[3]
_RULING = re.compile(r"\bD-[A-Z0-9]+(?:-[A-Z0-9]+)*-[0-9]+\b")


def ruling_rows() -> dict[str, str]:
    """Every ruling table row of the decision files, by its ID."""
    files = [REPO / "docs" / "DECISIONS-FOUNDATION.md"]
    files += sorted((REPO / "docs" / "decisions.d").glob("*.md"))
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
    if pin.commit[:12] not in intro:
        problems.append(f"17.10 intro does not name the commit {pin.commit[:12]}")
    if pin.tag is not None:
        if pin.tag not in intro:
            problems.append(f"17.10 intro does not name the tag {pin.tag}")
        return problems
    named = set(_RULING.findall(pin_text)) & set(_RULING.findall(intro))
    if not any(pin.commit in rows.get(ruling, "") for ruling in named):
        problems.append(
            "a pin without a release tag needs a ruling that names its full commit, "
            "named in the pin file and in the 17.10 intro"
        )
    return problems
