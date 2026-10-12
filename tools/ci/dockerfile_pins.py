# SPDX-License-Identifier: Apache-2.0
"""Pin check: every base image of a tracked Dockerfile is pinned by digest (SEC-SC-4).

docs/spec/18-release.md "Server-Images" rule 5 and docs/spec/23-qualitaet.md "Lieferkette":
base images are pinned by digest, Renovate keeps the digests current, and this check in
`make lint` rejects every `FROM` without one. A tag alone can be moved by whoever controls the
registry; a digest names exactly one image index.

Rules per file:

* every `FROM` names an image with `@sha256:` and 64 lower-case hex digits, except `scratch` and
  the name of an earlier stage of the same file (`FROM build`); flags such as
  `--platform=$BUILDPLATFORM` and the suffix `AS <name>` are understood, line continuations
  are joined;
* the same holds for images that enter a build without `FROM`: `COPY --from=<image>` and
  `RUN --mount=...,from=<image>` (a stage name or a stage index passes);
* a `# syntax=` parser directive (only the leading comment block counts, as for BuildKit) is
  pinned the same way, because it names the frontend image that parses the file.

A reference built from a variable (`FROM ${BASE}`) cannot be verified and is a finding unless
the digest itself is written out. Files are the tracked files that `is_dockerfile` accepts:
`Dockerfile` and `Containerfile` in any case, also with a prefix or suffix after a dot.

Usage: python tools/ci/dockerfile_pins.py [--repo DIR] [FILE ...]
Exit codes: 0 every reference is pinned, 1 at least one finding, 2 a named file is missing.
"""

from __future__ import annotations

import argparse
import re
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path, PurePosixPath

#: A reference ends in a sha256 digest of exactly 64 lower-case hex digits.
PINNED = re.compile(r"@sha256:[0-9a-f]{64}$")
#: A parser directive line of the leading comment block (`# key=value`).
DIRECTIVE = re.compile(r"^#\s*([A-Za-z][A-Za-z0-9_-]*)\s*=\s*(.*?)\s*$")


@dataclass(frozen=True)
class Finding:
    """One unpinned or unreadable image reference.

    Attributes:
        path: file name as given (repository-relative for tracked files).
        line: 1-based line of the instruction (its first line when it continues).
        reference: the image reference as written, empty when the instruction names none.
        message: why the reference fails.
    """

    path: str
    line: int
    reference: str
    message: str

    def __str__(self) -> str:
        """Return `path:line: reference: message`, the form editors and CI logs link."""
        return f"{self.path}:{self.line}: {self.reference}: {self.message}"


def is_dockerfile(path: str) -> bool:
    """Return whether `path` is a Dockerfile for this check.

    Args:
        path: a repository-relative path.

    Returns:
        True when the base name, in any case, is `Dockerfile` or `Containerfile`, starts with it
        and a dot (`Dockerfile.dev`) or ends with a dot and it (`dev.Dockerfile`); ignore files
        (`*.dockerignore`) are not Dockerfiles.

    Example:
        >>> is_dockerfile("services/server/Dockerfile"), is_dockerfile("x/Dockerfile.dockerignore")
        (True, False)
    """
    name = PurePosixPath(path).name.lower()
    if name.endswith(".dockerignore"):
        return False
    return any(
        name == base or name.startswith(base + ".") or name.endswith("." + base)
        for base in ("dockerfile", "containerfile")
    )


def _instructions(text: str) -> list[tuple[int, str]]:
    """Split `text` into instructions with the line they start on; continuations are joined."""
    out: list[tuple[int, str]] = []
    start, parts = 0, list[str]()
    for number, raw in enumerate(text.splitlines(), start=1):
        stripped = raw.strip()
        if not parts and (not stripped or stripped.startswith("#")):
            continue
        if parts and stripped.startswith("#"):
            # A comment inside a continued instruction is dropped by the Dockerfile parser.
            continue
        if not parts:
            start = number
        if stripped.endswith("\\"):
            parts.append(stripped[:-1])
            continue
        parts.append(stripped)
        out.append((start, " ".join(parts)))
        parts = []
    if parts:
        out.append((start, " ".join(parts)))
    return out


def _directives(text: str) -> list[tuple[int, str, str]]:
    """Parser directives of the leading comment block: (line, lower-case key, value)."""
    found: list[tuple[int, str, str]] = []
    for number, raw in enumerate(text.splitlines(), start=1):
        m = DIRECTIVE.match(raw.strip())
        if not m:
            break
        found.append((number, m.group(1).lower(), m.group(2)))
    return found


