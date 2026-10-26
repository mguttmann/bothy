# SPDX-License-Identifier: Apache-2.0
"""Trial release on the build host: `make release-trial COMPONENT=<name>` (D-W-127).

Runs the release targets of a component against throwaway local targets and verifies the
result like an instance operator, so the chain has run once for real before the owner sets the
first tag (RK-57, docs/operations/release.md section 2). For `server`:

1. a registry container pinned by digest on 127.0.0.1 with a random port;
2. `make -C services/server release-build` (the binaries, as in release.yml);
3. `bothy-server` for all platforms in one build (it cross compiles) and `bothy-postgres` per
   platform, both pushed by digest to the local registry; the postgres digests are joined
   into one index (tools/ci/release_chain.py `join`);
4. a cosign key pair generated for this run, SBOMs, signatures with the signing configuration
   of the private phase, SHA256SUMS and its signature (`sbom`, `sign`);
5. `cosign verify --key`, `cosign verify-blob` and `sha256sum --check` (`verify`), then the
   version tags `bothy-server:<version>` and `bothy-postgres:pg18-<version>` (`tag`), and a
   check that every image `infra/compose/compose.yaml` names with `${BOTHY_VERSION}` exists
   under exactly that version and digest;
6. three negative probes that must fail: a changed file against SHA256SUMS, an unsigned digest
   against `cosign verify`, a release without one SBOM against `sign`.

For `bothy` (the product, brief M7-29 Design note 3) the trial first runs every step of `server`
above, then, against the same local registry:

7. a throwaway git repository with the bundle paths of the commit and `BOTHY_VERSION` pinned to
   the server version of this trial (the committed template carries the placeholder);
8. `tools/ci/product_release.py` (the code of `make release-build`) with
   `RELEASE_TRIAL_REGISTRY=<the local registry>` and `RELEASE_TRIAL_BASE_URL` pointing at a static
   HTTP server on 127.0.0.1 that serves the result;
9. the check an installation makes: `install.sh` and the bundle downloaded from that address,
   the bundle's SHA-256 equal to the value built into `install.sh`, `BOTHY_REGISTRY` and
   `BOTHY_VERSION` of the bundle equal to the trial registry and server version, every image the
   bundle's `compose.yaml` names present there with the signed digest, and `docker pull` of each
   by the Docker daemon (as Compose pulls them). The pulled images are removed again.

The bundle carries the Grafana panel plugin (brief M7-26), so step 8 first runs
`make -C plugins/grafana-bothy-live release-build` with the plugin's own version and hands the ZIP
to the product build; step 9 also checks the plugin version inside the bundle.

For `grafana-bothy-live` (the Grafana panel plugin, brief M7-26) the trial needs no registry:

1. `make -C plugins/grafana-bothy-live release-build` writes `bothy-live-panel-<version>.zip`;
2. a cosign key pair generated for this run, the SPDX and CycloneDX SBOMs of the ZIP, the
   signature of the ZIP, SHA256SUMS and its signature (`sbom`, `sign`, as release.yml job
   plugin-signing runs them);
3. `cosign verify-blob` and `sha256sum --check` (`verify`) and the check of the ZIP that the
   product build makes (`plugin_release.read_zip`);
4. two negative probes that must fail: a changed ZIP against SHA256SUMS, and `sign` of a release
   without its SBOM.

It never pushes to another registry, never tags the repository, never creates a release, and it
refuses to run when GITHUB_ACTIONS is set, so it never runs with release credentials. The key
pair, the registry, the buildx builder and the temporary directory are removed at the end,
also after a failure. Vulnerability scan and ScanCode are not part of the trial; they are
separate targets (`make trivy-image`, `make scancode`) that release.yml runs.

Usage:
  release_trial.py --component <name> [--platforms linux/amd64,linux/arm64]
                   [--cosign <path>] [--syft <path>] [--keep]
  <name> is "server", "bothy" (the product) or "grafana-bothy-live" (the panel plugin).
Exit codes: 0 trial passed, 1 a step failed, 2 refused (GitHub Actions) or bad arguments.
"""

