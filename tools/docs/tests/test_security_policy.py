# SPDX-License-Identifier: Apache-2.0
"""SECURITY.md links to the instructions of its reporting path (SEC-ALERTS-2026-10-31).

OpenSSF Scorecard (Security-Policy) rates a policy without a URL or an e-mail address as having
no linked content. The policy has no e-mail address on purpose, so it links the GitHub
instructions for private vulnerability reporting. The pattern is the one Scorecard uses.
"""

from __future__ import annotations

import re
from pathlib import Path

REPO = Path(__file__).resolve().parents[3]
# checks/raw/security_policy.go of ossf/scorecard.
_SCORECARD_URL = re.compile(r"(http|https)://[a-zA-Z0-9./?=_%:-]*")
_REPORTING_DOCS = "https://docs.github.com/en/code-security/"


def test_security_policy_links_the_reporting_instructions() -> None:
    text = (REPO / "SECURITY.md").read_text(encoding="utf-8")
    reporting = text.split("## Reporting a vulnerability", 1)[1].split("\n## ", 1)[0]
    links = [m.group(0) for m in _SCORECARD_URL.finditer(reporting)]
    assert any(link.startswith(_REPORTING_DOCS) for link in links), links
