# SPDX-License-Identifier: Apache-2.0
"""Changelog and ruling fragments: one file per ticket instead of one shared file.

Parallel pull requests that all append to the same `CHANGELOG.md` or `docs/DECISIONS.md`
conflict with each other, and GitHub's merge ignores git merge drivers. So every ticket writes
its own files (docs/quality/gates.md "Fragments", docs/DECISIONS.md PL/24):

- `<component>/changelog.d/<ticket>.md` next to the `CHANGELOG.md` of each release component
  of tools/ci/components.py (`changelog.d/<ticket>.md` for the repository as a whole): only
  Keep a Changelog headings `### Added`, `### Changed`, `### Deprecated`, `### Removed`,
  `### Fixed`, `### Security`, each followed by list items (`- `, continuation lines indented).
  `make release-prep` (and `make changelog-preview` without writing) puts the entries under
  "Unreleased" of the component's `CHANGELOG.md`, in file name order after the entries already
  there, and deletes the fragments.
- `docs/decisions.d/<ticket>.md`: the rulings of one ticket, starting with a heading
  `# <ticket> <title>`; the docs site lists them on a generated index page
  (tools/docs/mkdocs_ruling_fragments.py).

`check` is the gate `make fragments-check`: every fragment has a valid name and form and sits
where it belongs, `docs/DECISIONS.md` has no section of a ticket or an integration wave, and a
pull request (`--range BASE..HEAD`, the merge base of the two against HEAD) adds no line under
"Unreleased" of a `CHANGELOG.md`.

Usage:
  fragments.py check [--range BASE..HEAD] [--repo DIR]
  fragments.py assemble --component NAME [--repo DIR] [--print]
Exit codes: 0 fine, 1 a finding (named on stderr), 2 usage error.
"""

from __future__ import annotations

import argparse
import difflib
import re
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path

import components

CHANGELOG_DIR = "changelog.d"
DECISIONS = "docs/DECISIONS.md"
DECISIONS_DIR = "docs/decisions.d"
# The index page of docs/decisions.d exists only in the site build.
DECISIONS_INDEX = "index.md"
# Keep a Changelog 1.1.0 kinds of change, in the order of its sections.
KINDS = ("Added", "Changed", "Deprecated", "Removed", "Fixed", "Security")
# A ticket ID (M2-05) or another short name of letters, digits, dots, dashes and underscores.
NAME = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]*\.md$")
_KIND = re.compile(r"^### (.+?)\s*$")
_ITEM = re.compile(r"^- \S")
_UNRELEASED = re.compile(r"^## \[Unreleased\][ \t]*$", re.MULTILINE)
_H2 = re.compile(r"^## ", re.MULTILINE)
_H3 = re.compile(r"^### ", re.MULTILINE)
# A section of one ticket (`## M2-05 ...`) or of an integration wave (`## Integration ...`).
_TICKET_SECTION = re.compile(r"^## (?:[A-Z]+\d*-\d+\b|Integration\b).*$", re.MULTILINE)


class FragmentError(Exception):
    """A fragment or a changelog cannot be read or assembled."""


@dataclass(frozen=True)
class Fragment:
    """One changelog fragment: its path and its entries per kind of change."""

    path: Path
    entries: dict[str, list[str]]


def changelog_dirs() -> list[str]:
    """Return the directories of the release components, `.` for the repository."""
    return sorted({directory for directory, _ in components.RELEASE_COMPONENTS.values()})


def parse_changelog_fragment(text: str, name: str) -> dict[str, list[str]]:
    r"""Return the entries of a changelog fragment, one string per list item, per kind.

    Args:
        text: the content of the fragment.
        name: its path, for the messages.

    Returns:
        Kind of change to its list items (an item keeps its indented continuation lines).

    Raises:
        FragmentError: text before the first heading, an unknown or repeated heading, a line
            that is neither a list item nor indented, a heading without items or no entry.

    Example: `parse_changelog_fragment("### Fixed\n\n- A crash.\n", "x.md")` is
    `{"Fixed": ["- A crash."]}`.
    """
    entries: dict[str, list[str]] = {}
    kind: str | None = None
    for number, line in enumerate(text.splitlines(), start=1):
        where = f"{name}:{number}"
        heading = _KIND.match(line)
        if heading:
            kind = heading.group(1)
            if kind not in KINDS:
                raise FragmentError(f"{where}: '### {kind}' is not one of {', '.join(KINDS)}")
            if kind in entries:
                raise FragmentError(f"{where}: '### {kind}' appears twice")
            entries[kind] = []
            continue
        if not line.strip():
            continue
        if kind is None:
            raise FragmentError(f"{where}: text before the first '### <kind>' heading")
        if _ITEM.match(line):
            entries[kind].append(line)
        elif line.startswith("  ") and entries[kind]:
            entries[kind][-1] += "\n" + line
        else:
            raise FragmentError(f"{where}: expected a list item '- ...' or an indented line")
    for kind, items in entries.items():
        if not items:
            raise FragmentError(f"{name}: '### {kind}' has no list item")
    if not entries:
        raise FragmentError(f"{name}: no entry; write '### <kind>' and list items")
    return entries