from __future__ import annotations

import argparse
import functools
import hashlib
import http.server
import io
import json
import os
import platform
import re
import secrets
import shutil
import subprocess
import sys
import tarfile
import tempfile
import threading
import time
import urllib.error
import urllib.request
from collections.abc import Iterator, Sequence
from contextlib import ExitStack, contextmanager
from dataclasses import dataclass
from pathlib import Path

import plugin_release
import product_release
import release_chain as chain
import yaml

REPO_ROOT = Path(__file__).resolve().parents[2]
# renovate: datasource=docker depName=registry
REGISTRY_IMAGE = (
    "registry:3.1.2@sha256:ddf754342cfc8acc51a56d5d0ab6af06826461864460636d8bd5c546dab2a7b8"
)
COMPONENTS = ("server", "bothy", "grafana-bothy-live")
# The component folder of the Grafana panel plugin (tools/ci/components.py).
PLUGIN_COMPONENT_DIR = "plugins/grafana-bothy-live"
SUPPORTED_PLATFORMS = ("linux/amd64", "linux/arm64")
SOURCE_URL = "https://github.com/OWNER/bothy"
_MANIFEST_TYPES = ", ".join(
    [
        "application/vnd.oci.image.index.v1+json",
        "application/vnd.oci.image.manifest.v1+json",
        "application/vnd.docker.distribution.manifest.list.v2+json",
        "application/vnd.docker.distribution.manifest.v2+json",
    ]
)
_COMPOSE_IMAGE = re.compile(r"^\$\{BOTHY_REGISTRY[^}]*\}/(?P<name>[a-z0-9-]+):(?P<tag>.+)$")
_BOTHY_VERSION = re.compile(r"\$\{BOTHY_VERSION[^}]*\}")


class TrialError(Exception):
    """The trial release failed or was refused."""


@dataclass(frozen=True)
class ServerImages:
    """Dockerfiles of the server component and the tag form of each image (18 section 7)."""

    server_dockerfile: str = "services/server/Dockerfile"
    postgres_dockerfile: str = "services/server/images/bothy-postgres/Dockerfile"

    @staticmethod
    def tags(version: str) -> dict[str, str]:
        """Return the version tag per image name: the forms `compose.yaml` pulls."""
        return {"bothy-server": version, "bothy-postgres": f"pg18-{version}"}


def refuse_in_ci(environ: dict[str, str] | os._Environ[str]) -> None:
    """Raise TrialError when running under GitHub Actions (release credentials could exist)."""
    if environ.get("GITHUB_ACTIONS"):
        raise TrialError("refuses to run under GitHub Actions (GITHUB_ACTIONS is set)")


def parse_platforms(text: str) -> list[str]:
    """Return the platforms of a comma list; raise TrialError for an unsupported one."""
    platforms = [p.strip() for p in text.split(",") if p.strip()]
    bad = [p for p in platforms if p not in SUPPORTED_PLATFORMS]
    if not platforms or bad:
        raise TrialError(f"platforms must be a subset of {','.join(SUPPORTED_PLATFORMS)}: {text}")
    return platforms


def compose_images(compose: Path, version: str) -> dict[str, str]:
    """Return name to tag of every image `compose.yaml` names with `${BOTHY_VERSION}`.

    Example: `${BOTHY_REGISTRY}/bothy-postgres:pg18-${BOTHY_VERSION}` gives
    `{"bothy-postgres": "pg18-<version>"}`.
    """
    doc = yaml.safe_load(compose.read_text(encoding="utf-8"))
    found: dict[str, str] = {}

    def walk(node: object) -> None:
        if isinstance(node, dict):
            for key, value in node.items():
                if key == "image" and isinstance(value, str):
                    m = _COMPOSE_IMAGE.match(value)
                    if m and _BOTHY_VERSION.search(m.group("tag")):
                        found[m.group("name")] = _BOTHY_VERSION.sub(version, m.group("tag"))
                else:
                    walk(value)
        elif isinstance(node, list):
            for item in node:
                walk(item)

    walk(doc)
    return found


