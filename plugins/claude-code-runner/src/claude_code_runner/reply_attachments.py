# SPDX-License-Identifier: Apache-2.0
r"""The checks of answer attachments (docs/spec/09-agents.md 18.10, brief M3-12 notes 2 to 8).

Claude announces a file with a line ``ATTACH: <path>`` in its answer. Every decision about such
a file lives here: which lines are ``ATTACH:`` lines (`split_attach_lines`), which directories
may hold an attachment (`allowed_roots`), which names are secrets (`DeniedPatterns`), whether
one file passes (`check_candidate`), whether the whole set may go out (`check_set`, all or
none) and the notice that names the files that were not sent (`build_notice`). Only
`allowed_roots` and `check_candidate` read the file system (``realpath`` and ``stat``); the
other functions are pure. Staging and network I/O live in `claude_code_runner.reply_upload`.

Nothing here logs: a file name, a path or a file content never reaches a log line (SEC-LOG-4).

Examples:
    >>> split_attach_lines("Done.\nATTACH: dist/report.md\n")
    ('Done.', ['dist/report.md'])
"""

from __future__ import annotations

import fnmatch
import os
import stat
import unicodedata
from collections.abc import Iterable, Sequence
from dataclasses import dataclass
from pathlib import Path

__all__ = [
    "DEFAULT_DENIED_PATTERNS",
    "REASONS",
    "ROOT_NAMES",
    "AllowedRoots",
    "Candidate",
    "DeniedPatterns",
    "Limits",
    "Outcome",
    "Rejection",
    "SetVerdict",
    "allowed_roots",
    "build_notice",
    "check_allowed_root_names",
    "check_candidate",
    "check_denied_patterns",
    "check_set",
    "denied_match",
    "display_name",
    "split_attach_lines",
    "with_notice",
]

ROOT_NAMES = ("project", "thread_inbox", "runner_temp")
"""The names of ``runner.attach_allowed_roots`` and its default, in this order (note 3)."""

DEFAULT_DENIED_PATTERNS = (
    ".env*",
    "*.pem",
    "*.key",
    "id_*",
    ".netrc",
    ".ssh",
    ".claude",
    ".config",
    ".aws",
    ".azure",
    ".gnupg",
)
"""The patterns of 18.10 point 3: the default of ``runner.attach_denied_patterns`` and the part
every configured list must contain (note 4)."""

REASONS = (
    "not_found",
    "outside_allowed_roots",
    "denied_pattern",
    "not_a_regular_file",
    "hard_link",
    "empty_file",
    "changed_during_check",
    "too_large",
    "too_many",
    "limits_unavailable",
    "upload_refused",
    "upload_failed",
    "not_attached",
    "set_not_sent",
)
"""The closed set of reason codes of the notice (note 8)."""

EMPTY_NAME = "(empty)"
"""The name of an ``ATTACH:`` line without a path in the notice."""

_PREFIX = "ATTACH:"
_NAME_MAX_BYTES = 255  # the default of file.name_max_bytes (12); display text only, note 8
_BIDI = frozenset("\u061c\u200e\u200f\u202a\u202b\u202c\u202d\u202e\u2066\u2067\u2068\u2069")
_FENCE_CHARS = "`~"
_FENCE_MIN = 3
_FENCE_INDENT_MAX = 3
_PROBLEM_REASONS = frozenset({"upload_refused", "not_attached"})


# ---- note 2: ATTACH lines ------------------------------------------------------------------


def _fence(line: str) -> tuple[str, int, str] | None:
    """The fence character, its run length and the rest of a fence line, or ``None``."""
    stripped = line.lstrip(" ")
    if len(line) - len(stripped) > _FENCE_INDENT_MAX or not stripped:
        return None
    char = stripped[0]
    if char not in _FENCE_CHARS:
        return None
    run = len(stripped) - len(stripped.lstrip(char))
    if run < _FENCE_MIN:
        return None
    return char, run, stripped[run:]


