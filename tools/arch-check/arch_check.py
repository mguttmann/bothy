# SPDX-License-Identifier: Apache-2.0
"""Architecture test of the Dart workspace (docs/spec/15-client-architecture.md 3.1, brief M4-01).

The rules come from `rules.yaml` next to this file. For every member of the workspace the test
reads the manifest and every `import` and `export` directive under `lib/`, `test/`,
`integration_test/` and `bin/`, and reports each place that breaks a rule of 15 3.1:

- rule 1 and 7: Flutter (`package:flutter/`, `dart:ui`, a Flutter SDK dependency) in a pure
  Dart member; `dart:io` in the library of `bothy_noise`; a runtime dependency of
  `bothy_noise` other than `cryptography`;
- rule 2: a package that imports an app, or a relative import that leaves the member;
- rule 3, 6 and 7: a member that reaches `sync_engine` or `local_store` through any dependency;
- rule 4: a native source outside `device_key` and `apps/client/{ios,macos,android,windows}`;
- rule 8: a dependency or import of a member that the diagram does not reach (allow list);
- rule 9: a manifest under `apps/` or `packages/` that is not a member of the rules and of the
  root `workspace:` list, or a member without its manifest;
- rule 10: a test library (`package:<member>/testing.dart` or its `src/testing/` folder,
  brief M4-17) imported or exported outside the `test/` and `integration_test/` folders, also
  relatively inside its own member.

Usage: arch_check.py [--root <repository>] [--rules <rules.yaml>]
Exit 0 prints `arch-check: <n> members, <m> files, ok`; exit 1 prints one line per finding
(`<file>:<line>: rule <n> (15 3.1): <message>`); exit 2 means the rules file is invalid.
"""

from __future__ import annotations

import argparse
import os
import posixpath
import re
import sys
from collections.abc import Iterator
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

import yaml

KINDS = frozenset({"dart", "flutter", "plugin", "app"})
SOURCE_DIRS = ("lib", "test", "integration_test", "bin")
NATIVE_SUFFIXES = frozenset({".swift", ".m", ".mm", ".kt", ".java", ".cpp", ".cc", ".h"})
# Build output and tool state are not sources; hidden folders hold editor and tool state.
SKIP_DIRS = frozenset({"build", ".dart_tool", "node_modules"})
FLUTTER_PREFIX = "package:flutter/"
RULES_NAME = "tools/arch-check/rules.yaml"

# Start of a directive at the beginning of a line; the directive runs to the next `;`.
_DIRECTIVE = re.compile(r"^[ \t]*(?:import|export)\b", re.MULTILINE)
_STRING = re.compile(r"'([^'\n]*)'|\"([^\"\n]*)\"")


class RulesError(Exception):
    """The rules file does not describe a valid workspace."""


@dataclass(frozen=True)
class Forbidden:
    """Imports a member must not have."""

    rule: int
    uris: tuple[str, ...]
    dirs: frozenset[str] | None

    def matches(self, uri: str, top: str) -> bool:
        """Whether `uri`, found in the top-level folder `top`, is forbidden."""
        if self.dirs is not None and top not in self.dirs:
            return False
        return any(uri.startswith(u) if u.endswith("/") else uri == u for u in self.uris)


@dataclass(frozen=True)
class Limit:
    """A rule number with a list of names (never reached members, allowed dependencies)."""

    rule: int
    names: tuple[str, ...]


@dataclass(frozen=True)
class Member:
    """One member of the workspace as `rules.yaml` describes it."""

    name: str
    path: str
    kind: str
    edges: tuple[str, ...]
    forbidden: tuple[Forbidden, ...] = ()
    never: Limit | None = None
    runtime_dependencies: Limit | None = None
    platform_code: frozenset[str] | None = None

    @property
    def is_app(self) -> bool:
        """Whether the member is one of the apps."""
        return self.kind == "app"

    def flutter_rule(self) -> int | None:
        """The rule that keeps the member free of Flutter everywhere, if any."""
        for entry in self.forbidden:
            if entry.dirs is None and FLUTTER_PREFIX in entry.uris:
                return entry.rule
        return None


@dataclass(frozen=True)
class TestLibraries:
    """Test support libraries: where a library with this file name may be imported."""

    rule: int
    library: str
    dirs: frozenset[str]

    @property
    def folder(self) -> str:
        """The source folder of the library below `lib/` (`src/testing/` for `testing.dart`)."""
        return f"src/{self.library.removesuffix('.dart')}/"

    def is_test_library(self, rel: str) -> bool:
        """Whether `rel` (a path below a member's `lib/`) is the test library or its folder."""
        return rel == self.library or rel.startswith(self.folder)