def manifest_digest(registry: str, name: str, reference: str) -> str | None:
    """Return the digest of a manifest in the local registry, or None if it does not exist."""
    if not registry.startswith("127.0.0.1:"):
        raise TrialError(f"refusing a registry outside 127.0.0.1: {registry}")
    req = urllib.request.Request(
        f"http://{registry}/v2/{name}/manifests/{reference}", headers={"Accept": _MANIFEST_TYPES}
    )
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:  # noqa: S310  # nosec B310
            return "sha256:" + hashlib.sha256(resp.read()).hexdigest()
    except urllib.error.HTTPError as exc:
        if exc.code == 404:
            return None
        raise


class Trial:
    """One trial run of the server component with its throwaway resources."""

    def __init__(
        self, tools: chain.Tools, workdir: Path, platforms: Sequence[str], repo: Path = REPO_ROOT
    ) -> None:
        """Prepare a trial over tools in workdir for platforms of the repository repo."""
        self.tools = tools
        self.workdir = workdir
        self.platforms = list(platforms)
        self.repo = repo
        self.dist = workdir / "dist"
        self.registry = ""
        self.builder = f"bothy-trial-{secrets.token_hex(4)}"
        self.version = (repo / "services/server/VERSION").read_text(encoding="utf-8").strip()

    @contextmanager
    def local_registry(self) -> Iterator[str]:
        """Start the pinned registry on 127.0.0.1 with a random port; stop it at the end."""
        cid = self.tools(
            [self.tools.docker, "run", "-d", "--rm", "-p", "127.0.0.1::5000", REGISTRY_IMAGE]
        ).stdout.strip()
        try:
            port = self.tools([self.tools.docker, "port", cid, "5000/tcp"]).stdout.split()[0]
            host = "127.0.0.1:" + port.rsplit(":", 1)[1]
            self._wait_for(host)
            self.registry = host
            yield host
        finally:
            self.tools([self.tools.docker, "stop", cid])

    @staticmethod
    def _wait_for(host: str, timeout_s: float = 60) -> None:
        deadline = time.monotonic() + timeout_s
        while True:
            try:
                with urllib.request.urlopen(f"http://{host}/v2/", timeout=5):  # nosec B310
                    return
            except OSError:
                if time.monotonic() > deadline:
                    raise TrialError(f"local registry on {host} did not answer") from None
                time.sleep(1)

    @contextmanager
    def builder_instance(self) -> Iterator[str]:
        """A docker-container buildx builder in the host network, so it reaches 127.0.0.1."""
        self.tools(
            [
                self.tools.docker,
                "buildx",
                "create",
                "--name",
                self.builder,
                "--driver",
                "docker-container",
                "--driver-opt",
                "network=host",
            ]
        )
        try:
            yield self.builder
        finally:
            self.tools([self.tools.docker, "buildx", "rm", "--force", self.builder])

    def _push(self, dockerfile: str, name: str, platforms: Sequence[str]) -> str:
        """Build an image and push it by digest only; return the digest."""
        if not self.registry.startswith("127.0.0.1:"):
            raise TrialError("the trial pushes only to the local registry")
        meta = self.workdir / f"{name}-{len(list(self.workdir.glob(name + '-*.json')))}.json"
        revision = self.tools(["git", "-C", str(self.repo), "rev-parse", "HEAD"]).stdout.strip()
        self.tools(
            [
                self.tools.docker,
                "buildx",
                "build",
                "--builder",
                self.builder,
                "--file",
                dockerfile,
                "--platform",
                ",".join(platforms),
                "--build-arg",
                f"VERSION={self.version}",
                "--build-arg",
                f"REVISION={revision}",
                "--build-arg",
                f"SOURCE_URL={SOURCE_URL}",
                "--metadata-file",
                str(meta),
                "--output",
                f"type=image,name={self.registry}/{name},push-by-digest=true,"
                "name-canonical=true,push=true,registry.insecure=true",
                ".",
            ],
            cwd=self.repo,
        )
        digest = json.loads(meta.read_text(encoding="utf-8")).get("containerimage.digest", "")
        if not chain._DIGEST.match(digest):
            raise TrialError(f"{name}: buildx reported no digest")
        return str(digest)

    def build(self) -> list[chain.Image]:
        """Release-build the binaries, push both images by digest, join the postgres index."""
        self.tools(
            [
                "make",
                "-C",
                str(self.repo / "services/server"),
                "release-build",
                f"VERSION={self.version}",
                f"OUT={self.dist}",
            ]
        )
        img = ServerImages()
        server = self._push(img.server_dockerfile, "bothy-server", self.platforms)
        per_platform = [
            self._push(img.postgres_dockerfile, "bothy-postgres", [p]) for p in self.platforms
        ]
        postgres = chain.join(self.tools, f"{self.registry}/bothy-postgres", per_platform)
        return [
            chain.Image("bothy-server", f"{self.registry}/bothy-server", server),
            chain.Image("bothy-postgres", f"{self.registry}/bothy-postgres", postgres),
        ]

    def binaries(self) -> list[Path]:
        """The binaries of release-build in the dist directory, one per platform.

        Raises:
            TrialError: release-build wrote no binary for a platform.
        """
        out = [self.dist / f"bothy_{self.version}_{p.replace('/', '_')}" for p in self.platforms]
        missing = [b.name for b in out if not b.is_file()]
        if missing:
            raise TrialError(f"release-build wrote no {', '.join(missing)}")
        return out

    def sign_and_verify(self, images: Sequence[chain.Image]) -> Path:
        """Generate the throwaway key pair, then SBOM, sign and verify; return the public key."""
        pub = trial_keys(self.tools, self.workdir)
        chain.make_sboms(
            self.tools, self.dist, self.version, images, self.binaries(), self.platforms
        )
        chain.sign(
            self.tools, "private", self.dist, self.version, images, self.binaries(), self.platforms
        )
        chain.verify(self.tools, "private", self.dist, images, key=pub)
        return pub

    def tag_and_check(self, images: Sequence[chain.Image]) -> dict[str, str]:
        """Set the version tags and check the Compose images against them."""
        tags = ServerImages.tags(self.version)
        refs = {img.name: chain.tag(self.tools, img, tags[img.name]) for img in images}
        wanted = compose_images(self.repo / "infra/compose/compose.yaml", self.version)
        signed = {img.name: img.digest for img in images}
        for name, version_tag in sorted(wanted.items()):
            if name not in signed:
                raise TrialError(f"compose.yaml names {name}, which the release does not build")
            got = manifest_digest(self.registry, name, version_tag)
            if got != signed[name]:
                raise TrialError(
                    f"{name}:{version_tag} is {got}, the signed digest is {signed[name]}"
                )
        return refs

    def product_repo(self) -> Path:
        """A throwaway repository with the bundle paths of HEAD and the server version pinned.

        The committed template carries the placeholder until the first product release; the
        trial pins the server version it has just released, as `make release-prep
        COMPONENT=bothy SERVER_VERSION=<version>` would, and commits it there.
        """
        repo = self.workdir / "product-repo"
        repo.mkdir()
        paths = [
            *product_release.BUNDLE_PATHS,
            product_release.INSTALL_SOURCE,
            product_release.PLUGIN_VERSION_FILE,
        ]
        try:
            raw = subprocess.run(
                ["git", "-C", str(self.repo), "archive", "--format=tar", "HEAD", *paths],
                check=True,
                capture_output=True,
            ).stdout
        except subprocess.CalledProcessError as exc:
            raise TrialError(f"git archive of the bundle paths failed: {exc.stderr!r}") from exc
        with tarfile.open(fileobj=io.BytesIO(raw)) as archive:
            archive.extractall(repo, filter="data")
        env_file = repo / product_release.ENV_TEMPLATE
        text = env_file.read_text(encoding="utf-8")
        env_file.write_text(
            re.sub(r"^BOTHY_VERSION=.*$", f"BOTHY_VERSION={self.version}", text, flags=re.M),
            encoding="utf-8",
        )
        ident = ["-c", "user.name=Release trial", "-c", "user.email=trial@example.com"]
        for args in (["init", "-q"], ["add", "-A"], ["commit", "-q", "-m", "trial: pin server"]):
            self.tools(["git", "-C", str(repo), *ident, *args])
        return repo

    def plugin_zip(self, product_repo: Path) -> tuple[str, Path]:
        """Build the plugin's release ZIP of the version the product repository pins.

        The product bundle carries it (brief M7-26); `product_release.build` checks it again.

        Returns:
            The plugin version and the path of the ZIP.
        """
        version = (
            (product_repo / product_release.PLUGIN_VERSION_FILE).read_text(encoding="utf-8").strip()
        )
        return version, build_plugin_zip(self.tools, self.repo, version, self.workdir / "plugin")

    @contextmanager
    def static_server(self, root: Path) -> Iterator[str]:
        """Serve `root` over HTTP on 127.0.0.1 with a random port; yield the base address."""
        handler = functools.partial(_QuietHandler, directory=str(root))
        server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), handler)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            yield f"http://127.0.0.1:{server.server_address[1]}/"
        finally:
            server.shutdown()
            server.server_close()

    def product(self, images: Sequence[chain.Image]) -> list[Path]:
        """Build bundle and install.sh against the local registry and check them as installed."""
        repo = self.product_repo()
        out = self.workdir / "product"
        out.mkdir()
        product_version = (repo / "VERSION").read_text(encoding="utf-8").strip()
        plugin_version, plugin_zip = self.plugin_zip(repo)
        with self.static_server(out) as base_url:
            env = {
                **os.environ,
                "RELEASE_TRIAL_REGISTRY": self.registry,
                "RELEASE_TRIAL_BASE_URL": base_url,
            }
            env.pop("GITHUB_ACTIONS", None)
            try:
                written = product_release.build(
                    repo, product_version, out, env, docker=self.tools.docker, plugin_zip=plugin_zip
                )
            except product_release.ProductReleaseError as exc:
                raise TrialError(f"release-build of bothy: {exc}") from exc
            self.check_product(base_url, product_version, images, plugin_version)
        return written

    def check_product(
        self,
        base_url: str,
        product_version: str,
        images: Sequence[chain.Image],
        plugin_version: str | None = None,
    ) -> None:
        """Fetch install.sh and the bundle like an installation and check what they point at.

        Raises:
            TrialError: checksum, registry, server version or an image does not match.
        """
        script = _fetch(base_url + "install.sh").decode("utf-8")
        bundle = _fetch(base_url + product_release.bundle_name(product_version))
        built_in = dict(re.findall(r"^(BOTHY_INSTALL_[A-Z0-9_]+)='([^']*)'$", script, re.M))
        digest = hashlib.sha256(bundle).hexdigest()
        if built_in.get("BOTHY_INSTALL_BUNDLE_SHA256") != digest:
            raise TrialError("install.sh names another SHA-256 than the bundle it downloads")
        # plugin_version None: a caller that checks a bundle without the plugin build step.
        if built_in.get("BOTHY_INSTALL_BASE_URL") != base_url:
            raise TrialError(f"install.sh downloads from {built_in.get('BOTHY_INSTALL_BASE_URL')}")
        prefix = f"bothy-{product_version}/infra/compose/"
        with tarfile.open(fileobj=io.BytesIO(bundle)) as archive:
            env_text = _member(archive, prefix + ".env.example")
            compose_text = _member(archive, prefix + "compose.yaml")
            manifest = f"bothy-{product_version}/{product_release.PLUGIN_DIR}/plugin.json"
            plugin_json = _member(archive, manifest)
        if plugin_version is not None:
            try:
                plugin_release.check_manifest(plugin_json.encode("utf-8"), plugin_version, manifest)
            except plugin_release.PluginReleaseError as exc:
                raise TrialError(f"bundle: {exc}") from exc
        values = dict(re.findall(r"^(BOTHY_REGISTRY|BOTHY_VERSION)=(.*)$", env_text, re.M))
        if values.get("BOTHY_REGISTRY") != self.registry:
            raise TrialError(f"bundle pins the registry {values.get('BOTHY_REGISTRY')}")
        if values.get("BOTHY_VERSION") != self.version:
            raise TrialError(f"bundle pins the server version {values.get('BOTHY_VERSION')}")
        signed = {img.name: img.digest for img in images}
        for ref in product_release.compose_images(compose_text, self.version):
            name, tag = ref.split(":", 1)
            if manifest_digest(self.registry, name, tag) != signed.get(name):
                raise TrialError(f"{name}:{tag} of the bundle is not the signed digest")
            pulled = f"{self.registry}/{ref}"
            self.tools([self.tools.docker, "pull", "--quiet", pulled])
            self.tools([self.tools.docker, "image", "rm", pulled])
        print(f"  product: install.sh and bundle from {base_url}, images from {self.registry}")

    def probes(self, images: Sequence[chain.Image], pub: Path) -> None:
        """Negative probes: each must fail, otherwise the verification proves nothing."""
        probe = self.workdir / "probe"
        shutil.copytree(self.dist, probe)
        target = probe / self.binaries()[0].name
        with target.open("ab") as fh:
            fh.write(b"\0")
        self._must_fail(
            "changed file against SHA256SUMS",
            lambda: self.tools(["sha256sum", "--check", "--strict", chain.CHECKSUMS], cwd=probe),
        )
        raw = self.tools(
            [self.tools.docker, "buildx", "imagetools", "inspect", "--raw", images[0].ref]
        ).stdout
        child = json.loads(raw)["manifests"][0]["digest"]
        unsigned = chain.Image(images[0].name, images[0].repo, child)
        self._must_fail(
            "unsigned digest against cosign verify",
            lambda: chain.verify(self.tools, "private", self.dist, [unsigned], key=pub),
        )
        nosbom = self.workdir / "nosbom"
        shutil.copytree(self.dist, nosbom)
        next(nosbom.glob("*.spdx.json")).unlink()
        self._must_fail(
            "release without one SBOM against sign",
            lambda: chain.require_sboms(
                nosbom, chain.sbom_names(self.version, images, self.binaries(), self.platforms)
            ),
        )

    @staticmethod
    def _must_fail(what: str, step: object) -> None:
        try:
            step()  # type: ignore[operator]
        except chain.ChainError:
            print(f"  probe ok: {what} fails")
            return
        raise TrialError(f"probe passed although it must fail: {what}")


