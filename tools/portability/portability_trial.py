# SPDX-License-Identifier: Apache-2.0
"""Local proof of the portability test: ``make portability-trial`` (brief M7-14 Design note 2).

No product tag exists yet and tags stay with the owner (D-W-125), so this proves the whole test
end to end against the trial release of the product (D-W-127, M7-29):

1. the trial release of the product (``make release-trial`` of the product) for the platform of
   this host: the pinned registry on 127.0.0.1, the server images signed with a throwaway key,
   then install.sh and the bundle of the product pointing at that registry, in a release folder;
2. ``make portability`` with ``PORTABILITY_TAG=v<product version>``,
   ``PORTABILITY_INSTALL_SOURCE=release-download``, ``PORTABILITY_RELEASE_DIR`` of the trial and the
   trial's public key in ``PORTABILITY_TRIAL_PUBLIC_KEY``: both storage runs must pass;
3. a forced failure right after the installation (``PORTABILITY_TRIAL_FAIL_AFTER_INSTALL=1``, one
   storage run): it must fail and print ``docker compose ps --all`` and the logs of its
   installation before the cleanup;
4. the tag of ``bothy-server`` moved to the platform manifest of the signed index (the same image,
   but its digest carries no signature): the run must fail at the verification, before install.sh
   runs.

The images are built with the default builder of this host (`HostBuildTrial`), so the trial
reuses its cache. Docker treats the registry on 127.0.0.1 as plain HTTP without an
``insecure-registries`` entry (the trial of M7-10 pulls from it the same way). The registry, the
pulled images and the temporary directory are removed at the end, also after a failure. The
trial refuses to run when ``GITHUB_ACTIONS`` is set.

Usage:
  trial.py --cosign <path> --syft <path> [--platforms linux/arm64] [--keep]
Exit codes: 0 every case behaved as required, 1 a case did not, 2 refused or bad arguments.
"""

from __future__ import annotations

import argparse
import json
import os
import platform
import re
import shutil
import signal
import subprocess
import sys
import tempfile
import threading
import time
import urllib.request
from collections.abc import Callable, Mapping, Sequence
from contextlib import ExitStack
from dataclasses import dataclass
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "tools" / "ci"))
import product_release  # noqa: E402 - the product release of the trial
import release_chain as chain  # noqa: E402
import release_trial  # noqa: E402

# Lines of the runner (tools/portability/portability_run.py) the cases look for.
INSTALL_LINE = "portability: install ("
DIAGNOSE_LINES = ("docker compose ps --all:", "log lines of every service:")
UNSIGNED_LINE = "carries no valid signature"
FAIL_LINE = "portability: FAIL: "


class CaseError(Exception):
    """A case of the trial did not behave as required."""


class HostBuildTrial(release_trial.Trial):
    """The server trial with the images built by the default builder of this host.

    The trial of M7-10 builds in a fresh ``docker-container`` builder, which pulls every base image
    into its own store; this one builds with the host's default builder and its cache, loads the
    image and pushes it with ``docker push`` to the trial registry, so the run needs a fraction of
    the disk and time (D-M7-14-2). Signing, verification, tags and the product are those of M7-10.
    The release chain itself is proven by ``make release-trial``, not here.
    """

    def _push(self, dockerfile: str, name: str, platforms: Sequence[str]) -> str:
        """Build one platform with the default builder, push it, return the digest of its index."""
        if not self.registry.startswith("127.0.0.1:") or len(platforms) != 1:
            raise release_trial.TrialError("the host build pushes one platform to 127.0.0.1 only")
        revision = self.tools(["git", "-C", str(self.repo), "rev-parse", "HEAD"]).stdout.strip()
        ref = f"{self.registry}/{name}:build-{platforms[0].replace('/', '-')}"
        self.tools(
            [
                self.tools.docker, "buildx", "build", "--builder", "default", "--file", dockerfile,
                "--platform", platforms[0], "--provenance=false", "--sbom=false",
                "--build-arg", f"VERSION={self.version}", "--build-arg", f"REVISION={revision}",
                "--build-arg", f"SOURCE_URL={release_trial.SOURCE_URL}", "--load", "--tag", ref,
                ".",
            ],
            cwd=self.repo,
        )  # fmt: skip
        out = self.tools([self.tools.docker, "push", ref]).stdout
        m = re.search(r"digest: (sha256:[0-9a-f]{64})", out)
        if m is None:
            raise release_trial.TrialError(f"docker push of {name} reported no digest")
        self.tools([self.tools.docker, "image", "rm", ref])
        # An index around the one manifest, as buildx pushes it in the release: the version tags
        # of tag_and_check copy an index unchanged, a bare manifest they would wrap anew.
        return str(chain.join(self.tools, f"{self.registry}/{name}", [m.group(1)]))