def split_attach_lines(text: str) -> tuple[str, list[str]]:
    r"""Remove the ``ATTACH:`` lines of an answer and return their paths (note 2).

    An ``ATTACH:`` line starts at column 0 with exactly ``ATTACH:`` followed by at least one
    space or tab; its path is the rest of the line without leading and trailing white space (an
    empty path is returned as ``""``). Lines inside a fenced code block (three or more backticks
    or tildes; the block ends at a fence of the same character and at least the same length, an
    unclosed block runs to the end) are text, so an answer can show the syntax. Every
    ``ATTACH:`` line goes together with its line break; trailing blank lines are trimmed.

    Args:
        text: The answer after the ``TITLE:`` line was removed.

    Returns:
        The text without the ``ATTACH:`` lines, and the paths in the order of their lines.

    Examples:
        >>> split_attach_lines("```\nATTACH: x\n```")
        ('```\nATTACH: x\n```', [])
    """
    kept: list[str] = []
    paths: list[str] = []
    open_fence: tuple[str, int] | None = None
    for raw in text.splitlines(keepends=True):
        line = raw.rstrip("\r\n")
        fence = _fence(line)
        if open_fence is not None:
            if (
                fence is not None
                and fence[0] == open_fence[0]
                and fence[1] >= open_fence[1]
                and not fence[2].strip()
            ):
                open_fence = None
            kept.append(raw)
            continue
        if fence is not None and not (fence[0] == "`" and "`" in fence[2]):
            open_fence = (fence[0], fence[1])
            kept.append(raw)
            continue
        rest = line[len(_PREFIX) :] if line.startswith(_PREFIX) else None
        if rest is not None and rest[:1] in (" ", "\t"):
            paths.append(rest.strip())
            continue
        kept.append(raw)
    while kept and not kept[-1].strip():
        kept.pop()
    if kept:
        kept[-1] = kept[-1].rstrip("\r\n")
    return "".join(kept), paths


# ---- note 3: allowed roots -----------------------------------------------------------------


def check_allowed_root_names(values: Sequence[str]) -> str | None:
    """Check the form of ``runner.attach_allowed_roots`` (note 3 a).

    Args:
        values: The configured list.

    Returns:
        ``None`` when the list is valid, else the problem as a short phrase that never repeats a
        value (configuration.md 3).
    """
    if any(v not in ROOT_NAMES for v in values):
        return "an entry is not project, thread_inbox or runner_temp"
    if len(set(values)) != len(values):
        return "an entry appears twice"
    return None


_Id = tuple[int, int]
"""A directory or file by device and inode."""


@dataclass(frozen=True)
class AllowedRoots:
    """The real directories one run may attach from (note 3).

    Places are compared by device and inode, never by their spelling: a file system that
    ignores case (or any other alias of a directory) must not let a path past a check.

    Attributes:
        roots: The real paths of the configured roots that exist, in configuration order.
        root_ids: Device and inode of each root, in the same order.
        state_id: Device and inode of ``runner.state_dir`` (``None`` when it does not exist).
        state_allowed: Device and inode of the roots under ``state_dir`` (the run's inbox and
            temporary directory); every other place under ``state_dir`` is never allowed.
    """

    roots: tuple[str, ...]
    root_ids: tuple[_Id, ...]
    state_id: _Id | None
    state_allowed: tuple[_Id, ...]


def _identity(path: str | Path) -> _Id | None:
    try:
        info = Path(path).stat()
    except (OSError, ValueError):
        return None
    return info.st_dev, info.st_ino


def _chain(real: str) -> list[_Id | None]:
    """Device and inode of every prefix of a real path, from ``/`` down to the path itself."""
    parts = Path(real).parts
    return [_identity(Path(*parts[: i + 1])) for i in range(len(parts))]


def _real_dir(path: Path | None) -> str | None:
    if path is None:
        return None
    try:
        real = os.path.realpath(path, strict=True)
    except (OSError, ValueError):
        return None
    return real if Path(real).is_dir() else None


