# SPDX-License-Identifier: Apache-2.0
"""Options and documented command of the setup wizard (docs/spec/16 sections 3.1 and 11.1).

Two checks for `make check`:

1. Options. The portability test (M7-14) passes to `install.sh` exactly the options of
   `tests/portability/install-options` (16 section 11.1, interface rule 2). They must equal
   the words after `sh -s --` of the second fenced `sh` block in the section `### 3.1` of the
   spec chapter `docs/spec/16-*.md`, compared as a sequence of options with their values
   (`--yes` is a flag, every other `--x` takes the next word). Only the value of `--storage`
   may differ, because the test sets it per run, and `--bundle <file>` is ignored on both
   sides, because the test adds it for the source `release-download`. Where the script comes
   from (the `curl ... |` part) does not matter.
2. Documented command. README.md and docs/operations/install-compose.md each name at least
   one URL `https://github.com/<owner>/bothy/releases/download/v<X>/install.sh`, and every
   such URL has X equal to the content of VERSION. `make release-prep` of the product
   rewrites these URLs together with VERSION (tools/ci/release_prep.py).

Usage:
  install_options_check.py check [--repo <dir>]
Exit codes: 0 both checks pass, 1 findings (named on stderr), 2 usage error.
"""

from __future__ import annotations

import argparse
import re
import shlex
import sys
from pathlib import Path

OPTIONS_FILE = "tests/portability/install-options"
SPEC_GLOB = "docs/spec/16-*.md"
# The files whose installation command must name the product version in VERSION.
DOCUMENTED_COMMAND_FILES = ("README.md", "docs/operations/install-compose.md")
# The documented command's URL; the owner is a group of its own because V1-03 replaces the
# placeholder OWNER, and both checks must keep working after that.
INSTALL_URL = re.compile(
    r"https://github\.com/(?P<owner>[A-Za-z0-9-]+)/bothy/releases/download/"
    r"v(?P<version>[0-9A-Za-z.+-]+)/install\.sh"
)
FLAGS = frozenset({"--yes"})
# The test sets --storage per run (only its value may differ) and adds --bundle.
ANY_VALUE = frozenset({"--storage"})
IGNORED = frozenset({"--bundle"})
_SECTION = re.compile(r"^### 3\.1\b.*$", re.MULTILINE)
_NEXT_HEADING = re.compile(r"^#{1,3} ", re.MULTILINE)
_SH_BLOCK = re.compile(r"^```sh[ \t]*\n(.*?)^```[ \t]*$", re.MULTILINE | re.DOTALL)
_INVOCATION = ["sh", "-s", "--"]

Option = tuple[str, str | None]


class OptionsError(Exception):
    """The options cannot be read; the message names the file and the reason."""


def spec_option_words(text: str, name: str = SPEC_GLOB) -> list[str]:
    """Return the words after `sh -s --` of the second `sh` block in section 3.1.

    Continuation lines (a `\\` at the end of a line, a line starting with the `|` pipe) are
    joined into one command before it is split into shell words.

    Args:
        text: the spec chapter 16.
        name: the chapter's path, for messages.

    Returns:
        The words after `sh -s --`, for example `["--yes", "--public-url", ...]`.

    Raises:
        OptionsError: no section 3.1, fewer than two `sh` blocks in it, or no `sh -s --`
            in the second one.
    """
    head = _SECTION.search(text)
    if head is None:
        raise OptionsError(f"{name} has no section '### 3.1'")
    nxt = _NEXT_HEADING.search(text, head.end())
    blocks = _SH_BLOCK.findall(text[head.end() : nxt.start() if nxt else len(text)])
    if len(blocks) < 2:
        raise OptionsError(f"{name}: section 3.1 has no second fenced sh block")
    # A trailing backslash and a leading pipe both continue the command; joining the lines
    # with spaces gives the same words as the shell, since shlex keeps `|` a word of its own.
    command = " ".join(line.rstrip().removesuffix("\\") for line in blocks[1].splitlines())
    words = _split(command, name)
    for i in range(len(words) - len(_INVOCATION) + 1):
        if words[i : i + len(_INVOCATION)] == _INVOCATION:
            return words[i + len(_INVOCATION) :]
    raise OptionsError(f"{name}: the second sh block of section 3.1 has no 'sh -s --'")


def options_file_words(text: str, name: str = OPTIONS_FILE) -> list[str]:
    """Return the shell words of the one option line of the options file.

    Lines starting with `#` and blank lines are comments. The option line holds plain words
    without quotes, so that the portability test can split it like the shell does.

    Raises:
        OptionsError: not exactly one option line, or quotes or backslashes in it.
    """
    lines = [ln for ln in text.splitlines() if ln.strip() and not ln.lstrip().startswith("#")]
    if len(lines) != 1:
        raise OptionsError(f"{name}: expected exactly one line of options, found {len(lines)}")
    if re.search(r"['\"\\]", lines[0]):
        raise OptionsError(f"{name}: the options must be shell words without quote characters")
    return _split(lines[0], name)


def _split(text: str, name: str) -> list[str]:
    try:
        return shlex.split(text)
    except ValueError as exc:
        raise OptionsError(f"{name}: {exc}") from exc


