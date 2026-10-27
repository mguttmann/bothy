# SPDX-License-Identifier: Apache-2.0
"""Order of the server release in .github/workflows/release.yml (brief M7-10, 18 sections 6, 7).

The version tags are set in one place only, after the vulnerability scan, the ScanCode job, the
SBOMs, the signatures and the verification; images are pushed by digest before that; signing
runs only in the environment `release`; every release job resolves the tag itself.
"""

from __future__ import annotations

from pathlib import Path
from typing import Any

import check_runners as cr

REPO = Path(__file__).resolve().parents[3]
WORKFLOW = REPO / ".github" / "workflows" / "release.yml"


def jobs() -> dict[str, Any]:
    doc = cr.load_yaml(WORKFLOW.read_text(encoding="utf-8"))
    result: dict[str, Any] = doc["jobs"]
    return result


def steps(job: str) -> list[dict[str, Any]]:
    return list(jobs()[job]["steps"])


def index_of(job: str, needle: str) -> list[int]:
    return [i for i, s in enumerate(steps(job)) if needle in str(s.get("run", ""))]


def test_version_tags_only_after_scan_scancode_sboms_signatures_and_verification() -> None:
    tag_steps = [
        (name, i)
        for name, job in jobs().items()
        for i, s in enumerate(job.get("steps") or [])
        if "release_chain.py tag" in str(s.get("run", ""))
        or "imagetools create --tag" in str(s.get("run", ""))
    ]
    assert tag_steps == [("supply-chain", index_of("supply-chain", "release_chain.py tag")[0])]
    tag_at = tag_steps[0][1]
    for needle in (
        "release_chain.py join",
        "make trivy-image",
        "release_chain.py sbom",
        "release_chain.py sign --mode private",
        "release_chain.py verify --mode private",
        "release_chain.py sign --mode public",
        "release_chain.py verify --mode public",
    ):
        found = index_of("supply-chain", needle)
        assert found, needle
        assert max(found) < tag_at, needle
    scan = steps("supply-chain")[index_of("supply-chain", "make trivy-image")[0]]["run"]
    assert "$SERVER" in scan
    assert "$POSTGRES" in scan
    assert "scancode" in jobs()["supply-chain"]["needs"]
    assert index_of("scancode", "make scancode")


def test_images_are_pushed_by_digest_only() -> None:
    pushes = [
        s["with"]
        for job in jobs().values()
        for s in job.get("steps") or []
        if str(s.get("uses", "")).startswith("docker/build-push-action@")
    ]
    files = sorted({p["file"] for p in pushes})
    assert files == [
        "services/server/Dockerfile",
        "services/server/images/bothy-postgres/Dockerfile",
    ]
    for p in pushes:
        assert "push-by-digest=true" in p["outputs"]
        assert "tags" not in p


def test_postgres_builds_natively_per_architecture() -> None:
    # Two jobs instead of a matrix, so that each passes the id of its digest artifact.
    for name, runner in (
        ("postgres-build-amd64", "ubuntu-24.04"),
        ("postgres-build-arm64", "ubuntu-24.04-arm"),
    ):
        job = jobs()[name]
        assert job["runs-on"] == runner, name
        assert job["outputs"]["digest_id"] == "${{ steps.upload.outputs.artifact-id }}", name
        assert not any("setup-qemu" in str(s.get("uses", "")) for s in job["steps"]), name


def test_signing_and_secrets_only_in_the_release_environment() -> None:
    for name, job in jobs().items():
        text = str(job)
        if "COSIGN_PRIVATE_KEY" in text or "release_chain.py sign" in text:
            assert job.get("environment") == "release", name


def test_every_release_job_resolves_the_tag_itself() -> None:
    for name, job in jobs().items():
        if job.get("environment") == "release":
            runs = [str(s.get("run", "")) for s in job["steps"]]
            assert any("components.py release --tag" in r for r in runs), name


def test_resolve_checks_version_and_changelog_before_the_build() -> None:
    resolve = steps("resolve")
    assert any("components.py release --tag" in str(s.get("run", "")) for s in resolve)
    assert "resolve" in jobs()["gates"]["needs"]
    for name in (
        "build",
        "postgres-build-amd64",
        "postgres-build-arm64",
        "supply-chain",
        "publish",
    ):
        assert "resolve" in jobs()[name]["needs"], name


def test_publish_takes_the_signed_server_set_by_id() -> None:
    publish = steps("publish")
    signed = [s for s in publish if s.get("name") == "Signed artifacts of the server"]
    assert signed
    assert signed[0]["with"]["artifact-ids"] == "${{ needs.supply-chain.outputs.dist_id }}"