def allowed_roots(  # noqa: PLR0913 - the named roots and the two places that bound them
    names: Sequence[str],
    *,
    project_dir: Path | None,
    thread_inbox: Path,
    run_temp: Path,
    state_dir: Path,
    projects_root: Path,
) -> AllowedRoots:
    """Resolve the configured root names of one run to real directories (note 3 b and d).

    ``project`` is the turn's resolved working directory, ``thread_inbox`` the inbox of the
    run's thread, ``runner_temp`` the run's temporary directory. A root that does not exist is
    left out. A root that is ``runner.projects_root`` or contains it is left out too, so a
    misbuilt project can never reach another project; a ``project`` under
    ``runner.state_dir`` is left out as well, and so is an inbox or temporary directory that
    does not lie under it (a turn replaced it with a link). Places under ``runner.state_dir``
    other than the run's inbox and temporary directory stay forbidden in `check_candidate`
    whatever the roots.

    Args:
        names: ``runner.attach_allowed_roots``.
        project_dir: The resolved project of the run (``None`` when unknown).
        thread_inbox: ``<runner.state_dir>/files/<thread_id>``.
        run_temp: ``<runner.state_dir>/tmp/<run_id>``.
        state_dir: ``runner.state_dir``.
        projects_root: ``runner.projects_root``.

    Returns:
        The roots of the run.
    """
    state_id = _identity(state_dir)
    projects_real = _real_dir(projects_root)
    above_projects = set(_chain(projects_real)) if projects_real else set()
    paths = {"project": project_dir, "thread_inbox": thread_inbox, "runner_temp": run_temp}
    roots: list[str] = []
    ids: list[_Id] = []
    state_allowed: list[_Id] = []
    for name in names:
        real = _real_dir(paths.get(name))
        ident = _identity(real) if real is not None else None
        if real is None or ident is None or ident in ids:
            continue
        if ident in above_projects:
            continue  # the projects root itself or a directory above it
        under_state = state_id is not None and state_id in _chain(real)
        if name == "project" and under_state:
            continue
        if name != "project" and not under_state:
            continue  # the inbox or temporary directory was replaced by a link out of the state
        if under_state:
            state_allowed.append(ident)
        roots.append(real)
        ids.append(ident)
    return AllowedRoots(tuple(roots), tuple(ids), state_id, tuple(state_allowed))


# ---- note 4: denied patterns ---------------------------------------------------------------


def check_denied_patterns(values: Sequence[str]) -> str | None:
    """Check the form of ``runner.attach_denied_patterns`` (note 4 a).

    Args:
        values: The configured list.

    Returns:
        ``None`` when valid, else the problem; a missing default pattern is named, because the
        patterns are not secret.
    """
    for value in values:
        if not value or "/" in value:
            return "an entry is empty or holds a /"
        if any(unicodedata.category(c).startswith("C") or c in _BIDI for c in value):
            return "an entry holds a control or bidi character"
    present = {v.casefold() for v in values}
    for pattern in DEFAULT_DENIED_PATTERNS:
        if pattern.casefold() not in present:
            return f"the default pattern {pattern} is missing"
    return None


@dataclass(frozen=True)
class DeniedPatterns:
    """The denied patterns of the runner, folded for case-insensitive matching.

    Attributes:
        patterns: The patterns, ``casefold`` applied.
    """

    patterns: tuple[str, ...]

    @classmethod
    def of(cls, values: Iterable[str]) -> DeniedPatterns:
        """Build the patterns from ``runner.attach_denied_patterns``.

        Args:
            values: The configured list (its defaults are always part of it, note 4 a).

        Returns:
            The folded patterns.
        """
        return cls(tuple(dict.fromkeys(v.casefold() for v in values)))

    def matches(self, component: str) -> bool:
        """Whether one path component matches a pattern (case-insensitive).

        Args:
            component: A file or directory name.

        Returns:
            True when any pattern matches.
        """
        folded = component.casefold()
        return any(fnmatch.fnmatchcase(folded, p) for p in self.patterns)


def denied_match(patterns: DeniedPatterns, below: Sequence[str], announced: str) -> bool:
    """Whether a candidate is denied by a pattern (note 4 b).

    Tested are every component of the real path below its root (so a pattern for a directory
    such as ``.ssh`` denies every file inside it at any depth) and the last component of the
    path as announced (so a link named ``id_rsa`` is denied whatever it points to). The
    components of the root itself are not tested.

    Args:
        patterns: The denied patterns.
        below: The components of the resolved path below its root.
        announced: The path of the ``ATTACH:`` line.

    Returns:
        True when the candidate is denied.
    """
    last = _last_component(announced)
    return any(patterns.matches(c) for c in below) or (bool(last) and patterns.matches(last))


def _last_component(path: str) -> str:
    parts = [p for p in path.replace(os.sep, "/").split("/") if p]
    return parts[-1] if parts else ""