def _from_reference(words: list[str]) -> tuple[str, str]:
    """Return (image reference, stage name) of the words after `FROM`; empty when absent."""
    rest = [w for w in words if not w.startswith("--")]
    image = rest[0] if rest else ""
    stage = rest[2].lower() if len(rest) >= 3 and rest[1].lower() == "as" else ""
    return image, stage


def _flag_images(keyword: str, words: list[str]) -> list[str]:
    """Images named by flags of `COPY` (`--from=`) and `RUN` (`--mount=...,from=`)."""
    images: list[str] = []
    for word in words:
        if not word.startswith("--"):
            break
        name, _, value = word[2:].partition("=")
        if keyword == "copy" and name.lower() == "from":
            images.append(value)
        elif keyword == "run" and name.lower() == "mount":
            for option in value.split(","):
                key, _, val = option.partition("=")
                if key.strip().lower() == "from":
                    images.append(val.strip())
    return images


def _pinned_or_stage(image: str, stages: set[str]) -> bool:
    """Whether `image` is pinned, `scratch`, an earlier stage name or a stage index."""
    low = image.lower()
    return low == "scratch" or low in stages or image.isdigit() or bool(PINNED.search(image))


def check_text(path: str, text: str) -> list[Finding]:
    """Check the content of one Dockerfile.

    Args:
        path: the name used in findings.
        text: the file content.

    Returns:
        One finding per unpinned image of `FROM`, `COPY --from`, `RUN --mount=from` or a
        `# syntax=` directive, in file order; an empty list when everything is pinned (also for a
        file without `FROM`).
    """
    findings: list[Finding] = []
    for number, key, value in _directives(text):
        if key == "syntax" and not PINNED.search(value):
            findings.append(
                Finding(
                    path, number, value, "parser directive syntax is not pinned by @sha256 digest"
                )
            )
    stages: set[str] = set()
    for number, instruction in _instructions(text):
        words = instruction.split()
        keyword = words[0].lower()
        if keyword in ("copy", "run"):
            for image in _flag_images(keyword, words[1:]):
                if not _pinned_or_stage(image, stages):
                    msg = f"image in {keyword.upper()} is not pinned by @sha256 digest (SEC-SC-4)"
                    findings.append(Finding(path, number, image, msg))
            continue
        if keyword != "from":
            continue
        image, stage = _from_reference(words[1:])
        if not image:
            findings.append(Finding(path, number, "", "FROM names no image"))
        elif not _pinned_or_stage(image, stages):
            findings.append(
                Finding(
                    path, number, image, "base image is not pinned by @sha256 digest (SEC-SC-4)"
                )
            )
        if stage:
            stages.add(stage)
    return findings


def tracked_dockerfiles(root: Path) -> list[str]:
    """Return the tracked Dockerfiles of the repository at `root`, sorted.

    Args:
        root: a directory inside a git working tree.

    Returns:
        Repository-relative paths that `is_dockerfile` accepts.

    Raises:
        subprocess.CalledProcessError: when `root` is not inside a git working tree.
    """
    out = subprocess.run(
        ["git", "-C", str(root), "ls-files", "-z"],
        check=True,
        capture_output=True,
    ).stdout
    return sorted(p for p in out.decode("utf-8").split("\0") if p and is_dockerfile(p))


def main(argv: list[str] | None = None) -> int:
    """Check the named files, or every tracked Dockerfile; print findings on stderr.

    Args:
        argv: command line arguments without the program name.

    Returns:
        0 when every reference is pinned, 1 on a finding, 2 when a named file is missing.
    """
    parser = argparse.ArgumentParser(
        description="Every FROM of a tracked Dockerfile is pinned by digest."
    )
    parser.add_argument("--repo", type=Path, default=Path.cwd())
    parser.add_argument("files", nargs="*")
    args = parser.parse_args(argv)
    names: list[str] = args.files or tracked_dockerfiles(args.repo)
    findings: list[Finding] = []
    for name in names:
        try:
            text = (args.repo / name).read_text(encoding="utf-8")
        except FileNotFoundError:
            print(f"dockerfile_pins: {name}: no such file", file=sys.stderr)
            return 2
        findings.extend(check_text(name, text))
    for finding in findings:
        print(finding, file=sys.stderr)
    if findings:
        print(
            f"dockerfile_pins: {len(findings)} unpinned reference(s); pin each by @sha256 digest",
            file=sys.stderr,
        )
        return 1
    print(f"  dockerfile pins: ok ({len(names)} Dockerfile(s))")
    return 0


if __name__ == "__main__":
    sys.exit(main())
