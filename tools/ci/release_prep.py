# SPDX-License-Identifier: Apache-2.0
"""Release preparation of one component: `make release-prep COMPONENT=<name> [CHANNEL=beta]`.

ADR 0060 items 1 to 5. Directory, tag prefix and the paths whose commits count come from
tools/ci/components.py. The pinned git-cliff (configuration in cliff.toml) selects the commits
of the component since its last stable tag (`--tag-pattern` for the prefix, `--include-path`
per path, `--unreleased --context`) and parses them as Conventional Commits; `next_stable`
applies the version rule to them: fix gives a patch, feat a minor, a break a major, before
1.0.0 a minor. `--bumped-version` is not used, because git-cliff 2.14.2 computes it against
that rule in two cases: a `BREAKING CHANGE` footer before 1.0.0 gives 1.0.0, and every other
commit type (docs, chore) gives a patch (docs/DECISIONS.md D-M7-10-6). `CHANNEL=beta` appends
`-beta.N`, N one above the highest existing beta tag of that version. The section "Unreleased"
of the component's CHANGELOG.md becomes `[X.Y.Z] - <date>` under a new empty "Unreleased",
`VERSION` gets the version, and the commits of the component that did not change its
CHANGELOG.md are printed as a checklist.

For the product `bothy` (ADR 0060 item 6, docs/spec/18-release.md section 2.1 rule 1) the
same run also writes the server version the bundle installs: `--server-version V`
(`SERVER_VERSION=` of `make release-prep`) replaces the one line `BOTHY_VERSION=` of
infra/compose/.env.example with V, if V is SemVer without "v", the tag `server/vV` exists and
V carries no pre-release identifier unless the channel is beta. Without `--server-version`
the line stays as it is, unless it still carries the placeholder. The documented
installation command in README.md and docs/operations/install-compose.md names the product
version (`.../releases/download/v<version>/install.sh`, held equal to VERSION by
tools/ci/install_options_check.py); every such URL gets the new version.

Everything is computed before the first write: on any error no file changes. The script
never commits and never tags; the maintainer reviews, commits `chore(release): ...` and sets the
signed tag after the merge (ADR 0060 item 4).

Usage:
  release_prep.py --component <name> --git-cliff <path> [--channel stable|beta]
                  [--server-version <version>] [--repo <dir>] [--config <cliff.toml>]
                  [--date YYYY-MM-DD]
Exit codes: 0 prepared, 1 nothing written because of an error (named on stderr), 2 usage
error (for example `--server-version` for a component other than `bothy`).
"""

from __future__ import annotations

import argparse
import datetime
import json
import os
import re
import subprocess
import sys
import tempfile
from dataclasses import dataclass
from pathlib import Path

import components
import install_options_check

CHANNELS = ("stable", "beta")
# First version of a component that has no stable tag yet.
INITIAL_VERSION = "0.1.0"
_STABLE = r"(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)"
_UNRELEASED = re.compile(r"^## \[Unreleased\][ \t]*$", re.MULTILINE)
_HEADING = re.compile(r"^## ", re.MULTILINE)
_CONVENTIONAL = re.compile(r"^([A-Za-z]+)(?:\([^)]*\))?!?:")
# Commit types that give a patch release; other types without a break give none.
PATCH_KINDS = frozenset({"fix", "perf"})
# The component whose bundle pins the server version (docs/spec/18-release.md, 2.1).
PRODUCT = "bothy"
# The line of the bundle template; the prefix group keeps any indentation as it is.
_PIN_LINE = re.compile(r"^([ \t]*BOTHY_VERSION=)(.*)$", re.MULTILINE)


class ReleasePrepError(Exception):
    """The release cannot be prepared; nothing was written."""


@dataclass(frozen=True)
class Commit:
    """A commit of the checklist: it changed the component but not its CHANGELOG.md."""

    sha: str
    subject: str


@dataclass(frozen=True)
class Result:
    """What `prepare` wrote.

    Attributes:
        component: the release component.
        version: the new version without `v`, for example `0.2.0-beta.1`.
        tag: the tag the maintainer sets after the merge, for example `server/v0.2.0-beta.1`.
        changelog: repository-relative path of the changed CHANGELOG.md.
        version_file: repository-relative path of the changed VERSION.
        checklist: commits since the last stable tag that did not change the changelog.
        other_files: further written files of the product `bothy`: the bundle template and
            the files with the documented installation command.
    """

    component: str
    version: str
    tag: str
    changelog: str
    version_file: str
    checklist: list[Commit]
    other_files: tuple[str, ...] = ()


