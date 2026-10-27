# SPDX-License-Identifier: Apache-2.0
"""Text of a GitHub release page from .github/release-template.md and the changelog section.

The publish job of .github/workflows/release.yml renders the page with this script before
`gh release create --notes-file` (brief M7-28 item 15). The changelog section of the version is
the only source; the template fixes the order:

1. the summary for users (the `Summary:` paragraph, tools/ci/changelog_summary.py), first;
2. what changed for users: text before the first `###` heading and the Keep a Changelog
   categories (USER_HEADINGS);
3. upgrade notes: the headings of the server release notes of docs/spec/18-release.md
   section 4 (UPGRADE_HEADINGS);
4. technical entries: every other `###` heading of the section;
5. verification: fixed text of the template with the tag.

A part without content gets NONE_LISTED. A section without a summary, or with the placeholder
of release-prep, is an error: no release page without a summary.

Placeholders of the template are `{{name}}` with the names of PLACEHOLDERS; HTML comments of
the template (its usage notes) are removed.

Usage:
  release_notes.py --changelog FILE --version X.Y.Z --tag TAG --repository OWNER/NAME
                   [--template FILE] [--output FILE]
Exit codes: 0 written, 1 the page cannot be rendered (reason on stderr), 2 usage error.
"""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path

import changelog_summary

TEMPLATE = Path(__file__).resolve().parents[2] / ".github" / "release-template.md"
USER_HEADINGS = ("Added", "Changed", "Deprecated", "Removed", "Fixed", "Security")
UPGRADE_HEADINGS = ("Upgrade notes", "Migrations", "Compatibility", "Known issues")
PLACEHOLDERS = (
    "summary",
    "changes",
    "upgrade_notes",
    "technical",
    "tag",
    "version",
    "repository",
)
NONE_LISTED = "None listed in the changelog."
_PLACEHOLDER = re.compile(r"\{\{\s*([A-Za-z_]+)\s*\}\}")
_COMMENT = re.compile(r"<!--.*?-->\n?", re.DOTALL)


class ReleaseNotesError(Exception):
    """The release page cannot be rendered; nothing is written."""


def _blocks(lines: list[str]) -> list[tuple[str | None, list[str]]]:
    """Split section lines into (heading, lines) at `### ` headings; the first may be None."""
    blocks: list[tuple[str | None, list[str]]] = [(None, [])]
    for line in lines:
        if line.startswith("### "):
            blocks.append((line[4:].strip(), [line]))
        else:
            blocks[-1][1].append(line)
    return blocks


def _join(parts: list[list[str]]) -> str:
    """Join blocks of lines with one empty line between them; NONE_LISTED when all are empty."""
    texts = ["\n".join(p).strip() for p in parts]
    return "\n\n".join(t for t in texts if t) or NONE_LISTED


def parts(changelog: str, version: str) -> dict[str, str]:
    """Return the parts of the release page taken from the changelog section of `version`.

    Args:
        changelog: the content of the component's CHANGELOG.md.
        version: the version without `v`, for example `0.2.0`.

    Returns:
        The values of the placeholders `summary`, `changes`, `upgrade_notes` and `technical`.

    Raises:
        ReleaseNotesError: the changelog has no section for `version`, or the section has no
            valid summary (missing, empty or the placeholder of release-prep).
    """
    found = [s for s in changelog_summary.release_sections(changelog) if s.version == version]
    if not found:
        raise ReleaseNotesError(f"CHANGELOG.md has no section [{version}]")
    section = found[0]
    problem = changelog_summary.problem_of(section)
    summary = changelog_summary.summary_of(section)
    if problem is not None or summary is None:
        raise ReleaseNotesError(f"section [{version}]: {problem}")
    body = list(section.body)
    while body and not body[0].strip():
        body.pop(0)
    rest = body[len(changelog_summary.summary_paragraph(section)) :]
    user: list[list[str]] = []
    upgrade: list[list[str]] = []
    technical: list[list[str]] = []
    for heading, lines in _blocks(rest):
        if heading is None or heading in USER_HEADINGS:
            user.append(lines)
        elif heading in UPGRADE_HEADINGS:
            upgrade.append(lines)
        else:
            technical.append(lines)
    return {
        "summary": summary,
        "changes": _join(user),
        "upgrade_notes": _join(upgrade),
        "technical": _join(technical),
    }


def render(template: str, changelog: str, version: str, tag: str, repository: str) -> str:
    """Render the release page.

    Args:
        template: the content of .github/release-template.md.
        changelog: the content of the component's CHANGELOG.md.
        version: the version without `v`.
        tag: the release tag, for example `server/v0.2.0`.
        repository: `owner/name` of the repository, for the links of the verification part.

    Returns:
        The Markdown text of the page, ending with one newline.

    Raises:
        ReleaseNotesError: see `parts`, or the template uses a placeholder that is not one of
            PLACEHOLDERS.
    """
    values = parts(changelog, version) | {"tag": tag, "version": version, "repository": repository}
    text = _COMMENT.sub("", template)
    unknown = sorted({m.group(1) for m in _PLACEHOLDER.finditer(text)} - set(PLACEHOLDERS))
    if unknown:
        raise ReleaseNotesError(f"unknown placeholder in the template: {', '.join(unknown)}")
    text = _PLACEHOLDER.sub(lambda m: values[m.group(1)], text)
    return text.strip() + "\n"


def main(argv: list[str] | None = None) -> int:
    """Command line entry point; see the module documentation."""
    parser = argparse.ArgumentParser(description="Render the text of a GitHub release page.")
    parser.add_argument("--changelog", type=Path, required=True)
    parser.add_argument("--version", required=True)
    parser.add_argument("--tag", required=True)
    parser.add_argument("--repository", required=True)
    parser.add_argument("--template", type=Path, default=TEMPLATE)
    parser.add_argument("--output", type=Path, default=None)
    args = parser.parse_args(argv)
    try:
        page = render(
            args.template.read_text(encoding="utf-8"),
            args.changelog.read_text(encoding="utf-8"),
            args.version,
            args.tag,
            args.repository,
        )
    except (ReleaseNotesError, OSError) as exc:
        print(f"release-notes: {exc}", file=sys.stderr)
        return 1
    if args.output is None:
        sys.stdout.write(page)
    else:
        args.output.write_text(page, encoding="utf-8")
    return 0


if __name__ == "__main__":
    sys.exit(main())
