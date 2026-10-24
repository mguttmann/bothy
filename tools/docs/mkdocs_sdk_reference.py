# SPDX-License-Identifier: Apache-2.0
"""MkDocs hook: mount the typedoc reference of the TypeScript SDK into the site.

The reference is generated from the TSDoc comments of `sdk/typescript/src`
(docs/spec/23-quality.md "Documentation in code", R-QA-14). The recipe `docs` of the root
Makefile runs `npm --prefix sdk/typescript run docs` before `mkdocs build`, which writes the HTML to
`sdk/typescript/build/reference/`. This hook adds every file of that directory to the build as a
generated static file under `api/typescript-reference/` (`File.generated` with `abs_src_path`),
so the strict build validates the link of `docs/api/sdk-typescript.md` to
`typescript-reference/index.html` like any other link (docs/decisions.d/M2-30.md D-M2-30-13).

- Package tracked (`git ls-files --error-unmatch sdk/typescript/package.json`, the same test as
  the Makefile) and the directory missing or empty: `PluginError` naming the npm command, so a
  build without the reference fails instead of publishing a broken link (23 1 point 3).
- Package not tracked: nothing happens.
- A committed file under `docs/api/typescript-reference/` fails the build, so a stale copy cannot
  be checked in.

`make linkcheck` checks the tracked Markdown files on disk, where the mount does not exist; its
lychee call excludes exactly `LINKCHECK_EXCLUDE` (held by `tests/test_mkdocs_sdk_reference.py`).
Registered in mkdocs.yml under `hooks:`.
"""

from __future__ import annotations

import subprocess  # nosec B404 - runs git ls-files on the repository of the site
from pathlib import Path

from mkdocs.config.defaults import MkDocsConfig
from mkdocs.exceptions import PluginError
from mkdocs.structure.files import File, Files

PACKAGE = "sdk/typescript/package.json"
REFERENCE_DIR = "sdk/typescript/build/reference"
MOUNT = "api/typescript-reference/"
NPM_COMMAND = "npm --prefix sdk/typescript run docs"
# lychee gives a local link target as a file URL of its absolute path; the pattern matches the
# mount below docs/ and nothing else (`--exclude` takes regular expressions, lychee 0.24).
LINKCHECK_EXCLUDE = "^file://.*/docs/api/typescript-reference/"


def repo_root(config: MkDocsConfig) -> Path:
    """Return the repository root: the parent of `docs_dir`."""
    return Path(config.docs_dir).resolve().parent


def package_tracked(root: Path) -> bool:
    """Return whether git tracks the package manifest of the TypeScript SDK under `root`.

    A failing git (no repository, no git) counts as not tracked, like the Makefile's test.
    """
    try:
        result = subprocess.run(
            ["git", "-C", str(root), "ls-files", "--error-unmatch", "--", PACKAGE],
            capture_output=True,
            check=False,
        )
    except OSError:
        return False
    return result.returncode == 0


def reference_files(root: Path) -> list[Path]:
    """Return every file of the built reference, sorted; empty when the directory is missing."""
    base = root / REFERENCE_DIR
    if not base.is_dir():
        return []
    return sorted(p for p in base.rglob("*") if p.is_file())


def on_files(files: Files, config: MkDocsConfig) -> Files:
    """Add the typedoc reference to the build (MkDocs event `on_files`).

    Args:
        files: The files MkDocs found in docs_dir.
        config: The site configuration; the repository root is the parent of `docs_dir`.

    Returns:
        `files` with every file of the reference under `MOUNT`, or unchanged without the package.

    Raises:
        PluginError: the package is tracked and the reference is missing or empty, or a file
            under `MOUNT` is committed in docs_dir.
    """
    root = repo_root(config)
    if not package_tracked(root):
        return files
    found = reference_files(root)
    if not found:
        raise PluginError(
            f"typescript reference: {REFERENCE_DIR} is missing or empty; "
            f"build it with `{NPM_COMMAND}` (make docs does this first)"
        )
    committed = [f.src_uri for f in files if f.src_uri.startswith(MOUNT) and f.generated_by is None]
    if committed:
        raise PluginError(
            f"typescript reference: {committed[0]} is committed under docs/; the reference is "
            "generated in the build, remove the copy"
        )
    base = root / REFERENCE_DIR
    for path in found:
        uri = MOUNT + path.relative_to(base).as_posix()
        files.append(File.generated(config, uri, abs_src_path=str(path)))
    return files