# ---- note 5: one candidate -----------------------------------------------------------------


@dataclass(frozen=True)
class Rejection:
    """Why a file was not sent.

    Attributes:
        code: One of `REASONS`.
        problem: The problem code of the server for ``upload_refused`` and ``not_attached``.
        reason: The problem's ``reason``, if it had one.
    """

    code: str
    problem: str | None = None
    reason: str | None = None


@dataclass(frozen=True)
class Candidate:
    """A file that passed the checks of note 5 a and b.

    Attributes:
        announced: The path of its ``ATTACH:`` line.
        real_path: The resolved path.
        size: The size ``stat`` reported.
        device: The device of ``stat``.
        inode: The inode of ``stat``.
    """

    announced: str
    real_path: str
    size: int
    device: int
    inode: int

    @property
    def name(self) -> str:
        """The upload name: the last component of the announced path (12 6.2 cleans it)."""
        return _last_component(self.announced)


def check_candidate(  # noqa: PLR0911 - one exit per check, in the order of note 5
    announced: str, cwd: Path | None, roots: AllowedRoots, patterns: DeniedPatterns
) -> Candidate | Rejection:
    """Check one announced path; the first failing check gives the reason (note 5 a and b).

    A relative path is joined with the turn's working directory; ``~`` is not expanded. The
    path is resolved with ``realpath`` (symbolic links followed) and must lie strictly below a
    root, never in a place under ``runner.state_dir`` other than the run's inbox and temporary
    directory; then the denied patterns; then the real path must be a regular file with one
    link and at least one byte.

    Args:
        announced: The path of the ``ATTACH:`` line.
        cwd: The turn's working directory (the resolved project).
        roots: The run's roots.
        patterns: The denied patterns.

    Returns:
        The candidate, or the rejection.
    """
    if not announced:
        return Rejection("not_found")
    path = Path(announced)
    if not path.is_absolute():
        if cwd is None:
            return Rejection("not_found")
        path = cwd / path
    try:
        real = os.path.realpath(path, strict=True)
    except (OSError, ValueError, RuntimeError):
        return Rejection("not_found")
    # Strictly below a root: a root among the ancestors of the file (by device and inode).
    ancestors = _chain(real)[:-1]
    index = next((ancestors.index(r) for r in roots.root_ids if r in ancestors), None)
    if index is None:
        return Rejection("outside_allowed_roots")
    if roots.state_id in ancestors and not any(r in ancestors for r in roots.state_allowed):
        return Rejection("outside_allowed_roots")
    below = Path(real).parts[index + 1 :]
    if denied_match(patterns, below, announced):
        return Rejection("denied_pattern")
    try:
        info = Path(real).lstat()
    except OSError:
        return Rejection("not_found")
    if not stat.S_ISREG(info.st_mode):
        return Rejection("not_a_regular_file")
    if info.st_nlink > 1:
        return Rejection("hard_link")
    if info.st_size == 0:
        return Rejection("empty_file")
    return Candidate(announced, real, info.st_size, info.st_dev, info.st_ino)


# ---- note 6: the set -----------------------------------------------------------------------


@dataclass(frozen=True)
class Limits:
    """The server's limits of ``InstanceLimits`` (03 9, read with ``getInstance``).

    Attributes:
        max_attachments: ``message_max_attachments``.
        max_upload_bytes: ``file_max_upload_bytes``.
    """

    max_attachments: int
    max_upload_bytes: int


@dataclass(frozen=True)
class Outcome:
    """What happened to one announced file, in position order.

    Attributes:
        announced: The path of its ``ATTACH:`` line.
        rejection: Why it was not sent; ``None`` while it is part of a set that may go out.
    """

    announced: str
    rejection: Rejection | None = None


@dataclass(frozen=True)
class SetVerdict:
    """The decision on the whole set of one answer (note 6).

    Attributes:
        outcomes: One per distinct announced file, in position order.
        accepted: The candidates in position order when the set may go out, else empty.
    """

    outcomes: tuple[Outcome, ...]
    accepted: tuple[Candidate, ...]

    @property
    def ok(self) -> bool:
        """Whether no file was rejected."""
        return all(o.rejection is None for o in self.outcomes)


