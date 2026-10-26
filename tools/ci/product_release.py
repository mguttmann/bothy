# SPDX-License-Identifier: Apache-2.0
"""Release build of the product: `make release-build VERSION=<v> OUT=<dir>` (component `bothy`).

docs/spec/18-release.md section 2.1 rules 1 to 4, in this order, all before the first write:

1. `VERSION` of the tagged commit equals `--version`; `BOTHY_VERSION=` of
   `infra/compose/.env.example` in that commit is SemVer without `v`, not the placeholder, and no
   server pre-release in a stable product (the second safeguard after `release-prep` and the
   gate `release-pins`).
2. The registry is `ghcr.io/<owner>` with `<owner>` from `GITHUB_REPOSITORY_OWNER` in lower case;
   the download address is `https://github.com/<GITHUB_REPOSITORY>/releases/download/v<v>/`. Both
   come from the build, not from the source, so a tag whose source still names `OWNER` yields an
   `install.sh` that finds its bundle.
3. Every image `infra/compose/compose.yaml` names with `${BOTHY_VERSION}` exists under that
   registry (`docker manifest inspect`; the release job is logged in to the registry).
4. The bundle `bothy-compose-<v>.tar.gz` is the commit's tree of the bundle paths with only the
   line `BOTHY_REGISTRY=` changed; `install.sh` is the commit's `infra/compose/install.sh` with its
   three built-in lines set (version, SHA-256 of the bundle, download address). Both are scanned
   for `OWNER` and placeholders.
5. The bundle carries the Grafana panel plugin (brief M7-26): the files of the plugin's release
   ZIP (`--plugin-zip`, written by `make -C plugins/grafana-bothy-live release-build`) unpacked
   into `infra/observability/grafana/plugins/bothy-live-panel/`, the folder Compose mounts into
   Grafana. The ZIP must carry the version of `plugins/grafana-bothy-live/VERSION` in the commit
   and pass the checks of `tools/ci/plugin_release.py read_zip`; its files get the time of the
   commit, mode 0644 and owner 0.

Any failure removes the partial output, writes nothing and names the value. The bundle is
reproducible: file order, owners, modes and times come from the commit, the gzip header has no
time.

Trial (docs/DECISIONS.md D-W-127, D-M7-29-3): `RELEASE_TRIAL_REGISTRY=127.0.0.1:<port>` replaces
`ghcr.io/<owner>` and `RELEASE_TRIAL_BASE_URL=http://127.0.0.1:<port>/` the download address;
both are refused under GitHub Actions and for any host but 127.0.0.1, and the image check then
uses `docker manifest inspect --insecure` against the local plain HTTP registry.

Usage:
  product_release.py build --version <v> --out <dir> --plugin-zip <file> [--repo <dir>]
                           [--docker <path>]
Exit codes: 0 written, 1 refused or failed (named on stderr), 2 usage error.
"""

from __future__ import annotations

import argparse
import gzip
import hashlib
import io
import os
import re
import subprocess
import sys
import tarfile
import tempfile
from collections.abc import Mapping
from dataclasses import dataclass
from pathlib import Path

import components
import plugin_release
import yaml

