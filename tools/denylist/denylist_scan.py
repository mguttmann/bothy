# SPDX-License-Identifier: Apache-2.0
"""White-label scanner: find instance-specific terms before they reach a public repository.

The private deny list lives OUTSIDE this repository (an operator's private repo). It is
located, in this order, by --list, the environment variable BOTHY_DENYLIST_FILE, or a
git-ignored file `.denylist-path` in the repository root that contains the path. A
missing or empty list fails the scan (fail closed) unless --allow-missing is given;
then only the public generic rules (tools/denylist/generic.rules) run. That mode is meant
for fork pull requests that cannot read the private list.

List format: one rule per line; '#' starts a comment; 're:' marks a Python regular
expression; everything else is a literal substring. Matching ignores case.
'@exclude-path: <glob>' excludes repository paths from the rules of that file.

Allowed commit identities come from --allow-authors, BOTHY_DENYLIST_ALLOW_AUTHORS or
`allow-authors.txt` next to the list. Lines are 'Name <email>' or '<email>'. They are allowed
only as author or committer, and in a Signed-off-by trailer that repeats the author or
committer of the same commit; anywhere else (Co-authored-by, another sign-off, text) the
private rules apply as usual. Identity trailers (Signed-off-by, Co-authored-by) of the exact
form 'Key: Name <email>' are checked against the private list as a whole, like author and
committer, because contributors sign off with their own addresses; their name part also gets
the generic rules. A trailer line with any other text, and every other trailer, is message
text and gets all rules.

Scanned by default: content and path of every tracked file (working tree), the current
branch name and all local branch and tag names, annotated tag messages, message plus
identities of every commit in --range (default: all commits reachable from HEAD), and the
pull request title and body from PR_TITLE, PR_BODY and PR_BODY_FILE when set. The body is
checked like a commit message; a Signed-off-by trailer in it is allowed when it names a full
identity from the allow list, because the squash commit has no author to compare it with yet.
With --history additionally every blob and path ever committed on any ref, and every
commit on any ref.

Output names the location and the rule's line number, never the matched text, so that
CI logs do not reveal the list. --reveal prints the match (local use only).
Exit codes: 0 clean, 1 findings, 2 configuration error.
"""

from __future__ import annotations

import argparse
import bisect
import fnmatch
import os
import re
import struct
import subprocess
import sys
import zlib
from collections.abc import Iterable
from dataclasses import dataclass, field
from pathlib import Path

ENV_LIST = "BOTHY_DENYLIST_FILE"
ENV_ALLOW = "BOTHY_DENYLIST_ALLOW_AUTHORS"
# Extra names to check, one per line (CI passes the pull request branch name here, through
# the environment and never through a command line, because branch names are untrusted).
ENV_NAMES = "BOTHY_DENYLIST_NAMES"
# The pull request title and body become the squash commit on main (header and message).
# CI passes them from the event payload through the environment, never through a command
# line; locally they hold the planned squash message (PR_BODY_FILE: a file with the body).
ENV_PR_TITLE = "PR_TITLE"
ENV_PR_BODY = "PR_BODY"
ENV_PR_BODY_FILE = "PR_BODY_FILE"
DOTFILE = ".denylist-path"
DEFAULT_GENERIC = Path(__file__).resolve().parent / "generic.rules"
# An identity trailer is exactly "Key: Name <email>"; any other text on such a line makes it
# ordinary message text, so that a trailer cannot hide text from the rules.
_IDENTITY_TRAILER = re.compile(
    r"^(signed-off-by|co-authored-by):[ \t]*([^<>]*?)[ \t]*<([^<>\s]+)>$", re.IGNORECASE
)
_EMAIL = re.compile(r"<([^>]+)>")


class ConfigError(Exception):
    """The scanner cannot run safely (missing list, broken rule)."""


@dataclass(frozen=True)
class Rule:
    """One rule: origin list, line number in that list, literal or regex, compiled pattern."""

    source: str
    lineno: int
    kind: str
    pattern: re.Pattern[str]


@dataclass
class RuleSet:
    """The rules of one list file plus its `@exclude-path` globs."""

    source: str
    rules: list[Rule] = field(default_factory=list)
    exclude: list[str] = field(default_factory=list)

    def applies_to(self, path: str | None) -> bool:
        """Return True unless `path` matches an exclude glob of this list (None always applies)."""
        return path is None or not any(fnmatch.fnmatch(path, g) for g in self.exclude)


@dataclass(frozen=True)
class Hit:
    """One match of a rule at a character offset of the scanned text."""

    offset: int
    text: str
    rule: Rule