def parse_options(words: list[str], name: str) -> list[Option]:
    """Turn shell words into options with their values, in order.

    `--yes` is a flag (value None); every other `--x` takes the next word. `--bundle` and
    its value are dropped and the value of `--storage` becomes None, because the
    portability test changes both (16 section 11.1, interface rule 2).

    Args:
        words: the words after `sh -s --`.
        name: the source of the words, for messages.

    Returns:
        The options, for example `[("--yes", None), ("--edge", "direct")]`.

    Raises:
        OptionsError: a word that is not an option where one is expected, or an option
            without its value.
    """
    out: list[Option] = []
    i = 0
    while i < len(words):
        word = words[i]
        if not word.startswith("--"):
            raise OptionsError(f"{name}: '{word}' is not an option")
        if word in FLAGS:
            out.append((word, None))
            i += 1
            continue
        if i + 1 >= len(words) or words[i + 1].startswith("--"):
            raise OptionsError(f"{name}: option {word} has no value")
        if word not in IGNORED:
            out.append((word, None if word in ANY_VALUE else words[i + 1]))
        i += 2
    return out


def compare_options(spec: list[Option], given: list[Option]) -> list[str]:
    """Return the differences between the spec's options and those of the options file.

    Args:
        spec: options of the second block of 16 section 3.1.
        given: options of the options file.

    Returns:
        One finding per missing, extra, duplicate or changed option, or one for a different
        order when the options and values agree; empty when both sequences are equal.

    Example: `compare_options([("--edge", "direct")], [("--edge", "tailscale")])` names
    `--edge` with both values.
    """
    if spec == given:
        return []
    findings: list[str] = []
    for label, options in (("the spec block", spec), (OPTIONS_FILE, given)):
        names = [o for o, _ in options]
        findings += [
            f"{label} gives {o} twice" for o in sorted({o for o in names if names.count(o) > 1})
        ]
    want, have = dict(spec), dict(given)
    for option, value in want.items():
        if option not in have:
            findings.append(f"{OPTIONS_FILE}: option {option} of the spec block is missing")
        elif have[option] != value:
            findings.append(
                f"{OPTIONS_FILE}: option {option} is '{have[option]}', the spec block says"
                f" '{value}'"
            )
    findings += [
        f"{OPTIONS_FILE}: option {o} is not in the spec block" for o in have if o not in want
    ]
    if not findings:
        first = next(a for a, b in zip(spec, given, strict=True) if a != b)
        findings.append(
            f"{OPTIONS_FILE}: the options are in a different order than in the spec block"
            f" (first difference at {first[0]})"
        )
    return findings


def option_findings(repo: Path) -> list[str]:
    """Check 1: the options file equals the second block of 16 section 3.1.

    Args:
        repo: the repository root.

    Returns:
        The findings; empty when the options agree.
    """
    chapters = sorted(repo.glob(SPEC_GLOB))
    if len(chapters) != 1:
        return [f"expected exactly one spec chapter {SPEC_GLOB}, found {len(chapters)}"]
    spec_name = chapters[0].relative_to(repo).as_posix()
    options = repo / OPTIONS_FILE
    if not options.is_file():
        return [f"{OPTIONS_FILE} is missing"]
    try:
        spec = parse_options(
            spec_option_words(chapters[0].read_text(encoding="utf-8"), spec_name), spec_name
        )
        given = parse_options(options_file_words(options.read_text(encoding="utf-8")), OPTIONS_FILE)
    except OptionsError as exc:
        return [str(exc)]
    return compare_options(spec, given)


def documented_command_findings(repo: Path) -> list[str]:
    """Check 2: every documented installation command names the version in VERSION.

    Args:
        repo: the repository root.

    Returns:
        A finding per file that is missing or names no command, and per URL whose version
        differs from VERSION (with file and line).
    """
    version_file = repo / "VERSION"
    if not version_file.is_file():
        return ["VERSION is missing"]
    version = version_file.read_text(encoding="utf-8").strip()
    findings: list[str] = []
    for name in DOCUMENTED_COMMAND_FILES:
        path = repo / name
        if not path.is_file():
            findings.append(f"{name} is missing; it must name the installation command")
            continue
        found = False
        for number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), start=1):
            for m in INSTALL_URL.finditer(line):
                found = True
                if m["version"] != version:
                    findings.append(
                        f"{name}:{number}: the installation command names v{m['version']},"
                        f" VERSION is {version}"
                    )
        if not found:
            findings.append(
                f"{name} names no installation command"
                " (https://github.com/OWNER/bothy/releases/download/v<VERSION>/install.sh)"
            )
    return findings


def check(repo: Path) -> list[str]:
    """Run both checks.

    Args:
        repo: the repository root.

    Returns:
        All findings, options first; empty when the repository passes.
    """
    return option_findings(repo) + documented_command_findings(repo)


def main(argv: list[str] | None = None) -> int:
    """Command line entry point; see the module docstring for usage and exit codes."""
    parser = argparse.ArgumentParser(description="Check the setup wizard options and command.")
    sub = parser.add_subparsers(dest="cmd", required=True)
    p_check = sub.add_parser("check")
    p_check.add_argument("--repo", type=Path, default=Path.cwd())
    args = parser.parse_args(argv)
    findings = check(args.repo)
    for finding in findings:
        print(f"install-options: {finding}", file=sys.stderr)
    if findings:
        return 1
    print("install-options: options and documented installation command agree with the spec")
    return 0


if __name__ == "__main__":
    sys.exit(main())
