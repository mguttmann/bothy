# SPDX-License-Identifier: Apache-2.0
"""Tests for `make release-trial` (tools/ci/release_trial.py, D-W-127).

Argument checks, the refusal under GitHub Actions, the Compose image list and the order and
cleanup of the trial run against a fake docker. The abort without an SBOM and the checksum and
signature negatives of the chain itself are in test_release_chain.py; the trial repeats them
as probes against the real images (`make release-trial COMPONENT=server`).
"""

from __future__ import annotations

import hashlib
import json
import subprocess
import tarfile
import types
from collections.abc import Sequence
from pathlib import Path

import plugin_release
import pytest
import release_chain as chain
import release_trial as trial

REPO = Path(__file__).resolve().parents[3]
DIGEST = "sha256:" + "c" * 64


class FakeDocker:
    """Answers the docker, git and make commands of a trial; fails at `fail_at` if set."""

    def __init__(self, workdir: Path, fail_at: tuple[str, ...] | None = None) -> None:
        self.workdir = workdir
        self.fail_at = fail_at
        self.calls: list[list[str]] = []

    def __call__(
        self, cmd: Sequence[str], *, env: dict[str, str] | None = None, cwd: Path | None = None
    ) -> subprocess.CompletedProcess[str]:
        cmd = list(cmd)
        self.calls.append(cmd)
        if self.fail_at and tuple(cmd[: len(self.fail_at)]) == self.fail_at:
            raise chain.ChainError(f"{' '.join(self.fail_at)} failed")
        out = ""
        if cmd[:2] == ["docker", "run"]:
            out = "cid123\n"
        elif cmd[:2] == ["docker", "port"]:
            out = "127.0.0.1:45555\n"
        elif cmd[:3] == ["docker", "buildx", "build"]:
            meta = Path(cmd[cmd.index("--metadata-file") + 1])
            meta.write_text(json.dumps({"containerimage.digest": DIGEST}))
        elif cmd[:2] == ["git", "-C"]:
            out = "0" * 40 + "\n"
        elif cmd[0] == "make" and cmd[2].endswith(trial.PLUGIN_COMPONENT_DIR):
            fake_plugin_build(cmd, self.workdir)
        return subprocess.CompletedProcess(cmd, 0, out, "")

    def ran(self, *prefix: str) -> bool:
        return any(tuple(c[: len(prefix)]) == prefix for c in self.calls)


def fake_plugin_build(cmd: Sequence[str], root: Path) -> Path:
    """What `make -C plugins/grafana-bothy-live release-build VERSION=<v> OUT=<dir>` writes."""
    args = dict(a.split("=", 1) for a in cmd if "=" in a)
    dist = root / f"fake-plugin-dist-{args['VERSION']}"
    if not dist.exists():
        dist.mkdir()
        doc = {"type": "panel", "id": "bothy-live-panel", "info": {"version": args["VERSION"]}}
        (dist / "plugin.json").write_text(json.dumps(doc), encoding="utf-8")
        (dist / "module.js").write_text("define([],function(){})\n", encoding="utf-8")
    return plugin_release.pack(dist, args["VERSION"], Path(args["OUT"]))