def _natural(path: Path) -> list[object]:
    return [int(p) if p.isdigit() else p.lower() for p in re.split(r"(\d+)", path.name)]


def read_fragments(directory: Path) -> list[Fragment]:
    """Return the changelog fragments of `directory/changelog.d` in file name order."""
    folder = directory / CHANGELOG_DIR
    if not folder.is_dir():
        return []
    found = (p for p in folder.iterdir() if p.is_file() and p.suffix == ".md")
    paths = sorted((p for p in found if not p.name.startswith(".")), key=_natural)
    return [
        Fragment(p, parse_changelog_fragment(p.read_text(encoding="utf-8"), str(p))) for p in paths
    ]


def assemble(text: str, fragments: list[Fragment]) -> str:
    """Return the changelog with the entries of `fragments` under "Unreleased".

    Each item goes to the end of its `### <kind>` subsection of "Unreleased"; a missing
    subsection is added in the order of KINDS. Everything else stays byte for byte.

    Raises:
        FragmentError: the changelog has no section '## [Unreleased]'.
    """
    head = _UNRELEASED.search(text)
    if head is None:
        raise FragmentError("CHANGELOG.md has no section '## [Unreleased]'")
    nxt = _H2.search(text, head.end())
    end = nxt.start() if nxt else len(text)
    body = text[head.end() : end]
    first = _H3.search(body)
    intro = body[: first.start()] if first else body
    sections: list[tuple[str, str]] = []
    if first:
        parts = re.split(r"(?m)^(?=### )", body[first.start() :])
        for part in parts:
            if part:
                title = part.splitlines()[0][4:].strip()
                sections.append((title, part.rstrip("\n")))
    for fragment in fragments:
        for kind, items in fragment.entries.items():
            block = "\n".join(items)
            for i, (title, part) in enumerate(sections):
                if title == kind:
                    sections[i] = (title, f"{part}\n{block}")
                    break
            else:
                new = (kind, f"### {kind}\n\n{block}")
                rank = KINDS.index(kind)
                at = next(
                    (
                        i
                        for i, (title, _) in enumerate(sections)
                        if title in KINDS and KINDS.index(title) > rank
                    ),
                    len(sections),
                )
                sections.insert(at, new)
    if not sections:
        return text
    intro = intro.rstrip("\n") + "\n\n" if intro.strip() else "\n\n"
    new_body = intro + "\n\n".join(part for _, part in sections) + "\n"
    rest = text[end:]
    return text[: head.end()] + new_body + ("\n" + rest if rest else "")


def assemble_component(repo: Path, component: str) -> tuple[str, list[Path]]:
    """Return the assembled CHANGELOG.md of `component` and the fragment files it used.

    Raises:
        FragmentError: unknown component, no CHANGELOG.md, or a fragment that cannot be read.
    """
    if component not in components.RELEASE_COMPONENTS:
        raise FragmentError(f"unknown release component '{component}'")
    directory = repo / components.RELEASE_COMPONENTS[component][0]
    changelog = directory / "CHANGELOG.md"
    fragments = read_fragments(directory)
    if not changelog.is_file():
        if fragments:
            raise FragmentError(f"{changelog.relative_to(repo)} is missing")
        return "", []
    text = changelog.read_text(encoding="utf-8")
    return assemble(text, fragments), [f.path for f in fragments]


def _git(repo: Path, *args: str, check: bool = True) -> str:
    out = subprocess.run(
        ["git", "-C", str(repo), *args], check=check, capture_output=True, text=True
    )
    return out.stdout


