# SPDX-License-Identifier: Apache-2.0
r"""Fail on German text in the tracked files of the repository.

All repository content is English (ruling D-L5). This gate reads every tracked text file and
reports two signs of German:

- German prose: a run of `WINDOW` consecutive words of which at least `MIN_HITS` are German
  function words (articles, pronouns, conjunctions, prepositions, auxiliary verbs). The list
  leaves out words that English text uses as well ("die", "was", "also", "an", "so", "will"),
  so an English sentence does not reach the threshold. One finding per German passage.
- A word with an umlaut or sharp s (a, o, u with diaeresis, sharp s), which English text does
  not need; also when JSON or code writes the letter as an escape such as `\u00fc`.

The allow list (default tools/docs/language-allow.txt) names exceptions: a line `path: <glob>`
excludes the matching files (the German-English glossary, German test samples, this module
with its word list), any other line is a word that may appear verbatim (a proper name).
German that the product itself specifies, such as the output of the `de` locale that a golden
test holds byte for byte, stands between a marker line `language-check: off` and a marker line
`language-check: on`; those lines are skipped. A marker line holds only the marker, optionally
inside a comment (`<!-- ... -->`, `#`, `//`, `%%`), so prose that names a marker is no marker.
An `off` marker without a later `on` marker is a finding.
Binary files (NUL byte in the first block) and text that is not UTF-8 are skipped;
tools/hooks/text_rules.py reports the latter.

Not covered: a single German word without umlaut inside English text (a noun such as
"Abschnitt"). Review and docs/glossary.md handle those.

Usage: python tools/docs/language_check.py [--root DIR] [--allow FILE] [FILE...]
(exit 1 on a finding; without FILE arguments every tracked file of DIR is checked)
"""

from __future__ import annotations

import argparse
import fnmatch
import re
import subprocess
import sys
from collections.abc import Iterable
from dataclasses import dataclass
from pathlib import Path

REPO = Path(__file__).resolve().parents[2]
DEFAULT_ALLOW = REPO / "tools" / "docs" / "language-allow.txt"
WINDOW = 12
MIN_HITS = 4
_BINARY_PROBE = 8192
# German function words that are not also common English words or code tokens; one string
# keeps the list readable.
GERMAN_WORDS = frozenset(
    """
    aber alle allen aller alles als auch auf aus bei beim beide beiden bereits bzw dabei
    dadurch dafür daher damit danach dann darf darauf darin darum das dass davon dazu dem denn
    der deren derselbe dessen dich die dies diese diesem diesen dieser dieses doch dort durch
    dürfen ein eine einem einen einer eines einmal etwa euch falls für gegen gibt haben hat
    hatte hier ihm ihn ihnen ihr ihre ihrem ihren ihrer ihres immer indem jede jedem jeden
    jeder jedes jetzt kann kein keine keinem keinen keiner können könnte mehr mich mit
    muss müssen nach nicht nichts noch nur ob oder ohne seine seinem seinen seiner seit
    selbst sich sie sind soll sollen sondern sonst sowie über und uns unser unter vom von
    vor wann warum weil welche welchem welchen welcher welches wenn werden wer wie wieder
    wir wird wurde wurden während zum zur zwischen zwar
    """.split()  # noqa: SIM905 (a word list, not a structure)
)
_UMLAUT = re.compile(r"[äöüÄÖÜß]")
_ESCAPED_UMLAUT = re.compile(r"\\u00(e4|f6|fc|c4|d6|dc|df)", re.IGNORECASE)
_WORD = re.compile(r"[A-Za-zÄÖÜäöüß]+(?:-[A-Za-zÄÖÜäöüß]+)*")
# A word with an umlaut inside markup or code (`[word](x.md)`, `title="word"`, `f("word")`).
_UMLAUT_WORD = re.compile(r"[A-Za-zÄÖÜäöüß]*[ÄÖÜäöüß][A-Za-zÄÖÜäöüß]*(?:-[A-Za-zÄÖÜäöüß]+)*")
_STRIP = "\"'`*_()[]{}<>.,;:!?|/\\#=+~^"
OFF = "language-check: off"
ON = "language-check: on"
_MARKER = re.compile(r"^\s*(?:<!--|#|//|%%)?\s*language-check: (off|on)\s*(?:-->)?\s*$")


@dataclass(frozen=True)
class AllowList:
    """Excluded path globs and words that may appear verbatim."""

    paths: tuple[str, ...]
    words: frozenset[str]

    def excludes(self, name: str) -> bool:
        """Return True if the repository-relative path `name` matches an excluded glob."""
        return any(fnmatch.fnmatchcase(name, glob) for glob in self.paths)


