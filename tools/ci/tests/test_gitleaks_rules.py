# SPDX-License-Identifier: Apache-2.0
"""The custom rules of .gitleaks.toml fire on their formats, tested with the pinned gitleaks
binary. Sample secrets are assembled at runtime, so no secret-like string is committed."""

from __future__ import annotations

import json
import random
import secrets
import string
import subprocess
import zlib
from pathlib import Path

import pytest
import tracked_tree

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


# Fake values of the first API draft that stay in the history until it is rewritten
# (.gitleaks.toml, allowlist "Fake example values in the history ...").
HISTORICAL_WEBHOOK_EXAMPLE = "wh" + "sec_" + "MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw"

ALLOWED = {
    "marked example": "webhook_secret = " + "wh" + "sec_" + "EXAMPLE" + _rand(36) + "=",
    "uuid identifier": "credential_"
    + "holder_id: "
    + "-".join(_rand(n, "0123456789abcdef") for n in (8, 4, 4, 4, 12)),
    "unsigned example jwt": "access_token: eyJ" + _rand(30) + ".e30.sig",
    "checksum manifest": '    "icons/key.svg": "' + _rand(64, "0123456789abcdef") + '",',
    "historical webhook example": "hmac_secret: " + HISTORICAL_WEBHOOK_EXAMPLE,
    "historical agent token example": "secret: " + "agt_7Hq2." + "kJ8sV2pQ0wX5yZ1aB3cD4eF6gH",
    "historical push token example": "token: " + "a1b2" + "c3d4e5f6",
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


# The Base62 alphabet and the vectors of the API token format (ADR 0061); the Go package
# services/server/internal/identity/domain/apitoken reads the same file.
BASE62 = string.digits + string.ascii_uppercase + string.ascii_lowercase
VECTORS = REPO / "services/server/internal/identity/domain/apitoken/testdata/checksum_vectors.json"


def _base62(n: int, width: int) -> str:
    digits = ""
    while n:
        n, d = divmod(n, 62)
        digits = BASE62[d] + digits
    assert len(digits) <= width
    return digits.rjust(width, "0")


def _checksum(random_part: str) -> str:
    return _base62(zlib.crc32(random_part.encode("ascii")), 6)


def _findings(tmp_path: Path, text: str) -> set[str]:
    (tmp_path / "sample.txt").write_text(text + "\n")
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
    return {item["RuleID"] for item in json.loads(report.read_text())}


def test_bothy_token_with_valid_checksum_is_detected(tmp_path: Path) -> None:
    """A token built like the server builds it (256 random bits in Base62, CRC-32 checksum)
    is a finding; the same token with a 5-character checksum is not."""
    random_part = _base62(int.from_bytes(secrets.token_bytes(32), "big"), 43)
    token = "bothy" + "_agt_" + random_part + "_" + _checksum(random_part)
    assert len(token) == 60
    assert "bothy-api-token" in _findings(tmp_path, "BOTHY_TOKEN=" + token)
    assert "bothy-api-token" not in _findings(tmp_path, "BOTHY_TOKEN=" + token[:-1])


def test_pairing_code_with_valid_checksum_is_detected(tmp_path: Path) -> None:
    """A pairing code of a host (kind hpc, 24 section 3.2) built like the server builds it is a
    finding of the rule bothy-api-token (T-TM-06, SEC-SEC-4)."""
    random_part = _base62(int.from_bytes(secrets.token_bytes(32), "big"), 43)
    code = "bothy" + "_hpc_" + random_part + "_" + _checksum(random_part)
    assert len(code) == 60
    assert "bothy-api-token" in _findings(tmp_path, "BOTHY_PAIRING_CODE=" + code)


def test_checksum_vectors_match_go() -> None:
    """Python and Go agree on every checksum vector of the Go testdata file."""
    vectors = json.loads(VECTORS.read_text())["vectors"]
    assert len(vectors) >= 3
    assert any(v["checksum"].startswith("0") for v in vectors)
    for v in vectors:
        assert v["random"].startswith("EXAMPLE")
        assert len(v["random"]) == 43
        assert _checksum(v["random"]) == v["checksum"], v["random"]


def test_history_allowlist_takes_exact_values_only(tmp_path: Path) -> None:
    """One changed character makes the historical example a finding again."""
    near = HISTORICAL_WEBHOOK_EXAMPLE[:-1] + "x"
    (tmp_path / "sample.txt").write_text("hmac_secret: " + near + "\n")
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
    assert "standard-webhooks-secret" in found


def test_repository_tree_is_clean(tmp_path: Path) -> None:
    """The working tree as a commit would see it; ignored caches and other worktrees are not
    part of the repository (tracked_tree.py)."""
    tree = tmp_path / "tree"
    tracked_tree.snapshot(REPO, tree)
    proc = subprocess.run(
        [
            str(_gitleaks()),
            "dir",
            str(tree),
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
