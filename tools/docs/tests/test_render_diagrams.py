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


THEMED = (
    '<svg xmlns="x" viewBox="0 0 200 100" '
    'style="font-family:&quot;Atkinson Hyperlegible Next&quot;"></svg>'
)


def stamped(root: Path, name: str, svg: str = THEMED) -> str:
    return rd.stamp_svg(svg, rd.source_digest(root, name))


def test_missing_or_invalid_svg_sibling(tmp_path: Path) -> None:
    (tmp_path / "a.mmd").write_text("flowchart LR\n")
    (tmp_path / "b.mmd").write_text("flowchart LR\n")
    (tmp_path / "b.svg").write_text("not svg")
    (tmp_path / "c.mmd").write_text("flowchart LR\n")
    (tmp_path / "c.svg").write_text('<?xml version="1.0"?>\n' + stamped(tmp_path, "c.mmd"))
    problems = rd.svg_problems(tmp_path, ["a.mmd", "b.mmd", "c.mmd"])
    assert problems == ["a.mmd: rendered a.svg is missing", "b.mmd: b.svg is not an SVG file"]


def test_svg_without_design_theme_is_flagged(tmp_path: Path) -> None:
    (tmp_path / "a.mmd").write_text("flowchart LR\n")
    (tmp_path / "a.svg").write_text(
        stamped(
            tmp_path, "a.mmd", '<svg viewBox="0 0 100 100" style="font-family:trebuchet ms"></svg>'
        )
    )
    assert rd.svg_problems(tmp_path, ["a.mmd"]) == [
        "a.mmd: a.svg is not rendered with design/build/mermaid.config.json (run with --write)"
    ]


@pytest.mark.parametrize(("width", "height"), [(900, 100), (100, 900)])
def test_too_wide_or_too_tall_svg_is_flagged(tmp_path: Path, width: int, height: int) -> None:
    (tmp_path / "a.mmd").write_text("flowchart LR\n")
    (tmp_path / "a.svg").write_text(
        stamped(tmp_path, "a.mmd", THEMED.replace("0 0 200 100", f"0 0 {width} {height}"))
    )
    problems = rd.svg_problems(tmp_path, ["a.mmd"])
    assert len(problems) == 1
    assert "aspect ratio 9.0:1" in problems[0]


def test_colour_outside_the_tokens_is_flagged(tmp_path: Path) -> None:
    (tmp_path / "a.mmd").write_text("flowchart LR\n  classDef x fill:#136F69,stroke:#5e35b1\n")
    problems = rd.colour_problems(tmp_path, ["a.mmd"], {"#136f69"})
    assert problems == [
        "a.mmd:2: colour #5e35b1 is not a design-token colour (design/build/tokens.resolved.json)"
    ]


def test_token_colours_reads_base_and_every_context(tmp_path: Path) -> None:
    (tmp_path / "design" / "build").mkdir(parents=True)
    (tmp_path / "design" / "build" / "tokens.resolved.json").write_text(
        '{"base": {"palette.a": {"type": "color", "value": "#ABCDEF"},'
        ' "space.1": {"type": "dimension", "value": "4px"}},'
        ' "contexts": {"theme": {"light": {"color.s": {"type": "color",'
        ' "value": {"hex": "#000000", "alpha": 0.5}}},'
        ' "dark": {"color.t": {"type": "color", "value": "#111111"}}}}}'
    )
    assert rd.token_colours(tmp_path) == {"#abcdef", "#111111"}


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