def load_allow_list(path: Path) -> AllowList:
    """Read an allow list: `path: <glob>` lines, word lines, `#` comments and blank lines."""
    paths: list[str] = []
    words: set[str] = set()
    for raw in path.read_text(encoding="utf-8").splitlines():
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        if line.startswith("path:"):
            paths.append(line.removeprefix("path:").strip())
        else:
            words.add(line)
    return AllowList(tuple(paths), frozenset(words))


def _words(text: str) -> tuple[list[tuple[int, str]], int | None]:
    """Return (line, word) of each plain word outside marked regions and an unclosed `off` line."""
    out: list[tuple[int, str]] = []
    off: int | None = None
    for number, line in enumerate(text.splitlines(), start=1):
        marker = _MARKER.match(line)
        if off is None and marker and marker.group(1) == "off":
            off = number
            continue
        if off is not None:
            if marker and marker.group(1) == "on":
                off = None
            continue
        for token in line.split():
            word = token.strip(_STRIP)
            if _WORD.fullmatch(word):
                out.append((number, word))
            else:
                # Markup or code around a word (`[word](x.md)`, `title="word"`, `f("word")`):
                # its umlaut words still count; a lone letter (a dollar-quote tag) is no word.
                out += [(number, w) for w in _UMLAUT_WORD.findall(token) if len(w) > 1]
    return out, off


def _prose(words: list[tuple[int, str]]) -> Iterable[tuple[int, int]]:
    """Yield (line, hits) for each passage where a window reaches `MIN_HITS` German words."""
    hits = [w.lower() in GERMAN_WORDS for _, w in words]
    index = 0
    while index < len(words):
        count = sum(hits[index : index + WINDOW])
        if count >= MIN_HITS:
            start = index + hits[index : index + WINDOW].index(True)
            end = start
            # Extend the passage while the window keeps reaching the threshold.
            while end < len(words) and sum(hits[end : end + WINDOW]) >= MIN_HITS:
                end += 1
            yield words[start][0], count
            index = end + WINDOW
        else:
            index += 1


def check_text(text: str, allowed: frozenset[str]) -> list[tuple[int, str]]:
    """Return (line, message) for German prose and for umlaut words not in `allowed`."""
    words, unclosed = _words(_ESCAPED_UMLAUT.sub(lambda m: chr(int(m.group(1), 16)), text))
    found = [
        (line, f"German prose ({hits} of {WINDOW} words are German function words)")
        for line, hits in _prose(words)
    ]
    for line, word in words:
        if _UMLAUT.search(word) and word not in allowed:
            found.append((line, f"word with umlaut or sharp s: {word!r}"))
    if unclosed is not None:
        found.append((unclosed, f"{OFF!r} without a later {ON!r}"))
    return sorted(found)


def check_file(path: Path, allow: AllowList) -> list[str]:
    """Return one message per finding in `path` as `path:line: message`."""
    data = path.read_bytes()
    if b"\x00" in data[:_BINARY_PROBE]:
        return []
    try:
        text = data.decode("utf-8")
    except UnicodeDecodeError:
        return []
    return [f"{path}:{line}: {message}" for line, message in check_text(text, allow.words)]


def tracked_files(root: Path) -> list[str]:
    """Return the tracked files of the repository at `root`, relative to it."""
    out = subprocess.run(
        ["git", "-C", str(root), "ls-files", "-z"], check=True, capture_output=True
    ).stdout
    return [p for p in out.decode("utf-8").split("\0") if p]


def main(argv: list[str] | None = None) -> int:
    """Check the given or all tracked files; returns 1 if German text was found."""
    parser = argparse.ArgumentParser(description="Fail on German text in tracked files.")
    parser.add_argument("--root", type=Path, default=REPO)
    parser.add_argument("--allow", type=Path, default=DEFAULT_ALLOW)
    parser.add_argument("files", nargs="*")
    args = parser.parse_args(argv)
    allow = load_allow_list(args.allow)
    names = args.files or tracked_files(args.root)
    problems: list[str] = []
    for name in names:
        rel = Path(name).relative_to(args.root) if Path(name).is_absolute() else Path(name)
        path = args.root / rel
        if allow.excludes(rel.as_posix()) or not path.is_file():
            continue
        problems += [
            p.replace(f"{path}:", f"{rel.as_posix()}:", 1) for p in check_file(path, allow)
        ]
    for problem in problems:
        print(f"language: {problem}", file=sys.stderr)
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main())