def trial_keys(tools: chain.Tools, workdir: Path) -> Path:
    """Generate the throwaway cosign key pair of this run and hand it to cosign.

    The private key and its random password go into the tool environment (as the secrets
    COSIGN_PRIVATE_KEY and COSIGN_PASSWORD do in release.yml); returns the public key.
    """
    keys = workdir / "keys"
    keys.mkdir()
    password = secrets.token_urlsafe(24)
    tools([tools.cosign, "generate-key-pair"], env={"COSIGN_PASSWORD": password}, cwd=keys)
    tools.env.update(
        {
            "COSIGN_PASSWORD": password,
            "COSIGN_PRIVATE_KEY": (keys / "cosign.key").read_text(encoding="utf-8"),
        }
    )
    return keys / "cosign.pub"


def build_plugin_zip(tools: chain.Tools, repo: Path, version: str, out: Path) -> Path:
    """Run the plugin's `release-build` (as release.yml does) and return the written ZIP.

    Raises:
        TrialError: release-build wrote no `bothy-live-panel-<version>.zip` into `out`.
    """
    tools(
        [
            "make",
            "-C",
            str(repo / PLUGIN_COMPONENT_DIR),
            "release-build",
            f"VERSION={version}",
            f"OUT={out}",
        ]
    )
    zipped = out / plugin_release.zip_name(version)
    if not zipped.is_file():
        raise TrialError(f"release-build wrote no {zipped.name}")
    return zipped


