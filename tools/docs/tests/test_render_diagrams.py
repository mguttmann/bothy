# SPDX-License-Identifier: Apache-2.0
"""Tests for the diagram render check (the renderer itself is replaced by a fake)."""

from __future__ import annotations

from pathlib import Path

import pytest
import render_diagrams as rd

MD = """# Title

```mermaid
flowchart LR
  A-->B
```

Text.

```python
print("not a diagram")
```

```mermaid
sequenceDiagram
  A->>B: hi
```
"""


def test_extract_blocks_with_line_numbers() -> None:
    blocks = rd.extract_blocks(MD)
    assert [(line, src.splitlines()[0]) for line, src in blocks] == [
        (3, "flowchart LR"),
        (14, "sequenceDiagram"),
    ]


def test_collect_jobs_from_mmd_and_markdown(tmp_path: Path) -> None:
    (tmp_path / "docs").mkdir()
    (tmp_path / "docs" / "a.mmd").write_text("flowchart LR\n  X-->Y\n")
    (tmp_path / "docs" / "a.svg").write_text("<svg></svg>")
    (tmp_path / "docs" / "page.md").write_text(MD)
    jobs = rd.collect_jobs(tmp_path, ["docs/a.mmd", "docs/a.svg", "docs/page.md"])
    assert [j.origin for j in jobs] == ["docs/a.mmd", "docs/page.md:3", "docs/page.md:14"]


def test_missing_or_invalid_svg_sibling(tmp_path: Path) -> None:
    (tmp_path / "a.mmd").write_text("flowchart LR\n")
    (tmp_path / "b.mmd").write_text("flowchart LR\n")
    (tmp_path / "b.svg").write_text("not svg")
    (tmp_path / "c.mmd").write_text("flowchart LR\n")
    (tmp_path / "c.svg").write_text('<?xml version="1.0"?>\n<svg xmlns="x"></svg>')
    problems = rd.svg_problems(tmp_path, ["a.mmd", "b.mmd", "c.mmd"])
    assert problems == ["a.mmd: rendered a.svg is missing", "b.mmd: b.svg is not an SVG file"]


class FakeRenderer:
    """Fails for any source that contains the word BROKEN, like mmdc would."""

    def __init__(self) -> None:
        self.calls = 0

    def __call__(self, source_md: Path, out_md: Path) -> tuple[bool, str]:
        self.calls += 1
        if "BROKEN" in source_md.read_text():
            return False, "UnknownDiagramError: No diagram type detected\nstack..."
        return True, ""


def _jobs(*sources: str) -> list[rd.Job]:
    return [rd.Job(f"f{i}.mmd", src) for i, src in enumerate(sources)]


def test_render_all_succeeds_with_one_batch() -> None:
    fake = FakeRenderer()
    assert rd.render(_jobs("flowchart LR\n", "graph TD\n"), fake) == []
    assert fake.calls == 1


def test_render_isolates_failing_job() -> None:
    fake = FakeRenderer()
    errors = rd.render(_jobs("flowchart LR\n", "BROKEN\n", "graph TD\n"), fake)
    assert errors == ["f1.mmd: UnknownDiagramError: No diagram type detected"]
    assert fake.calls == 4  # one batch, then one per job


def test_mmdc_command_uses_puppeteer_config(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    cfg = tmp_path / "p.json"
    monkeypatch.setenv("MMDC_PUPPETEER_CONFIG", str(cfg))
    cmd = rd.mmdc_command(Path("/x/mmdc"), Path("in.md"), Path("out.md"))
    assert cmd == ["/x/mmdc", "-q", "-i", "in.md", "-o", "out.md", "-p", str(cfg)]


def test_mmdc_command_without_config(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("MMDC_PUPPETEER_CONFIG", raising=False)
    cmd = rd.mmdc_command(Path("/x/mmdc"), Path("in.md"), Path("out.md"))
    assert "-p" not in cmd


def _repo_with_diagrams(tmp_path: Path) -> Path:
    import subprocess

    repo = tmp_path / "repo"
    repo.mkdir()
    subprocess.run(["git", "init", "-q", str(repo)], check=True)
    (repo / "a.mmd").write_text("flowchart LR\n  A-->B\n")
    (repo / "a.svg").write_text("<svg></svg>")
    (repo / "page.md").write_text(MD)
    subprocess.run(["git", "-C", str(repo), "add", "."], check=True)
    return repo


def _fake_mmdc(tmp_path: Path, exit_code: int) -> Path:
    script = tmp_path / "mmdc"
    script.write_text(f"#!/bin/sh\necho 'Error: fake renderer' >&2\nexit {exit_code}\n")
    script.chmod(0o755)
    return script


def test_main_with_working_renderer(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    repo = _repo_with_diagrams(tmp_path)
    rc = rd.main(["--repo", str(repo), "--mmdc", str(_fake_mmdc(tmp_path, 0))])
    assert rc == 0
    assert "3 diagram(s) render" in capsys.readouterr().out


def test_main_with_failing_renderer(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    repo = _repo_with_diagrams(tmp_path)
    rc = rd.main(["--repo", str(repo), "--mmdc", str(_fake_mmdc(tmp_path, 1))])
    out = capsys.readouterr().out
    assert rc == 1
    assert "a.mmd: Error: fake renderer" in out
    assert "page.md:14: Error: fake renderer" in out


def test_main_reports_missing_renderer(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    rc = rd.main(["--repo", str(tmp_path), "--mmdc", str(tmp_path / "nope")])
    assert rc == 2
    assert "make bootstrap" in capsys.readouterr().err