@dataclass
class Rules:
    """The members in the order of 15 3 and the reach of the diagram."""

    members: dict[str, Member]
    _reach: dict[str, frozenset[str]] = field(default_factory=dict)
    test_libraries: TestLibraries | None = None

    def reach(self, name: str) -> frozenset[str]:
        """Every member the diagram reaches from `name`, without `name` itself."""
        if name not in self._reach:
            self._reach[name] = _closure(name, lambda n: self.members[n].edges)
        return self._reach[name]

    def by_path(self) -> dict[str, Member]:
        """The members keyed by their folder."""
        return {m.path: m for m in self.members.values()}


@dataclass(frozen=True, order=True)
class Finding:
    """One place that breaks a rule."""

    file: str
    line: int
    rule: int
    message: str

    def __str__(self) -> str:
        """Return the finding as `file:line: rule <n> (15 3.1): message`."""
        return f"{self.file}:{self.line}: rule {self.rule} (15 3.1): {self.message}"


def _closure(start: str, edges: Any) -> frozenset[str]:
    seen: set[str] = set()
    stack = list(edges(start))
    while stack:
        node = stack.pop()
        if node in seen:
            continue
        seen.add(node)
        stack.extend(edges(node))
    seen.discard(start)
    return frozenset(seen)


def _names(value: Any, where: str) -> tuple[str, ...]:
    if not isinstance(value, list) or not all(isinstance(v, str) for v in value):
        raise RulesError(f"{where} must be a list of names")
    return tuple(value)


def _limit(value: Any, where: str, key: str) -> Limit | None:
    if value is None:
        return None
    if not isinstance(value, dict) or not isinstance(value.get("rule"), int):
        raise RulesError(f"{where} needs a rule number")
    return Limit(value["rule"], _names(value.get(key), f"{where}.{key}"))


def _member(item: Any) -> Member:
    if not isinstance(item, dict) or not isinstance(item.get("name"), str):
        raise RulesError("every member needs a name")
    name = item["name"]
    if not isinstance(item.get("path"), str):
        raise RulesError(f"{name}: path is missing")
    if item.get("kind") not in KINDS:
        raise RulesError(f"{name}: unknown kind {item.get('kind')}")
    forbidden = []
    for entry in item.get("forbidden") or []:
        if not isinstance(entry, dict) or not isinstance(entry.get("rule"), int):
            raise RulesError(f"{name}: every forbidden entry needs a rule number")
        dirs = entry.get("dirs")
        forbidden.append(
            Forbidden(
                entry["rule"],
                _names(entry.get("uris"), f"{name}.forbidden.uris"),
                None if dirs is None else frozenset(_names(dirs, f"{name}.forbidden.dirs")),
            )
        )
    platform = item.get("platform_code")
    return Member(
        name=name,
        path=item["path"],
        kind=item["kind"],
        edges=_names(item.get("edges", []), f"{name}.edges"),
        forbidden=tuple(forbidden),
        never=_limit(item.get("never"), f"{name}.never", "members"),
        runtime_dependencies=_limit(
            item.get("runtime_dependencies"), f"{name}.runtime_dependencies", "allowed"
        ),
        platform_code=None
        if platform is None
        else frozenset(_names(platform, f"{name}.platform_code")),
    )


def load_rules(path: Path) -> Rules:
    """Read and validate `rules.yaml`: known kinds, known edges, no cycle."""
    data = yaml.safe_load(path.read_text(encoding="utf-8"))
    items = data.get("members") if isinstance(data, dict) else None
    if not isinstance(items, list):
        raise RulesError(f"{path.name}: members must be a list")
    members: dict[str, Member] = {}
    for item in items:
        member = _member(item)
        if member.name in members:
            raise RulesError(f"{member.name}: listed twice")
        members[member.name] = member
    for member in members.values():
        for target in member.edges:
            if target not in members:
                raise RulesError(f"{member.name}: edge to unknown member {target}")
    for name, member in members.items():
        if any(name in _closure(e, lambda n: members[n].edges) | {e} for e in member.edges):
            raise RulesError(f"{name}: the edges form a cycle")
    return Rules(members, test_libraries=_test_libraries(data.get("test_libraries")))


def _test_libraries(value: Any) -> TestLibraries | None:
    if value is None:
        return None
    if (
        not isinstance(value, dict)
        or not isinstance(value.get("rule"), int)
        or not isinstance(value.get("library"), str)
        or not value["library"].endswith(".dart")
    ):
        raise RulesError("test_libraries needs a rule number and a library file name")
    return TestLibraries(
        value["rule"],
        value["library"],
        frozenset(_names(value.get("dirs"), "test_libraries.dirs")),
    )


def directives(text: str) -> Iterator[tuple[int, str]]:
    """Every URI of an `import` or `export` directive with its line number.

    A directive may span lines and carry conditional URIs (`if (dart.library.io) '...'`); it
    ends at the next `;`. Line comments never start a directive.
    """
    for match in _DIRECTIVE.finditer(text):
        end = text.find(";", match.end())
        span_end = len(text) if end < 0 else end
        for literal in _STRING.finditer(text, match.end(), span_end):
            uri = literal.group(1) if literal.group(1) is not None else literal.group(2)
            yield text.count("\n", 0, literal.start()) + 1, uri


