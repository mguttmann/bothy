# SPDX-License-Identifier: Apache-2.0
"""Tests of the alert setup of brief M7-20: ``infra/compose/alert-setup.sh`` (Design note 1)
and the start guard ``infra/observability/alertmanager/start.sh`` (requirement 16).

The scripts run under ``sh`` in ``tmp_path``; no Docker, no network.
"""

from __future__ import annotations

import hashlib
import os
import stat
import subprocess
import sys
from pathlib import Path

import pytest
import yaml

REPO = Path(__file__).resolve().parents[3]
SETUP = REPO / "infra" / "compose" / "alert-setup.sh"
GUARD = REPO / "infra" / "observability" / "alertmanager" / "start.sh"
COMPOSE = REPO / "infra" / "compose" / "compose.yaml"
AM_CONFIG = REPO / "infra" / "observability" / "alertmanager" / "alertmanager.yaml"

HOOK = (
    "https://chat.example.com/api/v1/hooks/0192f4a0-7b1c-7000-8a00-000000000901/"
    "bothy_whi_EXAMPLE000000000000000000000000000000000000_2WwD3e"
)
INTERNAL = (
    "http://api:8081/api/v1/hooks/0192f4a0-7b1c-7000-8a00-000000000901/"
    "bothy_whi_EXAMPLE000000000000000000000000000000000000_2WwD3e"
)
ONCALL = "https://oncall.example.com/hook"
HEARTBEAT = "https://heartbeat.example.com/ping"


def setup(secrets: Path, *args: str, stdin: str = HOOK + "\n") -> subprocess.CompletedProcess[str]:
    """Runs alert-setup.sh with ``--secrets-dir secrets`` and ``args``, the URL on stdin."""
    return subprocess.run(
        ["sh", str(SETUP), "--secrets-dir", str(secrets), *args],
        input=stdin,
        capture_output=True,
        text=True,
        check=False,
    )


def fingerprint(value: str) -> str:
    """The fingerprint of secrets-init (16 5.3 item 5): sha256: and 12 hex characters."""
    return "sha256:" + hashlib.sha256(value.encode()).hexdigest()[:12]


def test_writes_internal_url_with_mode_0600(tmp_path: Path) -> None:
    secrets = tmp_path / "secrets"
    res = setup(secrets, "channel")
    assert res.returncode == 0, res.stderr
    target = secrets / "alertmanager_bothy_webhook_url"
    assert target.read_text(encoding="utf-8") == INTERNAL
    assert stat.S_IMODE(target.stat().st_mode) == 0o600
    assert stat.S_IMODE(secrets.stat().st_mode) == 0o700
    assert res.stdout == f"created alertmanager_bothy_webhook_url {fingerprint(INTERNAL)}\n"


def test_never_prints_the_url_or_its_token(tmp_path: Path) -> None:
    secrets = tmp_path / "secrets"
    for _ in range(2):
        res = setup(secrets, "channel")
        token = HOOK.rsplit("/", 1)[1]
        assert token not in res.stdout + res.stderr
        assert "0192f4a0-7b1c-7000-8a00-000000000901" not in res.stdout + res.stderr
    bad = setup(secrets, "channel", stdin="https://chat.example.com/api/v1/other/secretvalue\n")
    assert bad.returncode == 2
    assert "secretvalue" not in bad.stdout + bad.stderr


def test_second_run_with_same_url_changes_nothing(tmp_path: Path) -> None:
    secrets = tmp_path / "secrets"
    assert setup(secrets, "channel").returncode == 0
    target = secrets / "alertmanager_bothy_webhook_url"
    before = target.stat()
    res = setup(secrets, "channel")
    assert res.returncode == 0, res.stderr
    assert res.stdout == f"kept alertmanager_bothy_webhook_url {fingerprint(INTERNAL)}\n"
    after = target.stat()
    assert (after.st_ino, after.st_mtime_ns) == (before.st_ino, before.st_mtime_ns)
    assert sorted(p.name for p in secrets.iterdir()) == ["alertmanager_bothy_webhook_url"]