def _git(repo: Path, *args: str) -> str:
    out = subprocess.run(
        ["git", "-C", str(repo), *args], check=True, capture_output=True, text=True
    )
    return out.stdout


def _version_key(version: str) -> tuple[int, int, int]:
    major, minor, patch = version.split(".")
    return int(major), int(minor), int(patch)


def last_stable_tag(repo: Path, prefix: str) -> str | None:
    """Return the highest stable tag `<prefix>vX.Y.Z` reachable from HEAD, or None."""
    pattern = re.compile(rf"^{re.escape(prefix)}v{_STABLE}$")
    tags = [t for t in _git(repo, "tag", "--list", "--merged", "HEAD").split() if pattern.match(t)]
    if not tags:
        return None
    return max(tags, key=lambda t: _version_key(t[len(prefix) + 1 :]))


def _beta_numbers(repo: Path, prefix: str, version: str) -> list[int]:
    pattern = re.compile(rf"^{re.escape(prefix)}v{re.escape(version)}-beta\.([1-9]\d*)$")
    return sorted(
        int(m.group(1)) for t in _git(repo, "tag", "--list").split() if (m := pattern.match(t))
    )


def _changed_files(repo: Path, rev: str) -> list[str]:
    return _git(repo, "diff-tree", "--no-commit-id", "--name-only", "-r", "--root", rev).split()


@dataclass(frozen=True)
class Change:
    """A commit of the component since its last stable tag, as git-cliff parsed it."""

    sha: str
    kind: str
    breaking: bool


def unreleased_changes(
    repo: Path, git_cliff: Path, config: Path, prefix: str, paths: tuple[str, ...]
) -> tuple[str | None, list[Change]]:
    """Return the last stable tag git-cliff sees and the commits of the component after it.

    git-cliff selects the commits (`--tag-pattern` for the prefix, `--include-path` per path,
    `--unreleased`) and parses them as Conventional Commits, breaking footers included.

    Args:
        repo: the repository.
        git_cliff: the pinned git-cliff binary.
        config: cliff.toml.
        prefix: tag prefix of the component (`server/`, empty for the product).
        paths: repository paths whose commits count (empty: all commits).

    Returns:
        The previous tag (None without one) and the changes, oldest first.

    Raises:
        ReleasePrepError: git-cliff failed or printed no context.
    """
    cmd = [
        str(git_cliff),
        "--config",
        str(config),
        "--context",
        "--unreleased",
        "--tag-pattern",
        rf"^{re.escape(prefix)}v[0-9]+\.[0-9]+\.[0-9]+$",
    ]
    for path in paths:
        cmd += ["--include-path", f"{path}/**"]
    proc = subprocess.run(cmd, cwd=repo, capture_output=True, text=True, check=False)
    try:
        releases = json.loads(proc.stdout) if proc.returncode == 0 else None
    except json.JSONDecodeError:
        releases = None
    if not isinstance(releases, list):
        detail = (proc.stderr.strip().splitlines() or ["no output"])[-1]
        raise ReleasePrepError(f"git-cliff printed no commits of the component ({detail})")
    if not releases:
        return None, []
    release = releases[0]
    previous = (release.get("previous") or {}).get("version")
    changes = []
    for commit in release.get("commits", []):
        m = _CONVENTIONAL.match(commit.get("raw_message", ""))
        kind = m.group(1).lower() if m else ""
        changes.append(Change(commit["id"], kind, bool(commit.get("breaking"))))
    return previous, changes


def next_stable(last: str | None, changes: list[Change]) -> str | None:
    """Apply the version rule of 18 section 3 and ADR 0060 item 2.

    Args:
        last: the last stable version `X.Y.Z`, None before the first release.
        changes: the commits since then.

    Returns:
        The next stable version, or None when no commit is a fix, a feature or a break.
        A break gives a major from 1.0.0 on and a minor before; a feature a minor; a fix (or
        `perf`) a patch. The first release is INITIAL_VERSION.

    Example: `next_stable("0.4.2", [Change("a", "feat", True)])` is `"0.5.0"`.
    """
    breaking = any(c.breaking for c in changes)
    feature = any(c.kind == "feat" for c in changes)
    fix = any(c.kind in PATCH_KINDS for c in changes)
    if not (breaking or feature or fix):
        return None
    if last is None:
        return INITIAL_VERSION
    major, minor, patch = _version_key(last)
    if breaking and major >= 1:
        return f"{major + 1}.0.0"
    if breaking or feature:
        return f"{major}.{minor + 1}.0"
    return f"{major}.{minor}.{patch + 1}"