@dataclass(frozen=True)
class Finding:
    """One reported match: kind of place, location, rule and text (printed only with --reveal)."""

    kind: str
    location: str
    rule: Rule
    text: str


@dataclass
class Report:
    """Result of a scan: findings, whether the private list was loaded, and the rule sets used."""

    findings: list[Finding]
    private_loaded: bool
    rulesets: list[RuleSet] = field(default_factory=list)

    def mask(self, text: str) -> str:
        """Replace every rule match in `text` with asterisks (keeps output free of list terms)."""
        for ruleset in self.rulesets:
            for rule in ruleset.rules:
                text = rule.pattern.sub(lambda m: "*" * len(m.group(0)), text)
        return text


def parse_rules(text: str, source: str) -> RuleSet:
    """Parse a rule file (`source`: `private` or `generic`); raises ConfigError for a bad regex."""
    ruleset = RuleSet(source)
    for lineno, raw in enumerate(text.splitlines(), start=1):
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        if line.startswith("@exclude-path:"):
            ruleset.exclude.append(line.split(":", 1)[1].strip())
            continue
        if line.startswith("re:"):
            try:
                pattern = re.compile(line[3:], re.IGNORECASE)
            except re.error as exc:
                raise ConfigError(
                    f"{source} list line {lineno}: invalid regular expression ({exc.msg})"
                ) from None
            ruleset.rules.append(Rule(source, lineno, "regex", pattern))
        else:
            ruleset.rules.append(
                Rule(source, lineno, "literal", re.compile(re.escape(line), re.IGNORECASE))
            )
    return ruleset


def load_private_list_text(text: str) -> RuleSet:
    """Parse the private list; raises ConfigError if it has no rules (fail closed)."""
    ruleset = parse_rules(text, "private")
    if not ruleset.rules:
        raise ConfigError("the private deny list has no rules; refusing to report a clean scan")
    return ruleset


def match_text(text: str, ruleset: RuleSet) -> list[Hit]:
    """Return all matches of `ruleset` in `text`, ordered by offset."""
    hits = [
        Hit(m.start(), m.group(0), rule)
        for rule in ruleset.rules
        for m in rule.pattern.finditer(text)
        if m.group(0)
    ]
    return sorted(hits, key=lambda h: (h.offset, h.rule.source, h.rule.lineno))


def _line_col(text: str, offset: int, newlines: list[int]) -> tuple[int, int]:
    line = bisect.bisect_right(newlines, offset - 1)
    start = newlines[line - 1] + 1 if line else 0
    return line + 1, offset - start + 1


PNG_SIGNATURE = b"\x89PNG\r\n\x1a\n"
PNG_PIXEL_CHUNK = b"IDAT"


def _png_text(data: bytes) -> str | None:
    """Return the scannable text of a well-formed PNG, or None if `data` is not one.

    Only the pixel data (IDAT payloads) is left out: it is compressed noise, and a
    deny term found there is chance, not text. Everything else stays in raw form
    (chunk names, eXIf, private chunks, bytes after IEND), and the compressed text
    of zTXt, iTXt and iCCP is added decompressed. A truncated or malformed PNG
    returns None and is scanned raw by the caller.
    """
    if not data.startswith(PNG_SIGNATURE):
        return None
    kept: list[bytes] = [data[: len(PNG_SIGNATURE)]]
    inflated: list[bytes] = []
    pos = len(PNG_SIGNATURE)
    try:
        while pos + 8 <= len(data):
            (length,) = struct.unpack(">I", data[pos : pos + 4])
            ctype = data[pos + 4 : pos + 8]
            end = pos + 12 + length
            if end > len(data):
                return None
            body = data[pos + 8 : pos + 8 + length]
            if ctype == PNG_PIXEL_CHUNK:
                kept.append(data[pos : pos + 8])
            else:
                kept.append(data[pos:end])
                inflated.extend(_png_inflated(ctype, body))
            pos = end
            if ctype == b"IEND":
                kept.append(data[pos:])
                text = b"\n".join([b"".join(kept), *inflated])
                return text.decode("utf-8", errors="replace")
    except (struct.error, zlib.error, IndexError):
        return None
    return None


def _png_inflated(ctype: bytes, body: bytes) -> list[bytes]:
    """Decompress the compressed part of a zTXt, iTXt or iCCP chunk body."""
    _keyword, _, rest = body.partition(b"\x00")
    if ctype in (b"zTXt", b"iCCP"):
        return [zlib.decompress(rest[1:])]
    if ctype == b"iTXt" and rest[:1] == b"\x01":
        rest = rest[2:]
        _language, _, rest = rest.partition(b"\x00")
        _translated, _, rest = rest.partition(b"\x00")
        return [zlib.decompress(rest)]
    return []


