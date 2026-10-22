# SPDX-License-Identifier: Apache-2.0
"""MkDocs hook: the index page of the ruling fragments in docs/decisions.d/.

Every build ticket writes its rulings into its own file `docs/decisions.d/<ticket>.md`
(docs/DECISIONS.md section 10, ruling PL/24), so parallel pull requests do not conflict in one
shared log. A committed index would conflict the same way, so this hook writes the page
`decisions.d/index.md` at build time (`File.generated`): one link per file, titled with the
file's first heading, in natural name order (M1-9 before M1-10). A committed index fails the
build. The single files stay out of the navigation (`not_in_nav` in mkdocs.yml).

Registered in mkdocs.yml under `hooks:`.
"""

from __future__ import annotations

import re

from mkdocs.config.defaults import MkDocsConfig
from mkdocs.exceptions import PluginError
from mkdocs.structure.files import File, Files

FOLDER = "decisions.d/"
INDEX = FOLDER + "index.md"
TITLE = "Ruling log: tickets"


def _natural(uri: str) -> list[object]:
    return [int(p) if p.isdigit() else p.lower() for p in re.split(r"(\d+)", uri)]


def _title(file: File) -> str:
    first = file.content_string.split("\n", 1)[0]
    return first[2:].strip() if first.startswith("# ") else file.name


def render(files: list[File]) -> str:
    """Return the Markdown of the index page for the fragment `files`."""
    lines = [
        f"# {TITLE}",
        "",
        "The rulings of each build ticket, one file per ticket (ruling log section 10,",
        "[DECISIONS.md](../DECISIONS.md)). The sections up to M1-16 are in",
        "[DECISIONS-FOUNDATION.md](../DECISIONS-FOUNDATION.md).",
        "",
    ]
    for file in sorted(files, key=lambda f: _natural(f.src_uri)):
        name = file.src_uri[len(FOLDER) :]
        lines.append(f"- [{_title(file)}]({name})")
    return "\n".join(lines) + "\n"


def on_files(files: Files, config: MkDocsConfig) -> Files:
    """Add the generated index page of docs/decisions.d/ (MkDocs event `on_files`).

    Raises:
        PluginError: `decisions.d/index.md` is committed under docs_dir.
    """
    if files.get_file_from_path(INDEX) is not None:
        raise PluginError(
            f"ruling fragments: {INDEX} is generated at build time by "
            "tools/docs/mkdocs_ruling_fragments.py; remove the committed copy"
        )
    fragments = [f for f in files if f.src_uri.startswith(FOLDER) and f.src_uri.endswith(".md")]
    files.append(File.generated(config, INDEX, content=render(fragments)))
    return files