def release_changelog(text: str, version: str, date: str, stable_of_beta: str | None) -> str:
    """Return the changelog with "Unreleased" renamed to `[version] - date`.

    A new empty "Unreleased" stays on top for the next tickets.

    Args:
        text: the CHANGELOG.md of the component.
        version: the new version.
        date: release date `YYYY-MM-DD`.
        stable_of_beta: the last beta of the same version for a stable release after betas,
            whose entries already stand in the beta sections; its section may be empty.

    Returns:
        The new text.

    Raises:
        ReleasePrepError: no "Unreleased" section, no entries in it (unless it is the stable
            release after a beta), or a section for `version` exists already.
    """
    if re.search(rf"^## \[{re.escape(version)}\]", text, re.MULTILINE):
        raise ReleasePrepError(f"CHANGELOG.md already has a section [{version}]")
    head = _UNRELEASED.search(text)
    if head is None:
        raise ReleasePrepError("CHANGELOG.md has no section '## [Unreleased]'")
    nxt = _HEADING.search(text, head.end())
    end = nxt.start() if nxt else len(text)
    body = text[head.end() : end].strip()
    if not body:
        if stable_of_beta is None:
            raise ReleasePrepError("CHANGELOG.md has no entries under '## [Unreleased]'")
        body = (
            f"Stable release of {stable_of_beta}; the changes are listed in the beta sections "
            "below."
        )
    section = f"## [Unreleased]\n\n## [{version}] - {date}\n\n{body}\n"
    rest = text[end:]
    return text[: head.start()] + section + ("\n" + rest if rest else "")


def checklist(repo: Path, rev_range: str, paths: tuple[str, ...], changelog: str) -> list[Commit]:
    """Return the commits of `rev_range` under `paths` that did not change `changelog`."""
    log = _git(repo, "log", "--reverse", "--format=%H%x1f%s", rev_range, "--", *(paths or (".",)))
    out: list[Commit] = []
    for line in log.splitlines():
        sha, _, subject = line.partition("\x1f")
        if changelog not in _changed_files(repo, sha):
            out.append(Commit(sha, subject))
    return out


def pin_server_version(
    text: str, server_version: str | None, *, channel: str, server_tags: set[str]
) -> str:
    """Return the bundle template with the server version pinned (18 section 2.1 rule 1).

    Args:
        text: infra/compose/.env.example.
        server_version: the value of `SERVER_VERSION`, None when it was not given.
        channel: `stable` or `beta` of the product release.
        server_tags: the existing tags `server/v...` of the repository.

    Returns:
        The text with exactly the line `BOTHY_VERSION=` replaced by `server_version`, or
        unchanged without one; every other byte stays.

    Raises:
        ReleasePrepError: not exactly one such line; without `server_version` the line still
            carries the placeholder; `server_version` is not SemVer without "v", its tag
            `server/v<version>` is missing, or it is a pre-release in a stable product.

    Example: `pin_server_version("BOTHY_VERSION=0.0.0-placeholder\\n", "1.4.0",
    channel="stable", server_tags={"server/v1.4.0"})` is `"BOTHY_VERSION=1.4.0\\n"`.
    """
    template = components.BUNDLE_ENV_TEMPLATE
    lines = list(_PIN_LINE.finditer(text))
    if len(lines) != 1:
        raise ReleasePrepError(
            f"{template}: expected exactly one line BOTHY_VERSION=, found {len(lines)}"
        )
    line = lines[0]
    if server_version is None:
        if line.group(2).strip() == components.SERVER_VERSION_PLACEHOLDER:
            raise ReleasePrepError(
                f"{template} still pins {components.SERVER_VERSION_PLACEHOLDER}; give the"
                " server version of the bundle with SERVER_VERSION=<version>"
            )
        return text
    # The private SemVer pattern is the one components.py checks the committed pin with, so
    # release-prep cannot write a value that release-pins rejects.
    semver = components._SEMVER.match(server_version)
    if semver is None:
        raise ReleasePrepError(
            f"SERVER_VERSION '{server_version}' is not a SemVer version without 'v'"
        )
    if f"server/v{server_version}" not in server_tags:
        raise ReleasePrepError(
            f"SERVER_VERSION {server_version}: the tag server/v{server_version} does not exist"
        )
    if semver.group(4) and channel != "beta":
        raise ReleasePrepError(
            f"SERVER_VERSION {server_version} is a server pre-release: a stable product pins no"
            " server pre-release; use CHANNEL=beta"
        )
    return f"{text[: line.start()]}{line.group(1)}{server_version}{text[line.end() :]}"


