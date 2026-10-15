# SPDX-License-Identifier: Apache-2.0
"""API documentation of the Dart packages for the documentation site (brief M4-01, note 5).

For every member of `tools/arch-check/rules.yaml` that is a package (not an app) the script runs
`dart doc --output <out>/<member>` in the member folder, after one `flutter pub get
--enforce-lockfile` at the workspace root. A non-zero exit or any output line with `warning:`
or `error:` fails the run (documentation in code is a gate, docs/spec/23-qualitaet.md), except a
warning located in generated code, which the doc lint excludes as well. The
pages are generated for every build and never tracked (docs/DECISIONS.md D-M4-01-6); MkDocs
copies them into the site, and docs/reference/dart-api.md links them.

`dart doc` links its fonts from Google Fonts; the site loads nothing from another origin
(tools/docs/external_assets_check.py, M7-19), so the script removes those `<link>` elements
from every generated page and the pages use the browser's fonts (docs/DECISIONS.md INT-W20/3).

Usage: dart_api_docs.py --out docs/api/dart [--root <repository>] [--rules <rules.yaml>]
"""

from __future__ import annotations

import argparse
import re
import shutil
import subprocess
import sys
from pathlib import Path

import yaml

_MARKERS = ("warning:", "error:")
# The location line `dart doc` prints below a warning: `from <element>: (file://<path>:<l>:<c>)`.
_FROM = re.compile(r"^\s*from .*\(file://(?P<path>[^)]*?):\d+:\d+\)\s*$")
# Generated Dart code, the same patterns as the doc lint of the root analysis_options.yaml
# (docs/DECISIONS.md D-M4-01-4): its doc comments come from the generator.
_GENERATED = re.compile(r"/lib/src/generated/|\.(?:g|freezed|drift)\.dart$")


def findings(output: str) -> list[str]:
    """The warning and error lines of `dart doc` output that count.

    A warning whose location line points into generated code is left out (the doc lint
    excludes generated code, docs/DECISIONS.md D-M4-02-7); every error counts, and so does a
    warning without a location.
    """
    lines = output.splitlines()
    kept = []
    for i, line in enumerate(lines):
        if not any(m in line for m in _MARKERS):
            continue
        where = _FROM.match(lines[i + 1]) if i + 1 < len(lines) else None
        if "warning:" in line and where and _GENERATED.search(where.group("path")):
            continue
        kept.append(line)
    return kept


# The Google Fonts stylesheets and their preconnect in the head of every `dart doc` page.
_FONT_LINK = re.compile(
    r"[ \t]*<link\b[^>]*\bhref=[\"']https://fonts\.(?:googleapis|gstatic)\.com[^>]*>[ \t]*\n?",
    re.IGNORECASE,
)


def drop_font_links(folder: Path) -> None:
    """Remove the Google Fonts `<link>` elements from every HTML page below `folder`."""
    for page in folder.rglob("*.html"):
        text = page.read_text(encoding="utf-8")
        cleaned = _FONT_LINK.sub("", text)
        if cleaned != text:
            page.write_text(cleaned, encoding="utf-8")


def packages(rules: Path) -> list[tuple[str, str]]:
    """Name and folder of every package (not app) of the rules, in their order."""
    data = yaml.safe_load(rules.read_text(encoding="utf-8"))
    return [(m["name"], m["path"]) for m in data["members"] if m["kind"] != "app"]


def _run(command: list[str], cwd: Path) -> tuple[int, str]:
    result = subprocess.run(
        command, cwd=cwd, capture_output=True, text=True, check=False, encoding="utf-8"
    )
    output = result.stdout + result.stderr
    sys.stdout.write(output)
    return result.returncode, output


def main(argv: list[str] | None = None) -> int:
    """Command line entry point; see the module documentation."""
    repo = Path(__file__).resolve().parents[2]
    parser = argparse.ArgumentParser(description="Generate the Dart API pages of the site.")
    parser.add_argument("--out", type=Path, required=True)
    parser.add_argument("--root", type=Path, default=repo)
    parser.add_argument("--rules", type=Path, default=None)
    args = parser.parse_args(argv)
    root = args.root.resolve()
    out = (root / args.out).resolve()
    if root not in out.parents:
        print(f"dart-api-docs: --out {args.out} must lie inside the repository", file=sys.stderr)
        return 2
    rules = args.rules or root / "tools" / "arch-check" / "rules.yaml"
    code, _ = _run(["flutter", "pub", "get", "--enforce-lockfile"], root)
    if code != 0:
        print(f"dart-api-docs: flutter pub get failed with exit code {code}", file=sys.stderr)
        return 1
    for name, path in packages(rules):
        target = out / name
        # A page of an earlier build must not survive in the site.
        if target.exists():
            shutil.rmtree(target)
        code, output = _run(["dart", "doc", "--output", str(target)], root / path)
        if code != 0:
            print(f"dart-api-docs: {path}: dart doc failed with exit code {code}", file=sys.stderr)
            return 1
        if findings(output):
            print(f"dart-api-docs: {path}: dart doc printed a warning or an error", file=sys.stderr)
            return 1
        drop_font_links(target)
    print(f"dart-api-docs: {len(packages(rules))} packages documented in {args.out}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
