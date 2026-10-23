# SPDX-License-Identifier: Apache-2.0
"""The API reference of the Python SDK hand layer (docs/sdk/python.md, R-QA-14).

The page holds one mkdocstrings block per public module of `bothy_sdk`, and the strict build
renders the docstrings of the SDK into it. The generated core `bothy_sdk.generated` has no
block: `docs/api/openapi.yaml` documents it (docs/DECISIONS.md, reference without the
generated core).
"""

from __future__ import annotations

import re
import shutil
import subprocess  # nosec B404 - runs mkdocs of the locked tools environment
import sys
from pathlib import Path

import yaml

REPO = Path(__file__).resolve().parents[3]
PAGE = REPO / "docs" / "sdk" / "python.md"
SRC = REPO / "sdk" / "python" / "src"
_BLOCK = re.compile(r"^::: (?P<module>[\w.]+)\s*$", re.MULTILINE)


def public_modules() -> set[str]:
    """Every public module and package below `bothy_sdk`, without the generated core."""
    found: set[str] = set()
    for path in (SRC / "bothy_sdk").rglob("*.py"):
        parts = list(path.relative_to(SRC).with_suffix("").parts)
        if parts[-1] == "__init__":
            parts.pop()
        if len(parts) < 2 or parts[1] == "generated" or any(p.startswith("_") for p in parts):
            continue
        found.add(".".join(parts))
    return found


def first_docstring_line(module: str, name: str) -> str:
    """The summary line of the docstring of class `name` in `module`, read from the source."""
    path = SRC / Path(*module.split(".")).with_suffix(".py")
    match = re.search(
        rf'^class {name}\b[^\n]*:\n    """(?P<line>[^\n]+)', path.read_text(encoding="utf-8"), re.M
    )
    assert match, f"{name} in {path.name} has no docstring"
    return match.group("line").strip()


def test_reference_page_lists_every_public_module() -> None:
    blocks = set(_BLOCK.findall(PAGE.read_text(encoding="utf-8")))
    modules = public_modules()
    assert {"bothy_sdk.adapter", "bothy_sdk.testing", "bothy_sdk.testing.fakeserver"} <= modules
    assert modules - blocks == set()
    assert not {b for b in blocks if b.startswith("bothy_sdk.generated")}


class _IgnoreTags(yaml.SafeLoader):
    """Reads mkdocs.yml without constructing its `!!python/...` objects."""


_IgnoreTags.add_multi_constructor("tag:yaml.org,2002:python/", lambda *_: None)


def reduced_config(tmp_path: Path) -> Path:
    """A site with only the SDK page and the mkdocstrings plugin as mkdocs.yml configures it.

    The full site needs the generated Dart API pages and the API reference hooks; this test is
    about the docstrings, so it builds the one page. Links to pages outside the reduced site
    are reported, not fatal; `make docs` and `make linkcheck` check them on the full site.
    """
    config = yaml.load((REPO / "mkdocs.yml").read_text(encoding="utf-8"), Loader=_IgnoreTags)  # noqa: S506 - SafeLoader subclass
    [plugin] = [p["mkdocstrings"] for p in config["plugins"] if "mkdocstrings" in p]
    python = plugin["handlers"]["python"]
    python["paths"] = [str(REPO / p) for p in python["paths"]]
    docs = tmp_path / "docs"
    (docs / "sdk").mkdir(parents=True)
    shutil.copy(PAGE, docs / "sdk" / "python.md")
    cfg = tmp_path / "mkdocs.yml"
    cfg.write_text(
        yaml.safe_dump(
            {
                "site_name": "t",
                "docs_dir": str(docs),
                "theme": {"name": "material", "font": False},
                "plugins": [{"mkdocstrings": plugin}],
                "markdown_extensions": ["admonition", "attr_list", "tables", "toc"],
                "validation": {"links": {"not_found": "info", "anchors": "warn"}},
                "nav": [{"Python SDK": "sdk/python.md"}],
            }
        ),
        encoding="utf-8",
    )
    return cfg


def test_reference_renders_docstrings(tmp_path: Path) -> None:
    site = tmp_path / "site"
    result = subprocess.run(  # nosec B603 - fixed argument list
        [
            *(sys.executable, "-m", "mkdocs", "build", "--strict"),
            *("--config-file", str(reduced_config(tmp_path)), "--site-dir", str(site)),
        ],
        capture_output=True,
        text=True,
        check=False,
    )
    assert result.returncode == 0, result.stderr
    html = (site / "sdk" / "python" / "index.html").read_text(encoding="utf-8")
    for module, name in [
        ("bothy_sdk.adapter", "Adapter"),
        ("bothy_sdk.invocation", "InvocationVerifier"),
    ]:
        assert f'id="{module}.{name}"' in html
        words = re.sub(r"<[^>]+>", "", html)
        line = re.sub(r"[`*]", "", first_docstring_line(module, name))
        assert line in words.replace("&#39;", "'").replace("&amp;", "&"), name
