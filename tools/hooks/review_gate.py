# SPDX-License-Identifier: Apache-2.0
"""Pre-push gate: a pushed branch head needs an independent review with verdict JA.

Owner requirement 23 asks for a local AI review before every push. The review itself runs
outside this tool (an independent, read-only reviewer as in docs/quality/red-team-gate.md);
this gate checks its evidence: the file `.review/<sha>.md` for the pushed commit, whose
last non-empty line is exactly `Urteil: JA`. The folder is git-ignored and the report stays
local; the pull request names the reviewed commit and the verdict in fresh words (section
"Review" of the template), never the raw report, because the body becomes the squash commit.

The pushed commit comes from pre-commit (`PRE_COMMIT_TO_REF`, else
`PRE_COMMIT_LOCAL_BRANCH`), else HEAD. Tag pushes need no report: a tag points to a
commit that was reviewed before its merge. `REVIEW_SKIP=1` is the explicit switch for
contributors from forks; it prints a warning, and a maintainer reviews before the merge
(R-QA-02).
Exit codes: 0 passed or skipped, 1 report missing or verdict not JA.
"""

from __future__ import annotations

import argparse
import os
import subprocess
import sys
from pathlib import Path

REVIEW_DIR = ".review"
VERDICT_YES = "Urteil: JA"


def pushed_sha(repo: Path) -> str:
    """Return the full SHA of the commit being pushed."""
    ref = os.environ.get("PRE_COMMIT_TO_REF") or os.environ.get("PRE_COMMIT_LOCAL_BRANCH") or "HEAD"
    return subprocess.run(
        ["git", "-C", str(repo), "rev-parse", "--verify", f"{ref}^{{commit}}"],
        check=True,
        capture_output=True,
        text=True,
    ).stdout.strip()


def verdict_problem(report: Path) -> str | None:
    """Return why `report` is no valid approval, or None if its last line is the JA verdict."""
    if not report.is_file():
        return "missing"
    lines = [ln.strip() for ln in report.read_text(encoding="utf-8").splitlines() if ln.strip()]
    if not lines or lines[-1] != VERDICT_YES:
        return f"last line is not '{VERDICT_YES}'"
    return None


def main(argv: list[str] | None = None) -> int:
    """Check the review evidence of the pushed commit; returns the process exit code."""
    parser = argparse.ArgumentParser(description="Require review evidence before a push.")
    parser.add_argument("--repo", type=Path, default=Path.cwd())
    args = parser.parse_args(argv)
    if os.environ.get("PRE_COMMIT_REMOTE_BRANCH", "").startswith("refs/tags/"):
        return 0
    if os.environ.get("REVIEW_SKIP") == "1":
        print(
            "review: WARNING REVIEW_SKIP=1 (fork contribution); a maintainer reviews before merge"
        )
        return 0
    sha = pushed_sha(args.repo)
    rel = f"{REVIEW_DIR}/{sha}.md"
    problem = verdict_problem(args.repo / rel)
    if problem is None:
        print(f"review: {rel} approves {sha[:12]}")
        return 0
    print(
        f"review: {rel} {problem}; run the independent review over the pushed diff first "
        "(docs/quality/red-team-gate.md)"
    )
    return 1


if __name__ == "__main__":
    sys.exit(main())