def documented_command(text: str, name: str, version: str) -> str:
    """Return a file with every documented installation command naming `version`.

    Only URLs of the form `https://github.com/<owner>/bothy/releases/download/v<X>/install.sh`
    change (pattern `install_options_check.INSTALL_URL`); X becomes `version`.

    Args:
        text: README.md or docs/operations/install-compose.md.
        name: the file's repository path, for the message.
        version: the new product version.

    Returns:
        The new text.

    Raises:
        ReleasePrepError: the text names no such URL.
    """

    def replace(m: re.Match[str]) -> str:
        return f"https://github.com/{m['owner']}/bothy/releases/download/v{version}/install.sh"

    new, count = install_options_check.INSTALL_URL.subn(replace, text)
    if count == 0:
        raise ReleasePrepError(
            f"{name} names no installation command"
            " (https://github.com/OWNER/bothy/releases/download/v<version>/install.sh)"
        )
    return new


def _product_files(
    repo: Path, version: str, server_version: str | None, channel: str
) -> dict[str, str]:
    """Return the new text of each further file of a product release, keyed by path."""
    out: dict[str, str] = {}
    template = repo / components.BUNDLE_ENV_TEMPLATE
    if not template.is_file():
        raise ReleasePrepError(f"{components.BUNDLE_ENV_TEMPLATE} is missing")
    server_tags = set(_git(repo, "tag", "--list", "server/v*").split())
    text = template.read_text(encoding="utf-8")
    pinned = pin_server_version(text, server_version, channel=channel, server_tags=server_tags)
    if pinned != text:
        out[components.BUNDLE_ENV_TEMPLATE] = pinned
    for name in install_options_check.DOCUMENTED_COMMAND_FILES:
        path = repo / name
        if not path.is_file():
            raise ReleasePrepError(f"{name} is missing; it names the installation command")
        out[name] = documented_command(path.read_text(encoding="utf-8"), name, version)
    return out


def _write(path: Path, text: str) -> None:
    fd, tmp = tempfile.mkstemp(dir=path.parent, prefix=f".{path.name}.")
    with os.fdopen(fd, "w", encoding="utf-8") as fh:
        fh.write(text)
    Path(tmp).replace(path)


