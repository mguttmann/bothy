# SPDX-License-Identifier: Apache-2.0
"""VZ-T16: no WebView and no iframe in the apps.

Sources: docs/spec/25-visualization.md 9.1 and 12; docs/spec/19-security.md SEC-VZ-1; brief
M6-16 Design note 5; docs/decisions.d/M6-16.md D-M6-16-5.

The visualizations of the apps are native; "Open in Grafana" leaves the app for the system
browser. A WebView would need a second login, bypass permissions and design, and open a
clickjacking and cookie surface. The check fails when

- the manifest of the app (`apps/client/pubspec.yaml`: dependencies, dev dependencies and
  overrides) or the resolved packages of the workspace (`pubspec.lock` at the root, which the
  app resolves through) name a WebView package: one of `KNOWN_WEBVIEW_PACKAGES` or any package
  whose name contains `webview`;
- a Dart file of the app uses `HtmlElementView` or creates an iframe element;
- a source or build file of a native folder of the app (`ios`, `macos`, `android`, `windows`)
  names `WKWebView`, `android.webkit.WebView` or `WebView2`.

Comment lines (`//`, `#`, `/*`, `*`) may name what is banned. Build output, tool caches and
hidden folders are not read.

Usage: no_webview.py [--root <repository>]
Exit 0 prints `no-webview: <n> files, ok`; exit 1 prints one line per finding
(`<file>:<line>: VZ-T16: <message>`).
"""

from __future__ import annotations

import argparse
import re
import sys
from collections.abc import Iterator
from pathlib import Path

import yaml

APP = Path("apps/client")

KNOWN_WEBVIEW_PACKAGES = frozenset(
    {
        "webview_flutter",
        "flutter_inappwebview",
        "webview_windows",
        "desktop_webview_window",
    }
)

NATIVE_FOLDERS = ("ios", "macos", "android", "windows")

NATIVE_SUFFIXES = frozenset(
    {
        ".swift",
        ".m",
        ".mm",
        ".h",
        ".kt",
        ".kts",
        ".java",
        ".cpp",
        ".cc",
        ".c",
        ".gradle",
        ".xml",
        ".plist",
        ".pbxproj",
        ".cmake",
        ".txt",
        ".storyboard",
        ".xib",
    }
)

# Folders that hold build output or fetched code, never sources of the app.
SKIPPED_DIRS = frozenset(
    {"build", ".dart_tool", "Pods", ".symlinks", "ephemeral", ".gradle", ".cxx"}
)

DART_PATTERNS = (
    (re.compile(r"\bHtmlElementView\b"), "HtmlElementView embeds a platform view"),
    (
        re.compile(r"\b(?:HTMLIFrameElement|IFrameElement)\b"),
        "an iframe element",
    ),
    (
        re.compile(r"""createElement\(\s*['"]iframe['"]""", re.IGNORECASE),
        "an iframe element",
    ),
)

NATIVE_PATTERNS = (
    (re.compile(r"\bWKWebView\b"), "WKWebView"),
    (re.compile(r"android\.webkit\.WebView\b"), "android.webkit.WebView"),
    (re.compile(r"\bWebView2\b"), "WebView2"),
)


def is_webview_package(name: str) -> bool:
    """Whether the package `name` is a WebView package."""
    return name in KNOWN_WEBVIEW_PACKAGES or "webview" in name.lower()


def _files(root: Path, suffixes: frozenset[str]) -> Iterator[Path]:
    """The files below `root` with one of `suffixes`, without build output and hidden folders."""
    if not root.is_dir():
        return
    for path in sorted(root.rglob("*")):
        rel = path.relative_to(root).parts
        if any(p in SKIPPED_DIRS or p.startswith(".") for p in rel[:-1]):
            continue
        if path.is_file() and path.suffix in suffixes:
            yield path


def _is_comment(line: str) -> bool:
    stripped = line.lstrip()
    return stripped.startswith(("//", "#", "/*", "*"))


def _scan(path: Path, patterns: tuple[tuple[re.Pattern[str], str], ...], rel: str) -> list[str]:
    findings = []
    text = path.read_text(encoding="utf-8", errors="replace")
    for number, line in enumerate(text.splitlines(), start=1):
        if _is_comment(line):
            continue
        for pattern, what in patterns:
            if pattern.search(line):
                findings.append(f"{rel}:{number}: VZ-T16: {what} in the app")
    return findings


def _manifest_findings(root: Path) -> list[str]:
    manifest = root / APP / "pubspec.yaml"
    rel = str(APP / "pubspec.yaml")
    if not manifest.is_file():
        return [f"{rel}:1: VZ-T16: the manifest of the app is missing"]
    data = yaml.safe_load(manifest.read_text(encoding="utf-8")) or {}
    findings = []
    for section in ("dependencies", "dev_dependencies", "dependency_overrides"):
        for name in data.get(section) or {}:
            if is_webview_package(str(name)):
                findings.append(f"{rel}:1: VZ-T16: WebView package {name} in {section}")
    return findings


def _lock_findings(root: Path) -> list[str]:
    lock = root / "pubspec.lock"
    if not lock.is_file():
        return []
    data = yaml.safe_load(lock.read_text(encoding="utf-8")) or {}
    return [
        f"pubspec.lock:1: VZ-T16: WebView package {name} resolved for the workspace"
        for name in sorted(data.get("packages") or {})
        if is_webview_package(str(name))
    ]


def check(root: Path) -> tuple[list[str], int]:
    """Runs VZ-T16 below the repository `root`; returns the findings and the files read."""
    findings = _manifest_findings(root) + _lock_findings(root)
    count = 2
    app = root / APP
    for path in _files(app, frozenset({".dart"})):
        count += 1
        findings += _scan(path, DART_PATTERNS, str(path.relative_to(root)))
    for folder in NATIVE_FOLDERS:
        for path in _files(app / folder, NATIVE_SUFFIXES):
            count += 1
            findings += _scan(path, NATIVE_PATTERNS, str(path.relative_to(root)))
    return findings, count


def main(argv: list[str] | None = None) -> int:
    """Command line entry point; see the module docstring."""
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parents[2])
    args = parser.parse_args(argv)
    findings, count = check(args.root)
    for finding in findings:
        print(finding)
    if findings:
        return 1
    print(f"no-webview: {count} files, ok")
    return 0


if __name__ == "__main__":
    sys.exit(main())