def _content_text(data: bytes) -> tuple[str, bool]:
    """Return the scannable text of a file and whether it is binary."""
    png = _png_text(data)
    if png is not None:
        return png, True
    return data.decode("utf-8", errors="replace"), b"\x00" in data[:8192]


def _newlines(text: str) -> list[int]:
    return [i for i, ch in enumerate(text) if ch == "\n"]


# --- configuration --------------------------------------------------------------


def _git_bytes(repo: Path, *args: str) -> bytes:
    return subprocess.run(["git", "-C", str(repo), *args], check=True, capture_output=True).stdout


def _text(repo: Path, *args: str) -> str:
    return _git_bytes(repo, *args).decode("utf-8", errors="replace")


def resolve_list_path(repo: Path, explicit: Path | None) -> Path | None:
    """Return the list path from --list, BOTHY_DENYLIST_FILE or `.denylist-path`, else None."""
    if explicit is not None:
        return explicit
    env = os.environ.get(ENV_LIST)
    if env:
        return Path(env)
    dotfile = repo / DOTFILE
    if dotfile.is_file():
        value = dotfile.read_text(encoding="utf-8").strip()
        if value:
            path = Path(value).expanduser()
            return path if path.is_absolute() else (repo / path).resolve()
    return None


def load_allowed(path: Path | None) -> tuple[set[str], set[str]]:
    """Return (full identities 'name <email>', bare e-mails), all lower case."""
    identities: set[str] = set()
    emails: set[str] = set()
    if path is None or not path.is_file():
        return identities, emails
    for raw in path.read_text(encoding="utf-8").splitlines():
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        if line.startswith("<") and line.endswith(">"):
            emails.add(line[1:-1].strip().lower())
        else:
            identities.add(" ".join(line.lower().split()))
    return identities, emails


def _normalize(identity: str) -> str:
    return " ".join(identity.strip().lower().split())


def _is_allowed(identity: str, allowed: tuple[set[str], set[str]]) -> bool:
    normalized = _normalize(identity)
    m = _EMAIL.search(normalized)
    return normalized in allowed[0] or (m is not None and m.group(1).strip() in allowed[1])


# --- scanning -------------------------------------------------------------------