class PluginTrial:
    """The trial of the component grafana-bothy-live: ZIP, SBOMs, signatures, verification.

    Example:
        `PluginTrial(tools, Path(tempfile.mkdtemp())).run()` returns the release files.
    """

    def __init__(self, tools: chain.Tools, workdir: Path, repo: Path = REPO_ROOT) -> None:
        """Prepare a trial over tools in workdir for the plugin of the repository repo."""
        self.tools = tools
        self.workdir = workdir
        self.repo = repo
        self.dist = workdir / "dist"
        self.version = (repo / PLUGIN_COMPONENT_DIR / "VERSION").read_text(encoding="utf-8").strip()
        self.zip = self.dist / plugin_release.zip_name(self.version)
        self.pub: Path | None = None

    def build(self) -> Path:
        """Write the release ZIP with the plugin's `release-build`; return its path."""
        return build_plugin_zip(self.tools, self.repo, self.version, self.dist)

    def sign(self) -> None:
        """Sign the ZIP and SHA256SUMS; `chain.sign` aborts first when an SBOM is missing."""
        if self.pub is None:
            self.pub = trial_keys(self.tools, self.workdir)
        chain.sign(self.tools, "private", self.dist, self.version, [], [self.zip], [])

    def run(self) -> list[Path]:
        """Build, SBOM, sign and verify like release.yml (job plugin-signing); return the files.

        Raises:
            TrialError: the ZIP is missing or is not the plugin of this version.
            chain.ChainError: an SBOM, a signature or a checksum does not verify.
        """
        self.build()
        try:
            plugin_release.read_zip(self.zip.read_bytes(), self.version)
        except plugin_release.PluginReleaseError as exc:
            raise TrialError(f"{self.zip.name}: {exc}") from exc
        chain.make_sboms(self.tools, self.dist, self.version, [], [self.zip], [])
        self.sign()
        chain.verify(self.tools, "private", self.dist, [], key=self.pub)
        return sorted(p for p in self.dist.iterdir() if p.is_file())

    def probes(self) -> None:
        """Negative probes: each must fail, otherwise the verification proves nothing."""
        changed = self.workdir / "probe"
        shutil.copytree(self.dist, changed)
        with (changed / self.zip.name).open("ab") as fh:
            fh.write(b"\0")
        Trial._must_fail(
            "changed ZIP against SHA256SUMS",
            lambda: self.tools(["sha256sum", "--check", "--strict", chain.CHECKSUMS], cwd=changed),
        )
        nosbom = self.workdir / "nosbom"
        shutil.copytree(self.dist, nosbom)
        next(nosbom.glob("*.spdx.json")).unlink()
        Trial._must_fail(
            "release without its SBOM against sign",
            lambda: chain.sign(
                self.tools, "private", nosbom, self.version, [], [nosbom / self.zip.name], []
            ),
        )


