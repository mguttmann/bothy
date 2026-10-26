# tools/arch-check

Architecture test of the Dart workspace: holds the dependency rules of
[docs/spec/15-client-architecture.md](../../docs/spec/15-client-architecture.md), section 3.1,
for every member of the workspace on every run of the gate (`make arch`, brief M4-01).

| File | Content |
|---|---|
| `rules.yaml` | the single source of the rules: per member its folder, kind, the arrows of the diagram, forbidden imports, members it must never reach, allowed runtime dependencies and the folders that may hold native code |
| `arch_check.py` | the check (Python standard library and PyYAML from the pinned `tools/` environment) |
| `tests/test_arch_check.py` | tests on throwaway workspaces, the real rules against the diagram of 15 3.1 and the real repository |
| `no_webview.py` | VZ-T16 (docs/spec/25-visualization.md 12, SEC-VZ-1): no WebView package in the manifest of `apps/client` or the workspace lock, no `HtmlElementView` or iframe element in its Dart files, no `WKWebView`, `android.webkit.WebView` or `WebView2` in its native folders |
| `tests/test_no_webview.py` | VZ-T16 on throwaway repositories with each banned thing, and on the real app |

Run: `make arch` (with the Go part), or alone
`uv run --project tools --locked --quiet python tools/arch-check/arch_check.py` and
`uv run --project tools --locked --quiet python tools/arch-check/no_webview.py`.

## What it checks

Each finding names the file, the line and the rule of 15 3.1:

| Rule | Check |
|---|---|
| 1, 7 | `package:flutter/` or `dart:ui` imported by `domain`, `api_client`, `auth`, `local_store`, `sync_engine` or `bothy_noise`, or a Flutter SDK dependency of one of them; `dart:io` in `lib/` of `bothy_noise` (its tests read vector files with it); a runtime dependency of `bothy_noise` other than `cryptography` |
| 2 | a package that depends on or imports an app; a relative import (`../`) that leaves the member |
| 3, 6, 7 | `apps/console`, `bothy_viz` or `terminal_ui` that reaches `sync_engine` or `local_store`, directly or through another member |
| 4 | a native source (`*.swift`, `*.m`, `*.mm`, `*.kt`, `*.java`, `*.cpp`, `*.cc`, `*.h`) outside `packages/device_key` and `apps/client/{ios,macos,android,windows}` |
| 8 | a dependency (`dependencies`, `dev_dependencies`) or an import of a member that the diagram does not reach from the member |
| 9 | a `pubspec.yaml` under `apps/` or `packages/` that is not a member of `rules.yaml` and of the root `workspace:` list, a member without its manifest, or a manifest whose name differs from the rules |

Imports and exports are read in `lib/`, `test/`, `integration_test/` and `bin/`, also when a
directive spans lines or carries conditional URIs (`if (dart.library.io) '...'`). Build output
(`build/`, `.dart_tool/`) and hidden folders are not read.

## Why an allow list

The reach of the diagram is an allow list (docs/DECISIONS.md D-M4-01-3): a member may use what
the arrows reach from it (the arrows and their transitive closure), nothing else. A deny list
would let every new edge in silently. A new edge therefore changes 15 3.1 and `rules.yaml` in
the same change; `test_rules_edges_equal_the_spec_diagram` holds both equal.
