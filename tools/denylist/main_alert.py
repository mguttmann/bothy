# SPDX-License-Identifier: Apache-2.0
"""Open or update the issue for a red run of .github/workflows/denylist-main.yml.

A failed deny-list scan of main must not go unnoticed: the next push scans only its own
range and can be green. The job `alert` of that workflow runs after a failed scan on main
and calls `issue`: it opens an issue with the label LABEL, or comments on the open one.

The issue is public, and the private list is a secret. So the text names only the run, the
commit, the number of findings per kind and the masked locations from the scanner summary
(`denylist_scan.py --summary`: file:line, path or commit id), never a matched term, a rule
or a rule line. The summary is parsed defensively: unknown fields are dropped, and a
location keeps only safe characters and a bounded length.

Commands:
  output FILE      print `summary=<compact JSON>` for $GITHUB_OUTPUT (`summary=` if FILE is
                   missing or unreadable)
  issue [--dry-run]
                   open or update the issue from SUMMARY, RUN_URL, REPO, EVENT_NAME and SHA in
                   the environment through the GitHub CLI (GH_TOKEN); --dry-run prints the
                   gh commands and the body instead of running them
Exit codes: 0 done, 2 missing environment.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import shlex
import subprocess
import sys
from collections.abc import Callable
from pathlib import Path
from typing import Any

LABEL = "denylist-main"
LABEL_COLOR = "B60205"
LABEL_DESCRIPTION = "A deny-list scan of main failed (denylist-main.yml)"
TITLE = "denylist-main: deny-list scan of main failed"
DOC = 'docs/quality/push-discipline.md, section "Deny-list hit on `main`"'
MAX_LOCATIONS = 50
MAX_LOCATION_LENGTH = 200
# A location from the scanner: path characters, spaces, ':' and '@' for line and offset.
# Backticks, newlines, '<', '>' and '#' are dropped so that the text stays inside its block.
_UNSAFE = re.compile(r"[^A-Za-z0-9 ._/:@*+=,()\[\]{}~-]")
_KIND = re.compile(r"^[a-z][a-z-]{0,39}$")

Gh = Callable[[list[str], "str | None"], str]


def _summary(raw: str) -> dict[str, Any] | None:
    """Return the usable part of a scanner summary, or None if it has no findings data."""
    try:
        data = json.loads(raw) if raw.strip() else None
    except json.JSONDecodeError:
        return None
    if not isinstance(data, dict) or data.get("exit") != 1:
        return None
    findings = data.get("findings")
    if not isinstance(findings, int) or isinstance(findings, bool) or findings < 1:
        return None
    raw_kinds = data.get("kinds")
    kinds = {
        k: v
        for k, v in (raw_kinds.items() if isinstance(raw_kinds, dict) else [])
        if isinstance(k, str) and _KIND.match(k) and isinstance(v, int) and v > 0
    }
    raw_locations = data.get("locations")
    locations = [
        _UNSAFE.sub("", loc)[:MAX_LOCATION_LENGTH]
        for loc in (raw_locations if isinstance(raw_locations, list) else [])
        if isinstance(loc, str)
    ]
    locations = [loc for loc in locations if loc.strip()][:MAX_LOCATIONS]
    truncated = data.get("truncated")
    more = truncated if isinstance(truncated, int) and truncated > 0 else 0
    return {"findings": findings, "kinds": kinds, "locations": locations, "more": more}


def issue_body(raw_summary: str, run_url: str, event: str, sha: str) -> str:
    """Return the issue or comment text: run, commit, counts and locations, no terms."""
    lines = [
        f"The deny-list scan of `main` failed in run {run_url}",
        "",
        f"- Event: `{_UNSAFE.sub('', event)}`",
        f"- Commit: `{_UNSAFE.sub('', sha)}`",
    ]
    data = _summary(raw_summary)
    if data is None:
        lines += [
            "",
            "The run left no finding summary (a configuration error such as a missing list, "
            "or a step before the scan failed). Open the run for the log.",
        ]
    else:
        kinds = ", ".join(f"{k} {v}" for k, v in sorted(data["kinds"].items()))
        lines.append(f"- Findings: {data['findings']}" + (f" ({kinds})" if kinds else ""))
        lines += [
            "",
            "Locations (file:line, path or commit id; the matched text is never shown here):",
            "",
            "```text",
            *data["locations"],
            "```",
        ]
        if data["more"]:
            lines.append(f"... and {data['more']} more locations (see the run).")
    lines += [
        "",
        f"Follow {DOC}: look at the hit locally with `--reveal`, never in CI or here. "
        "Close this issue when the scan of `main` is green again.",
        "",
    ]
    return "\n".join(lines)


def run_gh(args: list[str], stdin: str | None = None) -> str:
    """Run the GitHub CLI and return its standard output (raises on failure)."""
    result = subprocess.run(
        ["gh", *args], input=stdin, capture_output=True, text=True, check=True, timeout=120
    )
    return result.stdout


def open_or_update(gh: Gh, repo: str, body: str) -> None:
    """Comment on the open issue with LABEL, or open one; the label is created if missing."""
    gh(
        [
            "label",
            "create",
            LABEL,
            "--repo",
            repo,
            "--color",
            LABEL_COLOR,
            "--description",
            LABEL_DESCRIPTION,
            "--force",
        ],
        None,
    )
    found = gh(
        [
            "issue",
            "list",
            "--repo",
            repo,
            "--label",
            LABEL,
            "--state",
            "open",
            "--json",
            "number",
            "--jq",
            "sort_by(.number) | .[0].number // empty",
        ],
        None,
    ).strip()
    if found.isdigit():
        gh(["issue", "comment", found, "--repo", repo, "--body-file", "-"], body)
    else:
        gh(
            [
                "issue",
                "create",
                "--repo",
                repo,
                "--title",
                TITLE,
                "--label",
                LABEL,
                "--body-file",
                "-",
            ],
            body,
        )


def _dry_gh(args: list[str], stdin: str | None = None) -> str:
    print("gh " + shlex.join(args))
    if stdin is not None:
        print(stdin)
    return ""


def _output(path: Path) -> int:
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError):
        print("summary=")
        return 0
    print("summary=" + json.dumps(data, separators=(",", ":")))
    return 0


def main(argv: list[str] | None = None) -> int:
    """Command line entry point (see the module docstring)."""
    parser = argparse.ArgumentParser(description="Issue for a red denylist-main run.")
    sub = parser.add_subparsers(dest="command", required=True)
    out = sub.add_parser("output")
    out.add_argument("path", type=Path)
    issue = sub.add_parser("issue")
    issue.add_argument("--dry-run", action="store_true")
    args = parser.parse_args(sys.argv[1:] if argv is None else argv)
    if args.command == "output":
        return _output(args.path)
    env = os.environ
    missing = [n for n in ("RUN_URL", "REPO", "EVENT_NAME", "SHA") if not env.get(n)]
    if missing:
        print(f"denylist-main alert: missing {', '.join(missing)}", file=sys.stderr)
        return 2
    body = issue_body(env.get("SUMMARY", ""), env["RUN_URL"], env["EVENT_NAME"], env["SHA"])
    open_or_update(_dry_gh if args.dry_run else run_gh, env["REPO"], body)
    return 0


if __name__ == "__main__":
    sys.exit(main())
