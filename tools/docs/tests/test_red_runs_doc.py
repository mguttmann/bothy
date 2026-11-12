# SPDX-License-Identifier: Apache-2.0
"""docs/quality/gates.md, "Red runs", names the workflows that run on a schedule.

The section says what tells a maintainer about a red run and which runs report nothing
(AUD-CG1/7). Its list of scheduled workflows is held to the workflows that have a `schedule`
trigger, and its sentence that only denylist-main.yml reports its own red run to the jobs that
may write an issue after a failure.
"""

from __future__ import annotations

import re
from pathlib import Path

import yaml

REPO = Path(__file__).resolve().parents[3]
DOC = REPO / "docs" / "quality" / "gates.md"
WORKFLOWS = REPO / ".github" / "workflows"


def _section() -> str:
    text = DOC.read_text(encoding="utf-8")
    return text.split("\n## Red runs\n", 1)[1].split("\n## ", 1)[0]


def _triggers(path: Path) -> dict[str, object]:
    # PyYAML reads the key `on` as True.
    on = yaml.safe_load(path.read_text(encoding="utf-8"))[True]
    return on if isinstance(on, dict) else dict.fromkeys(on if isinstance(on, list) else [on])


def test_the_section_lists_every_scheduled_workflow() -> None:
    sentence = _section().split("The scheduled workflows are", 1)[1].split("(", 1)[0]
    listed = set(re.findall(r"`([a-z0-9-]+\.yml)`", sentence))
    scheduled = {p.name for p in WORKFLOWS.glob("*.yml") if "schedule" in _triggers(p)}
    assert listed == scheduled


def test_only_the_deny_list_scan_of_main_reports_its_own_red_run() -> None:
    """A job that runs after a failure and may write issues is an alert for a red run."""
    alerting = set()
    for path in WORKFLOWS.glob("*.yml"):
        for job in yaml.safe_load(path.read_text(encoding="utf-8"))["jobs"].values():
            writes = (job.get("permissions") or {}).get("issues") == "write"
            if writes and "failure()" in str(job.get("if", "")):
                alerting.add(path.name)
    assert alerting == {"denylist-main.yml"}
    assert "It is the only workflow that reports its own\n  red run." in _section()
