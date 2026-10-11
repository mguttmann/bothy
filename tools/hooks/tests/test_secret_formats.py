# SPDX-License-Identifier: Apache-2.0
"""Tests for the Stripe webhook secret format check (GitHub secret scanning).

Samples are assembled at runtime, so this file holds no value in a provider format.
"""

from __future__ import annotations

import re
from pathlib import Path

import pytest
import secret_formats
import yaml

PREFIX = "wh" + "sec_"
REPO = Path(__file__).resolve().parents[3]


@pytest.mark.parametrize(
    "value",
    [
        PREFIX + "A" * 32,
        # GitHub reported this length (stripe_webhook_signing_secret, first public push).
        PREFIX + "exampleOnlyNotASecret" + "0" * 10,
        PREFIX + "Ab+/" * 22 + "==",
        PREFIX + "x" * 16,
    ],
    ids=["32 characters", "31 characters", "88 characters", "16 characters"],
)
def test_webhook_secret_format_is_reported(tmp_path: Path, value: str) -> None:
    f = tmp_path / "doc.yaml"
    f.write_text(f"first: line\n    hmac_secret: {value}\n", encoding="utf-8")
    findings = secret_formats.scan_file(f)
    assert [(x.line, x.column, x.secret_type) for x in findings] == [
        (2, 18, "stripe_webhook_signing_secret")
    ]


@pytest.mark.parametrize(
    "text",
    [
        PREFIX + "EXAMPLE",
        PREFIX + "EXAMPLE...",
        PREFIX + "<base64>",
        PREFIX + "x" * 15,
        "'''^whsec[_]" + "A" * 32 + "$'''",
        "the prefix " + PREFIX + " alone",
    ],
    ids=["marker", "elided", "placeholder", "15 characters", "split pattern", "prefix only"],
)
def test_elided_and_short_examples_pass(tmp_path: Path, text: str) -> None:
    f = tmp_path / "doc.md"
    f.write_text(text + "\n", encoding="utf-8")
    assert secret_formats.scan_file(f) == []


def test_binary_file_is_skipped(tmp_path: Path) -> None:
    f = tmp_path / "blob.bin"
    f.write_bytes(b"\x00\x01" + (PREFIX + "A" * 32).encode())
    assert secret_formats.scan_file(f) == []


def test_text_that_is_not_utf8_is_still_scanned(tmp_path: Path) -> None:
    f = tmp_path / "latin1.txt"
    f.write_bytes(b"caf\xe9 " + (PREFIX + "A" * 32).encode() + b"\n")
    assert [x.column for x in secret_formats.scan_file(f)] == [6]


def test_main_returns_one_and_prints_location(
    tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    good = tmp_path / "good.md"
    good.write_text(PREFIX + "EXAMPLE\n", encoding="utf-8")
    bad = tmp_path / "bad.md"
    bad.write_text("x\n" + PREFIX + "A" * 32 + "\n", encoding="utf-8")
    rc = secret_formats.main([str(good), str(bad)])
    out = capsys.readouterr().out
    assert rc == 1
    assert f"{bad}:2:1: stripe_webhook_signing_secret" in out
    assert str(good) not in out


def test_unreadable_file_is_reported_without_traceback(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    locked = tmp_path / "locked.md"
    locked.write_text("ok\n", encoding="utf-8")
    good = tmp_path / "good.md"
    good.write_text("ok\n", encoding="utf-8")
    real_read_bytes = Path.read_bytes

    def read_bytes(self: Path) -> bytes:
        if self == locked:
            raise PermissionError(13, "Permission denied")
        return real_read_bytes(self)

    monkeypatch.setattr(Path, "read_bytes", read_bytes)
    rc = secret_formats.main([str(locked), str(good)])
    out = capsys.readouterr().out
    assert rc == 1
    assert f"{locked}: cannot be read" in out
    assert str(good) not in out


def test_allowlisted_fixture_is_skipped(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    fixture = tmp_path / "fixtures" / "secret.txt"
    fixture.parent.mkdir()
    fixture.write_text(PREFIX + "A" * 32 + "\n", encoding="utf-8")
    other = tmp_path / "other.txt"
    other.write_text(PREFIX + "A" * 32 + "\n", encoding="utf-8")
    monkeypatch.setattr(secret_formats, "ROOT", tmp_path)
    monkeypatch.setattr(secret_formats, "ALLOWED_FIXTURES", frozenset({"fixtures/secret.txt"}))
    assert secret_formats.main([str(fixture)]) == 0
    assert secret_formats.main([str(other)]) == 1


def test_main_returns_zero_when_clean(tmp_path: Path) -> None:
    good = tmp_path / "good.txt"
    good.write_text("ok\n", encoding="utf-8")
    assert secret_formats.main([str(good)]) == 0


def test_repository_is_clean() -> None:
    """The gate itself: no tracked file holds a value in the whsec_ format."""
    assert secret_formats.main(["--all"]) == 0


def test_hook_runs_on_every_commit_and_in_lint() -> None:
    config = yaml.safe_load((REPO / ".pre-commit-config.yaml").read_text(encoding="utf-8"))
    hooks = {hook["id"]: hook for repo in config["repos"] for hook in repo["hooks"]}
    assert "secret-formats" in hooks, "pre-commit hook secret-formats is missing"
    hook = hooks["secret-formats"]
    assert hook["entry"].split()[-1] == "tools/hooks/secret_formats.py"
    assert hook.get("stages", config["default_stages"]) == ["pre-commit"]
    assert hook.get("pass_filenames", True)
    makefile = (REPO / "Makefile").read_text(encoding="utf-8")
    lint = re.search(r"^\tSKIP=(\S+) .*pre-commit run --all-files", makefile, re.MULTILINE)
    assert lint, "make lint no longer runs the pre-commit hooks over all files"
    assert "secret-formats" not in lint.group(1).split(",")
