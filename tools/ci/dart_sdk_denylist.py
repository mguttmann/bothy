# SPDX-License-Identifier: Apache-2.0
"""Fail when a Dart package depends on an OpenTelemetry or analytics SDK (gate `licenses`).

ADR 0057 "Confirmation" and brief M4-31 Design note 6: the apps send their few metrics to their
own server and crash reports only through `sentry_flutter` (15 16, 17); no third-party SDK may
collect data on the side. The tool reads the root `pubspec.lock` of the Dart workspace and reports
every package whose name matches a pattern of DENY_PATTERNS, except the names of EXEMPT.

`pubspec.lock` lists every resolved package but not the dependency graph, so "sentry_flutter and
its own dependencies" are named explicitly: `sentry_flutter` and its Dart core `sentry`. No other
package of the Sentry SDK matches a pattern today; a new one that does fails the gate until it is
added here with a reason.

Usage: dart_sdk_denylist.py [--lock <pubspec.lock>]

Exit codes: 0 no finding; 1 a denied package; 2 an unreadable or malformed lock file.
"""

from __future__ import annotations

import argparse
import fnmatch
import sys
from pathlib import Path

import yaml

# Patterns of package names (fnmatch, lower case), each with the reason it is denied.
DENY_PATTERNS: dict[str, str] = {
    # OpenTelemetry SDKs (the community Dart SDK and its exporters): 17 rule 2, export only
    # through the server and its Collector (R-CL-10).
    "opentelemetry*": "OpenTelemetry SDK",
    # Google Analytics for Firebase: product analytics to a third party.
    "firebase_analytics": "analytics SDK",
    # Product analytics vendors.
    "amplitude*": "analytics SDK",
    "mixpanel*": "analytics SDK",
    "segment*": "analytics SDK",
    "posthog*": "analytics SDK",
    # Monitoring and RUM vendors with their own collection path.
    "datadog*": "monitoring SDK",
    "newrelic*": "monitoring SDK",
    "appcenter*": "analytics and crash SDK",
}

# The crash path of 15 16 and its Dart core; never extended without a reason in DECISIONS.md.
EXEMPT = frozenset({"sentry_flutter", "sentry"})


def packages(lock: Path) -> list[str]:
    """Return the package names of `lock`; raise ValueError for a malformed file."""
    loaded = yaml.safe_load(lock.read_text(encoding="utf-8")) or {}
    pkgs = loaded.get("packages", {}) if isinstance(loaded, dict) else None
    if not isinstance(pkgs, dict):
        raise ValueError(f"{lock}: no packages mapping")
    return sorted(str(name) for name in pkgs)


def findings(lock: Path) -> list[str]:
    """Return one line per denied package of `lock`, sorted by name."""
    out = []
    for name in packages(lock):
        if name in EXEMPT:
            continue
        for pattern in DENY_PATTERNS:
            if fnmatch.fnmatchcase(name.lower(), pattern):
                out.append(f"{name}: matches the deny pattern {pattern}")
                break
    return out


def main(argv: list[str] | None = None) -> int:
    """Run the gate; see the module docstring for the exit codes."""
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--lock", type=Path, default=Path("pubspec.lock"))
    args = parser.parse_args(argv)
    try:
        found = findings(args.lock)
    except (OSError, ValueError, yaml.YAMLError) as e:
        print(f"dart-sdk-denylist: {e}", file=sys.stderr)
        return 2
    for line in found:
        print(f"dart-sdk-denylist: {line}", file=sys.stderr)
    if found:
        return 1
    print(f"dart-sdk-denylist: ok ({args.lock})")
    return 0


if __name__ == "__main__":
    sys.exit(main())