@dataclass(frozen=True)
class Outcome:
    """Exit code, stderr and duration of one ``make portability``."""

    code: int
    stderr: str
    seconds: float


Make = Callable[[Mapping[str, str]], Outcome]


def run_make(env: Mapping[str, str]) -> Outcome:
    """Run ``make portability`` with env; its stderr is shown and kept."""
    started = time.monotonic()
    proc = subprocess.Popen(  # nosec B603
        ["make", "--no-print-directory", "portability"],
        cwd=ROOT, env=dict(env), stderr=subprocess.PIPE, text=True,
    )  # fmt: skip
    if proc.stderr is None:  # never with stderr=PIPE
        raise CaseError("make portability gave no standard error")
    lines = []
    for line in proc.stderr:
        sys.stderr.write(line)
        lines.append(line)
    code = proc.wait()
    return Outcome(code, "".join(lines), time.monotonic() - started)


def interface_env(
    base: Mapping[str, str], version: str, release_dir: Path, key: Path
) -> dict[str, str]:
    """The environment of ``make portability`` against the trial release."""
    env = {k: v for k, v in base.items() if not k.startswith("PORTABILITY_")}
    env.pop("GITHUB_ACTIONS", None)
    env.update(
        {
            "PORTABILITY_TAG": f"v{version}",
            "PORTABILITY_INSTALL_SOURCE": "release-download",
            "PORTABILITY_RELEASE_DIR": str(release_dir),
            "PORTABILITY_TRIAL_PUBLIC_KEY": str(key),
        }
    )
    return env


def check_pass(out: Outcome) -> list[str]:
    """Both storage runs passed; return the report lines of the runner (digests, durations)."""
    if out.code != 0:
        raise CaseError(f"make portability failed with exit code {out.code}")
    report = [
        ln.removeprefix("portability:   ")
        for ln in out.stderr.splitlines()
        if ln.startswith("portability:   ")
    ]
    for storage in ("fs", "s3"):
        if not any(r.startswith(f"{storage}: passed") for r in report):
            raise CaseError(f"the run {storage} reported no pass")
    return report


def failed_in_a_step(out: Outcome) -> bool:
    """The runner ended with a failed step (its exit code 1), not with an interface error.

    make itself answers 2 for any failed recipe, so the runner's last line tells the cause.
    """
    return out.code != 0 and FAIL_LINE in out.stderr


def check_forced_failure(out: Outcome) -> None:
    """The forced failure failed after the install and printed the state before the cleanup."""
    if not failed_in_a_step(out):
        raise CaseError(f"the forced failure ended with exit code {out.code} without a failed step")
    if INSTALL_LINE not in out.stderr:
        raise CaseError("the forced failure did not reach install.sh")
    for line in DIAGNOSE_LINES:
        if line not in out.stderr:
            raise CaseError(f"the forced failure printed no '{line}'")
    if "cleanup left objects" in out.stderr:
        raise CaseError("the forced failure left objects of its project behind")


