# SPDX-License-Identifier: Apache-2.0
"""Unit tests of the runner of the portability test (tools/portability/portability_run.py)."""

from __future__ import annotations

import io
import json
import re
import subprocess
import tarfile
from collections.abc import Sequence
from pathlib import Path
from typing import Any

import portability_run as pt
import pytest

VERSION = "0.1.0"
TAG = "v0.1.0"


def _env(**extra: str) -> dict[str, str]:
    base = {
        "PORTABILITY_TAG": TAG,
        "PORTABILITY_INSTALL_SOURCE": "url",
        "GITHUB_REPOSITORY": "alex/bothy",
    }
    base.update(extra)
    return {k: v for k, v in base.items() if v != "<unset>"}


def _release_dir(tmp_path: Path) -> Path:
    d = tmp_path / "release"
    d.mkdir()
    (d / "install.sh").write_text("#!/bin/sh\n", encoding="utf-8")
    (d / f"bothy-compose-{VERSION}.tar.gz").write_bytes(b"x")
    return d


class Recorder:
    """A fake runner that records every command and answers from a table of prefixes."""

    def __init__(self, answers: dict[str, tuple[int, str]] | None = None) -> None:
        self.calls: list[list[str]] = []
        self.kwargs: list[dict[str, Any]] = []
        self.answers = answers or {}

    def __call__(self, args: Sequence[str], **kwargs: Any) -> subprocess.CompletedProcess[str]:
        cmd = list(args)
        self.calls.append(cmd)
        self.kwargs.append(kwargs)
        joined = " ".join(cmd)
        for prefix, (code, out) in self.answers.items():
            if joined.startswith(prefix):
                return subprocess.CompletedProcess(cmd, code, out, "")
        return subprocess.CompletedProcess(cmd, 0, "", "")


# ---- interface --------------------------------------------------------------------------------


@pytest.mark.parametrize(
    ("env", "variable"),
    [
        (_env(PORTABILITY_TAG="<unset>"), "PORTABILITY_TAG"),
        (_env(PORTABILITY_TAG="0.1.0"), "PORTABILITY_TAG"),
        (_env(PORTABILITY_TAG="server/v0.1.0"), "PORTABILITY_TAG"),
        (_env(PORTABILITY_TAG="v0.1.0-rc.1"), "PORTABILITY_TAG"),
        (_env(PORTABILITY_INSTALL_SOURCE="<unset>"), "PORTABILITY_INSTALL_SOURCE"),
        (_env(PORTABILITY_INSTALL_SOURCE="curl"), "PORTABILITY_INSTALL_SOURCE"),
        (_env(GITHUB_REPOSITORY="<unset>"), "GITHUB_REPOSITORY"),
        (_env(GITHUB_REPOSITORY="alex/bothy/extra"), "GITHUB_REPOSITORY"),
        (_env(PORTABILITY_INSTALL_SOURCE="release-download"), "PORTABILITY_RELEASE_DIR"),
        (
            _env(
                PORTABILITY_INSTALL_SOURCE="release-download",
                PORTABILITY_RELEASE_DIR="/nonexistent/x",
            ),
            "PORTABILITY_RELEASE_DIR",
        ),
        (
            _env(GITHUB_ACTIONS="true", PORTABILITY_TRIAL_PUBLIC_KEY="/tmp/k"),
            "PORTABILITY_TRIAL_PUBLIC_KEY",
        ),
        (
            _env(GITHUB_ACTIONS="true", PORTABILITY_TRIAL_FAIL_AFTER_INSTALL="1"),
            "PORTABILITY_TRIAL_FAIL_AFTER_INSTALL",
        ),
        (_env(PORTABILITY_TRIAL_STORAGES="nfs"), "PORTABILITY_TRIAL_STORAGES"),
        (_env(PORTABILITY_TRIAL_FAIL_AFTER_INSTALL="yes"), "PORTABILITY_TRIAL_FAIL_AFTER_INSTALL"),
    ],
)
def test_invalid_value_aborts_and_names_the_variable(env: dict[str, str], variable: str) -> None:
    with pytest.raises(pt.InterfaceError, match=variable):
        pt.read_interface(env)


