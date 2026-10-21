# SPDX-License-Identifier: Apache-2.0
"""Supply-chain steps of a component release: join, SBOM, sign, verify, tag.

release.yml runs these commands for the server after its images are pushed by digest, and
`make release-trial` (tools/ci/release_trial.py) runs the same functions against a local
registry, so the trial proves the code the release runs (docs/spec/18-release.md sections 6
and 7, ADR 0036, ADR 0062, docs/DECISIONS.md D-W-127). The order is fixed by the callers:
join, vulnerability scan, `sbom`, `sign` (aborts before signing anything if an SBOM is
missing), `verify`, and only then `tag` with the version.

Signing modes (ADR 0062):

* `private`: a cosign key pair, the private key in COSIGN_PRIVATE_KEY and its password in
  COSIGN_PASSWORD (secrets of the environment `release`), and a signing configuration without
  Rekor, Fulcio, timestamp authority and OIDC provider; every bundle is checked to carry no
  transparency log entry. Verification: `cosign verify --key <pub> --insecure-ignore-tlog`.
* `public`: keyless with the workflow identity of GitHub Actions (OIDC token of the job).

Usage:
  release_chain.py join --repo <registry/name> --digest <sha256:...> [--digest ...]
  release_chain.py sbom --dist <dir> --version <v> [--image <name>=<ref@digest>]...
                        [--binary <file>]... [--platforms linux/amd64,linux/arm64]
  release_chain.py sign --mode private|public --dist <dir> --version <v>
                        [--image <name>=<ref@digest>]... [--binary <file>]... [--platforms ...]
  release_chain.py verify --mode private|public --dist <dir> [--key <pub>]
                        [--identity-regexp <re> --issuer <url>] [--image <name>=<ref@digest>]...
  release_chain.py tag --image <name>=<ref@digest> --tag <tag>
Tools: --cosign and --syft name the pinned binaries; without them tools/bin/fetch_tools.py
resolves them. Exit codes: 0 done, 1 a step failed (named on stderr).
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import subprocess
import sys
import tempfile
from collections.abc import Callable, Sequence
from dataclasses import dataclass, field
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[2]
DEFAULT_PLATFORMS = ("linux/amd64", "linux/arm64")
MODES = ("private", "public")
SBOM_FORMATS = {"spdx": "spdx-json", "cdx": "cyclonedx-json"}
CHECKSUMS = "SHA256SUMS"
BUNDLE_SUFFIX = ".sigstore.json"
IMAGES_FILE = "images.txt"
_DIGEST = re.compile(r"^sha256:[0-9a-f]{64}$")
_SERVICE_KEYS = ("caUrls", "oidcUrls", "rekorTlogUrls", "tsaUrls")

Runner = Callable[..., subprocess.CompletedProcess[str]]


class ChainError(Exception):
    """A supply-chain step failed; the release stops before anything is tagged."""


def run(
    cmd: Sequence[str], *, env: dict[str, str] | None = None, cwd: Path | None = None
) -> subprocess.CompletedProcess[str]:
    """Run a command and return its result; raise ChainError when it fails.

    Args:
        cmd: program and arguments.
        env: extra environment variables on top of the current environment.
        cwd: working directory.

    Returns:
        The completed process with captured text output.

    Raises:
        ChainError: the command exited with a non-zero status (its stderr is in the message).
    """
    proc = subprocess.run(
        list(cmd),
        env={**os.environ, **(env or {})},
        cwd=cwd,
        capture_output=True,
        text=True,
        check=False,
    )
    if proc.returncode != 0:
        detail = (proc.stderr.strip() or proc.stdout.strip()).splitlines()[-3:]
        raise ChainError(f"{Path(cmd[0]).name} {cmd[1] if len(cmd) > 1 else ''} failed: {detail}")
    return proc


@dataclass
class Tools:
    """The external programs of the chain and how to run them.

    Attributes:
        cosign: path of the pinned cosign.
        syft: path of the pinned syft.
        docker: the docker CLI (with buildx).
        runner: runs one command (tests pass a fake).
        env: extra environment for cosign and syft only (the trial gives them an empty
            DOCKER_CONFIG, which the docker CLI must not get: it holds its plugins there).
    """

    cosign: str
    syft: str
    docker: str = "docker"
    runner: Runner = run
    env: dict[str, str] = field(default_factory=dict)

    def __call__(
        self, cmd: Sequence[str], *, env: dict[str, str] | None = None, cwd: Path | None = None
    ) -> subprocess.CompletedProcess[str]:
        """Run cmd; cosign and syft get the environment of this tool set on top of env."""
        own = self.env if cmd and cmd[0] in (self.cosign, self.syft) else {}
        return self.runner(cmd, env={**own, **(env or {})}, cwd=cwd)


@dataclass(frozen=True)
class Image:
    """A release image: its name (`bothy-server`) and its reference by digest."""

    name: str
    repo: str
    digest: str

    @property
    def ref(self) -> str:
        """`<repo>@<digest>`."""
        return f"{self.repo}@{self.digest}"

    @classmethod
    def parse(cls, text: str) -> Image:
        """Parse `<name>=<repo>@sha256:<hex>`.

        Raises:
            ChainError: the text has another form.
        """
        name, sep, ref = text.partition("=")
        repo, _, digest = ref.rpartition("@")
        if not sep or not name or not repo or not _DIGEST.match(digest):
            raise ChainError(f"image '{text}' is not <name>=<repo>@sha256:<64 hex>")
        return cls(name, repo, digest)


def join(tools: Tools, repo: str, digests: Sequence[str]) -> str:
    """Join per-platform images, pushed by digest, into one multi-arch index without a tag.

    `docker buildx imagetools create` needs a target reference; a tag would publish a name
    before the scan, so the target is the digest of the index itself, computed from the
    dry run (its output plus a final newline), and checked against what the registry stores.

    Args:
        tools: the programs.
        repo: registry and name, for example `ghcr.io/owner/bothy-postgres`.
        digests: the per-platform digests.

    Returns:
        The digest of the index.

    Raises:
        ChainError: a digest is malformed, a command failed or the registry holds another index.
    """
    if not digests or any(not _DIGEST.match(d) for d in digests):
        raise ChainError(f"join needs digests sha256:<64 hex>, got {list(digests)}")
    sources = [f"{repo}@{d}" for d in digests]
    dry = tools([tools.docker, "buildx", "imagetools", "create", "--dry-run", *sources]).stdout
    index = dry[:-1] if dry.endswith("\n") else dry
    digest = "sha256:" + hashlib.sha256(index.encode()).hexdigest()
    tools([tools.docker, "buildx", "imagetools", "create", "--tag", f"{repo}@{digest}", *sources])
    raw = tools([tools.docker, "buildx", "imagetools", "inspect", "--raw", f"{repo}@{digest}"])
    stored = raw.stdout[:-1] if raw.stdout.endswith("\n") else raw.stdout
    if "sha256:" + hashlib.sha256(stored.encode()).hexdigest() != digest:
        raise ChainError(f"{repo}: the registry holds another index than the dry run printed")
    return digest


def sbom_names(
    version: str, images: Sequence[Image], binaries: Sequence[Path], platforms: Sequence[str]
) -> list[str]:
    """Return the SBOM file names a release must contain.

    SPDX and CycloneDX per image and platform (`bothy-server_<v>_linux_amd64.spdx.json`) and
    per binary (`<binary>.cdx.json`).
    """
    stems = [f"{img.name}_{version}_{p.replace('/', '_')}" for img in images for p in platforms] + [
        b.name for b in binaries
    ]
    return [f"{stem}.{fmt}.json" for stem in stems for fmt in SBOM_FORMATS]


def make_sboms(
    tools: Tools,
    dist: Path,
    version: str,
    images: Sequence[Image],
    binaries: Sequence[Path],
    platforms: Sequence[str],
) -> list[Path]:
    """Write SPDX and CycloneDX SBOMs with syft for every image platform and every binary.

    Returns:
        The written files, in the order of `sbom_names`.
    """
    dist.mkdir(parents=True, exist_ok=True)
    written: list[Path] = []
    targets = [
        (f"registry:{img.ref}", ["--platform", p], f"{img.name}_{version}_{p.replace('/', '_')}")
        for img in images
        for p in platforms
    ] + [(f"file:{b}", [], b.name) for b in binaries]
    for source, extra, stem in targets:
        outs = [dist / f"{stem}.{fmt}.json" for fmt in SBOM_FORMATS]
        args = [f"{kind}={out}" for kind, out in zip(SBOM_FORMATS.values(), outs, strict=True)]
        tools(
            [tools.syft, "scan", source, *extra, "--quiet", *[a for o in args for a in ("-o", o)]]
        )
        written += outs
    return written


def require_sboms(dist: Path, names: Sequence[str]) -> None:
    """Abort unless every named SBOM exists in `dist` as a non-empty JSON document.

    Raises:
        ChainError: an SBOM is missing, empty or not JSON (nothing is signed then).
    """
    missing = []
    for name in names:
        path = dist / name
        try:
            ok = path.stat().st_size > 0 and isinstance(json.loads(path.read_text()), dict)
        except (OSError, ValueError):
            ok = False
        if not ok:
            missing.append(name)
    if missing:
        raise ChainError(f"SBOM missing or invalid, release stopped: {', '.join(missing)}")


def private_signing_config(tools: Tools, path: Path) -> Path:
    """Write the signing configuration of the private phase and check it names no service.

    Raises:
        ChainError: cosign failed or the configuration names a Rekor, Fulcio, TSA or OIDC URL.
    """
    tools(
        [
            tools.cosign,
            "signing-config",
            "create",
            "--no-default-rekor",
            "--no-default-fulcio",
            "--no-default-tsa",
            "--no-default-oidc",
            "--out",
            str(path),
        ]
    )
    config = json.loads(path.read_text(encoding="utf-8"))
    named = [k for k in _SERVICE_KEYS if config.get(k)]
    if named:
        raise ChainError(f"signing configuration names services: {named}")
    return path


def check_no_tlog(bundle: Path) -> None:
    """Raise ChainError if a Sigstore bundle carries a transparency log entry or a timestamp."""
    material = json.loads(bundle.read_text(encoding="utf-8")).get("verificationMaterial", {})
    if material.get("tlogEntries") or material.get("timestampVerificationData", {}).get(
        "rfc3161Timestamps"
    ):
        raise ChainError(f"{bundle.name} has a transparency log entry or timestamp")


def checksums(dist: Path) -> Path:
    """Write SHA256SUMS over every file of `dist` except signature bundles and itself.

    The format is that of `sha256sum` (`<hex>  <name>`), sorted by name.
    """
    lines = []
    for path in sorted(p for p in dist.iterdir() if p.is_file()):
        if path.name == CHECKSUMS or path.name.endswith(BUNDLE_SUFFIX):
            continue
        lines.append(f"{hashlib.sha256(path.read_bytes()).hexdigest()}  {path.name}\n")
    out = dist / CHECKSUMS
    out.write_text("".join(lines), encoding="utf-8")
    return out


def _key_args(config: Path | None) -> list[str]:
    """Key and signing configuration of the private phase; nothing (keyless) without a config."""
    if config is None:
        return []
    return ["--key", "env://COSIGN_PRIVATE_KEY", "--signing-config", str(config)]


def sign(
    tools: Tools,
    mode: str,
    dist: Path,
    version: str,
    images: Sequence[Image],
    binaries: Sequence[Path],
    platforms: Sequence[str],
) -> None:
    """Sign every image digest, every binary and SHA256SUMS (after checking the SBOMs).

    Writes `images.txt` (`<name> <repo>@<digest>` per image; not without images), one
    `<image>.sigstore.json` per image, a `.sigstore.json` bundle next to every binary and
    SHA256SUMS. In the private phase the private key comes from COSIGN_PRIVATE_KEY and its
    password from COSIGN_PASSWORD.

    Raises:
        ChainError: an SBOM is missing, the mode is unknown, a key is missing, cosign failed or
            a bundle of the private phase has a transparency log entry.
    """
    if mode not in MODES:
        raise ChainError(f"mode must be private or public, not '{mode}'")
    require_sboms(dist, sbom_names(version, images, binaries, platforms))
    if mode == "private" and not (
        os.environ.get("COSIGN_PRIVATE_KEY") or tools.env.get("COSIGN_PRIVATE_KEY")
    ):
        raise ChainError("COSIGN_PRIVATE_KEY is not set (secret of the environment release)")
    with tempfile.TemporaryDirectory(prefix="bothy-signing-") as tmp:
        config = (
            private_signing_config(tools, Path(tmp) / "signing-config.json")
            if mode == "private"
            else None
        )
        keys = _key_args(config)
        for img in images:
            bundle = dist / f"{img.name}{BUNDLE_SUFFIX}"
            tools([tools.cosign, "sign", "--yes", *keys, "--bundle", str(bundle), img.ref])
        # The product release signs files only; an empty list would be a misleading asset.
        if images:
            (dist / IMAGES_FILE).write_text(
                "".join(f"{img.name} {img.ref}\n" for img in images), encoding="utf-8"
            )
        for blob in binaries:
            bundle = Path(f"{blob}{BUNDLE_SUFFIX}")
            tools([tools.cosign, "sign-blob", "--yes", *keys, "--bundle", str(bundle), str(blob)])
        sums = checksums(dist)
        bundle = Path(f"{sums}{BUNDLE_SUFFIX}")
        tools([tools.cosign, "sign-blob", "--yes", *keys, "--bundle", str(bundle), str(sums)])
    if mode == "private":
        for bundle in sorted(dist.glob(f"*{BUNDLE_SUFFIX}")):
            check_no_tlog(bundle)


def verify(
    tools: Tools,
    mode: str,
    dist: Path,
    images: Sequence[Image],
    key: Path | None = None,
    identity_regexp: str | None = None,
    issuer: str | None = None,
) -> None:
    """Verify like an instance operator: every image, every file bundle, then SHA256SUMS.

    Private phase: `cosign verify --key <pub> --insecure-ignore-tlog` and
    `cosign verify-blob --key <pub> --insecure-ignore-tlog --bundle ...`; public phase: the
    certificate identity and OIDC issuer of the release workflow. Last `sha256sum --check`.

    Raises:
        ChainError: an argument of the mode is missing or any verification fails.
    """
    if mode == "private":
        if key is None:
            raise ChainError("verify --mode private needs --key")
        if not key.is_file():
            raise ChainError(
                f"public key {key} is missing: the key pair of the private phase is an owner "
                "step (docs/operations/release.md section 2.4)"
            )
        args = ["--key", str(key), "--insecure-ignore-tlog"]
    elif mode == "public":
        if not identity_regexp or not issuer:
            raise ChainError("verify --mode public needs --identity-regexp and --issuer")
        args = [
            "--certificate-identity-regexp",
            identity_regexp,
            "--certificate-oidc-issuer",
            issuer,
        ]
    else:
        raise ChainError(f"mode must be private or public, not '{mode}'")
    for img in images:
        tools([tools.cosign, "verify", *args, img.ref])
    bundles = sorted(dist.glob(f"*{BUNDLE_SUFFIX}"))
    blob_bundles = [b for b in bundles if (dist / b.name.removesuffix(BUNDLE_SUFFIX)).is_file()]
    if not any(b.name == CHECKSUMS + BUNDLE_SUFFIX for b in blob_bundles):
        raise ChainError(f"{CHECKSUMS} or its signature bundle is missing")
    for bundle in blob_bundles:
        target = dist / bundle.name.removesuffix(BUNDLE_SUFFIX)
        tools([tools.cosign, "verify-blob", *args, "--bundle", str(bundle), str(target)])
    tools(["sha256sum", "--check", "--strict", CHECKSUMS], cwd=dist)


def tag(tools: Tools, image: Image, version_tag: str) -> str:
    """Set a version tag on an image digest (after scan, signature and verification).

    Raises:
        ChainError: the tag is `latest` or empty (18 section 5 rule 3) or docker failed.
    """
    if not version_tag or version_tag == "latest" or version_tag.endswith("-latest"):
        raise ChainError(f"tag '{version_tag}' is not a version tag")
    target = f"{image.repo}:{version_tag}"
    tools([tools.docker, "buildx", "imagetools", "create", "--tag", target, image.ref])
    return target


def resolve_tool(name: str) -> str:
    """Return the path of a pinned release binary through tools/bin/fetch_tools.py."""
    proc = run([sys.executable, str(REPO_ROOT / "tools" / "bin" / "fetch_tools.py"), "path", name])
    return proc.stdout.strip()


def _parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Supply-chain steps of a component release.")
    parser.add_argument("--cosign")
    parser.add_argument("--syft")
    sub = parser.add_subparsers(dest="cmd", required=True)
    p = sub.add_parser("join")
    p.add_argument("--repo", required=True)
    p.add_argument("--digest", action="append", required=True)
    for name in ("sbom", "sign"):
        p = sub.add_parser(name)
        p.add_argument("--dist", type=Path, required=True)
        p.add_argument("--version", required=True)
        p.add_argument("--image", action="append", default=[])
        p.add_argument("--binary", type=Path, action="append", default=[])
        p.add_argument("--platforms", default=",".join(DEFAULT_PLATFORMS))
        if name == "sign":
            p.add_argument("--mode", required=True)
    p = sub.add_parser("verify")
    p.add_argument("--mode", required=True)
    p.add_argument("--dist", type=Path, required=True)
    p.add_argument("--image", action="append", default=[])
    p.add_argument("--key", type=Path)
    p.add_argument("--identity-regexp")
    p.add_argument("--issuer")
    p = sub.add_parser("tag")
    p.add_argument("--image", required=True)
    p.add_argument("--tag", required=True)
    return parser


def main(argv: list[str] | None = None, tools: Tools | None = None) -> int:
    """Command line entry point; see the module documentation."""
    args = _parser().parse_args(argv)
    try:
        if tools is None:
            tools = Tools(
                cosign=args.cosign or resolve_tool("cosign"), syft=args.syft or resolve_tool("syft")
            )
        if args.cmd == "join":
            print(join(tools, args.repo, args.digest))
        elif args.cmd == "tag":
            print(tag(tools, Image.parse(args.image), args.tag))
        else:
            images = [Image.parse(i) for i in args.image]
            if args.cmd == "verify":
                verify(
                    tools, args.mode, args.dist, images, args.key, args.identity_regexp, args.issuer
                )
            else:
                platforms = [p for p in args.platforms.split(",") if p]
                if args.cmd == "sbom":
                    make_sboms(tools, args.dist, args.version, images, args.binary, platforms)
                else:
                    sign(tools, args.mode, args.dist, args.version, images, args.binary, platforms)
    except ChainError as exc:
        print(f"release-chain: {exc}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