def prepare(
    component: str,
    *,
    channel: str = "stable",
    repo: Path,
    git_cliff: Path,
    config: Path,
    date: str | None = None,
    server_version: str | None = None,
) -> Result:
    """Prepare the release of one component (ADR 0060 items 1 to 3 and 6).

    Args:
        component: a release component of tools/ci/components.py.
        channel: `stable` or `beta`.
        repo: the repository root.
        git_cliff: the pinned git-cliff binary.
        config: cliff.toml.
        date: release date `YYYY-MM-DD`, today (UTC) when None.
        server_version: `SERVER_VERSION`, the server version the product bundle pins; only
            for the component `bothy`.

    Returns:
        The written version, its tag, the changed files and the checklist.

    Raises:
        ReleasePrepError: unknown component or channel, `server_version` for another component,
            nothing to release, a tag that git-cliff cannot see, a changelog problem, or for
            `bothy` a problem of the server pin or the documented installation command
            (`pin_server_version`, `documented_command`); nothing is written then.

    Example:
        `prepare("server", repo=Path("."), git_cliff=..., config=Path("cliff.toml"))` turns
        `services/server/VERSION` from `0.1.0` into `0.1.1` after a `fix(server): ...` commit.
    """
    if channel not in CHANNELS:
        raise ReleasePrepError(f"CHANNEL must be stable or beta, not '{channel}'")
    if server_version is not None and component != PRODUCT:
        raise ReleasePrepError(
            f"--server-version (SERVER_VERSION) applies only to the component {PRODUCT},"
            f" not to '{component}'"
        )
    try:
        prefix = components.tag_prefix(component)
        paths = components.release_paths(component)
    except components.ReleaseError as exc:
        raise ReleasePrepError(str(exc)) from exc
    directory = components.RELEASE_COMPONENTS[component][0]
    rel = (lambda name: name) if directory == "." else (lambda name: f"{directory}/{name}")
    last = last_stable_tag(repo, prefix)
    previous, changes = unreleased_changes(repo, git_cliff, config, prefix, paths)
    if not changes:
        raise ReleasePrepError(
            f"nothing to release: no commit of {component}" + (f" since {last}" if last else "")
        )
    if last is not None and previous != last:
        # git-cliff drops a tag whose commit is outside the include paths and then counts
        # from an older one; the release commit of ADR 0060 always changes VERSION.
        raise ReleasePrepError(
            f"git-cliff counts from {previous or 'the first commit'}, not from tag {last}: "
            f"the tag is on a commit that does not change {directory}"
        )
    stable = next_stable(last[len(prefix) + 1 :] if last else None, changes)
    if stable is None:
        raise ReleasePrepError(
            f"nothing to release: no fix, feat or breaking change of {component}"
            + (f" since {last}" if last else "")
        )
    rev_range = f"{last}..HEAD" if last else "HEAD"
    betas = _beta_numbers(repo, prefix, stable)
    version = f"{stable}-beta.{(betas[-1] if betas else 0) + 1}" if channel == "beta" else stable
    if f"{prefix}v{version}" in _git(repo, "tag", "--list").split():
        raise ReleasePrepError(f"tag {prefix}v{version} exists already")
    changelog_path = repo / rel("CHANGELOG.md")
    if not changelog_path.is_file():
        raise ReleasePrepError(f"{rel('CHANGELOG.md')} is missing")
    stable_of_beta = f"{stable}-beta.{betas[-1]}" if channel == "stable" and betas else None
    new_changelog = release_changelog(
        changelog_path.read_text(encoding="utf-8"),
        version,
        date or datetime.datetime.now(datetime.UTC).date().isoformat(),
        stable_of_beta,
    )
    others = _product_files(repo, version, server_version, channel) if component == PRODUCT else {}
    todo = checklist(repo, rev_range, paths, rel("CHANGELOG.md"))
    _write(changelog_path, new_changelog)
    _write(repo / rel("VERSION"), version + "\n")
    for name, text in others.items():
        _write(repo / name, text)
    return Result(
        component,
        version,
        f"{prefix}v{version}",
        rel("CHANGELOG.md"),
        rel("VERSION"),
        todo,
        tuple(others),
    )


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Prepare the release of one component.")
    parser.add_argument("--component", required=True)
    parser.add_argument("--channel", default="stable")
    parser.add_argument("--repo", type=Path, default=Path.cwd())
    parser.add_argument("--git-cliff", type=Path, required=True)
    parser.add_argument("--config", type=Path, default=None)
    parser.add_argument("--date", default=None)
    parser.add_argument("--server-version", default=None)
    args = parser.parse_args(argv)
    # An empty SERVER_VERSION from make means "not given", like a missing one.
    server_version = args.server_version or None
    if server_version is not None and args.component != PRODUCT:
        parser.error(f"--server-version applies only to the component {PRODUCT}")
    config = args.config or args.repo / "cliff.toml"
    try:
        res = prepare(
            args.component,
            channel=args.channel or "stable",
            repo=args.repo,
            git_cliff=args.git_cliff,
            config=config,
            date=args.date,
            server_version=server_version,
        )
    except (ReleasePrepError, subprocess.CalledProcessError) as exc:
        print(f"release-prep: {exc}", file=sys.stderr)
        return 1
    print(f"release-prep: {res.component} {res.version} (tag {res.tag})")
    print(f"  wrote {', '.join((res.version_file, res.changelog, *res.other_files))}")
    if res.checklist:
        print(f"  commits without a change to {res.changelog} (does the section cover them?):")
        for c in res.checklist:
            print(f"    {c.sha[:12]} {c.subject}")
    else:
        print(f"  every commit of the component changed {res.changelog}")
    print("  next: review the section (server: upgrade notes, migrations, compatibility,")
    print("  security), then commit and open the pull request:")
    print(f'    git commit -s -m "chore(release): {res.component} v{res.version}"')
    print(f"  after the merge, on the squash commit: git tag -s {res.tag} <commit>")
    return 0


if __name__ == "__main__":
    sys.exit(main())