def _files(repo: Path) -> list[str]:
    """Return the tracked and untracked, not ignored files of `repo` that exist.

    A fragment that release-prep deleted before the release commit is gone.
    """
    out = _git(repo, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
    return sorted({rel for rel in out.split("\0") if rel and (repo / rel).is_file()})


def check_tree(repo: Path) -> list[str]:
    """Return the problems of the fragment files and the ruling log of `repo`."""
    problems: list[str] = []
    allowed = {"" if d == "." else d for d in changelog_dirs()}
    for rel in _files(repo):
        path = Path(rel)
        if path.parent.name == CHANGELOG_DIR:
            owner = path.parent.parent.as_posix()
            owner = "" if owner == "." else owner
            if owner not in allowed:
                problems.append(
                    f"{rel}: changelog.d belongs next to a release component's CHANGELOG.md"
                )
                continue
            if not NAME.match(path.name):
                problems.append(f"{rel}: name a fragment <ticket>.md (letters, digits, . _ -)")
                continue
            if not (repo / path.parent.parent / "CHANGELOG.md").is_file():
                problems.append(f"{rel}: {path.parent.parent / 'CHANGELOG.md'} is missing")
                continue
            try:
                parse_changelog_fragment((repo / path).read_text(encoding="utf-8"), rel)
            except FragmentError as exc:
                problems.append(str(exc))
        elif path.parent.as_posix() == DECISIONS_DIR or rel.startswith(DECISIONS_DIR + "/"):
            if path.parent.as_posix() != DECISIONS_DIR:
                problems.append(f"{rel}: no folders under {DECISIONS_DIR}")
            elif path.name == DECISIONS_INDEX:
                problems.append(f"{rel}: the index page is generated by the site build")
            elif not NAME.match(path.name):
                problems.append(f"{rel}: name a ruling fragment <ticket>.md")
            else:
                first = (repo / path).read_text(encoding="utf-8").split("\n", 1)[0]
                if not re.match(r"^# \S", first):
                    problems.append(f"{rel}: the first line is the heading '# <ticket> <title>'")
    log = repo / DECISIONS
    if log.is_file():
        for heading in _TICKET_SECTION.findall(log.read_text(encoding="utf-8")):
            problems.append(
                f"{DECISIONS}: has the ticket section '{heading}'; move it to"
                f" {DECISIONS_DIR}/<ticket>.md (docs/quality/gates.md, Fragments)"
            )
    return problems


def _show(repo: Path, rev: str, rel: str) -> str:
    proc = subprocess.run(
        ["git", "-C", str(repo), "show", f"{rev}:{rel}"],
        capture_output=True,
        text=True,
        check=False,
    )
    return proc.stdout if proc.returncode == 0 else ""


def _unreleased_lines(text: str) -> list[str]:
    head = _UNRELEASED.search(text)
    if head is None:
        return []
    nxt = _H2.search(text, head.end())
    return text[head.end() : nxt.start() if nxt else len(text)].splitlines()


def added_unreleased(before: str, after: str) -> list[str]:
    """Return the non-blank lines `after` adds under "Unreleased" compared to `before`.

    An edited line replaces one line and adds none; only lines beyond the replaced ones count.
    """
    old, new = _unreleased_lines(before), _unreleased_lines(after)
    added: list[str] = []
    matcher = difflib.SequenceMatcher(a=old, b=new, autojunk=False)
    for op, a0, a1, b0, b1 in matcher.get_opcodes():
        if op == "insert" or (op == "replace" and b1 - b0 > a1 - a0):
            start = b0 + (a1 - a0 if op == "replace" else 0)
            added += [line for line in new[start:b1] if line.strip()]
    return added


def check_range(repo: Path, rev_range: str) -> list[str]:
    """Return the problems of the changes between the merge base of `rev_range` and its head."""
    base, _, head = rev_range.partition("..")
    head = head or "HEAD"
    merge_base = _git(repo, "merge-base", base, head).strip()
    problems: list[str] = []
    for directory in changelog_dirs():
        rel = "CHANGELOG.md" if directory == "." else f"{directory}/CHANGELOG.md"
        fragment = "changelog.d" if directory == "." else f"{directory}/changelog.d"
        for line in added_unreleased(_show(repo, merge_base, rel), _show(repo, head, rel)):
            problems.append(
                f"{rel}: adds '{line[:60]}' under Unreleased; write {fragment}/<ticket>.md"
                " instead (docs/quality/gates.md, Fragments)"
            )
    return problems


def main(argv: list[str] | None = None) -> int:
    """Command line entry point; see the module documentation."""
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    sub = parser.add_subparsers(dest="command", required=True)
    check = sub.add_parser("check")
    check.add_argument("--range", default="")
    check.add_argument("--repo", type=Path, default=Path.cwd())
    build = sub.add_parser("assemble")
    build.add_argument("--component", required=True)
    build.add_argument("--repo", type=Path, default=Path.cwd())
    build.add_argument("--print", action="store_true", dest="dry")
    args = parser.parse_args(argv)
    try:
        if args.command == "check":
            problems = check_tree(args.repo)
            if args.range:
                problems += check_range(args.repo, args.range)
            for problem in problems:
                print(f"fragments: {problem}", file=sys.stderr)
            return 1 if problems else 0
        text, used = assemble_component(args.repo, args.component)
    except (FragmentError, subprocess.CalledProcessError) as exc:
        print(f"fragments: {exc}", file=sys.stderr)
        return 1
    if args.dry:
        sys.stdout.write(text)
        return 0
    if used:
        directory = components.RELEASE_COMPONENTS[args.component][0]
        (args.repo / directory / "CHANGELOG.md").write_text(text, encoding="utf-8")
        for path in used:
            path.unlink()
    print(f"fragments: {len(used)} fragment(s) assembled for {args.component}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