def _walk(base: Path) -> Iterator[Path]:
    for directory, dirs, files in os.walk(base):
        dirs[:] = sorted(d for d in dirs if d not in SKIP_DIRS and not d.startswith("."))
        for name in sorted(files):
            yield Path(directory) / name


def _manifest(path: Path) -> dict[str, Any]:
    data = yaml.safe_load(path.read_text(encoding="utf-8"))
    return data if isinstance(data, dict) else {}


def _dependencies(manifest: dict[str, Any]) -> dict[str, Any]:
    deps: dict[str, Any] = {}
    for key in ("dependencies", "dev_dependencies"):
        section = manifest.get(key)
        if isinstance(section, dict):
            deps.update(section)
    return deps


class _Checker:
    def __init__(self, root: Path, rules: Rules) -> None:
        self.root = root
        self.rules = rules
        self.findings: list[Finding] = []
        self.files = 0
        self.manifests: dict[str, dict[str, Any]] = {}

    def add(self, file: str, line: int, rule: int, message: str) -> None:
        self.findings.append(Finding(file, line, rule, message))

    def membership(self) -> None:
        by_path = self.rules.by_path()
        found: set[str] = set()
        for base in ("apps", "packages"):
            for path in _walk(self.root / base):
                if path.name == "pubspec.yaml":
                    found.add(path.parent.relative_to(self.root).as_posix())
        for rel in sorted(found - set(by_path)):
            self.add(
                f"{rel}/pubspec.yaml",
                1,
                9,
                f"not a member of {RULES_NAME} and the root workspace",
            )
        for member in self.rules.members.values():
            manifest = f"{member.path}/pubspec.yaml"
            if member.path not in found:
                self.add(
                    manifest, 1, 9, f"member {member.name} of {RULES_NAME} has no pubspec.yaml"
                )
                continue
            data = _manifest(self.root / manifest)
            self.manifests[member.name] = data
            if data.get("name") != member.name:
                self.add(
                    manifest, 1, 9, f"name is {data.get('name')}, rules.yaml says {member.name}"
                )
        root = self.root / "pubspec.yaml"
        listed = _manifest(root).get("workspace") if root.is_file() else None
        listed = [str(p).rstrip("/") for p in listed] if isinstance(listed, list) else []
        for rel in listed:
            if rel not in by_path:
                self.add(
                    "pubspec.yaml", 1, 9, f"workspace lists {rel}, which {RULES_NAME} does not name"
                )
        for rel in by_path:
            if rel not in listed:
                self.add("pubspec.yaml", 1, 9, f"workspace does not list {rel}")

    def member_deps(self, name: str) -> tuple[str, ...]:
        deps = _dependencies(self.manifests.get(name, {}))
        return tuple(d for d in deps if d in self.rules.members and d != name)

    def dependencies(self, member: Member) -> None:
        if member.name not in self.manifests:
            return
        manifest = f"{member.path}/pubspec.yaml"
        data = self.manifests[member.name]
        never = set(member.never.names) if member.never else set()
        direct = self.member_deps(member.name)
        for dep in direct:
            target = self.rules.members[dep]
            if member.never and dep in never:
                self.add(manifest, 1, member.never.rule, f"{member.name} must not depend on {dep}")
            elif target.is_app and not member.is_app:
                self.add(manifest, 1, 2, f"{member.name} must not depend on the app {dep}")
            elif dep not in self.rules.reach(member.name):
                self.add(
                    manifest,
                    1,
                    8,
                    f"{member.name} may not depend on {dep}; the diagram does not reach it",
                )
        if member.never:
            for target_name in member.never.names:
                if target_name in direct:
                    continue
                for dep in direct:
                    if target_name in _closure(dep, self.member_deps):
                        self.add(
                            manifest,
                            1,
                            member.never.rule,
                            f"{member.name} must not depend on {target_name} (through {dep})",
                        )
                        break
        rule = member.flutter_rule()
        if rule is not None:
            sdk = sorted(
                d
                for d, v in _dependencies(data).items()
                if isinstance(v, dict) and v.get("sdk") == "flutter"
            )
            if sdk:
                self.add(
                    manifest,
                    1,
                    rule,
                    f"{member.name} must not depend on the Flutter SDK ({', '.join(sdk)})",
                )
        if member.runtime_dependencies:
            allowed = set(member.runtime_dependencies.names)
            runtime = data.get("dependencies")
            for dep in runtime if isinstance(runtime, dict) else {}:
                if dep not in allowed:
                    self.add(
                        manifest,
                        1,
                        member.runtime_dependencies.rule,
                        f"{member.name} may depend at runtime only on"
                        f" {', '.join(sorted(allowed))}, not on {dep}",
                    )

    def imports(self, member: Member) -> None:
        base = self.root / member.path
        for top in SOURCE_DIRS:
            for path in _walk(base / top):
                if path.suffix != ".dart":
                    continue
                self.files += 1
                rel = path.relative_to(self.root).as_posix()
                text = path.read_text(encoding="utf-8", errors="replace")
                for line, uri in directives(text):
                    self.uri(member, rel, line, uri, top)

    def test_library(self, member: Member, rel: str, line: int, uri: str, top: str) -> bool:
        """Report a test library reached from outside the test folders (rule 10)."""
        tests = self.rules.test_libraries
        if tests is None or top in tests.dirs:
            return False
        if uri.startswith("package:"):
            package, _, inner = uri[len("package:") :].partition("/")
            target = f"package:{package}/{tests.library}"
        elif re.match(r"^[a-z][a-z0-9+.-]*:", uri):
            return False
        else:
            lib = f"{member.path}/lib/"
            resolved = posixpath.normpath(posixpath.join(posixpath.dirname(rel), uri))
            own = rel.removeprefix(lib) if rel.startswith(lib) else None
            if not resolved.startswith(lib) or (own is not None and tests.is_test_library(own)):
                return False
            inner = resolved.removeprefix(lib)
            target = f"package:{member.name}/{tests.library}"
        if not tests.is_test_library(inner):
            return False
        allowed = " and ".join(f"{d}/" for d in sorted(tests.dirs))
        self.add(
            rel,
            line,
            tests.rule,
            f"{member.name} reaches the test library {target} outside {allowed}",
        )
        return True

    def uri(self, member: Member, rel: str, line: int, uri: str, top: str) -> None:
        if self.test_library(member, rel, line, uri, top):
            return
        for entry in member.forbidden:
            if entry.matches(uri, top):
                self.add(rel, line, entry.rule, f"{member.name} must not import {uri}")
                return
        if uri.startswith("package:"):
            package = uri[len("package:") :].split("/", 1)[0]
            if package == member.name or package not in self.rules.members:
                return
            target = self.rules.members[package]
            if member.never and package in member.never.names:
                self.add(rel, line, member.never.rule, f"{member.name} must not import {package}")
            elif target.is_app and not member.is_app:
                self.add(rel, line, 2, f"{member.name} must not import the app {package}")
            elif package not in self.rules.reach(member.name):
                self.add(
                    rel,
                    line,
                    8,
                    f"{member.name} may not import {package}; the diagram does not reach it",
                )
            return
        if re.match(r"^[a-z][a-z0-9+.-]*:", uri):
            return
        resolved = posixpath.normpath(posixpath.join(posixpath.dirname(rel), uri))
        if not (resolved + "/").startswith(member.path + "/"):
            self.add(rel, line, 2, f"{member.name} imports {uri}, which leaves the member")

    def native(self, member: Member) -> None:
        base = self.root / member.path
        allowed = member.platform_code or frozenset()
        for path in _walk(base):
            if path.suffix not in NATIVE_SUFFIXES:
                continue
            top = path.relative_to(base).parts[0]
            if top in allowed:
                continue
            self.add(
                path.relative_to(self.root).as_posix(),
                1,
                4,
                "native source outside device_key and apps/client/{ios,macos,android,windows}",
            )


