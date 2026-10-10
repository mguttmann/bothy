# SPDX-License-Identifier: Apache-2.0
"""Diagram gate: every Mermaid diagram of the repository renders.

Checks every tracked `*.mmd` file and every ```mermaid block in tracked Markdown files
with the pinned mermaid-cli (tools/node). All diagrams render in one browser session;
on failure each diagram is rendered alone to name the broken ones. A `*.mmd` file
also needs its rendered `*.svg` next to it (docs/quality/gates.md).

Environment: MMDC_PUPPETEER_CONFIG may point to a Puppeteer config file, e.g. to use a
locally installed Chromium instead of the one `make bootstrap` downloads.
"""

from __future__ import annotations

import argparse
import os
import re
import subprocess
import sys
import tempfile
from collections.abc import Callable
from dataclasses import dataclass
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[2]
DEFAULT_MMDC = REPO_ROOT / "tools" / "node" / "node_modules" / ".bin" / "mmdc"
_FENCE_OPEN = re.compile(r"^(?P<indent> {0,3})(?P<fence>`{3,}|~{3,})\s*mermaid\s*$")
_RENDER_TIMEOUT_S = 600

Renderer = Callable[[Path, Path], tuple[bool, str]]


@dataclass(frozen=True)
class Job:
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


def svg_problems(root: Path, names: list[str]) -> list[str]:
    problems = []
    for name in sorted(n for n in names if n.endswith(".mmd")):
        svg = Path(name).with_suffix(".svg")
        path = root / svg
        if not path.is_file():
            problems.append(f"{name}: rendered {svg.name} is missing")
            continue
        head = path.read_text(encoding="utf-8", errors="replace")[:512].lstrip()
        if head.startswith("<?xml"):
            head = head.split("?>", 1)[-1].lstrip()
        if not head.startswith("<svg"):
            problems.append(f"{name}: {svg.name} is not an SVG file")
    return problems


def _as_markdown(jobs: list[Job]) -> str:
    return "".join(f"```mermaid\n{job.source.rstrip()}\n```\n\n" for job in jobs)


def render(jobs: list[Job], renderer: Renderer) -> list[str]:
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


def mmdc_command(mmdc: Path, source: Path, out: Path) -> list[str]:
    cmd = [str(mmdc), "-q", "-i", str(source), "-o", str(out)]
    config = os.environ.get("MMDC_PUPPETEER_CONFIG")
    if config:
        cmd += ["-p", config]
    return cmd


def _subprocess_renderer(mmdc: Path) -> Renderer:
    def run(source: Path, out: Path) -> tuple[bool, str]:
        proc = subprocess.run(
            mmdc_command(mmdc, source, out),
            capture_output=True,
            text=True,
            timeout=_RENDER_TIMEOUT_S,
            check=False,
        )
        return proc.returncode == 0, proc.stderr

    return run


def tracked(root: Path) -> list[str]:
    out = subprocess.run(
        ["git", "-C", str(root), "ls-files", "-z", "--", "*.md", "*.mmd"],
        check=True,
        capture_output=True,
    ).stdout
    return [p for p in out.decode("utf-8").split("\0") if p]


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Render-check all Mermaid diagrams.")
    parser.add_argument("--repo", type=Path, default=REPO_ROOT)
    parser.add_argument("--mmdc", type=Path, default=DEFAULT_MMDC)
    args = parser.parse_args(argv)
    if not args.mmdc.exists():
        print(f"diagrams: {args.mmdc} not found; run `make bootstrap`", file=sys.stderr)
        return 2
    names = tracked(args.repo)
    jobs = collect_jobs(args.repo, names)
    problems = svg_problems(args.repo, names) + render(jobs, _subprocess_renderer(args.mmdc))
    for problem in problems:
        print(problem)
    if problems:
        return 1
    print(f"diagrams: {len(jobs)} diagram(s) render")
    return 0


if __name__ == "__main__":
    sys.exit(main())