def test_changed_url_updates_the_file(tmp_path: Path) -> None:
    secrets = tmp_path / "secrets"
    assert setup(secrets, "channel").returncode == 0
    other = HOOK.replace("2WwD3e", "3XxE4f")
    res = setup(secrets, "channel", stdin=other)
    assert res.returncode == 0, res.stderr
    assert res.stdout.startswith("updated alertmanager_bothy_webhook_url sha256:")
    target = secrets / "alertmanager_bothy_webhook_url"
    assert target.read_text(encoding="utf-8") == INTERNAL.replace("2WwD3e", "3XxE4f")
    assert stat.S_IMODE(target.stat().st_mode) == 0o600


def test_internal_base_option(tmp_path: Path) -> None:
    secrets = tmp_path / "secrets"
    res = setup(secrets, "--internal-base", "http://bothy-api:9081", "channel")
    assert res.returncode == 0, res.stderr
    text = (secrets / "alertmanager_bothy_webhook_url").read_text(encoding="utf-8")
    assert text == INTERNAL.replace("http://api:8081", "http://bothy-api:9081")


@pytest.mark.parametrize(
    ("kind", "url", "name"),
    [
        ("external", ONCALL, "alertmanager_external_webhook_url"),
        ("heartbeat", HEARTBEAT, "alertmanager_heartbeat_url"),
    ],
)
def test_external_urls_are_written_unchanged(
    tmp_path: Path, kind: str, url: str, name: str
) -> None:
    secrets = tmp_path / "secrets"
    res = setup(secrets, kind, stdin=url + "\r\n")
    assert res.returncode == 0, res.stderr
    assert (secrets / name).read_text(encoding="utf-8") == url
    assert res.stdout == f"created {name} {fingerprint(url)}\n"
    assert url not in res.stderr


@pytest.mark.parametrize(
    ("args", "stdin"),
    [
        ([], HOOK),
        (["unknown"], HOOK),
        (["channel", "extra"], HOOK),
        (["channel"], ""),
        (["channel"], "not a url"),
        (["channel"], "ftp://chat.example.com/api/v1/hooks/a/b"),
        (["channel"], HOOK + "/more"),
        (["channel"], HOOK + "?x=1"),
        (["external"], "oncall.example.com/hook"),
        (["external"], "https://oncall.example.com/a b"),
        (["--internal-base", "https://api:8081", "channel"], HOOK),
        (["--internal-base", "http://api:8081/", "channel"], HOOK),
        (["--secrets-dir"], HOOK),
    ],
)
def test_usage_and_input_errors_exit_2_and_write_nothing(
    tmp_path: Path, args: list[str], stdin: str
) -> None:
    secrets = tmp_path / "secrets"
    res = setup(secrets, *args, stdin=stdin)
    assert res.returncode == 2, (res.stdout, res.stderr)
    assert not secrets.exists() or not any(secrets.iterdir())


def test_unwritable_secrets_dir_is_a_runtime_error(tmp_path: Path) -> None:
    if os.geteuid() == 0:
        pytest.skip("root writes into a read-only directory")
    secrets = tmp_path / "secrets"
    secrets.mkdir(mode=0o500)
    try:
        res = setup(secrets, "channel")
    finally:
        secrets.chmod(0o700)
    assert res.returncode == 1
    assert not any(secrets.iterdir())


def guard(tmp_path: Path, files: dict[str, str | None]) -> subprocess.CompletedProcess[str]:
    """Runs start.sh with a copy of alertmanager.yaml whose url_file paths point to tmp_path and
    a fake binary that prints its arguments; ``files`` maps the secret names to their content
    (None: absent)."""
    secrets = tmp_path / "secrets"
    secrets.mkdir(exist_ok=True)
    for name, content in files.items():
        if content is not None:
            (secrets / name).write_text(content, encoding="utf-8")
    config = tmp_path / "alertmanager.yaml"
    config.write_text(
        AM_CONFIG.read_text(encoding="utf-8").replace("/run/bothy/secrets/", f"{secrets}/"),
        encoding="utf-8",
    )
    fake = tmp_path / "alertmanager"
    fake.write_text('#!/bin/sh\necho "started $*"\n', encoding="utf-8")
    fake.chmod(0o755)
    return subprocess.run(
        ["sh", str(GUARD), str(fake), f"--config.file={config}", "--storage.path=/x"],
        capture_output=True,
        text=True,
        check=False,
    )


