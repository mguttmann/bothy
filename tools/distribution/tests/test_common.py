# SPDX-License-Identifier: Apache-2.0
"""Tests of tools/distribution/common.py: runner, waits, the local update host, branding."""

from __future__ import annotations

import sys
import urllib.request
from pathlib import Path
from typing import Any

import common
import pytest
import yaml
from fakes import FakeRun


def test_runner_runs_prints_and_hides_passwords(capsys: pytest.CaptureFixture[str]) -> None:
    run = common.Runner()
    done = run(
        [sys.executable, "-c", "import sys; print(sys.stdin.read().upper())", "pw-123"],
        stdin="key",
        capture=True,
        redact=["pw-123"],
    )
    assert done.stdout.strip() == "KEY"
    printed = capsys.readouterr().out
    assert "***" in printed
    assert "pw-123" not in printed


def test_runner_reports_failures(capsys: pytest.CaptureFixture[str]) -> None:
    run = common.Runner()
    code = [sys.executable, "-c", "import sys; print('out'); sys.exit(3)", "secret-x"]
    with pytest.raises(common.StepError, match=r"exited with 3: out") as err:
        run(code, capture=True, redact=["secret-x"])
    assert "secret-x" not in str(err.value)
    assert run(code, capture=True, check=False).returncode == 3
    with pytest.raises(common.StepError, match="is not installed"):
        run(["no-such-program-of-the-trial"])
    env = run(
        [sys.executable, "-c", "import os; print(os.environ['TRIAL_X'])"],
        env={"TRIAL_X": "y"},
        capture=True,
    )
    assert env.stdout.strip() == "y"


def test_wait_for_returns_or_gives_up() -> None:
    now = [0.0]
    states = iter([False, False, True])
    common.wait_for(
        "x",
        lambda: next(states),
        clock=lambda: now[0],
        sleep=lambda s: now.__setitem__(0, now[0] + s),
    )
    with pytest.raises(common.StepError, match="gave up waiting for the installer"):
        common.wait_for(
            "the installer",
            lambda: False,
            patience=3,
            clock=lambda: now[0],
            sleep=lambda s: now.__setitem__(0, now[0] + s),
        )


def test_serve_on_loopback(tmp_path: Path) -> None:
    (tmp_path / "beta.appinstaller").write_text("<AppInstaller/>", encoding="utf-8")
    with common.serve(tmp_path) as base:
        assert base.startswith("http://127.0.0.1:")
        assert base.endswith("/")
        with urllib.request.urlopen(base + "beta.appinstaller", timeout=30) as resp:  # noqa: S310
            assert resp.headers["Content-Type"] == "application/appinstaller"
            assert resp.read() == b"<AppInstaller/>"


def test_branding_for_takes_the_identity_from_the_distribution(tmp_path: Path) -> None:
    cfg: dict[str, Any] = {
        "bundle_id": "org.acme.chat",
        "android": {"package_name": "org.acme.chat"},
    }
    out = common.branding_for(common.DEFAULT_BRANDING, cfg, tmp_path / "b.yaml")
    doc = yaml.safe_load(out.read_text(encoding="utf-8"))
    assert doc["bundle_id"] == "org.acme.chat"
    assert doc["application_id"] == "org.acme.chat"
    assert doc["app_name"] == "ExampleChat"
    with pytest.raises(common.StepError, match="branding file"):
        common.branding_for(tmp_path / "missing.yaml", cfg, tmp_path / "c.yaml")
    bad = tmp_path / "bad.yaml"
    bad.write_text("- a\n", encoding="utf-8")
    with pytest.raises(common.StepError, match="no mapping"):
        common.branding_for(bad, cfg, tmp_path / "d.yaml")


def test_branded_tree_restores_the_branding_after_a_failure(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setattr(common, "CLIENT", tmp_path)
    (tmp_path / "a.yaml").write_text("a: 1\n", encoding="utf-8")
    (tmp_path / "b.yaml").write_text("b: 1\n", encoding="utf-8")
    run = FakeRun()
    with (
        pytest.raises(RuntimeError),
        common.branded_tree(run, tmp_path / "a.yaml", tmp_path / "b.yaml"),
    ):
        raise RuntimeError
    assert [c[-1] for c in run.calls] == ["b.yaml", "a.yaml"]


def test_branding_inputs_are_relative_to_the_app(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    client = tmp_path / "client"
    client.mkdir()
    (client / "branding.example.yaml").write_text("a: 1\n", encoding="utf-8")
    outside = tmp_path / "release.yaml"
    outside.write_text("b: 2\n", encoding="utf-8")
    monkeypatch.setattr(common, "CLIENT", client)
    run = FakeRun()
    with common.branded_tree(run, client / "branding.example.yaml", outside):
        pass
    assert [c[-1] for c in run.calls] == [
        "build/branding/input-release.yaml",
        "branding.example.yaml",
    ]
    assert (client / "build" / "branding" / "input-release.yaml").read_text("utf-8") == "b: 2\n"
    assert all(c[:3] == ["dart", "run", "tool/branding_defines.dart"] for c in run.calls)


def test_require_env_names_every_missing_secret() -> None:
    assert common.require_env(["A"], {"A": "1"}, "release-x") == {"A": "1"}
    with pytest.raises(
        common.StepError, match=r"needs the secrets A, B of the environment release-x"
    ):
        common.require_env(["A", "B"], {"A": ""}, "release-x")


def test_fail(capsys: pytest.CaptureFixture[str]) -> None:
    assert common.fail("broken") == 1
    assert capsys.readouterr().err == "error: broken\n"