@dataclass
class Scanner:
    """Collects findings over the places of one repository."""

    repo: Path
    rulesets: list[RuleSet]
    private: RuleSet | None
    allowed: tuple[set[str], set[str]]
    findings: list[Finding] = field(default_factory=list)

    def _scan(
        self,
        kind: str,
        text: str,
        where: str,
        *,
        path: str | None = None,
        rulesets: Iterable[RuleSet] | None = None,
        position: str = "none",
    ) -> None:
        """Match `text`; `position` is 'lines' (file:line:col), 'offset' (binary) or 'none'."""
        newlines = _newlines(text) if position == "lines" else []
        for ruleset in rulesets if rulesets is not None else self.rulesets:
            if not ruleset.applies_to(path):
                continue
            for hit in match_text(text, ruleset):
                if position == "lines":
                    line, col = _line_col(text, hit.offset, newlines)
                    loc = f"{where}:{line}:{col}"
                elif position == "offset":
                    loc = f"{where}@{hit.offset}"
                else:
                    loc = where
                self.findings.append(Finding(kind, loc, hit.rule, hit.text))

    def scan_tracked_files(self) -> None:
        """Scan path and content of every tracked file in the working tree."""
        raw = _git_bytes(self.repo, "ls-files", "-z")
        for name in raw.decode("utf-8").split("\0"):
            if not name:
                continue
            self._scan("path", name, f"path {name}", path=name)
            file = self.repo / name
            if not file.is_file():
                continue
            text, binary = _content_text(file.read_bytes())
            self._scan("content", text, name, path=name, position="offset" if binary else "lines")

    def scan_refs(self, extra_names: list[str]) -> None:
        """Scan branch, remote and tag names, annotated tag messages and `extra_names`."""
        refs = _text(
            self.repo,
            "for-each-ref",
            "--format=%(refname)",
            "refs/heads",
            "refs/remotes",
            "refs/tags",
        )
        for ref in refs.splitlines():
            short = (
                ref.removeprefix("refs/heads/")
                .removeprefix("refs/remotes/")
                .removeprefix("refs/tags/")
            )
            self._scan("ref", short, f"ref {ref}")
        tags = _text(self.repo, "for-each-ref", "--format=%(refname)%1f%(contents)%1e", "refs/tags")
        for raw_record in tags.split("\x1e"):
            record = raw_record.strip("\n")
            if record and "\x1f" in record:
                ref, message = record.split("\x1f", 1)
                if message.strip():
                    self._scan("tag-message", message, f"tag-message {ref}", position="lines")
        for name in extra_names:
            self._scan("name", name, f"name {name}")

    def _message(
        self, body: str, kind: str, where: str, trailer_where: str, own: set[str] | None
    ) -> None:
        """Scan a commit message or pull request body; identity trailers get the private list.

        A trailer line of the exact form `Signed-off-by: Name <email>` (or Co-authored-by)
        is checked against the private list as a whole, and its name part also against the
        other rules (an address of a contributor is no finding of the generic rules).
        `own` holds the author and committer of the commit; a Signed-off-by trailer that
        repeats one of them with an allowed identity is not reported. `own` None (a pull
        request body, whose squash commit does not exist yet) accepts a Signed-off-by only
        with a full identity from the allow list, and the trailer location gets the line.
        """
        trailer_kind = "identity" if own is not None else f"{kind}-identity"
        others = [r for r in self.rulesets if r is not self.private]
        kept: list[str] = []
        for lineno, line in enumerate(body.splitlines(), start=1):
            trailer = _IDENTITY_TRAILER.match(line.strip())
            if not trailer:
                kept.append(line)
                continue
            identity = f"{trailer.group(2)} <{trailer.group(3)}>"
            if own is None:
                own_sign_off = _normalize(identity) in self.allowed[0]
            else:
                own_sign_off = _normalize(identity) in own and _is_allowed(identity, self.allowed)
            own_sign_off = own_sign_off and trailer.group(1).lower() == "signed-off-by"
            loc = trailer_where if own is not None else f"{trailer_where}:{lineno}"
            if self.private and not own_sign_off:
                self._scan(trailer_kind, line, loc, rulesets=[self.private])
            if trailer.group(2):
                self._scan(trailer_kind, trailer.group(2), loc, rulesets=others)
            kept.append("")
        self._scan(kind, "\n".join(kept), where, position="lines")

    def scan_pull_request(self, title: str, bodies: list[tuple[str, str]]) -> None:
        """Scan the pull request title and each (location, body) as the squash commit."""
        if title.strip():
            self._scan("pr-title", title, "pull request title")
        for where, body in bodies:
            if body.strip():
                self._message(body, "pr-body", where, where, None)

    def scan_commits(self, rev_args: list[str]) -> None:
        """Scan message, identities and identity trailers of the commits selected by `rev_args`."""
        fmt = "%H%x1f%an <%ae>%x1f%cn <%ce>%x1f%B%x1e"
        out = _text(self.repo, "log", f"--format={fmt}", *rev_args, "--")
        for raw_record in out.split("\x1e"):
            record = raw_record.strip("\n")
            if not record:
                continue
            sha, author, committer, body = record.split("\x1f", 3)
            short = sha[:12]
            for role, identity in (("author", author), ("committer", committer)):
                if self.private and not _is_allowed(identity, self.allowed):
                    self._scan(
                        "identity", identity, f"commit {short} {role}", rulesets=[self.private]
                    )
            own = {_normalize(author), _normalize(committer)}
            self._message(
                body, "commit-message", f"commit {short} message", f"commit {short} trailer", own
            )

    def scan_history_objects(self) -> None:
        """Scan every path and every blob ever committed on any ref that is not in the index."""
        listing = _text(self.repo, "rev-list", "--objects", "--all")
        paths: dict[str, str] = {}
        seen_paths: set[str] = set()
        for line in listing.splitlines():
            sha, _, path = line.partition(" ")
            if path:
                paths.setdefault(sha, path)
                if path not in seen_paths:
                    seen_paths.add(path)
                    self._scan("history-path", path, f"history-path {path}", path=path)
        if not paths:
            return
        batch = "\n".join(paths) + "\n"
        proc = subprocess.run(
            ["git", "-C", str(self.repo), "cat-file", "--batch"],
            input=batch.encode(),
            capture_output=True,
            check=True,
        )
        data = proc.stdout
        pos = 0
        current = _text(self.repo, "ls-files", "-s")
        current_blobs = {ln.split()[1] for ln in current.splitlines() if ln}
        while pos < len(data):
            header_end = data.index(b"\n", pos)
            sha, otype, size = data[pos:header_end].decode().split()
            body = data[header_end + 1 : header_end + 1 + int(size)]
            pos = header_end + 1 + int(size) + 1
            if otype != "blob" or sha in current_blobs:
                continue
            path = paths[sha]
            text, binary = _content_text(body)
            self._scan(
                "history-content",
                text,
                f"history {sha[:12]} {path}",
                path=path,
                position="offset" if binary else "lines",
            )