def test_release_dir_without_the_bundle_names_the_variable(tmp_path: Path) -> None:
    d = _release_dir(tmp_path)
    (d / f"bothy-compose-{VERSION}.tar.gz").unlink()
    env = _env(PORTABILITY_INSTALL_SOURCE="release-download", PORTABILITY_RELEASE_DIR=str(d))
    with pytest.raises(pt.InterfaceError, match="PORTABILITY_RELEASE_DIR has no bothy-compose"):
        pt.read_interface(env)


def test_url_reads_no_release_dir() -> None:
    iface = pt.read_interface(_env(PORTABILITY_RELEASE_DIR="/nonexistent/x"))
    assert iface.release_dir is None
    assert iface.repository == "alex/bothy"
    assert iface.storages == ("fs", "s3")


def test_release_download(tmp_path: Path) -> None:
    d = _release_dir(tmp_path)
    env = _env(
        PORTABILITY_INSTALL_SOURCE="release-download",
        PORTABILITY_RELEASE_DIR=str(d),
        GITHUB_REPOSITORY="<unset>",
    )
    iface = pt.read_interface(env)
    assert iface.release_dir == d.resolve()
    assert iface.repository is None
    assert iface.version == VERSION


def test_beta_tag() -> None:
    assert pt.read_interface(_env(PORTABILITY_TAG="v1.2.3-beta.4")).version == "1.2.3-beta.4"


def test_main_aborts_before_any_command(monkeypatch: pytest.MonkeyPatch) -> None:
    rec = Recorder()

    def no_download(*_: Any) -> None:
        raise AssertionError("downloaded although the interface is invalid")

    monkeypatch.setattr(pt, "_download", no_download)
    assert pt.main(_env(PORTABILITY_INSTALL_SOURCE="ftp"), rec) == 2
    assert rec.calls == []


def test_release_base_from_repository_and_tag() -> None:
    assert (
        pt.release_base("alex/bothy", "v0.1.0")
        == "https://github.com/alex/bothy/releases/download/v0.1.0/"
    )


def test_download_refuses_another_address(tmp_path: Path) -> None:
    with pytest.raises(pt.StepError, match="refusing"):
        pt._download("http://example.com/install.sh", tmp_path / "x")


# ---- options ----------------------------------------------------------------------------------


def test_options_come_from_the_file() -> None:
    opts = pt.read_options()
    assert opts[:1] == ["--yes"]
    assert "--storage" in opts
    assert not any(o.startswith("--bundle") or o.startswith("--dir") for o in opts)


def test_options_file_with_two_lines(tmp_path: Path) -> None:
    f = tmp_path / "opts"
    f.write_text("# c\n--yes\n--storage fs\n", encoding="utf-8")
    with pytest.raises(pt.StepError, match="exactly one line"):
        pt.read_options(f)


def test_install_args_sets_storage_and_appends_bundle() -> None:
    opts = ["--yes", "--storage", "s3", "--owner-handle", "alex"]
    assert pt.install_args(opts, "fs", None) == [
        "--yes",
        "--storage",
        "fs",
        "--owner-handle",
        "alex",
    ]
    assert pt.install_args(opts, "s3", Path("/b.tgz"))[-2:] == ["--bundle", "/b.tgz"]
    assert opts[2] == "s3"


@pytest.mark.parametrize(
    "opts", [["--yes"], ["--yes", "--storage"], ["--storage", "fs", "--bundle=x"]]
)
def test_install_args_refuses(opts: list[str]) -> None:
    with pytest.raises(pt.StepError):
        pt.install_args(opts, "fs", None)


def _ctx(tmp_path: Path, source: str) -> pt.Context:
    iface = pt.Interface(TAG, VERSION, source, repository="alex/bothy" if source == "url" else None)
    release = pt.Release(
        tmp_path / "install.sh",
        tmp_path / "b.tgz",
        "ghcr.io/alex",
        "1.0.0",
        ("bothy-server:1.0.0",),
    )
    return pt.Context(iface, tmp_path, release, "probe:x", None, {})