def check_set(
    checked: Sequence[tuple[str, Candidate | Rejection]], limits: Limits | None
) -> SetVerdict:
    """Decide on the whole set before any upload: all or none (note 6).

    The same real path announced twice counts once, at its first position. Without limits
    every file that passed its own checks gets ``limits_unavailable``; more distinct files than
    ``max_attachments`` give every such file ``too_many``; a file above ``max_upload_bytes``
    gets ``too_large``. A file rejected by its own checks keeps that reason. When any file is
    rejected, every other file gets ``set_not_sent`` and nothing is accepted.

    Args:
        checked: The announced path and the result of `check_candidate` per ``ATTACH:`` line,
            in line order.
        limits: The server's limits, or ``None`` when ``getInstance`` failed.

    Returns:
        The verdict.
    """
    entries: list[tuple[str, Candidate | Rejection]] = []
    seen: set[str] = set()
    for announced, result in checked:
        if isinstance(result, Candidate):
            if result.real_path in seen:
                continue
            seen.add(result.real_path)
        entries.append((announced, result))
    reasons: list[Rejection | None] = []
    for _, result in entries:
        if isinstance(result, Rejection):
            reasons.append(result)
        elif limits is None:
            reasons.append(Rejection("limits_unavailable"))
        elif len(entries) > limits.max_attachments:
            reasons.append(Rejection("too_many"))
        elif result.size > limits.max_upload_bytes:
            reasons.append(Rejection("too_large"))
        else:
            reasons.append(None)
    if any(r is not None for r in reasons):
        outcomes = tuple(
            Outcome(a, r or Rejection("set_not_sent"))
            for (a, _), r in zip(entries, reasons, strict=True)
        )
        return SetVerdict(outcomes, ())
    accepted = tuple(r for _, r in entries if isinstance(r, Candidate))
    return SetVerdict(tuple(Outcome(a) for a, _ in entries), accepted)


# ---- note 8: the notice --------------------------------------------------------------------


def display_name(announced: str) -> str:
    """The name of a file in the notice: the announced last component, cleaned (note 8).

    Control, bidi and backtick characters are removed and the name is cut to the default of
    ``file.name_max_bytes`` in UTF-8 bytes, never inside a character. Never a directory or a
    real path.

    Args:
        announced: The path of the ``ATTACH:`` line.

    Returns:
        The display name; ``(empty)`` when nothing is left.
    """
    name = "".join(
        c
        for c in _last_component(announced)
        if c != "`" and c not in _BIDI and not unicodedata.category(c).startswith("C")
    )
    raw = name.encode("utf-8")[:_NAME_MAX_BYTES]
    name = raw.decode("utf-8", "ignore")
    return name or EMPTY_NAME


def build_notice(outcomes: Sequence[Outcome], limit: int | None) -> str:
    r"""The notice of a set that was not sent (note 8); empty when every file was sent.

    Args:
        outcomes: The outcomes in position order.
        limit: ``message_max_attachments``: at most this many lines name files, the rest is
            summed up in one line; ``None`` (limits unknown) names every file.

    Returns:
        ``Attachments not sent:`` and one line per file, or ``""``.

    Examples:
        >>> build_notice([Outcome("a/.env", Rejection("denied_pattern"))], 3)
        'Attachments not sent:\n- `.env`: denied_pattern'
    """
    if all(o.rejection is None for o in outcomes):
        return ""
    lines = ["Attachments not sent:"]
    shown = outcomes if limit is None else outcomes[: max(limit, 0)]
    for outcome in shown:
        rejection = outcome.rejection or Rejection("set_not_sent")
        line = f"- `{display_name(outcome.announced)}`: {rejection.code}"
        if rejection.code in _PROBLEM_REASONS and rejection.problem:
            detail = rejection.problem
            if rejection.reason:
                detail += f", {rejection.reason}"
            line += f" ({detail})"
        lines.append(line)
    rest = len(outcomes) - len(shown)
    if rest > 0:
        lines.append(f"- and {rest} more")
    return "\n".join(lines)


def with_notice(text: str, notice: str) -> str:
    """Append a notice to an answer after one blank line.

    Args:
        text: The answer without ``ATTACH:`` lines.
        notice: The notice, or ``""``.

    Returns:
        The text to send.
    """
    if not notice:
        return text
    return f"{text}\n\n{notice}" if text else notice
