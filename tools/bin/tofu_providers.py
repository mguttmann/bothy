# SPDX-License-Identifier: Apache-2.0
"""Offline provider mirror of the OpenTofu gates (brief M7-16, docs/DECISIONS.md D-M7-16-2).

`make tofu-check` and the check `tofu` of `make ops-check` run `tofu init` without network: the
only provider source is a local filesystem mirror. This script fills that mirror. The pins are
the lock files of the test roots (`infra/tofu/test/*/.terraform.lock.hcl`, written by
`tofu providers lock` for every platform of the gates, kept current by Renovate): for each
provider and the current platform it asks the OpenTofu registry for the archive, accepts it
only when the registry's SHA-256 is one of the `zh:` hashes of the lock file, downloads it and
checks the bytes against that hash again. A cached archive is checked on every call. The
mirror uses the packed layout of OpenTofu
(`<cache>/registry.opentofu.org/<namespace>/<type>/terraform-provider-<type>_<version>_<os>_<arch>.zip`).

Usage:
  tofu_providers.py mirror [--lock <file>]... [--cache <dir>]
                                   ensure the archives, print the mirror directory
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import sys
import tempfile
import urllib.error
import urllib.request
from collections.abc import Callable, Iterable, Sequence
from dataclasses import dataclass
from pathlib import Path

import fetch_tools

REPO_ROOT = Path(__file__).resolve().parents[2]
DEFAULT_CACHE = REPO_ROOT / ".cache" / "tofu-providers"
TEST_ROOTS = REPO_ROOT / "infra" / "tofu" / "test"
REGISTRY = "registry.opentofu.org"

_PROVIDER = re.compile(
    r'^provider "' + re.escape(REGISTRY) + r'/([a-z0-9-]+/[a-z0-9-]+)" \{\n(.*?)^\}', re.M | re.S
)
_VERSION = re.compile(r'^\s+version\s+= "([^"]+)"', re.M)
_ZIP_HASH = re.compile(r'"zh:([0-9a-f]{64})"')

Fetch = Callable[[str], bytes]


class ProviderError(Exception):
    """A lock file is malformed, or a download does not match its pin."""


@dataclass(frozen=True)
class Provider:
    """One provider of a lock file.

    Attributes:
        source: `<namespace>/<type>` in the OpenTofu registry.
        version: the locked version.
        zip_hashes: the SHA-256 values (hex) of the release archives of every platform.
    """

    source: str
    version: str
    zip_hashes: frozenset[str]

    def archive(self, plat: str) -> str:
        """File name of the archive for a platform such as `linux-amd64`."""
        name = self.source.split("/")[1]
        return f"terraform-provider-{name}_{self.version}_{plat.replace('-', '_')}.zip"


def default_locks() -> list[Path]:
    """The lock files of the test roots, the pins of the gates."""
    return sorted(TEST_ROOTS.glob("*/.terraform.lock.hcl"))


def parse_lock(path: Path) -> list[Provider]:
    """Read the providers of one lock file.

    Raises:
        ProviderError: no provider block, or a block without version or `zh:` hash.
    """
    text = path.read_text(encoding="utf-8")
    providers = []
    for m in _PROVIDER.finditer(text):
        version = _VERSION.search(m.group(2))
        hashes = frozenset(_ZIP_HASH.findall(m.group(2)))
        if not version or not hashes:
            raise ProviderError(f"{path}: {m.group(1)} has no version or no zh: hash")
        providers.append(Provider(m.group(1), version.group(1), hashes))
    if not providers:
        raise ProviderError(f"{path}: no provider of {REGISTRY}")
    return providers


# The registry refuses the default User-Agent of urllib (HTTP 403, seen 2026-10-20).
_USER_AGENT = "bothy-tofu-providers"
_TIMEOUT_S = 300


def _fetch(url: str) -> bytes:
    if not url.startswith("https://"):
        raise ProviderError(f"refusing non-https download: {url}")
    request = urllib.request.Request(url, headers={"User-Agent": _USER_AGENT})  # noqa: S310
    with urllib.request.urlopen(request, timeout=_TIMEOUT_S) as resp:  # noqa: S310  # nosec B310
        data: bytes = resp.read()
    return data


def _sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def _merge(locks: Iterable[Path]) -> list[Provider]:
    merged: dict[str, Provider] = {}
    for lock in locks:
        for provider in parse_lock(lock):
            seen = merged.setdefault(provider.source, provider)
            if seen != provider:
                raise ProviderError(
                    f"{provider.source}: the lock files pin different versions or hashes"
                )
    return list(merged.values())


def ensure_mirror(
    locks: Sequence[Path], cache: Path, plat: str, fetch: Fetch | None = None
) -> Path:
    """Make sure the mirror holds the verified archive of every locked provider for `plat`.

    Args:
        locks: lock files; a provider in several must have the same version and hashes.
        cache: the mirror directory (packed layout).
        plat: platform key of fetch_tools, e.g. `darwin-arm64`.
        fetch: downloads a URL (tests inject their own).

    Returns:
        The mirror directory, for `filesystem_mirror { path = ... }`.

    Raises:
        ProviderError: malformed lock file, or a hash that does not match.
    """
    fetch = fetch or _fetch
    os_name, arch = plat.split("-")
    for provider in _merge(locks):
        target = cache / REGISTRY / provider.source / provider.archive(plat)
        if target.is_file() and _sha256(target) in provider.zip_hashes:
            continue
        meta_url = f"https://{REGISTRY}/v1/providers/{provider.source}/{provider.version}/download/{os_name}/{arch}"
        meta = json.loads(fetch(meta_url))
        expected = str(meta.get("shasum", ""))
        if expected not in provider.zip_hashes:
            raise ProviderError(
                f"{provider.source} {provider.version} {plat}: registry hash {expected} "
                "not in the lock file"
            )
        data = fetch(str(meta["download_url"]))
        actual = hashlib.sha256(data).hexdigest()
        if actual != expected:
            raise ProviderError(
                f"{provider.source} {provider.version} {plat}: sha256 {actual}, expected {expected}"
            )
        target.parent.mkdir(parents=True, exist_ok=True)
        fd, tmp = tempfile.mkstemp(dir=target.parent, prefix=".download-")
        with os.fdopen(fd, "wb") as handle:
            handle.write(data)
        Path(tmp).replace(target)
    return cache


def cli_config(mirror: Path, plugin_cache: Path) -> str:
    """CLI configuration whose only provider source is the mirror.

    The plugin cache lets several roots in one run share one unpacked copy; the archives in
    the mirror were checked against the lock files, so a root without a lock file of its own
    (a module directory in ops-check) may take them from the cache as well.
    """
    return (
        f'plugin_cache_dir = "{plugin_cache}"\n'
        "plugin_cache_may_break_dependency_lock_file = true\n"
        "provider_installation {\n"
        f'  filesystem_mirror {{\n    path = "{mirror}"\n  }}\n'
        "}\n"
    )


def main(argv: Sequence[str] | None = None) -> int:
    """Run the command line and return the exit code."""
    parser = argparse.ArgumentParser(
        description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter
    )
    sub = parser.add_subparsers(dest="command", required=True)
    mirror = sub.add_parser("mirror", help="ensure the archives, print the mirror directory")
    mirror.add_argument(
        "--lock", action="append", type=Path, help="lock file (default: the test roots)"
    )
    mirror.add_argument("--cache", type=Path, default=DEFAULT_CACHE)
    args = parser.parse_args(argv)
    try:
        path = ensure_mirror(
            args.lock or default_locks(), args.cache, fetch_tools.platform_key(), fetch=_fetch
        )
    except (
        ProviderError,
        fetch_tools.LockError,
        urllib.error.URLError,
        OSError,
        ValueError,
        KeyError,
    ) as exc:
        print(f"tofu_providers: {exc}", file=sys.stderr)
        return 1
    print(path)
    return 0


if __name__ == "__main__":
    sys.exit(main())
