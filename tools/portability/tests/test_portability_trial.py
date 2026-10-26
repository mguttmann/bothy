# SPDX-License-Identifier: Apache-2.0
"""Unit tests of the local proof of the portability test (portability_trial.py)."""

from __future__ import annotations

from pathlib import Path

import portability_trial as tr
import pytest

PASS = (
    "portability: PASS\nportability:   fs: 127.0.0.1:5000/bothy-server@sha256:a\n"
    "portability:   fs: passed in 300s\nportability:   s3: passed in 320s\n"
)


def test_interface_env_drops_github_actions_and_old_values(tmp_path: Path) -> None:
    env = tr.interface_env(
        {"GITHUB_ACTIONS": "true", "PORTABILITY_TRIAL_STORAGES": "fs", "PATH": "/bin"},
        "0.1.0",
        tmp_path,
        tmp_path / "k.pub",
    )
    assert env["PORTABILITY_TAG"] == "v0.1.0"
    assert env["PORTABILITY_INSTALL_SOURCE"] == "release-download"
    assert env["PORTABILITY_RELEASE_DIR"] == str(tmp_path)
    assert env["PORTABILITY_TRIAL_PUBLIC_KEY"] == str(tmp_path / "k.pub")
    assert "GITHUB_ACTIONS" not in env
    assert "PORTABILITY_TRIAL_STORAGES" not in env


def test_check_pass() -> None:
    report = tr.check_pass(tr.Outcome(0, PASS, 1.0))
    assert "fs: passed in 300s" in report
    with pytest.raises(tr.CaseError, match="exit code 1"):
        tr.check_pass(tr.Outcome(1, PASS, 1.0))
    with pytest.raises(tr.CaseError, match="s3 reported no pass"):
        tr.check_pass(tr.Outcome(0, PASS.replace("s3: passed", "s3: x"), 1.0))


FORCED = (
    "portability: install (fs): install.sh --yes\nportability: FAILED: state\n"
    "docker compose ps --all:\nlast 200 log lines of every service:\n"
    "portability: FAIL: run fs failed\n"
)


def test_check_forced_failure() -> None:
    tr.check_forced_failure(tr.Outcome(2, FORCED, 1.0))
    with pytest.raises(tr.CaseError, match=r"exit code 0"):
        tr.check_forced_failure(tr.Outcome(0, FORCED, 1.0))
    with pytest.raises(tr.CaseError, match=r"without a failed step"):
        tr.check_forced_failure(tr.Outcome(2, "portability: PORTABILITY_TAG must be\n", 1.0))
    with pytest.raises(tr.CaseError, match=r"docker compose ps"):
        tr.check_forced_failure(tr.Outcome(1, FORCED.replace("docker compose ps --all:", ""), 1.0))
    with pytest.raises(tr.CaseError, match="left objects"):
        tr.check_forced_failure(tr.Outcome(1, FORCED + "cleanup left objects of x\n", 1.0))
    with pytest.raises(tr.CaseError, match=r"did not reach install\.sh"):
        tr.check_forced_failure(tr.Outcome(1, FORCED.replace("portability: install (", ""), 1.0))


def test_check_unsigned() -> None:
    ok = (
        "portability: run fs: r@sha256:a carries no valid signature (cosign verify failed)\n"
        "portability: FAIL: run fs failed\n"
    )
    tr.check_unsigned(tr.Outcome(2, ok, 1.0))
    with pytest.raises(tr.CaseError, match=r"install\.sh ran"):
        tr.check_unsigned(tr.Outcome(2, ok + "portability: install (fs): x\n", 1.0))
    with pytest.raises(tr.CaseError, match="signature verification"):
        tr.check_unsigned(tr.Outcome(2, "portability: FAIL: other\n", 1.0))
    with pytest.raises(tr.CaseError, match=r"exit code 0"):
        tr.check_unsigned(tr.Outcome(0, ok, 1.0))


def test_main_refuses_under_github_actions(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("GITHUB_ACTIONS", "true")

    def never(_: object) -> tr.Outcome:
        raise AssertionError("ran make under GitHub Actions")

    assert tr.main(["--cosign", "c", "--syft", "s"], make=never) == 2


def test_main_needs_one_platform(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("GITHUB_ACTIONS", raising=False)
    assert tr.main(["--cosign", "c", "--syft", "s", "--platforms", "linux/amd64,linux/arm64"]) == 2


def test_host_platform() -> None:
    assert tr.release_trial_host_platform("arm64") == "linux/arm64"
    assert tr.release_trial_host_platform("x86_64") == "linux/amd64"


def test_unsigned_copy_puts_the_platform_manifest_under_the_tag() -> None:
    child = "sha256:" + "b" * 64
    calls: list[tuple[str, str, bytes | None]] = []

    def fetch(req: tr.urllib.request.Request) -> tuple[bytes, str]:
        calls.append(
            (req.get_method(), req.full_url, req.data if isinstance(req.data, bytes) else None)
        )
        if req.full_url.endswith("sha256:" + "a" * 64):
            return (
                b'{"manifests":[{"digest":"' + child.encode() + b'"}]}',
                "application/vnd.oci.image.index.v1+json",
            )
        return b"manifest-bytes", "application/vnd.oci.image.manifest.v1+json"

    got = tr.unsigned_copy("127.0.0.1:5000", "bothy-server", "sha256:" + "a" * 64, "0.0.0", fetch)
    assert got == child
    assert calls[-1] == (
        "PUT",
        "http://127.0.0.1:5000/v2/bothy-server/manifests/0.0.0",
        b"manifest-bytes",
    )


def test_fetch_refuses_another_host() -> None:
    with pytest.raises(tr.CaseError, match=r"outside 127\.0\.0\.1"):
        tr._fetch(tr.urllib.request.Request("http://example.com/v2/"))
