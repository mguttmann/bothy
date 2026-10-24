# SPDX-License-Identifier: Apache-2.0
"""Secret references after docs/spec/16-operations.md 5.2 and the redacting secret type."""

from __future__ import annotations

import os
import secrets
from pathlib import Path

import pytest

from cost_reconciler.secretref import (
    REDACTED,
    Secret,
    SecretRef,
    SecretRefError,
    parse_secret_ref,
    read_secret,
)


def canary() -> str:
    return "canary" + secrets.token_hex(12)


def secret_file(tmp_path: Path, value: str, mode: int, name: str = "secret") -> Path:
    path = tmp_path / name
    path.write_text(value, encoding="utf-8")
    path.chmod(mode)
    return path


@pytest.mark.parametrize("mode", [0o600, 0o400])
def test_file_with_private_mode_is_read(tmp_path: Path, mode: int) -> None:
    value = canary()
    path = secret_file(tmp_path, value + "\n", mode)
    assert read_secret(parse_secret_ref(f"file:{path}")).reveal() == value


@pytest.mark.parametrize("mode", [0o644, 0o640, 0o604, 0o700, 0o660])
def test_file_with_open_mode_is_rejected(tmp_path: Path, mode: int) -> None:
    value = canary()
    path = secret_file(tmp_path, value, mode)
    with pytest.raises(SecretRefError) as info:
        read_secret(parse_secret_ref(f"file:{path}"))
    assert info.value.code == "insecure_file"
    assert value not in str(info.value)


def test_one_line_end_is_stripped(tmp_path: Path) -> None:
    path = secret_file(tmp_path, "abc\r\n", 0o600)
    assert read_secret(parse_secret_ref(f"file:{path}")).reveal() == "abc"
    path = secret_file(tmp_path, "abc\n\n", 0o600, "two")
    assert read_secret(parse_secret_ref(f"file:{path}")).reveal() == "abc\n"


@pytest.mark.parametrize("content", ["", "\n"])
def test_empty_file_counts_as_not_found(tmp_path: Path, content: str) -> None:
    path = secret_file(tmp_path, content, 0o600)
    with pytest.raises(SecretRefError) as info:
        read_secret(parse_secret_ref(f"file:{path}"))
    assert info.value.code == "not_found"


def test_missing_file_is_not_found(tmp_path: Path) -> None:
    with pytest.raises(SecretRefError) as info:
        read_secret(parse_secret_ref(f"file:{tmp_path}/absent"))
    assert info.value.code == "not_found"


def test_directory_is_rejected(tmp_path: Path) -> None:
    folder = tmp_path / "dir"
    folder.mkdir(mode=0o700)
    with pytest.raises(SecretRefError) as info:
        read_secret(parse_secret_ref(f"file:{folder}"))
    assert info.value.code == "insecure_file"


def test_symlink_to_private_file_is_followed(tmp_path: Path) -> None:
    value = canary()
    target = secret_file(tmp_path, value, 0o600)
    link = tmp_path / "link"
    link.symlink_to(target)
    assert read_secret(parse_secret_ref(f"file:{link}")).reveal() == value


def test_env_is_read() -> None:
    value = canary()
    ref = parse_secret_ref("env:ADMIN_KEY")
    assert read_secret(ref, environ={"ADMIN_KEY": value}).reveal() == value
    for environ in ({}, {"ADMIN_KEY": ""}):
        with pytest.raises(SecretRefError) as info:
            read_secret(ref, environ=environ)
        assert info.value.code == "not_found"


def test_env_defaults_to_the_process_environment(monkeypatch: pytest.MonkeyPatch) -> None:
    value = canary()
    monkeypatch.setenv("COST_RECONCILER_TEST_SECRET", value)
    ref = parse_secret_ref("env:COST_RECONCILER_TEST_SECRET")
    assert read_secret(ref).reveal() == value


@pytest.mark.parametrize(
    ("text", "code"),
    [
        ("", "invalid_ref"),
        ("nocolon", "invalid_ref"),
        ("vault:x", "unsupported_scheme"),
        ("azkv://vault/name", "unsupported_scheme"),
        ("awssm://region/name", "unsupported_scheme"),
        ("FILE:/x", "unsupported_scheme"),
        ("file:relative", "invalid_ref"),
        ("file:", "invalid_ref"),
        ("file:/a//b", "invalid_ref"),
        ("file:/a/", "invalid_ref"),
        ("file:/a/./b", "invalid_ref"),
        ("file:/a/../b", "invalid_ref"),
        ("file:/a\x00b", "invalid_ref"),
        ("file:/", "invalid_ref"),
        ("env:", "invalid_ref"),
        ("env:lower", "invalid_ref"),
        ("env:1ABC", "invalid_ref"),
        ("env:A-B", "invalid_ref"),
    ],
)
def test_invalid_references(text: str, code: str) -> None:
    with pytest.raises(SecretRefError) as info:
        parse_secret_ref(text)
    assert info.value.code == code
    if text:
        assert text not in str(info.value)


def test_valid_references() -> None:
    assert parse_secret_ref("file:/run/secrets/token") == SecretRef("file", "/run/secrets/token")
    assert parse_secret_ref("env:_A1") == SecretRef("env", "_A1")


def test_secret_shows_only_the_redaction_mark() -> None:
    value = canary()
    secret = Secret(value)
    assert repr(secret) == REDACTED
    assert str(secret) == REDACTED
    assert f"{secret}" == REDACTED
    assert f"{secret!r}" == REDACTED
    assert f"{secret:>40}" == REDACTED
    assert "%s %r" % (secret, secret) == f"{REDACTED} {REDACTED}"  # noqa: UP031
    assert value not in repr([secret, {"k": secret}])
    assert secret.reveal() == value


def test_secret_rejects_empty_value() -> None:
    with pytest.raises(ValueError, match="empty"):
        Secret("")


def test_owner_check_rejects_a_foreign_file(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    path = secret_file(tmp_path, canary(), 0o600)
    monkeypatch.setattr(os, "geteuid", lambda: path.stat().st_uid + 1)
    with pytest.raises(SecretRefError) as info:
        read_secret(parse_secret_ref(f"file:{path}"))
    assert info.value.code == "insecure_file"
