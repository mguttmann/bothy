# SPDX-License-Identifier: Apache-2.0
"""Tests of VZ-T16 (tools/arch-check/no_webview.py, brief M6-16 Design note 5).

Each case writes a throwaway repository with a clean app and adds one banned thing: a WebView
package in the manifest or the lock, an `HtmlElementView` or an iframe element in Dart, a
`WKWebView`, `android.webkit.WebView` or `WebView2` in a native folder. The last case runs the
check against the real repository.
"""

from __future__ import annotations

from pathlib import Path

import no_webview
import pytest
import yaml

REPO = Path(__file__).resolve().parents[3]


def _write(root: Path, rel: str, text: str) -> None:
    path = root / rel
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text, encoding="utf-8")


@pytest.fixture
def repo(tmp_path: Path) -> Path:
    """A clean repository: the app's manifest, a lock, a Dart file and native sources."""
    _write(
        tmp_path,
        "apps/client/pubspec.yaml",
        yaml.safe_dump(
            {
                "name": "client",
                "dependencies": {"url_launcher": "^6.3.3", "flutter": {"sdk": "flutter"}},
                "dev_dependencies": {"flutter_test": {"sdk": "flutter"}},
            }
        ),
    )
    _write(
        tmp_path,
        "pubspec.lock",
        yaml.safe_dump({"packages": {"url_launcher": {"version": "6.3.3"}}}),
    )
    _write(
        tmp_path,
        "apps/client/lib/live.dart",
        "// No WebView or HtmlElementView here: the link opens the system browser.\n"
        "void open(Uri url) {}\n",
    )
    _write(tmp_path, "apps/client/macos/Runner/AppDelegate.swift", "import Cocoa\n")
    _write(
        tmp_path,
        "apps/client/android/app/src/main/kotlin/MainActivity.kt",
        "class MainActivity\n",
    )
    return tmp_path


def _codes(root: Path) -> list[str]:
    findings, _ = no_webview.check(root)
    return findings


def test_a_clean_app_passes(repo: Path) -> None:
    assert _codes(repo) == []
    assert no_webview.main(["--root", str(repo)]) == 0


@pytest.mark.parametrize(
    "package",
    [
        "webview_flutter",
        "flutter_inappwebview",
        "webview_windows",
        "desktop_webview_window",
        "my_webview_kit",
    ],
)
def test_a_webview_package_in_the_manifest_fails(repo: Path, package: str) -> None:
    manifest = yaml.safe_load((repo / "apps/client/pubspec.yaml").read_text())
    manifest["dependencies"][package] = "^1.0.0"
    _write(repo, "apps/client/pubspec.yaml", yaml.safe_dump(manifest))
    [finding] = _codes(repo)
    assert finding == (
        f"apps/client/pubspec.yaml:1: VZ-T16: WebView package {package} in dependencies"
    )
    assert no_webview.main(["--root", str(repo)]) == 1


def test_a_resolved_webview_package_fails(repo: Path) -> None:
    _write(
        repo,
        "pubspec.lock",
        yaml.safe_dump({"packages": {"webview_flutter_wkwebview": {"version": "3.0.0"}}}),
    )
    assert _codes(repo) == [
        "pubspec.lock:1: VZ-T16: WebView package webview_flutter_wkwebview resolved for the"
        " workspace"
    ]


@pytest.mark.parametrize(
    ("code", "what"),
    [
        ("Widget w() => const HtmlElementView(viewType: 'grafana');", "HtmlElementView"),
        ("final f = web.HTMLIFrameElement()..src = url;", "an iframe element"),
        ("final f = document.createElement('iframe');", "an iframe element"),
    ],
)
def test_html_element_view_or_an_iframe_in_dart_fails(repo: Path, code: str, what: str) -> None:
    _write(repo, "apps/client/lib/viz/embed.dart", f"// Embeds Grafana.\n{code}\n")
    [finding] = _codes(repo)
    assert finding.startswith("apps/client/lib/viz/embed.dart:2: VZ-T16: ")
    assert what in finding


@pytest.mark.parametrize(
    ("rel", "code"),
    [
        ("apps/client/ios/Runner/Grafana.swift", "let view = WKWebView(frame: .zero)"),
        ("apps/client/macos/Runner/Grafana.swift", "import WebKit; let v = WKWebView()"),
        (
            "apps/client/android/app/src/main/kotlin/Grafana.kt",
            "import android.webkit.WebView",
        ),
        ("apps/client/windows/runner/grafana.cpp", "#include <WebView2.h>\nauto w = WebView2();"),
    ],
)
def test_a_native_webview_fails(repo: Path, rel: str, code: str) -> None:
    _write(repo, rel, code + "\n")
    findings = _codes(repo)
    assert findings, rel
    assert all(f.startswith(f"{rel}:") for f in findings)


def test_build_output_and_comments_are_not_read(repo: Path) -> None:
    _write(repo, "apps/client/build/web/main.dart", "HtmlElementView()\n")
    _write(repo, "apps/client/ios/Pods/Some/WK.swift", "WKWebView()\n")
    _write(repo, "apps/client/macos/Flutter/ephemeral/x.swift", "WKWebView()\n")
    _write(repo, "apps/client/lib/note.dart", "/// Never a WKWebView or an HtmlElementView.\n")
    assert _codes(repo) == []


def test_the_real_app_passes() -> None:
    findings, count = no_webview.check(REPO)
    assert findings == []
    assert count > 100, "the check must read the sources of the app"