def check_unsigned(out: Outcome) -> None:
    """The unsigned image failed the verification and install.sh never ran."""
    if not failed_in_a_step(out):
        raise CaseError(f"the unsigned case ended with exit code {out.code} without a failed step")
    if UNSIGNED_LINE not in out.stderr:
        raise CaseError("the unsigned case did not fail at the signature verification")
    if INSTALL_LINE in out.stderr:
        raise CaseError("install.sh ran although an image is unsigned")


Fetch = Callable[[urllib.request.Request], tuple[bytes, str]]


def _fetch(req: urllib.request.Request) -> tuple[bytes, str]:
    """One request to the trial registry (127.0.0.1 only); return body and content type."""
    if not req.full_url.startswith("http://127.0.0.1:"):
        raise CaseError(f"refusing a request outside 127.0.0.1: {req.full_url}")
    with urllib.request.urlopen(req, timeout=60) as resp:  # noqa: S310  # nosec B310
        return resp.read(), resp.headers.get("Content-Type", "")


def unsigned_copy(
    registry: str, name: str, index_digest: str, tag: str, fetch: Fetch = _fetch
) -> str:
    """Point tag at the first platform manifest of the signed index; return its digest.

    The signatures sign the index digest, so the platform manifest is the same image without a
    signature. The manifest goes to the tag by the registry API, because ``imagetools create``
    would wrap it into the signed index again (a list of one manifest is the same document).
    """
    base = f"http://{registry}/v2/{name}/manifests/"
    accept = {"Accept": release_trial._MANIFEST_TYPES}
    index, _ = fetch(urllib.request.Request(base + index_digest, headers=accept))  # noqa: S310
    manifests = json.loads(index).get("manifests") or []
    if not manifests:
        raise CaseError(f"{name}@{index_digest} is no index with a platform manifest")
    child = str(manifests[0]["digest"])
    body, ctype = fetch(urllib.request.Request(base + child, headers=accept))  # noqa: S310
    put = urllib.request.Request(  # noqa: S310 - _fetch allows 127.0.0.1 only
        base + tag, data=body, method="PUT", headers={"Content-Type": ctype}
    )
    fetch(put)
    return child


def run_trial(
    tools: chain.Tools, platforms: Sequence[str], make: Make, keep: bool = False, hold: bool = False
) -> int:
    """Build the trial release and run the three cases; see the module documentation."""
    workdir = Path(tempfile.mkdtemp(prefix="bothy-portability-trial-"))
    docker_config = workdir / "docker-config"
    docker_config.mkdir()
    (docker_config / "config.json").write_text("{}\n", encoding="utf-8")
    tools.env["DOCKER_CONFIG"] = str(docker_config)
    pulled: list[str] = []
    try:
        trial = HostBuildTrial(tools, workdir / "trial", platforms)
        trial.workdir.mkdir()
        with ExitStack() as stack:
            registry = stack.enter_context(trial.local_registry())
            names = ", ".join(platforms)
            print(f"portability-trial: server {trial.version} on {names}, registry {registry}")
            images = trial.build()
            pub = trial.sign_and_verify(images)
            refs = trial.tag_and_check(images)
            pulled += list(refs.values())
            repo = trial.product_repo()
            release_dir = workdir / "release"
            release_dir.mkdir()
            version = (repo / "VERSION").read_text(encoding="utf-8").strip()
            base = stack.enter_context(trial.static_server(release_dir))
            env = {**os.environ, "RELEASE_TRIAL_REGISTRY": registry, "RELEASE_TRIAL_BASE_URL": base}
            env.pop("GITHUB_ACTIONS", None)
            product_release.build(repo, version, release_dir, env, docker=tools.docker)
            trial.check_product(base, version, images)
            print(f"portability-trial: product {version} in {release_dir}")
            iface = interface_env(os.environ, version, release_dir, pub)
            if hold:
                hold_release(iface)
                return 0

            server = next(img for img in images if img.name == "bothy-server")
            report = run_cases(tools, make, iface, registry, server, refs["bothy-server"])
    finally:
        for ref in pulled:
            subprocess.run(
                [tools.docker, "image", "rm", "--force", ref], capture_output=True, check=False
            )  # nosec B603
        if keep:
            print(f"portability-trial: kept {workdir}")
        else:
            shutil.rmtree(workdir, ignore_errors=True)
    print("portability-trial: passed")
    for line in report:
        print(f"  {line}")
    return 0