CODE = "en_ABCD-EFGH-JKMN-PQRS-TVWX-YZ01-2345-6789"


def _installer(code_out: str) -> Recorder:
    rec = Recorder()

    def run(args: Sequence[str], **kwargs: Any) -> subprocess.CompletedProcess[str]:
        rec(args, **kwargs)
        kwargs["stdout"].write(code_out)
        return subprocess.CompletedProcess(list(args), 0, None, None)

    rec.run = run  # type: ignore[attr-defined]
    return rec


@pytest.mark.parametrize("source", ["url", "release-download"])
def test_install_uses_exactly_the_options_of_the_file(tmp_path: Path, source: str) -> None:
    rec = _installer(f"Enrollment code (shown once): {CODE}\n")
    inst = pt.Installation(tmp_path, "s3", "p", {}, rec.run)  # type: ignore[attr-defined]
    assert pt.install(_ctx(tmp_path, source), inst) == CODE
    cmd = rec.calls[0]
    want = pt.install_args(pt.read_options(), "s3", None)
    if source == "url":
        assert cmd[:3] == ["sh", "-c", 'curl -fsSL "$0" | sh -s -- "$@"']
        assert cmd[3] == "https://github.com/alex/bothy/releases/download/v0.1.0/install.sh"
        assert cmd[4:] == want
    else:
        assert cmd[:2] == ["sh", str(tmp_path / "install.sh")]
        assert cmd[2:] == [*want, "--bundle", str(tmp_path / "b.tgz")]
    assert rec.kwargs[0]["start_new_session"] is True
    assert not (tmp_path / "install.out").exists()


def test_install_without_a_code_fails(tmp_path: Path) -> None:
    rec = _installer("nothing\n")
    inst = pt.Installation(tmp_path, "fs", "p", {}, rec.run)  # type: ignore[attr-defined]
    with pytest.raises(pt.StepError, match="0 enrollment codes"):
        pt.install(_ctx(tmp_path, "url"), inst)


def test_enrollment_code() -> None:
    assert pt.enrollment_code(f"x\nEnrollment code (shown once): {CODE}\n") == CODE
    with pytest.raises(pt.StepError, match="2 enrollment codes"):
        pt.enrollment_code(f"{CODE}\n{CODE[:-1]}A\n")


# ---- bundle and images -----------------------------------------------------------------------


def _bundle(env: str, compose: str) -> bytes:
    buf = io.BytesIO()
    with tarfile.open(fileobj=buf, mode="w:gz") as tar:
        for name, text in ((".env.example", env), ("compose.yaml", compose)):
            data = text.encode("utf-8")
            info = tarfile.TarInfo(f"bothy-{VERSION}/infra/compose/{name}")
            info.size = len(data)
            tar.addfile(info, io.BytesIO(data))
    return buf.getvalue()


COMPOSE = """
services:
  api:
    image: ${BOTHY_REGISTRY}/bothy-server:${BOTHY_VERSION}
  postgres:
    image: ${BOTHY_REGISTRY}/bothy-postgres:pg18-${BOTHY_VERSION}
  clamav:
    image: clamav/clamav-debian:1.5.4
"""


def test_bundle_values() -> None:
    data = _bundle("BOTHY_REGISTRY=127.0.0.1:5000\nBOTHY_VERSION=1.2.0\n", COMPOSE)
    registry, server, images = pt.bundle_values(data, VERSION)
    assert (registry, server) == ("127.0.0.1:5000", "1.2.0")
    assert images == ("bothy-postgres:pg18-1.2.0", "bothy-server:1.2.0")


def test_bundle_without_registry() -> None:
    with pytest.raises(pt.StepError, match="BOTHY_REGISTRY"):
        pt.bundle_values(_bundle("BOTHY_VERSION=1.2.0\n", COMPOSE), VERSION)