def check(root: Path, rules: Rules) -> tuple[list[Finding], int, int]:
    """Check the workspace below `root`; return the findings, the members and the files read."""
    checker = _Checker(root, rules)
    checker.membership()
    for member in rules.members.values():
        checker.dependencies(member)
        checker.imports(member)
        checker.native(member)
    findings = sorted(checker.findings, key=lambda f: (f.file, f.line))
    return findings, len(rules.members), checker.files


def main(argv: list[str] | None = None) -> int:
    """Command line entry point; see the module documentation."""
    here = Path(__file__).resolve().parent
    parser = argparse.ArgumentParser(description="Architecture test of the Dart workspace.")
    parser.add_argument("--root", type=Path, default=here.parents[1])
    parser.add_argument("--rules", type=Path, default=here / "rules.yaml")
    args = parser.parse_args(argv)
    try:
        rules = load_rules(args.rules)
    except RulesError as exc:
        print(f"arch-check: {exc}", file=sys.stderr)
        return 2
    findings, members, files = check(args.root, rules)
    if findings:
        for finding in findings:
            print(finding, file=sys.stderr)
        count = len(findings)
        print(f"arch-check: {count} finding{'s' if count != 1 else ''}", file=sys.stderr)
        return 1
    print(f"arch-check: {members} members, {files} files, ok")
    return 0


if __name__ == "__main__":
    sys.exit(main())