REPO_ROOT = Path(__file__).resolve().parents[2]
ENV_TEMPLATE = components.BUNDLE_ENV_TEMPLATE
INSTALL_SOURCE = "infra/compose/install.sh"
COMPOSE_FILE = "infra/compose/compose.yaml"
# The tracked paths of the bundle (18 section 2): the Compose installation and the folders its
# bind mounts name relative to it, without the test overlays and without the source of install.sh
# (the release ships the rendered install.sh next to the bundle; a second copy with unset
# built-in values would only mislead).
BUNDLE_PATHS = (
    "infra/compose",
    "infra/otel",
    "infra/observability",
    "LICENSE",
    "NOTICE",
    "CHANGELOG.md",
    "VERSION",
)
BUNDLE_EXCLUDES = ("infra/compose/test", INSTALL_SOURCE)
# The folder of the plugin build in the bundle (compose.yaml mounts it into Grafana) and the
# version file the plugin ZIP must match (docs/spec/25-visualization.md section 8.3).
PLUGIN_DIR = "infra/observability/grafana/plugins/bothy-live-panel"
PLUGIN_VERSION_FILE = "plugins/grafana-bothy-live/VERSION"
# The three built-in values of install.sh (16 section 3.1 step 2, 18 section 2.1 rule 3).
BUILT_IN = ("BOTHY_INSTALL_VERSION", "BOTHY_INSTALL_BUNDLE_SHA256", "BOTHY_INSTALL_BASE_URL")
_BUILT_IN_LINE = re.compile(rf"^({'|'.join(BUILT_IN)})='[^']*'$", re.MULTILINE)
_REGISTRY_LINE = re.compile(r"^BOTHY_REGISTRY=.*$", re.MULTILINE)
_SERVER_LINE = re.compile(r"^BOTHY_VERSION=(.*)$", re.MULTILINE)
# What may not survive into a release artifact: the owner placeholder `OWNER` of the source (in
# capitals: install.sh names its options --owner-handle and --owner-name) and the placeholder
# values of the template and of install.sh in any case.
_LEFTOVER = re.compile(r"OWNER|(?i:placeholder)")
_COMPOSE_IMAGE = re.compile(r"^\$\{BOTHY_REGISTRY[^}]*\}/(?P<name>[a-z0-9-]+):(?P<tag>.+)$")
_BOTHY_VERSION = re.compile(r"\$\{BOTHY_VERSION[^}]*\}")
_TRIAL_REGISTRY = re.compile(r"^127\.0\.0\.1:[0-9]{1,5}$")
_TRIAL_BASE_URL = re.compile(r"^http://127\.0\.0\.1:[0-9]{1,5}/([A-Za-z0-9._~/-]*/)?$")
_REPOSITORY = re.compile(r"^[A-Za-z0-9-]+/[A-Za-z0-9._-]+$")


class ProductReleaseError(Exception):
    """The product release cannot be built; nothing was written."""


@dataclass(frozen=True)
class Target:
    """Where the release points: registry of the images and download address of the bundle."""

    registry: str
    base_url: str
    trial: bool


def bundle_name(version: str) -> str:
    """Return the file name of the bundle, for example `bothy-compose-1.0.0.tar.gz`."""
    return f"bothy-compose-{version}.tar.gz"


def _git(repo: Path, *args: str) -> bytes:
    env = {
        k: v
        for k, v in os.environ.items()
        if k not in ("GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR")
    }
    try:
        return subprocess.run(
            ["git", "-C", str(repo), *args], env=env, check=True, capture_output=True
        ).stdout
    except subprocess.CalledProcessError as exc:
        raise ProductReleaseError(
            f"git {' '.join(args)}: {exc.stderr.decode(errors='replace').strip()}"
        ) from exc


def committed(repo: Path, path: str) -> str:
    """Return the text of `path` in the commit HEAD (the tagged commit, not the working tree)."""
    return _git(repo, "show", f"HEAD:{path}").decode("utf-8")


def check_versions(repo: Path, version: str) -> str:
    """Check the product version and the pinned server version of the commit; return the latter.

    Raises:
        ProductReleaseError: `VERSION` differs from `version`, or `BOTHY_VERSION` is the
            placeholder, not SemVer without `v`, or a pre-release in a stable product.
    """
    recorded = committed(repo, "VERSION").strip()
    if recorded != version:
        raise ProductReleaseError(f"VERSION of the commit says {recorded}, not {version}")
    if not components._SEMVER.match(version):
        raise ProductReleaseError(f"product version '{version}' is not SemVer without 'v'")
    values = [v.strip() for v in _SERVER_LINE.findall(committed(repo, ENV_TEMPLATE))]
    if len(values) != 1:
        raise ProductReleaseError(
            f"{ENV_TEMPLATE}: expected exactly one line BOTHY_VERSION=, found {len(values)}"
        )
    server = str(values[0])
    if server == components.SERVER_VERSION_PLACEHOLDER:
        raise ProductReleaseError(
            f"{ENV_TEMPLATE}: BOTHY_VERSION is {server}; pin the server version with "
            "make release-prep COMPONENT=bothy SERVER_VERSION=<version>"
        )
    if not components._SEMVER.match(server):
        raise ProductReleaseError(
            f"{ENV_TEMPLATE}: BOTHY_VERSION '{server}' is not a SemVer version without 'v'"
        )
    if components._is_prerelease(server) and not components._is_prerelease(version):
        raise ProductReleaseError(
            f"stable product {version} pins the server pre-release {server} ({ENV_TEMPLATE})"
        )
    return server


