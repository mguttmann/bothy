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

Allowed commit identities (author, committer, Signed-off-by and Co-authored-by trailers)
come from --allow-authors, BOTHY_DENYLIST_ALLOW_AUTHORS or `allow-authors.txt` next to the
list. Lines are 'Name <email>' or '<email>'.

Scanned by default: content and path of every tracked file (working tree), the current
branch name and all local branch and tag names, annotated tag messages, and message plus
identities of every commit in --range (default: all commits reachable from HEAD).
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
import subprocess
import sys
from collections.abc import Iterable
from dataclasses import dataclass, field
from pathlib import Path

ENV_LIST = "BOTHY_DENYLIST_FILE"
ENV_ALLOW = "BOTHY_DENYLIST_ALLOW_AUTHORS"
# Extra names to check, one per line (CI passes the pull request branch name here, through
# the environment and never through a command line, because branch names are untrusted).
ENV_NAMES = "BOTHY_DENYLIST_NAMES"
DOTFILE = ".denylist-path"
DEFAULT_GENERIC = Path(__file__).resolve().parent / "generic.rules"
_IDENTITY_TRAILER = re.compile(
    r"^(?:signed-off-by|co-authored-by|reviewed-by|acked-by|tested-by|reported-by):\s*(.+)$",
    re.IGNORECASE,
)
_EMAIL = re.compile(r"<([^>]+)>")


class ConfigError(Exception):
    """The scanner cannot run safely (missing list, broken rule)."""


@dataclass(frozen=True)
class Rule:
    source: str
    lineno: int
    kind: str
    pattern: re.Pattern[str]


@dataclass
class RuleSet:
    source: str
    rules: list[Rule] = field(default_factory=list)
    exclude: list[str] = field(default_factory=list)

    def applies_to(self, path: str | None) -> bool:
        return path is None or not any(fnmatch.fnmatch(path, g) for g in self.exclude)


@dataclass(frozen=True)
class Hit:
    offset: int
    text: str
    rule: Rule


@dataclass(frozen=True)
class Finding:
    kind: str
    location: str
    rule: Rule
    text: str


@dataclass
class Report:
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
    ruleset = parse_rules(text, "private")
    if not ruleset.rules:
        raise ConfigError("the private deny list has no rules; refusing to report a clean scan")
    return ruleset


def match_text(text: str, ruleset: RuleSet) -> list[Hit]:
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


def _newlines(text: str) -> list[int]:
    return [i for i, ch in enumerate(text) if ch == "\n"]


# --- configuration --------------------------------------------------------------


def _git_bytes(repo: Path, *args: str) -> bytes:
    return subprocess.run(["git", "-C", str(repo), *args], check=True, capture_output=True).stdout


def _text(repo: Path, *args: str) -> str:
    return _git_bytes(repo, *args).decode("utf-8", errors="replace")


def resolve_list_path(repo: Path, explicit: Path | None) -> Path | None:
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


def _is_allowed(identity: str, allowed: tuple[set[str], set[str]]) -> bool:
    normalized = " ".join(identity.strip().lower().split())
    m = _EMAIL.search(normalized)
    return normalized in allowed[0] or (m is not None and m.group(1).strip() in allowed[1])


# --- scanning -------------------------------------------------------------------


@dataclass
class Scanner:
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
        raw = _git_bytes(self.repo, "ls-files", "-z")
        for name in raw.decode("utf-8").split("\0"):
            if not name:
                continue
            self._scan("path", name, f"path {name}", path=name)
            file = self.repo / name
            if not file.is_file():
                continue
            data = file.read_bytes()
            binary = b"\x00" in data[:8192]
            text = data.decode("utf-8", errors="replace")
            self._scan("content", text, name, path=name, position="offset" if binary else "lines")

    def scan_refs(self, extra_names: list[str]) -> None:
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

    def scan_commits(self, rev_args: list[str]) -> None:
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
            kept: list[str] = []
            for line in body.splitlines():
                trailer = _IDENTITY_TRAILER.match(line.strip())
                if trailer:
                    if self.private and not _is_allowed(trailer.group(1), self.allowed):
                        self._scan(
                            "identity", line, f"commit {short} trailer", rulesets=[self.private]
                        )
                    kept.append("")
                else:
                    kept.append(line)
            self._scan(
                "commit-message", "\n".join(kept), f"commit {short} message", position="lines"
            )

    def scan_history_objects(self) -> None:
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
            binary = b"\x00" in body[:8192]
            text = body.decode("utf-8", errors="replace")
            self._scan(
                "history-content",
                text,
                f"history {sha[:12]} {path}",
                path=path,
                position="offset" if binary else "lines",
            )


def parse_args(argv: list[str]) -> argparse.Namespace:
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
    scanner = Scanner(repo, [r for r in (private, generic) if r], private, load_allowed(allow_path))
    scanner.scan_tracked_files()
    extra = [n for n in os.environ.get(ENV_NAMES, "").splitlines() if n.strip()]
    scanner.scan_refs([*args.name, *extra])
    if args.history:
        scanner.scan_commits(["--all"])
        scanner.scan_history_objects()
    else:
        scanner.scan_commits([args.rev_range] if args.rev_range else ["HEAD"])
    return Report(scanner.findings, private is not None, scanner.rulesets)


def main(argv: list[str] | None = None) -> int:
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
    mode = "history" if args.history else "tree, refs and commits"
    print(
        f"denylist: clean ({mode}; private list {'loaded' if report.private_loaded else 'absent'})"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
