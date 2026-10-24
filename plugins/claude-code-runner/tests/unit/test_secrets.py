# SPDX-License-Identifier: Apache-2.0
"""secret_ref values of the runner (16 5.2): fresh reads, never printed."""

from __future__ import annotations

from pathlib import Path

import pytest

from claude_code_runner.secrets import SecretError, parse_secret_ref, resolve


def test_file_ref(tmp_path: Path) -> None:
    path = tmp_path / "key"
    path.write_text("sk-canary\n")
    path.chmod(0o600)
    ref = parse_secret_ref(f"file:{path}")
    assert resolve(ref).reveal() == "sk-canary"
    path.write_text("sk-rotated\r\n")
    assert resolve(ref).reveal() == "sk-rotated"
    assert "sk-" not in repr(ref) + str(ref)


def test_env_ref(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("RUNNER_TEST_KEY", "sk-env")
    assert resolve(parse_secret_ref("env:RUNNER_TEST_KEY")).reveal() == "sk-env"
    monkeypatch.setenv("RUNNER_TEST_KEY", "")
    with pytest.raises(SecretError) as err:
        resolve(parse_secret_ref("env:RUNNER_TEST_KEY"))
    assert err.value.code == "empty"


@pytest.mark.parametrize(
    "text",
    [
        "vault:x",
        "file:relative",
        "file:/a/../b",
        "file:/a//b",
        "file:/a/",
        "env:lower",
        "env:",
        "nothing",
    ],
)
def test_grammar(text: str) -> None:
    with pytest.raises(SecretError):
        parse_secret_ref(text)


def test_file_errors(tmp_path: Path) -> None:
    missing = parse_secret_ref(f"file:{tmp_path / 'missing'}")
    with pytest.raises(SecretError) as err:
        resolve(missing)
    assert err.value.code == "missing"
    empty = tmp_path / "empty"
    empty.write_text("\n")
    empty.chmod(0o600)
    with pytest.raises(SecretError) as err:
        resolve(parse_secret_ref(f"file:{empty}"))
    assert err.value.code == "empty"
    wide = tmp_path / "wide"
    wide.write_text("sk-wide")
    wide.chmod(0o644)
    with pytest.raises(SecretError) as err:
        resolve(parse_secret_ref(f"file:{wide}"))
    assert err.value.code == "unsafe_mode"
    assert "sk-wide" not in str(err.value)
    assert str(wide) not in str(err.value)
    wide.chmod(0o400)
    assert resolve(parse_secret_ref(f"file:{wide}")).reveal() == "sk-wide"


def test_file_not_utf8(tmp_path: Path) -> None:
    path = tmp_path / "binary"
    path.write_bytes(b"sk-\xff\xfe")
    path.chmod(0o600)
    with pytest.raises(SecretError) as err:
        resolve(parse_secret_ref(f"file:{path}"))
    assert err.value.code == "not_utf8"
    assert "0xff" not in str(err.value)