def resolve_target(environ: Mapping[str, str], version: str) -> Target:
    """Return registry and download address from the environment of the build.

    Raises:
        ProductReleaseError: a variable is missing, names `OWNER`, or a trial variable is set
            under GitHub Actions or points anywhere but 127.0.0.1.
    """
    trial_registry = environ.get("RELEASE_TRIAL_REGISTRY", "")
    trial_base = environ.get("RELEASE_TRIAL_BASE_URL", "")
    for name, value in (
        ("RELEASE_TRIAL_REGISTRY", trial_registry),
        ("RELEASE_TRIAL_BASE_URL", trial_base),
    ):
        if value and environ.get("GITHUB_ACTIONS"):
            raise ProductReleaseError(
                f"{name} is for the trial release only and refused when GITHUB_ACTIONS is set"
            )
    if trial_registry and not _TRIAL_REGISTRY.match(trial_registry):
        raise ProductReleaseError(
            f"RELEASE_TRIAL_REGISTRY must be 127.0.0.1:<port>, not '{trial_registry}'"
        )
    if trial_base and not _TRIAL_BASE_URL.match(trial_base):
        raise ProductReleaseError(
            f"RELEASE_TRIAL_BASE_URL must be http://127.0.0.1:<port>/..., not '{trial_base}'"
        )
    if trial_registry:
        registry = trial_registry
    else:
        owner = environ.get("GITHUB_REPOSITORY_OWNER", "")
        if not owner:
            raise ProductReleaseError(
                "GITHUB_REPOSITORY_OWNER is not set (GitHub sets it; outside Actions set it)"
            )
        registry = f"ghcr.io/{owner.lower()}"
        if _LEFTOVER.search(owner):
            raise ProductReleaseError(f"BOTHY_REGISTRY would be {registry}: the owner is OWNER")
    if trial_base:
        base_url = trial_base
    else:
        repository = environ.get("GITHUB_REPOSITORY", "")
        if not repository:
            raise ProductReleaseError(
                "GITHUB_REPOSITORY is not set (GitHub sets it; outside Actions set it)"
            )
        if not _REPOSITORY.match(repository):
            raise ProductReleaseError(f"GITHUB_REPOSITORY '{repository}' is not <owner>/<name>")
        base_url = f"https://github.com/{repository}/releases/download/v{version}/"
    return Target(registry, base_url, bool(trial_registry or trial_base))


def compose_images(compose_text: str, server: str) -> list[str]:
    """Return `<name>:<tag>` of every image compose.yaml names with `${BOTHY_VERSION}`, sorted.

    Example: `${BOTHY_REGISTRY}/bothy-postgres:pg18-${BOTHY_VERSION}` with server 1.4.0 gives
    `bothy-postgres:pg18-1.4.0`.
    """
    found: set[str] = set()

    def walk(node: object) -> None:
        if isinstance(node, dict):
            for key, value in node.items():
                if key == "image" and isinstance(value, str):
                    m = _COMPOSE_IMAGE.match(value)
                    if m and _BOTHY_VERSION.search(m.group("tag")):
                        tag = _BOTHY_VERSION.sub(server, m.group("tag"))
                        found.add(f"{m.group('name')}:{tag}")
                else:
                    walk(value)
        elif isinstance(node, list):
            for item in node:
                walk(item)

    walk(yaml.safe_load(compose_text))
    return sorted(found)