def test_repository_of_and_pulled_digest() -> None:
    assert pt.repository_of("127.0.0.1:5000/bothy-server:1.0.0") == "127.0.0.1:5000/bothy-server"
    assert (
        pt.repository_of("ghcr.io/alex/bothy-postgres:pg18-1.0.0") == "ghcr.io/alex/bothy-postgres"
    )
    assert pt.repository_of("bothy-server:1.0.0") == "bothy-server"
    d = "sha256:" + "a" * 64
    out = f"1.0.0: Pulling from bothy-server\nDigest: {d}\nStatus: Image is up to date\n"
    assert pt.pulled_digest("r/bothy-server:1.0.0", out) == d
    with pytest.raises(pt.StepError, match="0 digests"):
        pt.pulled_digest("r/bothy-server:1.0.0", "Status: Downloaded\n")


def test_cosign_commands_key_then_keyless() -> None:
    cmds = pt.cosign_commands("r@sha256:x", Path("/k.pub"), "alex/bothy")
    assert cmds[0] == ["verify", "--key", "/key.pub", "--insecure-ignore-tlog", "r@sha256:x"]
    assert "--certificate-identity-regexp" in cmds[1]
    ident = cmds[1][cmds[1].index("--certificate-identity-regexp") + 1]
    assert (
        ident
        == r"^https://github\.com/alex/bothy/\.github/workflows/release\.yml@refs/tags/server/v"
    )
    assert pt.cosign_commands("r", None, None) == []


def _verify_answers(sig_ok: bool) -> dict[str, tuple[int, str]]:
    d = "sha256:" + "c" * 64
    return {
        "docker pull": (0, f"1.0.0: Pulling from alex/bothy-server\nDigest: {d}\n"),
        "docker image inspect": (0, f"sha256:{'9' * 64}"),
        "docker run --rm --network host": (0 if sig_ok else 1, ""),
    }


def test_verify_takes_the_digest_of_this_pull_not_an_older_repo_digest(tmp_path: Path) -> None:
    rec = Recorder(_verify_answers(sig_ok=True))
    rel = pt.Release(tmp_path, tmp_path, "ghcr.io/alex", "1.0.0", ("bothy-server:1.0.0",))
    out = pt.verify_images(rel, None, "alex/bothy", rec)
    cosign = next(c for c in rec.calls if c[:2] == ["docker", "run"])
    assert cosign[-1] == "ghcr.io/alex/bothy-server@sha256:" + "c" * 64
    assert out[0].digest == "sha256:" + "c" * 64


def test_verify_images_passes_and_pins_cosign(tmp_path: Path) -> None:
    rec = Recorder(_verify_answers(sig_ok=True))
    rel = pt.Release(tmp_path, tmp_path, "ghcr.io/alex", "1.0.0", ("bothy-server:1.0.0",))
    out = pt.verify_images(rel, None, "alex/bothy", rec)
    assert out == [
        pt.Verified("ghcr.io/alex/bothy-server:1.0.0", "sha256:" + "c" * 64, "sha256:" + "9" * 64)
    ]
    cosign = [c for c in rec.calls if c[:2] == ["docker", "run"]]
    assert cosign
    assert all(pt.COSIGN_IMAGE in c for c in cosign)
    assert "@sha256:" in pt.COSIGN_IMAGE
    assert ["docker", "login"] not in [c[:2] for c in rec.calls]


def test_unsigned_image_fails_before_anything_else(tmp_path: Path) -> None:
    rec = Recorder(_verify_answers(sig_ok=False))
    rel = pt.Release(
        tmp_path,
        tmp_path,
        "ghcr.io/alex",
        "1.0.0",
        ("bothy-server:1.0.0", "bothy-postgres:pg18-1.0.0"),
    )
    with pytest.raises(pt.StepError, match="carries no valid signature"):
        pt.verify_images(rel, Path("/k.pub"), "alex/bothy", rec)
    # key and keyless were tried for the first image, the second image was never pulled
    assert sum(1 for c in rec.calls if c[:2] == ["docker", "run"]) == 2
    assert not any("bothy-postgres" in " ".join(c) for c in rec.calls)