ALL = {
    "alertmanager_bothy_webhook_url": INTERNAL,
    "alertmanager_external_webhook_url": ONCALL,
    "alertmanager_heartbeat_url": HEARTBEAT,
}


def test_guard_starts_alertmanager_when_every_url_file_has_a_value(tmp_path: Path) -> None:
    res = guard(tmp_path, ALL)
    assert res.returncode == 0, res.stderr
    assert res.stdout.startswith("started --config.file=")
    assert "--storage.path=/x" in res.stdout


@pytest.mark.parametrize("name", sorted(ALL))
@pytest.mark.parametrize("content", [None, "", "\n"])
def test_guard_refuses_a_missing_or_empty_secret(
    tmp_path: Path, name: str, content: str | None
) -> None:
    res = guard(tmp_path, {**ALL, name: content})
    assert res.returncode == 1
    assert "started" not in res.stdout
    assert name in res.stderr
    assert "alert-setup.sh" in res.stderr
    for value in ALL.values():
        assert value not in res.stderr


def test_compose_starts_alertmanager_through_the_guard() -> None:
    svc = yaml.safe_load(COMPOSE.read_text(encoding="utf-8"))["services"]["alertmanager"]
    assert svc["entrypoint"] == ["/bin/sh", "/etc/alertmanager/start.sh", "/bin/alertmanager"]
    assert "../observability/alertmanager:/etc/alertmanager:ro" in svc["volumes"]


def test_every_receiver_reads_its_url_from_a_file_of_the_catalogue() -> None:
    spec = yaml.safe_load(AM_CONFIG.read_text(encoding="utf-8"))
    files = sorted(
        cfg["url_file"] for r in spec["receivers"] for cfg in r.get("webhook_configs", [])
    )
    assert files == sorted(f"/run/bothy/secrets/{n}" for n in ALL)
    assert all("url" not in cfg for r in spec["receivers"] for cfg in r["webhook_configs"])


def test_interrupt_at_the_hidden_prompt_restores_echo() -> None:
    """Ctrl-C at the hidden prompt leaves the terminal with echo on (CodeRabbit on M7-20)."""
    import pty
    import signal
    import termios

    # A shell started from a background job inherits SIGINT ignored and cannot trap it (POSIX);
    # the wrapper resets it to the default action and execs sh, as under an interactive shell.
    reset = (
        "import os, signal, sys; signal.signal(signal.SIGINT, signal.SIG_DFL); "
        "os.execvp('sh', sys.argv[1:])"
    )
    master, slave = pty.openpty()
    proc = subprocess.Popen(
        [
            sys.executable,
            "-c",
            reset,
            "sh",
            str(SETUP),
            "--secrets-dir",
            "/nonexistent-alert-setup-test",
            "channel",
        ],
        stdin=slave,
        stdout=subprocess.DEVNULL,
        stderr=slave,
    )
    try:
        seen = b""
        while b"(input hidden): " not in seen:
            chunk = os.read(master, 1024)
            assert chunk, "the prompt never appeared"
            seen += chunk
        # The prompt comes after stty -echo, so echo is off now.
        assert not termios.tcgetattr(slave)[3] & termios.ECHO
        proc.send_signal(signal.SIGINT)
        assert proc.wait() != 0
        assert termios.tcgetattr(slave)[3] & termios.ECHO
    finally:
        if proc.poll() is None:
            proc.kill()
            proc.wait()
        os.close(master)
        os.close(slave)