def _pull_request_bodies() -> list[tuple[str, str]]:
    """Return (location, text) of PR_BODY and PR_BODY_FILE; a missing body file fails closed."""
    bodies = [("pull request body", os.environ.get(ENV_PR_BODY, ""))]
    path = os.environ.get(ENV_PR_BODY_FILE, "")
    if path:
        try:
            bodies.append(("pull request body file", Path(path).read_text(encoding="utf-8")))
        except OSError:
            raise ConfigError(f"{ENV_PR_BODY_FILE} is set but cannot be read") from None
    return bodies


def parse_args(argv: list[str]) -> argparse.Namespace:
    """Parse the command line arguments of the scanner."""
    parser = argparse.ArgumentParser(description="White-label deny-list scanner.")
    parser.add_argument("--repo", type=Path, default=Path.cwd())
    parser.add_argument(
        "--list", type=Path, default=None, help=f"deny list (else ${ENV_LIST} or {DOTFILE})"
    )
    parser.add_argument("--allow-authors", type=Path, default=None)
    parser.add_argument("--generic", type=Path, default=DEFAULT_GENERIC)
    parser.add_argument(
        "--allow-missing",
        action="store_true",
        help="run generic rules only if the private list is absent (fork pull requests)",
    )
    parser.add_argument(
        "--range",
        dest="rev_range",
        default=None,
        help="commits to check (default: all reachable from HEAD)",
    )
    parser.add_argument(
        "--history", action="store_true", help="scan every object and commit on every ref"
    )
    parser.add_argument(
        "--name", action="append", default=[], help="extra name to check, e.g. a PR branch"
    )
    parser.add_argument("--reveal", action="store_true", help="print matched text (never in CI)")
    return parser.parse_args(argv)


def scan(args: argparse.Namespace) -> Report:
    """Run the scan described by `args`; raises ConfigError if the private list is missing."""
    repo: Path = args.repo.resolve()
    generic = parse_rules(args.generic.read_text(encoding="utf-8"), "generic")
    list_path = resolve_list_path(repo, args.list)
    private: RuleSet | None = None
    if list_path is not None and list_path.is_file():
        private = load_private_list_text(list_path.read_text(encoding="utf-8"))
    elif not args.allow_missing:
        where = str(list_path) if list_path else f"--list, ${ENV_LIST} or {DOTFILE}"
        raise ConfigError(f"deny list not found ({where}); refusing to scan without it")
    allow_path = args.allow_authors or (
        Path(os.environ[ENV_ALLOW]) if os.environ.get(ENV_ALLOW) else None
    )
    if allow_path is None and list_path is not None:
        allow_path = list_path.parent / "allow-authors.txt"
    bodies = _pull_request_bodies()
    scanner = Scanner(repo, [r for r in (private, generic) if r], private, load_allowed(allow_path))
    scanner.scan_tracked_files()
    extra = [n for n in os.environ.get(ENV_NAMES, "").splitlines() if n.strip()]
    scanner.scan_refs([*args.name, *extra])
    if args.history:
        scanner.scan_commits(["--all"])
        scanner.scan_history_objects()
    else:
        scanner.scan_commits([args.rev_range] if args.rev_range else ["HEAD"])
    scanner.scan_pull_request(os.environ.get(ENV_PR_TITLE, ""), bodies)
    return Report(scanner.findings, private is not None, scanner.rulesets)


def main(argv: list[str] | None = None) -> int:
    """Command line entry point; exit code 0 clean, 1 findings, 2 configuration error."""
    args = parse_args(sys.argv[1:] if argv is None else argv)
    try:
        report = scan(args)
    except ConfigError as exc:
        print(f"denylist: {exc}", file=sys.stderr)
        return 2
    if not report.private_loaded:
        print("denylist: WARNING private deny list absent (--allow-missing); generic rules only")
    for f in report.findings:
        location = f.location if args.reveal else report.mask(f.location)
        shown = f" -> {f.text!r}" if args.reveal else ""
        print(f"{location}: {f.kind} matches {f.rule.source} rule line {f.rule.lineno}{shown}")
    if report.findings:
        print(
            f"denylist: {len(report.findings)} finding(s); remove the terms or rewrite history "
            "(docs/quality/public-switch.md)"
        )
        return 1
    mode = "history" if args.history else "tree, refs, commits and pull request text"
    print(
        f"denylist: clean ({mode}; private list {'loaded' if report.private_loaded else 'absent'})"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