def test_product_build_logs_in_before_release_build_and_signs_bundle_and_install_sh() -> None:
    """Brief M7-29: release-build of the product checks the images in ghcr.io, so the build job logs
    in first; the bundle and install.sh are signed and verified like every release file."""
    build = steps("build")
    login = [
        i
        for i, s in enumerate(build)
        if str(s.get("uses", "")).startswith("docker/login-action@")
        and "'bothy'" in str(s.get("if", ""))
    ]
    make = index_of("build", "release-build")
    assert login
    assert make
    assert login[0] < make[0]
    job = jobs()["product-signing"]
    assert job["environment"] == "release"
    assert job["if"] == "needs.resolve.outputs.component == 'bothy'"
    runs = " ".join(str(s.get("run", "")) for s in job["steps"])
    for needle in (
        "release_chain.py sbom",
        "release_chain.py sign --mode private",
        "release_chain.py verify --mode private",
        "release_chain.py sign --mode public",
        "release_chain.py verify --mode public",
    ):
        assert needle in runs, needle
    assert "bothy-compose-${VERSION}.tar.gz" in runs
    assert "dist/install.sh" in runs
    assert "refs/tags/v" in runs
    publish = steps("publish")
    signed = [s for s in publish if s.get("name") == "Signed artifacts of the product"]
    assert signed[0]["with"]["artifact-ids"] == "${{ needs.product-signing.outputs.dist_id }}"
    assert "product-signing" in jobs()["publish"]["needs"]


def test_plugin_build_has_node_and_the_signing_job_signs_the_zip_with_its_sboms() -> None:
    """Brief M7-26: the plugin ZIP is a webpack build; its SBOMs, signature and SHA256SUMS come
    from release_chain.py like every release file, and publish takes that signed set by id."""
    build = steps("build")
    node = [
        i
        for i, s in enumerate(build)
        if str(s.get("uses", "")).startswith("actions/setup-node@")
        and "'grafana-bothy-live'" in str(s.get("if", ""))
        and "'bothy'" in str(s.get("if", ""))
    ]
    make = index_of("build", "release-build")
    assert node
    assert node[0] < make[0]
    job = jobs()["plugin-signing"]
    assert job["environment"] == "release"
    assert job["if"] == "needs.resolve.outputs.component == 'grafana-bothy-live'"
    runs = [str(s.get("run", "")) for s in job["steps"]]
    joined = " ".join(runs)
    sbom = next(i for i, r in enumerate(runs) if "release_chain.py sbom" in r)
    sign = next(i for i, r in enumerate(runs) if "release_chain.py sign --mode private" in r)
    assert sbom < sign
    for needle in (
        "release_chain.py verify --mode private",
        "release_chain.py sign --mode public",
        "release_chain.py verify --mode public",
        "refs/tags/grafana-bothy-live/v",
    ):
        assert needle in joined, needle
    assert joined.count('--binary "dist/bothy-live-panel-${VERSION}.zip"') == 3
    publish = steps("publish")
    signed = [s for s in publish if s.get("name") == "Signed artifacts of the plugin"]
    assert signed[0]["with"]["artifact-ids"] == "${{ needs.plugin-signing.outputs.dist_id }}"
    assert "plugin-signing" in jobs()["publish"]["needs"]
    for name in ("Artifacts of the Linux build", "SBOM of all release artifacts", "Checksums"):
        step = next(s for s in publish if s.get("name") == name)
        assert "!= 'grafana-bothy-live'" in str(step["if"]), name


