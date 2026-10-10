# SPDX-License-Identifier: Apache-2.0
"""The custom rules of .gitleaks.toml fire on their formats, tested with the pinned gitleaks
binary. Sample secrets are assembled at runtime, so no secret-like string is committed."""

from __future__ import annotations

import json
import random
import string
import subprocess
from pathlib import Path

import pytest

REPO = Path(__file__).resolve().parents[3]


def _gitleaks() -> Path:
    import sys

    sys.path.insert(0, str(REPO / "tools" / "bin"))
    import fetch_tools

    lock = fetch_tools.load_lock(fetch_tools.DEFAULT_LOCK)
    return fetch_tools.ensure(
        lock, "gitleaks", fetch_tools.platform_key(), fetch_tools.DEFAULT_CACHE
    )


def _rand(n: int, alphabet: str = string.ascii_letters + string.digits, seed: int = 7) -> str:
    rng = random.Random(seed + n)  # noqa: S311 (test data, not a secret)
    return "".join(rng.choice(alphabet) for _ in range(n))


SAMPLES = {
    "standard-webhooks-secret": "webhook_secret = " + "wh" + "sec_" + _rand(43) + "=",
    "anthropic-credential-any": "token = "
    + "sk-"
    + "ant-"
    + "oat01-"
    + _rand(60, string.ascii_letters + string.digits + "_-"),
    "tailscale-key": "TS_AUTHKEY=" + "ts" + "key-auth-" + _rand(12) + "-" + _rand(32),
    "sonarqube-token-prefixed": "token " + "sq" + "u_" + _rand(40, string.digits + "abcdef"),
    "bothy-api-token": "BOTHY_TOKEN=" + "bothy" + "_agt_" + _rand(43) + "_" + _rand(6),
}


@pytest.mark.parametrize("rule", sorted(SAMPLES))
def test_custom_rule_detects_its_format(tmp_path: Path, rule: str) -> None:
    (tmp_path / "sample.txt").write_text(SAMPLES[rule] + "\n")
    report = tmp_path / "report.json"
    proc = subprocess.run(
        [
            str(_gitleaks()),
            "dir",
            str(tmp_path / "sample.txt"),
            "--config",
            str(REPO / ".gitleaks.toml"),
            "--no-banner",
            "--redact",
            "--report-format",
            "json",
            "--report-path",
            str(report),
            "--exit-code",
            "0",
        ],
        capture_output=True,
        text=True,
        check=False,
    )
    assert proc.returncode == 0, proc.stderr
    found = {item["RuleID"] for item in json.loads(report.read_text())}
    assert rule in found


ALLOWED = {
    "marked example": "webhook_secret = " + "wh" + "sec_" + "EXAMPLE" + _rand(36) + "=",
    "uuid identifier": "credential_"
    + "holder_id: "
    + "-".join(_rand(n, "0123456789abcdef") for n in (8, 4, 4, 4, 12)),
    "unsigned example jwt": "access_token: eyJ" + _rand(30) + ".e30.sig",
    "checksum manifest": '    "icons/key.svg": "' + _rand(64, "0123456789abcdef") + '",',
}


@pytest.mark.parametrize("case", sorted(ALLOWED))
def test_allowlists_skip_documented_examples(tmp_path: Path, case: str) -> None:
    (tmp_path / "sample.txt").write_text(ALLOWED[case] + "\n")
    proc = subprocess.run(
        [
            str(_gitleaks()),
            "dir",
            str(tmp_path / "sample.txt"),
            "--config",
            str(REPO / ".gitleaks.toml"),
            "--no-banner",
            "--redact",
        ],
        capture_output=True,
        text=True,
        check=False,
    )
    assert proc.returncode == 0, proc.stdout + proc.stderr


def test_repository_tree_is_clean() -> None:
    proc = subprocess.run(
        [
            str(_gitleaks()),
            "dir",
            str(REPO),
            "--config",
            str(REPO / ".gitleaks.toml"),
            "--no-banner",
            "--redact",
        ],
        capture_output=True,
        text=True,
        check=False,
    )
    assert proc.returncode == 0, proc.stdout + proc.stderr
