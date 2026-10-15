# SPDX-License-Identifier: Apache-2.0
"""Trial release on the build host: `make release-trial COMPONENT=server` (D-W-127).

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

It never pushes to another registry, never tags the repository, never creates a release, and it
refuses to run when GITHUB_ACTIONS is set, so it never runs with release credentials. The key
pair, the registry, the buildx builder and the temporary directory are removed at the end,
also after a failure. Vulnerability scan and ScanCode are not part of the trial; they are
separate targets (`make trivy-image`, `make scancode`) that release.yml runs.

Usage:
  release_trial.py --component server [--platforms linux/amd64,linux/arm64]
                   [--cosign <path>] [--syft <path>] [--keep]
Exit codes: 0 trial passed, 1 a step failed, 2 refused (GitHub Actions) or bad arguments.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import platform
import re
import secrets
import shutil
import sys
import tempfile
import time
import urllib.error
import urllib.request
from collections.abc import Iterator, Sequence
from contextlib import ExitStack, contextmanager
from dataclasses import dataclass
from pathlib import Path

import release_chain as chain
import yaml

REPO_ROOT = Path(__file__).resolve().parents[2]
# renovate: datasource=docker depName=registry
REGISTRY_IMAGE = (
    "registry:3.1.2@sha256:ddf754342cfc8acc51a56d5d0ab6af06826461864460636d8bd5c546dab2a7b8"
)
COMPONENTS = ("server",)
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
        keys = self.workdir / "keys"
        keys.mkdir()
        password = secrets.token_urlsafe(24)
        self.tools(
            [self.tools.cosign, "generate-key-pair"], env={"COSIGN_PASSWORD": password}, cwd=keys
        )
        self.tools.env.update(
            {
                "COSIGN_PASSWORD": password,
                "COSIGN_PRIVATE_KEY": (keys / "cosign.key").read_text(encoding="utf-8"),
            }
        )
        chain.make_sboms(
            self.tools, self.dist, self.version, images, self.binaries(), self.platforms
        )
        chain.sign(
            self.tools, "private", self.dist, self.version, images, self.binaries(), self.platforms
        )
        chain.verify(self.tools, "private", self.dist, images, key=keys / "cosign.pub")
        return keys / "cosign.pub"

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


def run_trial(tools: chain.Tools, platforms: Sequence[str], keep: bool = False) -> int:
    """Run the trial of the server component and print what an operator would check."""
    workdir = Path(tempfile.mkdtemp(prefix="bothy-release-trial-"))
    docker_config = workdir / "docker-config"
    docker_config.mkdir()
    (docker_config / "config.json").write_text("{}\n", encoding="utf-8")
    # cosign and syft read registry credentials from DOCKER_CONFIG; an empty one keeps every
    # credential of this host out of the trial.
    tools.env["DOCKER_CONFIG"] = str(docker_config)
    try:
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
            print("release-trial: passed")
            for img in images:
                print(f"  {img.name}: {img.ref}  tag {refs[img.name]}")
            sums = (trial.dist / chain.CHECKSUMS).read_text(encoding="utf-8")
            print(
                f"  {chain.CHECKSUMS}:\n" + "".join(f"    {line}\n" for line in sums.splitlines())
            )
    finally:
        if keep:
            print(f"  kept {workdir}")
        else:
            shutil.rmtree(workdir, ignore_errors=True)
    return 0


def main(argv: list[str] | None = None, tools: chain.Tools | None = None) -> int:
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
        return run_trial(tools, platforms, args.keep)
    except (TrialError, chain.ChainError) as exc:
        print(f"release-trial: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    print(f"release-trial: host {platform.system()}/{platform.machine()}")
    sys.exit(main())
