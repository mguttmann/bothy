# SPDX-License-Identifier: Apache-2.0
"""Diagram gate: every Mermaid diagram of the repository renders and is readable.

Checks every tracked `*.mmd` file and every ```mermaid block in tracked Markdown files
with the pinned mermaid-cli (tools/node) and the Mermaid configuration generated from the
design tokens (design/build/mermaid.config.json). All diagrams render in one browser
session; on failure each diagram is rendered alone to name the broken ones.

A `*.mmd` file also needs its rendered `*.svg` next to it, rendered with that configuration
(the SVG names the design font) and no wider or taller than MAX_ASPECT. `--write` stamps each
SVG with the SHA-256 of its `.mmd` source and of the Mermaid configuration (STAMP_PREFIX);
the gate recomputes it and names every SVG whose source or theme changed since it was
rendered. The stamp replaces a byte comparison of the SVG, which depends on the browser.
Colours written in a `*.mmd` file must be design-token colours. The ERDs must match the data model
(erd_check.py). `--write` re-renders every `*.svg` (docs/quality/gates.md).

Environment: MMDC_PUPPETEER_CONFIG may point to a Puppeteer config file, e.g. to use a
locally installed Chromium instead of the one `make bootstrap` downloads.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import subprocess
import sys
import tempfile
from collections.abc import Callable
from dataclasses import dataclass
from pathlib import Path

import erd_check

REPO_ROOT = Path(__file__).resolve().parents[2]
DEFAULT_MMDC = REPO_ROOT / "tools" / "node" / "node_modules" / ".bin" / "mmdc"
_FENCE_OPEN = re.compile(r"^(?P<indent> {0,3})(?P<fence>`{3,}|~{3,})\s*mermaid\s*$")
_RENDER_TIMEOUT_S = 600
MERMAID_CONFIG = Path("design") / "build" / "mermaid.config.json"
RESOLVED_TOKENS = Path("design") / "build" / "tokens.resolved.json"
# Width to height (or height to width) of a rendered diagram. Wider diagrams shrink on a page
# until the text is unreadable; split them or change the direction.
MAX_ASPECT = 2.5
THEME_MARKER = "Atkinson Hyperlegible"
_HEX = re.compile(r"#[0-9a-fA-F]{6}\b|#[0-9a-fA-F]{3}\b")
STAMP_PREFIX = "bothy-diagram-source sha256="
_STAMP = re.compile(r"<!-- " + re.escape(STAMP_PREFIX) + r"([0-9a-f]{64}) -->")
_VIEWBOX = re.compile(r'viewBox="\s*[-\d.]+[\s,]+[-\d.]+[\s,]+([\d.]+)[\s,]+([\d.]+)\s*"')

Renderer = Callable[[Path, Path], tuple[bool, str]]


@dataclass(frozen=True)
class Job:
    """One diagram to render: where it comes from and its Mermaid source."""

    origin: str
    source: str


def extract_blocks(markdown: str) -> list[tuple[int, str]]:
    """Return (1-based line of the opening fence, block source) for each mermaid block."""
    blocks: list[tuple[int, str]] = []
    lines = markdown.splitlines()
    i = 0
    while i < len(lines):
        m = _FENCE_OPEN.match(lines[i])
        if not m:
            i += 1
            continue
        fence, start = m.group("fence"), i + 1
        body: list[str] = []
        i += 1
        while i < len(lines) and not lines[i].strip().startswith(fence):
            body.append(lines[i])
            i += 1
        blocks.append((start, "\n".join(body) + "\n"))
        i += 1
    return blocks


def collect_jobs(root: Path, names: list[str]) -> list[Job]:
    """Return one job per `.mmd` file and per Mermaid block in a Markdown file."""
    jobs: list[Job] = []
    for name in sorted(names):
        path = root / name
        if name.endswith(".mmd"):
            jobs.append(Job(name, path.read_text(encoding="utf-8")))
        elif name.endswith(".md"):
            jobs.extend(
                Job(f"{name}:{line}", src)
                for line, src in extract_blocks(path.read_text(encoding="utf-8"))
            )
    return jobs


def source_digest(root: Path, name: str) -> str:
    """Return the SHA-256 (hex) of a `.mmd` source together with the Mermaid configuration.

    Either input changes the rendered SVG; a missing configuration counts as empty.
    """
    config = root / MERMAID_CONFIG
    digest = hashlib.sha256()
    digest.update((root / name).read_bytes())
    digest.update(b"\0")
    digest.update(config.read_bytes() if config.is_file() else b"")
    return digest.hexdigest()


def stamp_svg(text: str, digest: str) -> str:
    """Return the SVG text with exactly one source stamp right after the `<svg ...>` tag.

    Raises ValueError if the text has no `<svg` start tag.
    """
    text = _STAMP.sub("", text)
    start = text.find("<svg")
    end = text.find(">", start)
    if start < 0 or end < 0:
        raise ValueError("no <svg> start tag")
    return f"{text[: end + 1]}<!-- {STAMP_PREFIX}{digest} -->{text[end + 1 :]}"


def svg_problems(root: Path, names: list[str]) -> list[str]:
    """Return problems of the `.svg` next to each `.mmd` file.

    Missing, not an SVG, not rendered with the design configuration, without a source stamp
    or stamped for another source (stale), or an aspect ratio above MAX_ASPECT.
    """
    problems = []
    for name in sorted(n for n in names if n.endswith(".mmd")):
        svg = Path(name).with_suffix(".svg")
        path = root / svg
        if not path.is_file():
            problems.append(f"{name}: rendered {svg.name} is missing")
            continue
        text = path.read_text(encoding="utf-8", errors="replace")
        head = text[:512].lstrip()
        if head.startswith("<?xml"):
            head = head.split("?>", 1)[-1].lstrip()
        if not head.startswith("<svg"):
            problems.append(f"{name}: {svg.name} is not an SVG file")
            continue
        if THEME_MARKER not in text:
            problems.append(
                f"{name}: {svg.name} is not rendered with {MERMAID_CONFIG} (run with --write)"
            )
        stamp = _STAMP.search(text)
        if stamp is None:
            problems.append(f"{name}: {svg.name} has no source stamp (run with --write {name})")
        elif stamp.group(1) != source_digest(root, name):
            problems.append(
                f"{name}: {svg.name} is stale, the source or {MERMAID_CONFIG} changed since it "
                f"was rendered (run with --write {name})"
            )
        m = _VIEWBOX.search(text[:4096])
        if m is None:
            problems.append(f"{name}: {svg.name} has no viewBox")
            continue
        width, height = float(m.group(1)), float(m.group(2))
        if width <= 0 or height <= 0:
            problems.append(f"{name}: {svg.name} has an empty viewBox")
            continue
        ratio = max(width / height, height / width)
        if ratio > MAX_ASPECT:
            problems.append(
                f"{name}: {svg.name} has aspect ratio {ratio:.1f}:1 "
                f"(more than {MAX_ASPECT}:1); split it or turn it"
            )
    return problems


def token_colours(root: Path) -> set[str]:
    """Return every opaque colour of the generated token file, lower case with '#'."""
    data = json.loads((root / RESOLVED_TOKENS).read_text(encoding="utf-8"))
    sets = [data["base"], *(ctx for group in data["contexts"].values() for ctx in group.values())]
    return {
        entry["value"].lower()
        for tokens in sets
        for entry in tokens.values()
        if entry["type"] == "color" and isinstance(entry["value"], str)
    }


def colour_problems(root: Path, names: list[str], allowed: set[str]) -> list[str]:
    """Return colours in `.mmd` files that are no design-token colour."""
    problems = []
    for name in sorted(n for n in names if n.endswith(".mmd")):
        for number, line in enumerate(
            (root / name).read_text(encoding="utf-8").splitlines(), start=1
        ):
            problems += [
                f"{name}:{number}: colour {hit} is not a design-token colour ({RESOLVED_TOKENS})"
                for hit in _HEX.findall(line)
                if hit.lower() not in allowed
            ]
    return problems


def _as_markdown(jobs: list[Job]) -> str:
    return "".join(f"```mermaid\n{job.source.rstrip()}\n```\n\n" for job in jobs)


def render(jobs: list[Job], renderer: Renderer) -> list[str]:
    """Render all jobs in one renderer call; returns one error line per failing diagram."""
    if not jobs:
        return []
    with tempfile.TemporaryDirectory(prefix="bothy-diagrams-") as tmp:
        work = Path(tmp)
        batch = work / "all.md"
        batch.write_text(_as_markdown(jobs), encoding="utf-8")
        ok, _ = renderer(batch, work / "all-out.md")
        if ok:
            return []
        errors = []
        for i, job in enumerate(jobs):
            single = work / f"job-{i}.md"
            single.write_text(_as_markdown([job]), encoding="utf-8")
            ok, stderr = renderer(single, work / f"job-{i}-out.md")
            if not ok:
                first = next((ln for ln in stderr.splitlines() if ln.strip()), "render failed")
                errors.append(f"{job.origin}: {first.strip()}")
        return errors or ["diagrams: batch render failed, but every diagram renders alone"]


def mmdc_command(mmdc: Path, source: Path, out: Path, config: Path | None = None) -> list[str]:
    """Return the mmdc command line.

    With the Mermaid configuration `config` (theme and background from it) and the Puppeteer
    config from MMDC_PUPPETEER_CONFIG.
    """
    cmd = [str(mmdc), "-q", "-i", str(source), "-o", str(out)]
    if config is not None:
        background = json.loads(config.read_text(encoding="utf-8"))["themeVariables"]["background"]
        cmd += ["-c", str(config), "-b", background]
    puppeteer = os.environ.get("MMDC_PUPPETEER_CONFIG")
    if puppeteer:
        cmd += ["-p", puppeteer]
    return cmd


def _subprocess_renderer(mmdc: Path, config: Path | None = None) -> Renderer:
    def run(source: Path, out: Path) -> tuple[bool, str]:
        proc = subprocess.run(
            mmdc_command(mmdc, source, out, config),
            capture_output=True,
            text=True,
            timeout=_RENDER_TIMEOUT_S,
            check=False,
        )
        return proc.returncode == 0, proc.stderr

    return run


def tracked(root: Path) -> list[str]:
    """Return the tracked Markdown and Mermaid files of the repository."""
    out = subprocess.run(
        ["git", "-C", str(root), "ls-files", "-z", "--", "*.md", "*.mmd"],
        check=True,
        capture_output=True,
    ).stdout
    return [p for p in out.decode("utf-8").split("\0") if p]


def write_svgs(root: Path, names: list[str], renderer: Renderer) -> list[str]:
    """Render each `.mmd` file to the stamped `.svg` next to it; one error line per failure."""
    errors = []
    for name in sorted(n for n in names if n.endswith(".mmd")):
        out = (root / name).with_suffix(".svg")
        ok, stderr = renderer(root / name, out)
        if not ok:
            first = next((ln for ln in stderr.splitlines() if ln.strip()), "render failed")
            errors.append(f"{name}: {first.strip()}")
            continue
        try:
            out.write_text(
                stamp_svg(out.read_text(encoding="utf-8"), source_digest(root, name)),
                encoding="utf-8",
            )
        except (OSError, ValueError) as exc:
            errors.append(f"{name}: cannot stamp {out.name}: {exc}")
            continue
        print(f"wrote {Path(name).with_suffix('.svg')}")
    return errors


def main(argv: list[str] | None = None) -> int:
    """Render every diagram and check SVG files; returns the process exit code."""
    parser = argparse.ArgumentParser(description="Render-check all Mermaid diagrams.")
    parser.add_argument("--repo", type=Path, default=REPO_ROOT)
    parser.add_argument("--mmdc", type=Path, default=DEFAULT_MMDC)
    parser.add_argument(
        "--write", action="store_true", help="re-render the .svg of every .mmd file first"
    )
    parser.add_argument(
        "only", nargs="*", help="with --write: only these .mmd files (repository paths)"
    )
    args = parser.parse_args(argv)
    if not args.mmdc.exists():
        print(f"diagrams: {args.mmdc} not found; run `make bootstrap`", file=sys.stderr)
        return 2
    config = args.repo / MERMAID_CONFIG
    if not config.is_file():
        print(
            f"diagrams: {MERMAID_CONFIG} missing; run `uv run tools/design-tokens/tokens.py build`",
            file=sys.stderr,
        )
        return 2
    renderer = _subprocess_renderer(args.mmdc, config)
    names = tracked(args.repo)
    if args.write:
        errors = write_svgs(args.repo, args.only or names, renderer)
        if errors:
            print("\n".join(errors))
            return 1
    jobs = collect_jobs(args.repo, names)
    problems = (
        svg_problems(args.repo, names)
        + colour_problems(args.repo, names, token_colours(args.repo))
        + erd_check.problems(args.repo)
        + render(jobs, renderer)
    )
    for problem in problems:
        print(problem)
    if problems:
        return 1
    print(f"diagrams: {len(jobs)} diagram(s) render")
    return 0


if __name__ == "__main__":
    sys.exit(main())