def check_images(docker: str, target: Target, images: list[str]) -> None:
    """Abort unless every image exists under the registry of the release.

    Raises:
        ProductReleaseError: compose.yaml names no such image, or one is missing.
    """
    if not images:
        raise ProductReleaseError(f"{COMPOSE_FILE} names no image with ${{BOTHY_VERSION}}")
    insecure = ["--insecure"] if target.trial else []
    for image in images:
        ref = f"{target.registry}/{image}"
        result = subprocess.run(
            [docker, "manifest", "inspect", *insecure, ref],
            capture_output=True,
            text=True,
            check=False,
        )
        if result.returncode != 0:
            raise ProductReleaseError(f"image {ref} is missing in the registry")


def plugin_files(repo: Path, plugin_zip: Path) -> dict[str, bytes]:
    """Return the files of the plugin's release ZIP after its checks (rule 5).

    Raises:
        ProductReleaseError: the ZIP is missing, or `read_zip` refuses it (another plugin, another
            version than `plugins/grafana-bothy-live/VERSION` of the commit, a bad member).
    """
    version = committed(repo, PLUGIN_VERSION_FILE).strip()
    try:
        data = plugin_zip.read_bytes()
    except OSError as exc:
        raise ProductReleaseError(f"plugin ZIP {plugin_zip}: {exc.strerror}") from exc
    try:
        return plugin_release.read_zip(data, version)
    except plugin_release.PluginReleaseError as exc:
        raise ProductReleaseError(f"plugin ZIP {plugin_zip.name}: {exc}") from exc


def bundle_bytes(
    repo: Path, version: str, registry: str, plugin: Mapping[str, bytes] | None = None
) -> bytes:
    """Return the gzip tar of the bundle: the commit's files with the registry line set.

    `plugin` (path below `bothy-live-panel/` to content) is added under `PLUGIN_DIR`, with the
    commit's time, mode 0644 and owner 0 like the files of `git archive`.

    Raises:
        ProductReleaseError: the template has not exactly one line `BOTHY_REGISTRY=`, or the
            resulting line still names `OWNER`.
    """
    prefix = f"bothy-{version}/"
    paths = [*BUNDLE_PATHS, *(f":(exclude){p}" for p in BUNDLE_EXCLUDES)]
    raw = _git(repo, "archive", "--format=tar", f"--prefix={prefix}", "HEAD", *paths)
    out = io.BytesIO()
    with (
        tarfile.open(fileobj=io.BytesIO(raw)) as source,
        gzip.GzipFile(fileobj=out, mode="wb", mtime=0) as zipped,
        tarfile.open(fileobj=zipped, mode="w", format=tarfile.PAX_FORMAT) as bundle,
    ):
        mtime = 0
        for member in source.getmembers():
            data = source.extractfile(member).read() if member.isfile() else None  # type: ignore[union-attr]
            if member.name == prefix + ENV_TEMPLATE and data is not None:
                text = data.decode("utf-8")
                if len(_REGISTRY_LINE.findall(text)) != 1:
                    raise ProductReleaseError(
                        f"{ENV_TEMPLATE}: expected exactly one line BOTHY_REGISTRY="
                    )
                text = _REGISTRY_LINE.sub(f"BOTHY_REGISTRY={registry}", text)
                if _LEFTOVER.search(_REGISTRY_LINE.findall(text)[0]):
                    raise ProductReleaseError(f"BOTHY_REGISTRY={registry} still names OWNER")
                data = text.encode("utf-8")
                member.size = len(data)
            bundle.addfile(member, io.BytesIO(data) if data is not None else None)
            mtime = member.mtime
        for rel, content in sorted((plugin or {}).items()):
            info = tarfile.TarInfo(f"{prefix}{PLUGIN_DIR}/{rel}")
            info.size, info.mode, info.mtime = len(content), 0o644, mtime
            info.uid = info.gid = 0
            info.uname = info.gname = "root"
            bundle.addfile(info, io.BytesIO(content))
    return out.getvalue()


