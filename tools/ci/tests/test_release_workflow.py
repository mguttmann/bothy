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
    files = sorted(p["file"] for p in pushes)
    assert files == [
        "services/server/Dockerfile",
        "services/server/images/bothy-postgres/Dockerfile",
    ]
    for p in pushes:
        assert "push-by-digest=true" in p["outputs"]
        assert "tags" not in p


def test_postgres_builds_natively_per_architecture() -> None:
    job = jobs()["postgres-build"]
    assert job["strategy"]["matrix"]["runner"] == ["ubuntu-24.04", "ubuntu-24.04-arm"]
    assert job["runs-on"] == "${{ matrix.runner }}"
    assert not any("setup-qemu" in str(s.get("uses", "")) for s in job["steps"])


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
    for name in ("build", "postgres-build", "supply-chain", "publish"):
        assert "resolve" in jobs()[name]["needs"], name


def test_publish_takes_the_signed_server_set_by_id() -> None:
    publish = steps("publish")
    signed = [s for s in publish if s.get("name") == "Signed artifacts of the server"]
    assert signed
    assert signed[0]["with"]["artifact-ids"] == "${{ needs.supply-chain.outputs.dist_id }}"
