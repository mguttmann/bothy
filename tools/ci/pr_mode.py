# SPDX-License-Identifier: Apache-2.0
"""Decide whether a ci.yml run gets the checks that need repository secrets (D-DEPS-3).

GitHub gives a run no repository secrets when the pull request comes from a fork, and also
when a dependency bot started it: a run whose actor is `dependabot[bot]` is treated like a
fork run (https://docs.github.com/en/code-security/dependabot/troubleshooting-dependabot/troubleshooting-dependabot-on-github-actions,
retrieved 2026-10-13). Such a run cannot load the private deny list or the SonarQube token.
Instead of failing on the missing secret, it runs the generic deny rules and skips SonarQube
with a visible note, like a fork (R-QA-02). Renovate is treated the same way, so both
dependency bots follow one rule.

A bot run is one where both the pull request author and the actor are a bot of BOT_LOGINS.
When a person starts the run (an edit of title or body, a push to the bot branch), it gets
every check with the secrets; a secret that is missing then fails closed.

Usage:
  pr_mode.py --event E --repo R --head-repo H --author A --actor C
             [--github-output PATH] [--summary PATH]

Writes `private_checks=true|false` to --github-output and, for a reduced run, a notice
annotation on standard output and a line in the step summary.
"""

from __future__ import annotations

import argparse
import sys
from dataclasses import dataclass
from pathlib import Path

# Pull request author logins of the dependency bots; tools/hooks/commit_msg.py BOT_AUTHORS
# lists the same logins (a test keeps both equal).
BOT_LOGINS = ("dependabot[bot]", "renovate[bot]")


@dataclass(frozen=True)
class Mode:
    """Result: whether the secret jobs run, and the note for a reduced run."""

    private_checks: bool
    note: str = ""


def decide(*, event: str, repo: str, head_repo: str, author: str, actor: str) -> Mode:
    """Return the mode of a run from the event payload fields."""
    if event != "pull_request":
        return Mode(private_checks=True)
    if head_repo != repo:
        return Mode(
            private_checks=False,
            note="fork pull request: no repository secrets, so the deny list runs with the "
            "generic rules only and sonar is skipped; a maintainer checks fully before the "
            "merge (docs/quality/gates.md)",
        )
    if author in BOT_LOGINS and actor in BOT_LOGINS:
        return Mode(
            private_checks=False,
            note=f"pull request by {author}, started by {actor}: runs like a fork pull "
            "request (GitHub gives Dependabot runs no repository secrets, and this project "
            "treats Renovate the same way), so the deny list runs with the generic rules "
            "only and sonar is skipped; a maintainer edit of title or body runs every check "
            "before the merge (docs/quality/push-discipline.md)",
        )
    return Mode(private_checks=True)


def main(argv: list[str] | None = None) -> int:
    """Print or write the mode of this run."""
    parser = argparse.ArgumentParser(description="Decide the secret checks of a ci.yml run.")
    parser.add_argument("--event", required=True)
    parser.add_argument("--repo", required=True)
    parser.add_argument("--head-repo", default="")
    parser.add_argument("--author", default="")
    parser.add_argument("--actor", default="")
    parser.add_argument("--github-output", type=Path)
    parser.add_argument("--summary", type=Path)
    args = parser.parse_args(argv)
    mode = decide(
        event=args.event,
        repo=args.repo,
        head_repo=args.head_repo,
        author=args.author,
        actor=args.actor,
    )
    line = f"private_checks={'true' if mode.private_checks else 'false'}\n"
    if args.github_output is not None:
        with args.github_output.open("a", encoding="utf-8") as fh:
            fh.write(line)
    print(line, end="")
    if mode.note:
        print(f"::notice title=Reduced checks::{mode.note}")
        if args.summary is not None:
            with args.summary.open("a", encoding="utf-8") as fh:
                fh.write(f"**Reduced checks:** {mode.note}\n")
    return 0


if __name__ == "__main__":  # pragma: no cover
    sys.exit(main())