def test_mmdc_command_reads_a_relative_puppeteer_config_from_the_repository_root(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    # CI gives the path relative to the repository root; pytest under `make test` runs in tools/.
    monkeypatch.setenv("MMDC_PUPPETEER_CONFIG", "tools/node/puppeteer-ci.json")
    monkeypatch.chdir(tmp_path)
    cmd = rd.mmdc_command(Path("/x/mmdc"), Path("in.md"), Path("out.md"))
    assert cmd[-2:] == ["-p", str(rd.REPO_ROOT / "tools" / "node" / "puppeteer-ci.json")]


def test_mmdc_command_without_config(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("MMDC_PUPPETEER_CONFIG", raising=False)
    cmd = rd.mmdc_command(Path("/x/mmdc"), Path("in.md"), Path("out.md"))
    assert "-p" not in cmd


def test_mmdc_command_with_mermaid_config(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> None:
    monkeypatch.delenv("MMDC_PUPPETEER_CONFIG", raising=False)
    config = tmp_path / "mermaid.config.json"
    config.write_text('{"theme": "base", "themeVariables": {"background": "#fbfeff"}}')
    cmd = rd.mmdc_command(Path("/x/mmdc"), Path("in.md"), Path("out.md"), config)
    assert cmd[-4:] == ["-c", str(config), "-b", "#fbfeff"]


DATA_MODEL = """# 03

## 4. Tables

### 4.1 Part

#### `a`

PK: `id`. FK: none.

## 5. Next
"""

ERD = "erDiagram\n  a {\n    uuid id PK\n    text name\n  }\n"


def _repo_with_diagrams(tmp_path: Path) -> Path:
    import subprocess

    repo = tmp_path / "repo"
    (repo / "design" / "build").mkdir(parents=True)
    (repo / "docs" / "spec").mkdir(parents=True)
    (repo / "docs" / "architecture").mkdir(parents=True)
    subprocess.run(["git", "init", "-q", str(repo)], check=True)
    (repo / "design" / "build" / "mermaid.config.json").write_text(
        '{"theme": "base", "themeVariables": {"background": "#fbfeff"}}'
    )
    (repo / "design" / "build" / "tokens.resolved.json").write_text('{"base": {}, "contexts": {}}')
    (repo / "docs" / "spec" / "03-data-model.md").write_text(DATA_MODEL)
    (repo / "docs" / "architecture" / "erd-1.mmd").write_text(ERD)
    (repo / "docs" / "architecture" / "erd-1.svg").write_text(
        stamped(repo, "docs/architecture/erd-1.mmd")
    )
    (repo / "a.mmd").write_text("flowchart LR\n  A-->B\n")
    (repo / "a.svg").write_text(stamped(repo, "a.mmd"))
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
    assert "4 diagram(s) render" in capsys.readouterr().out


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


def test_main_reports_missing_mermaid_config(
    tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    rc = rd.main(["--repo", str(tmp_path), "--mmdc", str(_fake_mmdc(tmp_path, 0))])
    assert rc == 2
    assert "tokens.py build" in capsys.readouterr().err


def test_write_renders_each_mmd_to_its_svg(tmp_path: Path) -> None:
    (tmp_path / "x.mmd").write_text("flowchart LR\n")
    (tmp_path / "y.mmd").write_text("BROKEN\n")
    calls: list[tuple[Path, Path]] = []

    def renderer(source: Path, out: Path) -> tuple[bool, str]:
        calls.append((source, out))
        out.write_text(THEMED)
        return ("BROKEN" not in source.read_text(), "Error: broken\n")

    errors = rd.write_svgs(tmp_path, ["x.mmd", "y.mmd", "page.md"], renderer)
    assert calls == [
        (tmp_path / "x.mmd", tmp_path / "x.svg"),
        (tmp_path / "y.mmd", tmp_path / "y.svg"),
    ]
    assert errors == ["y.mmd: Error: broken"]
    assert rd.svg_problems(tmp_path, ["x.mmd"]) == []


# --- source stamp: the .svg belongs to the current .mmd (K2-17) --------------------------------


def test_stamp_is_placed_once_after_the_svg_tag() -> None:
    once = rd.stamp_svg('<?xml version="1.0"?>\n' + THEMED, "a" * 64)
    twice = rd.stamp_svg(once, "b" * 64)
    assert twice.count(rd.STAMP_PREFIX) == 1
    assert ">" + "<!-- " + rd.STAMP_PREFIX + "b" * 64 + " -->" in twice
    assert twice.startswith('<?xml version="1.0"?>\n<svg ')


def test_stamp_needs_an_svg_tag() -> None:
    with pytest.raises(ValueError, match="no <svg>"):
        rd.stamp_svg("not svg", "a" * 64)


def test_svg_without_stamp_is_flagged(tmp_path: Path) -> None:
    (tmp_path / "a.mmd").write_text("flowchart LR\n")
    (tmp_path / "a.svg").write_text(THEMED)
    assert rd.svg_problems(tmp_path, ["a.mmd"]) == [
        "a.mmd: a.svg has no source stamp (run with --write a.mmd)"
    ]


def test_svg_of_an_older_source_is_stale(tmp_path: Path) -> None:
    (tmp_path / "a.mmd").write_text("flowchart LR\n  A-->B\n")
    (tmp_path / "a.svg").write_text(stamped(tmp_path, "a.mmd"))
    assert rd.svg_problems(tmp_path, ["a.mmd"]) == []
    (tmp_path / "a.mmd").write_text("flowchart LR\n  A-->C\n")
    problems = rd.svg_problems(tmp_path, ["a.mmd"])
    assert len(problems) == 1
    assert "a.svg is stale" in problems[0]


def test_svg_is_stale_after_a_theme_change(tmp_path: Path) -> None:
    (tmp_path / "design" / "build").mkdir(parents=True)
    config = tmp_path / "design" / "build" / "mermaid.config.json"
    config.write_text('{"theme": "base"}')
    (tmp_path / "a.mmd").write_text("flowchart LR\n")
    (tmp_path / "a.svg").write_text(stamped(tmp_path, "a.mmd"))
    config.write_text('{"theme": "dark"}')
    assert "stale" in rd.svg_problems(tmp_path, ["a.mmd"])[0]


def test_write_reports_an_svg_that_cannot_be_stamped(tmp_path: Path) -> None:
    (tmp_path / "x.mmd").write_text("flowchart LR\n")

    def renderer(source: Path, out: Path) -> tuple[bool, str]:
        out.write_text("garbage")
        return True, ""

    assert rd.write_svgs(tmp_path, ["x.mmd"], renderer) == [
        "x.mmd: cannot stamp x.svg: no <svg> start tag"
    ]


def test_repository_svgs_match_their_sources() -> None:
    """The gate itself on the real artefacts: every tracked .svg belongs to its .mmd."""
    root = Path(__file__).resolve().parents[3]
    names = rd.tracked(root)
    stale = [p for p in rd.svg_problems(root, names) if "stamp" in p or "stale" in p]
    assert stale == []