def test_host_packages_are_signed_on_their_platforms_and_signed_as_one_set() -> None:
    """Brief M7-23: build writes the binaries and the Linux packages, build-apple signs and
    notarizes the .pkg (environment release-apple), build-windows signs the MSI (environment
    release-windows); host-signing gives every file its SBOMs and cosign signature and
    SHA256SUMS, and publish takes that set by id and nothing else for the host."""
    every = jobs()
    for name, env, needle in (
        ("build-apple", "release-apple", "make -C services/host release-apple"),
        ("build-windows", "release-windows", "tools/distribution/hostdist.py windows-release"),
    ):
        job = every[name]
        assert job["environment"] == env
        assert "'host'" in job["if"], name
        host_steps = [s for s in job["steps"] if needle in str(s.get("run", ""))]
        assert len(host_steps) == 1, name
        assert host_steps[0]["if"] == "steps.info.outputs.component == 'host'", name
        go = [
            i
            for i, s in enumerate(job["steps"])
            if str(s.get("uses", "")).startswith("actions/setup-go@")
            and "'host'" in str(s.get("if", ""))
        ]
        assert go, name
        assert go[0] < job["steps"].index(host_steps[0]), name
        assert every[name]["steps"][go[0]]["with"]["go-version-file"] == "services/host/go.mod"
        client = [s for s in job["steps"] if "apps/client" in str(s.get("run", ""))]
        client += [s for s in job["steps"] if "windows.py release" in str(s.get("run", ""))]
        assert client, name
        assert all(s["if"] == "steps.info.outputs.component == 'client'" for s in client), name
    job = every["host-signing"]
    assert job["environment"] == "release"
    assert job["if"] == "needs.resolve.outputs.component == 'host'"
    assert {"build", "build-apple", "build-windows"} <= set(job["needs"])
    ids = [
        str(s["with"]["artifact-ids"]) for s in job["steps"] if "artifact-ids" in s.get("with", {})
    ]
    assert ids == [
        "${{ needs.build.outputs.dist_id }}",
        "${{ needs.build-apple.outputs.dist_id }}",
        "${{ needs.build-windows.outputs.dist_id }}",
    ]
    runs = [str(s.get("run", "")) for s in job["steps"]]
    sbom = next(i for i, r in enumerate(runs) if "release_chain.py sbom" in r)
    sign = next(i for i, r in enumerate(runs) if "release_chain.py sign --mode private" in r)
    assert sbom < sign
    joined = " ".join(runs)
    for needle in (
        "release_chain.py verify --mode private",
        "release_chain.py sign --mode public",
        "release_chain.py verify --mode public",
        "refs/tags/host/v",
    ):
        assert needle in joined, needle
    publish = steps("publish")
    signed = [s for s in publish if s.get("name") == "Signed artifacts of the host"]
    assert signed[0]["with"]["artifact-ids"] == "${{ needs.host-signing.outputs.dist_id }}"
    assert "host-signing" in every["publish"]["needs"]
    assert "needs.host-signing.result" in every["publish"]["if"]
    for name in (
        "Artifacts of the Linux build",
        "Artifacts of the Apple build",
        "Artifacts of the Windows build",
        "SBOM of all release artifacts",
        "Checksums",
    ):
        step = next(s for s in publish if s.get("name") == name)
        assert "!= 'host'" in str(step["if"]), name


def test_phone_apps_are_built_and_uploaded_only_in_their_environments() -> None:
    """Brief M7-13: build-apple builds, signs and uploads the iOS app after the macOS app
    (environment release-apple); build-android builds the App Bundle in the new environment
    release-android; publish waits for it and takes its artifact by id. Every store secret
    stays in its environment."""
    every = jobs()
    ios = [s for s in every["build-apple"]["steps"] if "release-ios" in str(s.get("run", ""))]
    assert len(ios) == 1
    assert ios[0]["if"] == "steps.info.outputs.component == 'client'"
    macos = index_of("build-apple", "make -C apps/client release-apple")
    assert macos
    assert macos[0] < every["build-apple"]["steps"].index(ios[0])
    job = every["build-android"]
    assert job["environment"] == "release-android"
    assert job["if"] == "needs.resolve.outputs.component == 'client'"
    assert job["runs-on"] == "ubuntu-24.04"
    runs = [str(s.get("run", "")) for s in job["steps"]]
    assert any("components.py release --tag" in r for r in runs)
    build = index_of("build-android", "make -C apps/client release-android")
    assert len(build) == 1
    assert job["steps"][build[0]]["if"] == "steps.info.outputs.component == 'client'"
    assert "PLAY_SERVICE_ACCOUNT_JSON" in str(job["steps"][build[0]]["env"])
    for name, other in every.items():
        text = str(other)
        if "ANDROID_UPLOAD_KEYSTORE" in text or "PLAY_SERVICE_ACCOUNT_JSON" in text:
            assert other.get("environment") == "release-android", name
        if "APPLE_DISTRIBUTION_P12" in text:
            assert other.get("environment") == "release-apple", name
    publish = every["publish"]
    assert "build-android" in publish["needs"]
    assert "needs.build-android.result" in publish["if"]
    android = [s for s in publish["steps"] if s.get("name") == "Artifacts of the Android build"]
    assert android[0]["with"]["artifact-ids"] == "${{ needs.build-android.outputs.dist_id }}"