def render_install(source: str, version: str, sha256: str, base_url: str) -> str:
    """Set the three built-in values of install.sh; scan the result for leftovers.

    Raises:
        ProductReleaseError: a built-in line is missing or doubled, or `OWNER` or a placeholder
            is left in the finished script.
    """
    values = {
        "BOTHY_INSTALL_VERSION": version,
        "BOTHY_INSTALL_BUNDLE_SHA256": sha256,
        "BOTHY_INSTALL_BASE_URL": base_url,
    }
    seen = [m.group(1) for m in _BUILT_IN_LINE.finditer(source)]
    for name in BUILT_IN:
        if seen.count(name) != 1:
            raise ProductReleaseError(
                f"{INSTALL_SOURCE}: expected exactly one line {name}='...', "
                f"found {seen.count(name)}"
            )
    text = _BUILT_IN_LINE.sub(lambda m: f"{m.group(1)}='{values[m.group(1)]}'", source)
    left = sorted({m.group(0) for m in _LEFTOVER.finditer(text)})
    if left:
        raise ProductReleaseError(f"install.sh still contains {', '.join(left)} after rendering")
    return text


def build(
    repo: Path,
    version: str,
    out: Path,
    environ: Mapping[str, str],
    docker: str = "docker",
    *,
    plugin_zip: Path,
) -> list[Path]:
    """Write the bundle and install.sh into `out`; on any error write nothing.

    `plugin_zip` is the release ZIP of the Grafana panel plugin; its files go into the bundle
    (rule 5 of the module documentation).

    Returns:
        The written files: the bundle, then install.sh.

    Raises:
        ProductReleaseError: any rule of 18 section 2.1 is violated (the message names the value).

    Example:
        `build(Path("."), "1.0.0", Path("dist"), os.environ, plugin_zip=zip_path)` writes
        `dist/bothy-compose-1.0.0.tar.gz` and `dist/install.sh`.
    """
    server = check_versions(repo, version)
    target = resolve_target(environ, version)
    plugin = plugin_files(repo, plugin_zip)
    check_images(docker, target, compose_images(committed(repo, COMPOSE_FILE), server))
    bundle = bundle_bytes(repo, version, target.registry, plugin)
    script = render_install(
        committed(repo, INSTALL_SOURCE),
        version,
        hashlib.sha256(bundle).hexdigest(),
        target.base_url,
    )
    out.mkdir(parents=True, exist_ok=True)
    final = [out / bundle_name(version), out / "install.sh"]
    with tempfile.TemporaryDirectory(dir=out, prefix=".release-build-") as tmp:
        staged = [Path(tmp) / p.name for p in final]
        staged[0].write_bytes(bundle)
        staged[1].write_text(script, encoding="utf-8")
        staged[1].chmod(0o755)
        for src, dst in zip(staged, final, strict=True):
            src.replace(dst)
    return final


def main(argv: list[str] | None = None) -> int:
    """Command line entry point; see the module documentation."""
    parser = argparse.ArgumentParser(description="Release build of the product bundle.")
    sub = parser.add_subparsers(dest="cmd", required=True)
    p_build = sub.add_parser("build")
    p_build.add_argument("--version", required=True)
    p_build.add_argument("--out", type=Path, required=True)
    p_build.add_argument("--repo", type=Path, default=REPO_ROOT)
    p_build.add_argument("--docker", default="docker")
    p_build.add_argument("--plugin-zip", type=Path, required=True)
    args = parser.parse_args(argv)
    try:
        written = build(
            args.repo, args.version, args.out, os.environ, args.docker, plugin_zip=args.plugin_zip
        )
    except ProductReleaseError as exc:
        print(f"release-build: {exc}", file=sys.stderr)
        return 1
    for path in written:
        print(f"  release-build: wrote {path}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