def test_verify_needs_a_way(tmp_path: Path) -> None:
    rel = pt.Release(tmp_path, tmp_path, "r", "1", ("a:1",))
    with pytest.raises(pt.StepError, match="no way to verify"):
        pt.verify_images(rel, None, None, Recorder())


V = [
    pt.Verified("r:5000/bothy-server:1", "sha256:a", "id-server"),
    pt.Verified("r:5000/bothy-postgres:pg18-1", "sha256:b", "id-pg"),
]


def test_compare_running_passes() -> None:
    running = {
        "api/x": ("r:5000/bothy-server:1", "id-server"),
        "postgres/y": ("r:5000/bothy-postgres:pg18-1", "id-pg"),
        "clamav/z": ("clamav/clamav-debian:1.5.4", "other"),
    }
    pt.compare_running(V, running)


@pytest.mark.parametrize(
    ("containers", "msg"),
    [
        (
            {
                "api/x": ("r:5000/bothy-server:1", "id-other"),
                "postgres/y": ("r:5000/bothy-postgres:pg18-1", "id-pg"),
            },
            "not the verified",
        ),
        (
            {
                "api/x": ("r:5000/bothy-server:2", "id-server"),
                "postgres/y": ("r:5000/bothy-postgres:pg18-1", "id-pg"),
            },
            "not a verified reference",
        ),
        ({"api/x": ("r:5000/bothy-server:1", "id-server")}, "no container runs"),
    ],
)
def test_compare_running_fails(containers: dict[str, tuple[str, str]], msg: str) -> None:
    with pytest.raises(pt.StepError, match=msg):
        pt.compare_running(V, containers)


# ---- environment, tailnet --------------------------------------------------------------------


def test_clean_env() -> None:
    env = pt.clean_env(
        {
            "PATH": "/bin",
            "BOTHY_VERSION": "x",
            "COMPOSE_PROFILES": "y",
            "OTEL_X": "z",
            "TS_AUTHKEY": "k",
        }
    )
    assert env == {"PATH": "/bin"}


PROC_TCP = """  sl  local_address rem_address   st
   0: 0100007F:1F90 00000000:0000 0A
   1: 00000000:1F91 00000000:0000 0A
   2: 0B00007F:A1B2 00000000:0000 0A
   3: 0100007F:1F90 0100007F:C350 01
"""


def test_tailnet_public_listener_on_loopback_only() -> None:
    pt.check_tailnet(PROC_TCP)


def test_tailnet_exposed_port_fails() -> None:
    with pytest.raises(pt.StepError, match=r"\[8080\] listen beyond loopback"):
        pt.check_tailnet(PROC_TCP + "   4: 0200000A:1F90 00000000:0000 0A\n")
    with pytest.raises(pt.StepError, match=r"\[8082\] listen beyond loopback"):
        pt.check_tailnet(
            PROC_TCP + "   5: 00000000000000000000000000000000:1F92 00000000:0000 0A\n"
        )


def test_tailnet_listeners_must_exist() -> None:
    with pytest.raises(pt.StepError, match="public listener"):
        pt.check_tailnet(
            PROC_TCP.replace("0100007F:1F90 00000000:0000 0A", "0100007F:1F99 00000000:0000 0A")
        )
    with pytest.raises(pt.StepError, match="internal listener"):
        pt.check_tailnet(PROC_TCP.replace("   1: 00000000:1F91 00000000:0000 0A\n", ""))


def test_is_loopback() -> None:
    assert pt.is_loopback("0100007F")
    assert pt.is_loopback("0B00007F")
    assert pt.is_loopback("00000000000000000000000001000000")
    assert not pt.is_loopback("00000000")
    assert not pt.is_loopback("0100000A")


# ---- failure path ------------------------------------------------------------------------------