def run_cases(
    tools: chain.Tools,
    make: Make,
    iface: Mapping[str, str],
    registry: str,
    server: chain.Image,
    server_ref: str,
) -> list[str]:
    """The three cases against the trial release; return the report of the passing run."""
    report = check_pass(make(iface))
    print("portability-trial: both storage runs passed")
    forced = {
        **iface,
        "PORTABILITY_TRIAL_STORAGES": "fs",
        "PORTABILITY_TRIAL_FAIL_AFTER_INSTALL": "1",
    }
    check_forced_failure(make(forced))
    print("portability-trial: the forced failure printed state and logs, then cleaned up")
    tag = server_ref.rsplit(":", 1)[1]
    child = unsigned_copy(registry, "bothy-server", server.digest, tag)
    print(f"portability-trial: bothy-server:{tag} now points at the unsigned {child}")
    tools([tools.docker, "image", "rm", "--force", server_ref])
    check_unsigned(make({**iface, "PORTABILITY_TRIAL_STORAGES": "fs"}))
    print("portability-trial: an unsigned image failed before install.sh")
    return report


def hold_release(iface: Mapping[str, str]) -> None:
    """Serve the trial release until SIGINT or SIGTERM, for runs of ``make portability`` by hand.

    Prints the interface of the release (no secret: the public key is a file path).
    """
    for key in sorted(k for k in iface if k.startswith("PORTABILITY_")):
        print(f"export {key}={iface[key]}", flush=True)
    stop = threading.Event()
    for sig in (signal.SIGINT, signal.SIGTERM):
        signal.signal(sig, lambda *_: stop.set())
    print("portability-trial: holding the trial release; stop with SIGTERM", flush=True)
    stop.wait()


def main(argv: list[str] | None = None, make: Make = run_make) -> int:
    """Command line entry point; see the module documentation."""
    parser = argparse.ArgumentParser(description="Local proof of the portability test.")
    parser.add_argument("--cosign", required=True)
    parser.add_argument("--syft", required=True)
    parser.add_argument("--platforms", default=release_trial_host_platform())
    parser.add_argument("--keep", action="store_true", help="keep the temporary directory")
    parser.add_argument(
        "--hold", action="store_true", help="build the trial release, serve it until stopped"
    )
    args = parser.parse_args(argv)
    try:
        release_trial.refuse_in_ci(os.environ)
        platforms = release_trial.parse_platforms(args.platforms)
        if len(platforms) != 1:
            raise release_trial.TrialError("the trial installs on this host; pass one platform")
    except release_trial.TrialError as exc:
        print(f"portability-trial: {exc}", file=sys.stderr)
        return 2
    try:
        tools = chain.Tools(cosign=args.cosign, syft=args.syft)
        return run_trial(tools, platforms, make, args.keep, args.hold)
    except (
        CaseError,
        release_trial.TrialError,
        chain.ChainError,
        product_release.ProductReleaseError,
    ) as exc:
        print(f"portability-trial: FAIL: {exc}", file=sys.stderr)
        return 1


def release_trial_host_platform(machine: str | None = None) -> str:
    """The Linux platform of this host's CPU: the images the installation here runs."""
    m = (machine or platform.machine()).lower()
    return "linux/arm64" if m in ("arm64", "aarch64") else "linux/amd64"


if __name__ == "__main__":
    sys.exit(main())