class _QuietHandler(http.server.SimpleHTTPRequestHandler):
    """Static file handler of the trial download address without a request log on stderr."""

    def log_message(self, format: str, *args: object) -> None:
        return


def _fetch(url: str) -> bytes:
    """Download a file of the trial download address (127.0.0.1 only)."""
    if not url.startswith("http://127.0.0.1:"):
        raise TrialError(f"refusing a download outside 127.0.0.1: {url}")
    with urllib.request.urlopen(url, timeout=30) as resp:  # noqa: S310  # nosec B310
        data: bytes = resp.read()
    return data


def _member(archive: tarfile.TarFile, name: str) -> str:
    """Return the text of one member of the bundle; raise TrialError if it is missing."""
    try:
        handle = archive.extractfile(name)
    except KeyError:
        handle = None
    if handle is None:
        raise TrialError(f"bundle has no {name}")
    return handle.read().decode("utf-8")


def run_trial(
    tools: chain.Tools, platforms: Sequence[str], keep: bool = False, component: str = "server"
) -> int:
    """Run the trial of a component and print what an operator would check."""
    workdir = Path(tempfile.mkdtemp(prefix="bothy-release-trial-"))
    docker_config = workdir / "docker-config"
    docker_config.mkdir()
    (docker_config / "config.json").write_text("{}\n", encoding="utf-8")
    # cosign and syft read registry credentials from DOCKER_CONFIG; an empty one keeps every
    # credential of this host out of the trial.
    tools.env["DOCKER_CONFIG"] = str(docker_config)
    try:
        if component == "grafana-bothy-live":
            plugin = PluginTrial(tools, workdir)
            print(f"release-trial: grafana-bothy-live {plugin.version}")
            written = plugin.run()
            plugin.probes()
            print("release-trial: passed")
            sums = (plugin.dist / chain.CHECKSUMS).read_text(encoding="utf-8")
            print(
                f"  {chain.CHECKSUMS}:\n" + "".join(f"    {line}\n" for line in sums.splitlines())
            )
            for path in written:
                print(f"  plugin: {path.name}")
            return 0
        trial = Trial(tools, workdir, platforms)
        with ExitStack() as stack:
            registry = stack.enter_context(trial.local_registry())
            stack.enter_context(trial.builder_instance())
            names = ", ".join(platforms)
            print(f"release-trial: server {trial.version} on {names}, registry {registry}")
            images = trial.build()
            pub = trial.sign_and_verify(images)
            refs = trial.tag_and_check(images)
            trial.probes(images, pub)
            product = trial.product(images) if component == "bothy" else []
            print("release-trial: passed")
            for img in images:
                print(f"  {img.name}: {img.ref}  tag {refs[img.name]}")
            sums = (trial.dist / chain.CHECKSUMS).read_text(encoding="utf-8")
            print(
                f"  {chain.CHECKSUMS}:\n" + "".join(f"    {line}\n" for line in sums.splitlines())
            )
            for path in product:
                print(f"  product: {path.name}")
    finally:
        if keep:
            print(f"  kept {workdir}")
        else:
            shutil.rmtree(workdir, ignore_errors=True)
    return 0