def test_failure_prints_state_and_logs_before_the_cleanup(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    rec = Recorder()
    ctx = _ctx(tmp_path, "release-download")
    ctx = pt.Context(
        pt.Interface(TAG, VERSION, "release-download", fail_after_install=True),
        tmp_path,
        ctx.release,
        "probe:x",
        None,
        {},
        rec,
    )
    monkeypatch.setattr(pt, "verify_images", lambda *_: [])

    def fake_install(_: pt.Context, inst: pt.Installation) -> str:
        inst.compose_dir.mkdir(parents=True)
        return CODE

    monkeypatch.setattr(pt, "install", fake_install)
    with pytest.raises(pt.StepError, match="forced failure"):
        pt.run_storage(ctx, "fs")
    compose = [c[2] for c in rec.calls if c[:2] == ["docker", "compose"]]
    assert compose[:4] == ["ps", "logs", "config", "down"]
    project = [
        k["env"]["COMPOSE_PROJECT_NAME"]
        for k, c in zip(rec.kwargs, rec.calls, strict=True)
        if c[:2] == ["docker", "compose"]
    ]
    assert len(set(project)) == 1
    assert project[0].startswith("bothy-portability-")
    assert not (tmp_path / "fs").exists()


def test_leftovers_fail_the_run(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    rec = Recorder({"docker volume ls": (0, "bothy-portability-x_pgdata\n")})
    base = _ctx(tmp_path, "release-download")
    ctx = pt.Context(
        pt.Interface(TAG, VERSION, "release-download", fail_after_install=True),
        tmp_path,
        base.release,
        "probe:x",
        None,
        {},
        rec,
    )
    monkeypatch.setattr(pt, "verify_images", lambda *_: [])
    monkeypatch.setattr(pt, "install", lambda *_: CODE)
    with pytest.raises(pt.StepError, match="cleanup left objects"):
        pt.run_storage(ctx, "fs")


def test_json_rows() -> None:
    assert pt._json_rows('{"ID":"a"}\n{"ID":"b"}\n') == [{"ID": "a"}, {"ID": "b"}]
    assert pt._json_rows('[{"ID":"a"}]') == [{"ID": "a"}]
    assert pt._json_rows("") == []


# ---- configuration keys of the installation ---------------------------------------------------


def test_compose_sets_only_known_server_keys() -> None:
    compose = (pt.ROOT / "infra" / "compose" / "compose.yaml").read_text(encoding="utf-8")
    config = (pt.ROOT / "docs" / "operations" / "configuration.md").read_text(encoding="utf-8")
    assert pt.unknown_server_keys(compose, config) == []


def test_unknown_server_key_is_found() -> None:
    config = (
        "| `file.scan.clamd_address` | `BOTHY_FILE__SCAN__CLAMD_ADDRESS` | S | s | x | active |\n"
        "| `file.later` | `BOTHY_FILE__LATER` | Server | string | x | y | planned (M9) |\n"
    )
    compose = """
x-env: &env
  BOTHY_FILE__SCAN__CLAMD_ADDRESS: a
  BOTHY_FILE__LATER: b
  BOTHY_TRUSTED_PROXIES: c
services:
  api:
    image: r/bothy-server:1
    environment:
      <<: *env
      BOTHY_SERVER__TYPO: d
  postgres:
    image: r/bothy-postgres:pg18-1
    environment:
      BOTHY_OTHER__KEY: e
"""
    assert pt.unknown_server_keys(compose, config) == ["BOTHY_FILE__LATER", "BOTHY_SERVER__TYPO"]


def test_down_activates_every_profile(tmp_path: Path) -> None:
    rec = Recorder(
        {"docker compose config --profiles": (0, "direct\nobservability\ns3\nscan\ntailscale\n")}
    )
    inst = pt.Installation(tmp_path, "fs", "bothy-portability-x", {}, rec, profiles="direct,scan")
    inst.compose_dir.mkdir(parents=True)
    assert inst.down() == []
    down = [
        k
        for c, k in zip(rec.calls, rec.kwargs, strict=True)
        if c[:3] == ["docker", "compose", "down"]
    ]
    assert down[0]["env"]["COMPOSE_PROFILES"] == "direct,observability,s3,scan,tailscale"


def test_release_download_takes_the_repository_for_keyless(tmp_path: Path) -> None:
    d = _release_dir(tmp_path)
    env = _env(PORTABILITY_INSTALL_SOURCE="release-download", PORTABILITY_RELEASE_DIR=str(d))
    assert pt.read_interface(env).repository == "alex/bothy"
    with pytest.raises(pt.InterfaceError, match="GITHUB_REPOSITORY"):
        pt.read_interface({**env, "GITHUB_REPOSITORY": "not a repo"})


def test_down_without_the_folder_only_looks_for_leftovers(tmp_path: Path) -> None:
    rec = Recorder()
    inst = pt.Installation(tmp_path / "never", "fs", "bothy-portability-x", {}, rec)
    assert inst.down() == []
    assert not any(c[:2] == ["docker", "compose"] for c in rec.calls)
    assert any(c[:3] == ["docker", "volume", "ls"] for c in rec.calls)


def test_backup_kinds() -> None:
    assert pt.backup_kinds("fs") == ("pg_full", "logical", "objects")
    assert pt.backup_kinds("s3") == ("pg_full", "logical")


def test_published_port_from_compose(tmp_path: Path) -> None:
    rec = Recorder({"docker compose port api 8080": (0, "127.0.0.1:49722\n")})
    inst = pt.Installation(tmp_path, "fs", "p", {}, rec)
    assert inst.published_port() == 49722
    bad = pt.Installation(tmp_path, "fs", "p", {}, Recorder({"docker compose port": (0, "")}))
    with pytest.raises(pt.StepError, match="no host port"):
        bad.published_port()


def test_run_publishes_every_port_as_zero(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    rec = Recorder()
    base = _ctx(tmp_path, "release-download")
    ctx = pt.Context(
        pt.Interface(TAG, VERSION, "release-download", fail_after_install=True),
        tmp_path,
        base.release,
        "probe:x",
        None,
        {},
        rec,
    )
    monkeypatch.setattr(pt, "verify_images", lambda *_: [])
    seen: dict[str, str] = {}

    def fake_install(_: pt.Context, inst: pt.Installation) -> str:
        seen.update(inst.env)
        return CODE

    monkeypatch.setattr(pt, "install", fake_install)
    with pytest.raises(pt.StepError):
        pt.run_storage(ctx, "fs")
    assert all(seen[name] == "0" for name in pt.PORT_VARIABLES)


# ---- VZ-T13: the Grafana panel plugin in the overlay local (brief M7-26) ---------------------

GRAFANA_LOG = "".join(
    line + "\n"
    for line in (
        'logger=settings level=info msg="Config overridden from Environment variable"'
        ' var="GF_PLUGINS_ALLOW_LOADING_UNSIGNED_PLUGINS=bothy-live-panel"',
        'logger=plugins.signature level=warn msg="Plugin is unsigned" id=bothy-live-panel'
        ' err="plugin is unsigned: could not find a MANIFEST.txt"',
        'logger=plugin.signature.validator level=warn msg="Permitting unsigned plugin.'
        ' This is not recommended" pluginId=bothy-live-panel',
        'logger=provisioning.dashboard level=info msg="finished to provision dashboards"',
    )
)
PANELS = [
    {"id": "bothy-live-panel", "signature": "unsigned"},
    {"id": "timeseries", "signature": "internal"},
]
DASHBOARD = {
    "meta": {"provisioned": True},
    "dashboard": {
        "uid": "bothy-live",
        "panels": [
            {"type": "bothy-live-panel"},
            {"type": "row", "panels": [{"type": "bothy-live-panel"}]},
        ],
    },
}


def test_grafana_check_passes_and_reports_the_unsigned_lines() -> None:
    lines = pt.check_grafana(PANELS, [], DASHBOARD, GRAFANA_LOG)
    assert lines == [
        line
        for line in GRAFANA_LOG.splitlines()
        if "id=bothy-live-panel" in line or "pluginId=bothy-live-panel" in line
    ]
    assert len(lines) == 2


@pytest.mark.parametrize(
    ("panels", "errors", "dashboard", "log", "named"),
    [
        ([PANELS[1]], [], DASHBOARD, GRAFANA_LOG, "bothy-live-panel is not loaded"),
        (
            [{"id": "bothy-live-panel", "signature": "valid"}],
            [],
            DASHBOARD,
            GRAFANA_LOG,
            "signature 'valid'",
        ),
        (PANELS, [{"pluginId": "x", "errorCode": "signatureMissing"}], DASHBOARD, GRAFANA_LOG, "x"),
        (
            PANELS,
            [],
            DASHBOARD,
            GRAFANA_LOG + 'level=warn msg="Plugin is unsigned" id=other-panel\n',
            "other-panel",
        ),
        (
            PANELS,
            [],
            DASHBOARD,
            GRAFANA_LOG + 'level=warn msg="Permitting unsigned plugin" pluginId="grafana-x-app"\n',
            "grafana-x-app",
        ),
        (PANELS, [], DASHBOARD, "", "names no unsigned plugin"),
        (PANELS, [], {"meta": {}, "dashboard": {}}, GRAFANA_LOG, "bothy-live"),
        (
            PANELS,
            [],
            {"meta": {"provisioned": True}, "dashboard": {"uid": "bothy-live", "panels": []}},
            GRAFANA_LOG,
            "no panel",
        ),
        (
            PANELS,
            [],
            {
                "meta": {"provisioned": True},
                "dashboard": {"uid": "bothy-live", "panels": [{"type": "missing-panel"}]},
            },
            GRAFANA_LOG,
            "missing-panel",
        ),
        (
            PANELS,
            [],
            DASHBOARD,
            GRAFANA_LOG + 'logger=provisioning.dashboard level=error msg="failed" file=x\n',
            "provisioning.dashboard",
        ),
    ],
)
def test_grafana_check_fails(
    panels: list[dict[str, Any]],
    errors: list[dict[str, Any]],
    dashboard: dict[str, Any],
    log: str,
    named: str,
) -> None:
    with pytest.raises(pt.StepError, match=re.escape(named)):
        pt.check_grafana(panels, errors, dashboard, log)


class GrafanaFake:
    """Answers the Compose commands of the Grafana step; records them."""

    def __init__(self) -> None:
        self.calls: list[list[str]] = []

    def __call__(self, args: Sequence[str], **kwargs: Any) -> subprocess.CompletedProcess[str]:
        cmd = list(args)
        self.calls.append(cmd)
        out = ""
        if cmd[2:5] == ["exec", "-T", "grafana"]:
            path = cmd[-1]
            out = {
                "/api/plugins?type=panel": json.dumps(PANELS),
                "/api/plugins/errors": "[]",
                "/api/dashboards/uid/bothy-live": json.dumps(DASHBOARD),
            }.get(path, "")
        elif cmd[2:3] == ["logs"]:
            out = GRAFANA_LOG
        return subprocess.CompletedProcess(cmd, 0, out, "")


def test_grafana_step_asks_the_api_on_loopback_without_a_secret_in_the_arguments(
    tmp_path: Path,
) -> None:
    fake = GrafanaFake()
    inst = pt.Installation(tmp_path, "fs", "p", {}, fake)
    lines = pt.grafana_check(inst)
    assert len(lines) == 2
    assert fake.calls[0][2:] == [
        "up", "--detach", "--wait", "--wait-timeout", "300", "--no-deps", "grafana",
    ]  # fmt: skip
    api = [c for c in fake.calls if c[2:5] == ["exec", "-T", "grafana"]]
    assert {c[-1] for c in api} == {
        "/api/health",
        "/api/plugins?type=panel",
        "/api/plugins/errors",
        "/api/dashboards/uid/bothy-live",
        "/public/plugins/bothy-live-panel/module.js",
    }
    for c in api:
        assert "http://127.0.0.1:3000" in " ".join(c)
        assert "/run/bothy/secrets/grafana_admin_password" in " ".join(c)
        assert all("admin:" not in arg or "$(" in arg for arg in c)
    assert fake.calls[-1][2:] == ["logs", "--no-color", "grafana"]
