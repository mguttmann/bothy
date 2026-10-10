# SPDX-License-Identifier: Apache-2.0
"""Validate and apply the repository labels (.github/labels.yml).

labels.py check   label file is valid; issue templates and GOVERNANCE.md use only
                  defined labels
labels.py sync    create or update every label with `gh label create --force`
                  (maintainers; acts on the GitHub repository of the current checkout)
"""

from __future__ import annotations

import argparse
import re
import shlex
import subprocess
import sys
from pathlib import Path
from typing import Any

import yaml

DESCRIPTION_MAX = 100  # GitHub's limit for label descriptions
_COLOR = re.compile(r"^[0-9a-fA-F]{6}$")
# Labels are written in backticks in GOVERNANCE.md, e.g. `status: triage`.
_GOVERNANCE_LABEL = re.compile(r"`((?:type|area|status|priority): [a-z-]+)`")


def load(path: Path) -> list[dict[str, Any]]:
    data = yaml.safe_load(path.read_text(encoding="utf-8")) or []
    if not isinstance(data, list):
        raise ValueError(f"{path}: expected a list of labels")
    return [dict(item) for item in data]


def validate(items: list[dict[str, Any]]) -> list[str]:
    problems: list[str] = []
    seen: set[str] = set()
    for item in items:
        name = str(item.get("name", ""))
        if name in seen:
            problems.append(f"{name}: defined twice")
        seen.add(name)
        if not _COLOR.match(str(item.get("color", ""))):
            problems.append(f"{name}: color must be six hex digits")
        if len(str(item.get("description", ""))) > DESCRIPTION_MAX:
            problems.append(f"{name}: description longer than {DESCRIPTION_MAX} characters")
    return problems


def reference_problems(repo: Path) -> list[str]:
    known = {str(i["name"]) for i in load(repo / ".github" / "labels.yml")}
    problems = []
    governance = repo / "GOVERNANCE.md"
    if governance.is_file():
        for name in _GOVERNANCE_LABEL.findall(governance.read_text(encoding="utf-8")):
            if name not in known:
                problems.append(f"GOVERNANCE.md: unknown label '{name}'")
    for template in sorted((repo / ".github" / "ISSUE_TEMPLATE").glob("*.yml")):
        doc = yaml.safe_load(template.read_text(encoding="utf-8")) or {}
        for name in doc.get("labels", []) if isinstance(doc, dict) else []:
            if name not in known:
                rel = template.relative_to(repo)
                problems.append(f"{rel}: unknown label '{name}'")
    return problems


def sync_commands(items: list[dict[str, Any]]) -> list[list[str]]:
    return [
        [
            "gh",
            "label",
            "create",
            str(i["name"]),
            "--color",
            str(i["color"]),
            "--description",
            str(i["description"]),
            "--force",
        ]
        for i in items
    ]


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Validate or sync repository labels.")
    parser.add_argument("command", choices=("check", "sync"))
    parser.add_argument("--repo", type=Path, default=Path.cwd())
    parser.add_argument("--dry-run", action="store_true", help="print gh commands only")
    args = parser.parse_args(argv)
    items = load(args.repo / ".github" / "labels.yml")
    if args.command == "check":
        problems = validate(items) + reference_problems(args.repo)
        for p in problems:
            print(f"labels: {p}")
        return 1 if problems else 0
    for cmd in sync_commands(items):
        print(shlex.join(cmd))
        if not args.dry_run:
            subprocess.run(cmd, check=True)
    return 0


if __name__ == "__main__":
    sys.exit(main())