def test_refuses_under_github_actions(
    monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    monkeypatch.setenv("GITHUB_ACTIONS", "true")
    fake = FakeDocker(Path())
    tools = chain.Tools("cosign", "syft", runner=fake)
    assert trial.main(["--component", "server"], tools) == 2
    assert "refuses to run under GitHub Actions" in capsys.readouterr().err
    assert fake.calls == []


def test_make_target_refuses_under_github_actions() -> None:
    proc = subprocess.run(
        ["make", "-s", "-C", str(REPO), "release-trial", "COMPONENT=server"],
        env={"PATH": "/usr/bin:/bin", "GITHUB_ACTIONS": "true"},
        capture_output=True,
        text=True,
        check=False,
    )
    assert proc.returncode == 2
    assert "refuses to run under GitHub Actions" in proc.stderr


@pytest.mark.parametrize(
    ("args", "message"),
    [
        (["--component", "client"], "has no trial yet"),
        (["--component", "server", "--platforms", "linux/s390x"], "subset of"),
        (["--component", "server", "--platforms", ","], "subset of"),
    ],
)
def test_argument_checks(
    args: list[str],
    message: str,
    monkeypatch: pytest.MonkeyPatch,
    capsys: pytest.CaptureFixture[str],
) -> None:
    monkeypatch.delenv("GITHUB_ACTIONS", raising=False)
    fake = FakeDocker(Path())
    assert trial.main(args, chain.Tools("cosign", "syft", runner=fake)) == 2
    assert message in capsys.readouterr().err
    assert fake.calls == []


def test_parse_platforms() -> None:
    assert trial.parse_platforms(" linux/arm64 ") == ["linux/arm64"]
    assert trial.parse_platforms("linux/amd64,linux/arm64") == ["linux/amd64", "linux/arm64"]


def test_compose_images_are_the_released_ones() -> None:
    found = trial.compose_images(REPO / "infra/compose/compose.yaml", "1.2.3")
    assert found == {"bothy-server": "1.2.3", "bothy-postgres": "pg18-1.2.3"}
    assert found == trial.ServerImages.tags("1.2.3")


def test_manifest_digest_only_for_the_local_registry() -> None:
    with pytest.raises(trial.TrialError, match=r"outside 127\.0\.0\.1"):
        trial.manifest_digest("ghcr.io", "bothy-server", "1.2.3")


def run_fake_trial(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, fake: FakeDocker
) -> trial.Trial:
    """A Trial whose chain steps are stubbed; returns it after build, sign and tag."""
    monkeypatch.setattr(trial.Trial, "_wait_for", staticmethod(lambda host: None))
    monkeypatch.setattr(chain, "join", lambda tools, repo, digests: DIGEST)
    tools = chain.Tools("cosign", "syft", runner=fake)
    t = trial.Trial(tools, tmp_path, ["linux/amd64", "linux/arm64"])
    with t.local_registry(), t.builder_instance():
        images = t.build()
        assert [i.ref for i in images] == [
            f"127.0.0.1:45555/bothy-server@{DIGEST}",
            f"127.0.0.1:45555/bothy-postgres@{DIGEST}",
        ]
    return t


def test_build_order_and_cleanup(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    fake = FakeDocker(tmp_path)
    run_fake_trial(tmp_path, monkeypatch, fake)
    builds = [c for c in fake.calls if c[:3] == ["docker", "buildx", "build"]]
    assert [c[c.index("--platform") + 1] for c in builds] == [
        "linux/amd64,linux/arm64",
        "linux/amd64",
        "linux/arm64",
    ]
    assert all("registry.insecure=true" in c[c.index("--output") + 1] for c in builds)
    assert all(
        c[c.index("--output") + 1].startswith("type=image,name=127.0.0.1:45555/") for c in builds
    )
    assert fake.calls[0][:5] == ["docker", "run", "-d", "--rm", "-p"]
    assert fake.calls[0][5] == "127.0.0.1::5000"
    assert fake.calls[0][6] == trial.REGISTRY_IMAGE
    assert fake.calls[-2][:3] == ["docker", "buildx", "rm"]
    assert fake.calls[-1] == ["docker", "stop", "cid123"]
    make = next(c for c in fake.calls if c[0] == "make")
    assert make[3:] == [
        "release-build",
        f"VERSION={trial.Trial(chain.Tools('c', 's'), tmp_path, []).version}",
        f"OUT={tmp_path / 'dist'}",
    ]


def test_failure_still_removes_builder_and_registry(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    fake = FakeDocker(tmp_path, fail_at=("docker", "buildx", "build"))
    with pytest.raises(chain.ChainError, match="buildx build failed"):
        run_fake_trial(tmp_path, monkeypatch, fake)
    assert fake.ran("docker", "buildx", "rm")
    assert fake.calls[-1] == ["docker", "stop", "cid123"]


def test_push_needs_the_local_registry(tmp_path: Path) -> None:
    t = trial.Trial(chain.Tools("c", "s", runner=FakeDocker(tmp_path)), tmp_path, ["linux/amd64"])
    t.registry = "ghcr.io/owner"
    with pytest.raises(trial.TrialError, match="only to the local registry"):
        t._push("services/server/Dockerfile", "bothy-server", ["linux/amd64"])


def test_tag_and_check_compares_the_compose_images(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    fake = FakeDocker(tmp_path)
    t = trial.Trial(chain.Tools("cosign", "syft", runner=fake), tmp_path, ["linux/amd64"])
    t.registry = "127.0.0.1:45555"
    images = [
        chain.Image("bothy-server", "127.0.0.1:45555/bothy-server", DIGEST),
        chain.Image("bothy-postgres", "127.0.0.1:45555/bothy-postgres", DIGEST),
    ]
    monkeypatch.setattr(trial, "manifest_digest", lambda registry, name, ref: DIGEST)
    refs = t.tag_and_check(images)
    assert refs == {
        "bothy-server": f"127.0.0.1:45555/bothy-server:{t.version}",
        "bothy-postgres": f"127.0.0.1:45555/bothy-postgres:pg18-{t.version}",
    }
    monkeypatch.setattr(trial, "manifest_digest", lambda registry, name, ref: None)
    with pytest.raises(trial.TrialError, match="the signed digest is"):
        t.tag_and_check(images)
    with pytest.raises(trial.TrialError, match="does not build"):
        t.tag_and_check(images[:1])


def test_probe_that_passes_is_an_error() -> None:
    with pytest.raises(trial.TrialError, match="probe passed although it must fail: x"):
        trial.Trial._must_fail("x", lambda: None)
    trial.Trial._must_fail("y", lambda: (_ for _ in ()).throw(chain.ChainError("no")))


def test_manifest_digest_of_a_served_manifest(monkeypatch: pytest.MonkeyPatch) -> None:
    body = b'{"schemaVersion": 2}'

    class Resp:
        def __enter__(self) -> Resp:
            return self

        def __exit__(self, *a: object) -> None:
            return None

        def read(self) -> bytes:
            return body

    monkeypatch.setattr(trial.urllib.request, "urlopen", lambda req, timeout: Resp())
    want = "sha256:" + hashlib.sha256(body).hexdigest()
    assert trial.manifest_digest("127.0.0.1:45555", "bothy-server", "1.2.3") == want


def test_binaries_are_exactly_one_per_platform(tmp_path: Path) -> None:
    t = trial.Trial(chain.Tools("c", "s"), tmp_path, ["linux/amd64", "linux/arm64"])
    t.dist.mkdir()
    with pytest.raises(trial.TrialError, match="wrote no"):
        t.binaries()
    for name in ("amd64", "arm64"):
        (t.dist / f"bothy_{t.version}_linux_{name}").write_bytes(b"x")
        (t.dist / f"bothy_{t.version}_linux_{name}.spdx.json").write_text("{}")
    assert [b.name for b in t.binaries()] == [
        f"bothy_{t.version}_linux_amd64",
        f"bothy_{t.version}_linux_arm64",
    ]


# ---- the product (the root component, brief M7-29 Design note 3) ----------------------------


class GitPassThrough(FakeDocker):
    """FakeDocker that runs git for real, so the throwaway product repository exists."""

    def __call__(
        self, cmd: Sequence[str], *, env: dict[str, str] | None = None, cwd: Path | None = None
    ) -> subprocess.CompletedProcess[str]:
        if list(cmd[:1]) == ["git"]:
            self.calls.append(list(cmd))
            return subprocess.run(list(cmd), check=True, capture_output=True, text=True)
        return super().__call__(cmd, env=env, cwd=cwd)


def test_product_builds_against_the_local_registry_and_checks_like_an_install(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.delenv("GITHUB_ACTIONS", raising=False)
    fake = GitPassThrough(tmp_path)
    t = trial.Trial(chain.Tools("cosign", "syft", runner=fake), tmp_path, ["linux/amd64"])
    t.registry = "127.0.0.1:45555"
    images = [
        chain.Image("bothy-server", "127.0.0.1:45555/bothy-server", DIGEST),
        chain.Image("bothy-postgres", "127.0.0.1:45555/bothy-postgres", DIGEST),
    ]
    checked: list[list[str]] = []
    monkeypatch.setattr(
        trial.product_release, "check_images", lambda docker, target, refs: checked.append(refs)
    )
    monkeypatch.setattr(trial, "manifest_digest", lambda registry, name, ref: DIGEST)
    written = t.product(images)
    assert [p.name for p in written] == [f"bothy-compose-{t.version}.tar.gz", "install.sh"]
    assert checked == [
        [f"bothy-postgres:pg18-{t.version}", f"bothy-server:{t.version}"],
    ]
    pulls = [c for c in fake.calls if c[:2] == ["docker", "pull"]]
    assert [c[-1] for c in pulls] == [
        f"127.0.0.1:45555/bothy-postgres:pg18-{t.version}",
        f"127.0.0.1:45555/bothy-server:{t.version}",
    ]
    assert fake.ran("docker", "image", "rm")


def test_product_check_rejects_another_digest(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.delenv("GITHUB_ACTIONS", raising=False)
    fake = GitPassThrough(tmp_path)
    t = trial.Trial(chain.Tools("cosign", "syft", runner=fake), tmp_path, ["linux/amd64"])
    t.registry = "127.0.0.1:45555"
    images = [
        chain.Image("bothy-server", "127.0.0.1:45555/bothy-server", DIGEST),
        chain.Image("bothy-postgres", "127.0.0.1:45555/bothy-postgres", DIGEST),
    ]
    monkeypatch.setattr(trial.product_release, "check_images", lambda docker, target, refs: None)
    monkeypatch.setattr(trial, "manifest_digest", lambda registry, name, ref: "sha256:" + "d" * 64)
    with pytest.raises(trial.TrialError, match="not the signed digest"):
        t.product(images)


def test_fetch_only_from_loopback() -> None:
    with pytest.raises(trial.TrialError, match=r"outside 127\.0\.0\.1"):
        trial._fetch("https://example.com/install.sh")


def test_product_bundle_carries_the_plugin_zip_of_the_plugin_version(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """Brief M7-26: the product trial builds the plugin's release ZIP and the bundle carries it."""
    monkeypatch.delenv("GITHUB_ACTIONS", raising=False)
    fake = GitPassThrough(tmp_path)
    t = trial.Trial(chain.Tools("cosign", "syft", runner=fake), tmp_path, ["linux/amd64"])
    t.registry = "127.0.0.1:45555"
    images = [
        chain.Image("bothy-server", "127.0.0.1:45555/bothy-server", DIGEST),
        chain.Image("bothy-postgres", "127.0.0.1:45555/bothy-postgres", DIGEST),
    ]
    monkeypatch.setattr(trial.product_release, "check_images", lambda docker, target, refs: None)
    monkeypatch.setattr(trial, "manifest_digest", lambda registry, name, ref: DIGEST)
    written = t.product(images)
    plugin_version = (REPO / trial.PLUGIN_COMPONENT_DIR / "VERSION").read_text().strip()
    make = [c for c in fake.calls if c[0] == "make"]
    assert make == [
        [
            "make",
            "-C",
            str(REPO / trial.PLUGIN_COMPONENT_DIR),
            "release-build",
            f"VERSION={plugin_version}",
            f"OUT={tmp_path / 'plugin'}",
        ]
    ]
    with tarfile.open(written[0]) as archive:
        prefix = f"bothy-{(REPO / 'VERSION').read_text().strip()}/"
        member = archive.extractfile(prefix + trial.product_release.PLUGIN_DIR + "/plugin.json")
        assert member is not None
        assert json.loads(member.read())["info"]["version"] == plugin_version


# ---- the Grafana panel plugin (component grafana-bothy-live, brief M7-26) --------------------


class FakeChain(FakeDocker):
    """FakeDocker that also writes what syft and cosign would write, so the chain runs."""

    def __call__(
        self, cmd: Sequence[str], *, env: dict[str, str] | None = None, cwd: Path | None = None
    ) -> subprocess.CompletedProcess[str]:
        cmd = list(cmd)
        if cmd[:2] == ["syft", "scan"]:
            self.calls.append(cmd)
            for i, arg in enumerate(cmd):
                if arg == "-o":
                    Path(cmd[i + 1].split("=", 1)[1]).write_text('{"packages": []}')
            return subprocess.CompletedProcess(cmd, 0, "", "")
        if cmd[:1] == ["cosign"] and "--bundle" in cmd:
            self.calls.append(cmd)
            if cmd[1] == "sign-blob":
                Path(cmd[cmd.index("--bundle") + 1]).write_text("{}")
            return subprocess.CompletedProcess(cmd, 0, "", "")
        if cmd[:3] == ["cosign", "signing-config", "create"]:
            self.calls.append(cmd)
            Path(cmd[cmd.index("--out") + 1]).write_text("{}")
            return subprocess.CompletedProcess(cmd, 0, "", "")
        if cmd[:2] == ["cosign", "generate-key-pair"]:
            self.calls.append(cmd)
            for name in ("cosign.key", "cosign.pub"):
                (Path(cwd or ".") / name).write_text(name)
            return subprocess.CompletedProcess(cmd, 0, "", "")
        if cmd[:1] == ["sha256sum"]:
            self.calls.append(cmd)
            res = subprocess.run(cmd, cwd=cwd, check=False, capture_output=True, text=True)
            if res.returncode != 0:
                raise chain.ChainError(f"sha256sum failed: {res.stdout}")
            return res
        return super().__call__(cmd, env=env, cwd=cwd)


def test_plugin_trial_builds_signs_and_verifies_the_zip_with_its_sbom(tmp_path: Path) -> None:
    fake = FakeChain(tmp_path)
    tools = chain.Tools("cosign", "syft", runner=fake)
    p = trial.PluginTrial(tools, tmp_path)
    written = p.run()
    zipped = f"bothy-live-panel-{p.version}.zip"
    assert sorted(f.name for f in written) == sorted(
        [
            zipped,
            f"{zipped}.spdx.json",
            f"{zipped}.cdx.json",
            f"{zipped}.sigstore.json",
            chain.CHECKSUMS,
            chain.CHECKSUMS + chain.BUNDLE_SUFFIX,
        ]
    )
    sums = (p.dist / chain.CHECKSUMS).read_text().splitlines()
    assert {line.split()[1] for line in sums} == {
        zipped,
        f"{zipped}.spdx.json",
        f"{zipped}.cdx.json",
    }
    assert fake.ran("syft", "scan", f"file:{p.dist / zipped}")
    assert fake.ran("cosign", "sign-blob")
    assert fake.ran("cosign", "verify-blob")
    assert not fake.ran("docker")


def test_plugin_trial_without_an_sbom_signs_nothing(tmp_path: Path) -> None:
    fake = FakeChain(tmp_path)
    p = trial.PluginTrial(chain.Tools("cosign", "syft", runner=fake), tmp_path)
    p.build()
    with pytest.raises(chain.ChainError, match="SBOM missing"):
        p.sign()
    assert not fake.ran("cosign", "sign-blob")


def test_plugin_trial_probes_fail_on_a_passing_probe(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    fake = FakeChain(tmp_path)
    p = trial.PluginTrial(chain.Tools("cosign", "syft", runner=fake), tmp_path)
    p.run()
    monkeypatch.setattr(chain, "require_sboms", lambda dist, names: None)
    with pytest.raises(trial.TrialError, match="must fail: release without its SBOM"):
        p.probes()


def test_plugin_trial_needs_the_zip_of_release_build(tmp_path: Path) -> None:
    def silent_make(cmd: Sequence[str], **_: object) -> subprocess.CompletedProcess[str]:
        return subprocess.CompletedProcess(list(cmd), 0, "", "")

    p = trial.PluginTrial(chain.Tools("cosign", "syft", runner=silent_make), tmp_path)
    with pytest.raises(trial.TrialError, match="wrote no bothy-live-panel-"):
        p.build()


def test_main_runs_the_plugin_trial_without_a_registry(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    monkeypatch.delenv("GITHUB_ACTIONS", raising=False)
    monkeypatch.setattr(
        trial, "tempfile", types.SimpleNamespace(mkdtemp=lambda prefix: str(tmp_path / "work"))
    )
    (tmp_path / "work").mkdir()
    fake = FakeChain(tmp_path)
    assert (
        trial.main(
            ["--component", "grafana-bothy-live"], chain.Tools("cosign", "syft", runner=fake)
        )
        == 0
    )
    out = capsys.readouterr().out
    assert "release-trial: passed" in out
    assert "probe ok: release without its SBOM against sign fails" in out
    assert "probe ok: changed ZIP against SHA256SUMS fails" in out
    assert not fake.ran("docker")
    assert not (tmp_path / "work").exists()