def main(argv: list[str] | None = None, tools: chain.Tools | None = None) -> int:
    """Command line entry point; see the module documentation."""
    parser = argparse.ArgumentParser(description="Trial release against local targets.")
    parser.add_argument("--component", required=True)
    parser.add_argument("--platforms", default=",".join(SUPPORTED_PLATFORMS))
    parser.add_argument("--cosign")
    parser.add_argument("--syft")
    parser.add_argument("--keep", action="store_true", help="keep the temporary directory")
    args = parser.parse_args(argv)
    try:
        refuse_in_ci(os.environ)
        if args.component not in COMPONENTS:
            raise TrialError(
                f"component '{args.component}' has no trial yet (known: {', '.join(COMPONENTS)})"
            )
        platforms = parse_platforms(args.platforms)
    except TrialError as exc:
        print(f"release-trial: {exc}", file=sys.stderr)
        return 2
    try:
        if tools is None:
            tools = chain.Tools(
                cosign=args.cosign or chain.resolve_tool("cosign"),
                syft=args.syft or chain.resolve_tool("syft"),
            )
        return run_trial(tools, platforms, args.keep, args.component)
    except (TrialError, chain.ChainError) as exc:
        print(f"release-trial: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    print(f"release-trial: host {platform.system()}/{platform.machine()}")
    sys.exit(main())
