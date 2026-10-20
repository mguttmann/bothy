# SPDX-License-Identifier: Apache-2.0
"""Tests for the supply-chain steps of the release (tools/ci/release_chain.py).

Signing and verification of files run the pinned cosign for real with a key pair made for the
test (offline: the private phase has no transparency log); image commands, which need a
registry, run against a recording fake. `make release-trial` runs the image path for real.
"""

from __future__ import annotations

import hashlib
import json
import subprocess
from collections.abc import Sequence
from pathlib import Path

import fetch_tools
import pytest
import release_chain as chain

DIGEST_A = "sha256:" + "a" * 64
DIGEST_B = "sha256:" + "b" * 64
PASSWORD = "test-password-for-a-throwaway-key"


def pinned(name: str) -> str:
    lock = fetch_tools.load_lock(fetch_tools.DEFAULT_LOCK)
    return str(
        fetch_tools.ensure(lock, name, fetch_tools.platform_key(), fetch_tools.DEFAULT_CACHE)
    )


@pytest.fixture(scope="session")
def cosign() -> str:
    return pinned("cosign")


@pytest.fixture(scope="session")
def keypair(tmp_path_factory: pytest.TempPathFactory, cosign: str) -> Path:
    keys = tmp_path_factory.mktemp("keys")
    chain.run([cosign, "generate-key-pair"], env={"COSIGN_PASSWORD": PASSWORD}, cwd=keys)
    return keys


class Fake:
    """Records commands; answers from a table of (argument prefix -> stdout or exception)."""

    def __init__(self, answers: dict[tuple[str, ...], str | Exception] | None = None) -> None:
        self.calls: list[list[str]] = []
        self.answers = answers or {}

    def __call__(
        self, cmd: Sequence[str], *, env: dict[str, str] | None = None, cwd: Path | None = None
    ) -> subprocess.CompletedProcess[str]:
        cmd = list(cmd)
        self.calls.append(cmd)
        for prefix, answer in self.answers.items():
            if tuple(cmd[: len(prefix)]) == prefix:
                if isinstance(answer, Exception):
                    raise answer
                return subprocess.CompletedProcess(cmd, 0, answer, "")
        return subprocess.CompletedProcess(cmd, 0, "", "")

    def ran(self, *prefix: str) -> list[list[str]]:
        return [c for c in self.calls if tuple(c[: len(prefix)]) == prefix]


def fake_tools(fake: Fake, cosign: str = "cosign") -> chain.Tools:
    return chain.Tools(cosign=cosign, syft="syft", docker="docker", runner=fake)


def test_image_parse() -> None:
    img = chain.Image.parse(f"bothy-server=127.0.0.1:5000/bothy-server@{DIGEST_A}")
    assert img.ref == f"127.0.0.1:5000/bothy-server@{DIGEST_A}"
    for bad in ("bothy-server", f"=r@{DIGEST_A}", "n=r@sha256:12", f"n=@{DIGEST_A}"):
        with pytest.raises(chain.ChainError):
            chain.Image.parse(bad)


def test_join_pushes_the_index_by_its_own_digest() -> None:
    index = '{\n  "schemaVersion": 2\n}'
    digest = "sha256:" + hashlib.sha256(index.encode()).hexdigest()
    fake = Fake(
        {
            ("docker", "buildx", "imagetools", "create", "--dry-run"): index + "\n",
            ("docker", "buildx", "imagetools", "inspect"): index + "\n",
        }
    )
    assert chain.join(fake_tools(fake), "r/pg", [DIGEST_A, DIGEST_B]) == digest
    [create] = fake.ran("docker", "buildx", "imagetools", "create", "--tag")
    assert create[5:] == [f"r/pg@{digest}", f"r/pg@{DIGEST_A}", f"r/pg@{DIGEST_B}"]


def test_join_rejects_another_stored_index_and_bad_digests() -> None:
    fake = Fake(
        {
            ("docker", "buildx", "imagetools", "create", "--dry-run"): "{}\n",
            ("docker", "buildx", "imagetools", "inspect"): '{"other": 1}\n',
        }
    )
    with pytest.raises(chain.ChainError, match="another index"):
        chain.join(fake_tools(fake), "r/pg", [DIGEST_A])
    with pytest.raises(chain.ChainError, match="digests"):
        chain.join(fake_tools(fake), "r/pg", ["sha256:12"])
    with pytest.raises(chain.ChainError, match="digests"):
        chain.join(fake_tools(fake), "r/pg", [])


def images() -> list[chain.Image]:
    return [chain.Image("bothy-server", "r/bothy-server", DIGEST_A)]


def test_sbom_names_cover_every_image_platform_and_binary() -> None:
    names = chain.sbom_names(
        "1.2.3", images(), [Path("d/bothy_1.2.3_linux_amd64")], ["linux/amd64"]
    )
    assert names == [
        "bothy-server_1.2.3_linux_amd64.spdx.json",
        "bothy-server_1.2.3_linux_amd64.cdx.json",
        "bothy_1.2.3_linux_amd64.spdx.json",
        "bothy_1.2.3_linux_amd64.cdx.json",
    ]


def test_make_sboms_calls_syft_per_platform_and_binary(tmp_path: Path) -> None:
    fake = Fake()
    binary = tmp_path / "bothy_1.2.3_linux_amd64"
    out = chain.make_sboms(
        fake_tools(fake), tmp_path, "1.2.3", images(), [binary], ["linux/amd64", "linux/arm64"]
    )
    assert [p.name for p in out] == chain.sbom_names(
        "1.2.3", images(), [binary], ["linux/amd64", "linux/arm64"]
    )
    calls = fake.ran("syft", "scan")
    assert [c[2] for c in calls] == [f"registry:r/bothy-server@{DIGEST_A}"] * 2 + [f"file:{binary}"]
    assert calls[1][3:5] == ["--platform", "linux/arm64"]


def write_sboms(dist: Path, names: Sequence[str]) -> None:
    for name in names:
        (dist / name).write_text('{"sbom": true}', encoding="utf-8")


def test_missing_sbom_aborts_before_any_signature(tmp_path: Path) -> None:
    binary = tmp_path / "bothy_1.2.3_linux_amd64"
    binary.write_bytes(b"\x7fELF")
    names = chain.sbom_names("1.2.3", images(), [binary], ["linux/amd64"])
    write_sboms(tmp_path, names[1:])
    (tmp_path / names[2]).write_text("", encoding="utf-8")
    fake = Fake()
    with pytest.raises(chain.ChainError, match=r"SBOM missing or invalid.*linux_amd64\.spdx"):
        chain.sign(
            fake_tools(fake),
            "public",
            tmp_path,
            "1.2.3",
            images(),
            [binary],
            ["linux/amd64"],
        )
    assert fake.calls == []


def test_sign_rejects_an_unknown_mode_and_a_missing_key(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.delenv("COSIGN_PRIVATE_KEY", raising=False)
    with pytest.raises(chain.ChainError, match="mode"):
        chain.sign(fake_tools(Fake()), "keyless", tmp_path, "1", [], [], [])
    with pytest.raises(chain.ChainError, match="COSIGN_PRIVATE_KEY"):
        chain.sign(fake_tools(Fake()), "private", tmp_path, "1", [], [], [])


def test_public_sign_order_images_binaries_then_checksums(tmp_path: Path) -> None:
    binary = tmp_path / "bothy_1.2.3_linux_amd64"
    binary.write_bytes(b"\x7fELF")
    write_sboms(tmp_path, chain.sbom_names("1.2.3", images(), [binary], ["linux/amd64"]))
    fake = Fake()
    chain.sign(fake_tools(fake), "public", tmp_path, "1.2.3", images(), [binary], ["linux/amd64"])
    signs = [c[1] for c in fake.calls]
    assert signs == ["sign", "sign-blob", "sign-blob"]
    assert fake.calls[0][-1] == f"r/bothy-server@{DIGEST_A}"
    assert "--key" not in fake.calls[0]
    assert fake.calls[-1][-1] == str(tmp_path / "SHA256SUMS")
    sums = (tmp_path / "SHA256SUMS").read_text()
    assert "images.txt" in sums
    assert "bothy_1.2.3_linux_amd64.spdx.json" in sums
    assert ".sigstore.json" not in sums
    assert (tmp_path / "images.txt").read_text() == f"bothy-server r/bothy-server@{DIGEST_A}\n"


def test_files_only_release_signs_files_and_writes_no_image_list(tmp_path: Path) -> None:
    """The product release (brief M7-29) signs the bundle and install.sh, no image."""
    files = [tmp_path / "bothy-compose-1.0.0.tar.gz", tmp_path / "install.sh"]
    for f in files:
        f.write_bytes(b"x")
    write_sboms(tmp_path, chain.sbom_names("1.0.0", [], files, []))
    fake = Fake()
    chain.sign(fake_tools(fake), "public", tmp_path, "1.0.0", [], files, [])
    assert [c[1] for c in fake.calls] == ["sign-blob", "sign-blob", "sign-blob"]
    assert not (tmp_path / "images.txt").exists()
    sums = (tmp_path / "SHA256SUMS").read_text()
    assert "install.sh" in sums
    assert "bothy-compose-1.0.0.tar.gz" in sums


def test_private_signing_config_names_no_service(tmp_path: Path, cosign: str) -> None:
    tools = chain.Tools(cosign=cosign, syft="syft")
    config = json.loads(chain.private_signing_config(tools, tmp_path / "c.json").read_text())
    assert not any(config.get(k) for k in ("caUrls", "oidcUrls", "rekorTlogUrls", "tsaUrls"))


def test_signing_config_with_a_service_is_rejected(tmp_path: Path) -> None:
    def runner(cmd: Sequence[str], **_: object) -> subprocess.CompletedProcess[str]:
        Path(cmd[-1]).write_text('{"rekorTlogUrls": [{"url": "https://rekor.example.com"}]}')
        return subprocess.CompletedProcess(list(cmd), 0, "", "")

    with pytest.raises(chain.ChainError, match="names services"):
        chain.private_signing_config(chain.Tools("cosign", "syft", runner=runner), tmp_path / "c")


def test_bundle_with_a_tlog_entry_is_rejected(tmp_path: Path) -> None:
    bundle = tmp_path / "x.sigstore.json"
    bundle.write_text(json.dumps({"verificationMaterial": {"tlogEntries": [{"logIndex": "1"}]}}))
    with pytest.raises(chain.ChainError, match="transparency log"):
        chain.check_no_tlog(bundle)


@pytest.fixture
def signed_dist(
    tmp_path: Path, cosign: str, keypair: Path, monkeypatch: pytest.MonkeyPatch
) -> Path:
    """A dist directory signed for real in the private mode (no images)."""
    monkeypatch.setenv("COSIGN_PASSWORD", PASSWORD)
    monkeypatch.setenv("COSIGN_PRIVATE_KEY", (keypair / "cosign.key").read_text())
    dist = tmp_path / "dist"
    dist.mkdir()
    binary = dist / "bothy_1.2.3_linux_amd64"
    binary.write_bytes(b"\x7fELF binary")
    write_sboms(dist, chain.sbom_names("1.2.3", [], [binary], ["linux/amd64"]))
    tools = chain.Tools(cosign=cosign, syft="syft")
    chain.sign(tools, "private", dist, "1.2.3", [], [binary], ["linux/amd64"])
    return dist


def test_private_signatures_verify_and_have_no_tlog(
    signed_dist: Path, cosign: str, keypair: Path
) -> None:
    tools = chain.Tools(cosign=cosign, syft="syft")
    chain.verify(tools, "private", signed_dist, [], key=keypair / "cosign.pub")
    bundles = sorted(p.name for p in signed_dist.glob("*.sigstore.json"))
    assert bundles == ["SHA256SUMS.sigstore.json", "bothy_1.2.3_linux_amd64.sigstore.json"]


def test_changed_file_fails_the_checksum(signed_dist: Path, cosign: str, keypair: Path) -> None:
    (signed_dist / "bothy_1.2.3_linux_amd64.spdx.json").write_text('{"changed": 1}')
    tools = chain.Tools(cosign=cosign, syft="syft")
    with pytest.raises(chain.ChainError, match="sha256sum"):
        chain.verify(tools, "private", signed_dist, [], key=keypair / "cosign.pub")


def test_changed_binary_fails_its_signature(signed_dist: Path, cosign: str, keypair: Path) -> None:
    with (signed_dist / "bothy_1.2.3_linux_amd64").open("ab") as fh:
        fh.write(b"\0")
    tools = chain.Tools(cosign=cosign, syft="syft")
    with pytest.raises(chain.ChainError, match="verify-blob"):
        chain.verify(tools, "private", signed_dist, [], key=keypair / "cosign.pub")


def test_other_key_fails_and_unsigned_checksums_fail(
    signed_dist: Path, cosign: str, tmp_path: Path
) -> None:
    other = tmp_path / "other"
    other.mkdir()
    chain.run([cosign, "generate-key-pair"], env={"COSIGN_PASSWORD": "x"}, cwd=other)
    tools = chain.Tools(cosign=cosign, syft="syft")
    with pytest.raises(chain.ChainError):
        chain.verify(tools, "private", signed_dist, [], key=other / "cosign.pub")
    (signed_dist / "SHA256SUMS.sigstore.json").unlink()
    with pytest.raises(chain.ChainError, match="SHA256SUMS or its signature bundle is missing"):
        chain.verify(tools, "private", signed_dist, [], key=other / "cosign.pub")


def test_verify_arguments_per_mode(tmp_path: Path) -> None:
    fake = Fake()
    tools = fake_tools(fake)
    with pytest.raises(chain.ChainError, match="--key"):
        chain.verify(tools, "private", tmp_path, images())
    with pytest.raises(chain.ChainError, match="owner step"):
        chain.verify(tools, "private", tmp_path, images(), key=tmp_path / "missing.pub")
    with pytest.raises(chain.ChainError, match="--identity-regexp"):
        chain.verify(tools, "public", tmp_path, images())
    with pytest.raises(chain.ChainError, match="mode"):
        chain.verify(tools, "other", tmp_path, images())
    (tmp_path / "SHA256SUMS").write_text("")
    (tmp_path / "SHA256SUMS.sigstore.json").write_text("{}")
    chain.verify(tools, "public", tmp_path, images(), identity_regexp="^x$", issuer="https://i")
    assert fake.calls[0][:3] == ["cosign", "verify", "--certificate-identity-regexp"]
    assert fake.calls[-1] == ["sha256sum", "--check", "--strict", "SHA256SUMS"]


def test_tag_refuses_latest_and_tags_the_digest() -> None:
    fake = Fake()
    img = images()[0]
    with pytest.raises(chain.ChainError, match="not a version tag"):
        chain.tag(fake_tools(fake), img, "latest")
    assert chain.tag(fake_tools(fake), img, "pg18-1.2.3") == "r/bothy-server:pg18-1.2.3"
    assert fake.calls == [
        ["docker", "buildx", "imagetools", "create", "--tag", "r/bothy-server:pg18-1.2.3", img.ref]
    ]


def test_run_raises_with_the_error_text() -> None:
    with pytest.raises(chain.ChainError, match="no such thing"):
        chain.run(["sh", "-c", "echo no such thing >&2; exit 4"])


def test_cli(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    fake = Fake()
    tools = fake_tools(fake)
    image = f"bothy-server=r/bothy-server@{DIGEST_A}"
    assert chain.main(["tag", "--image", image, "--tag", "1.2.3"], tools) == 0
    assert capsys.readouterr().out.strip() == "r/bothy-server:1.2.3"
    assert (
        chain.main(["sbom", "--dist", str(tmp_path), "--version", "1", "--image", image], tools)
        == 0
    )
    assert chain.main(["tag", "--image", "broken", "--tag", "1"], tools) == 1
    assert "release-chain:" in capsys.readouterr().err
    rc = chain.main(
        ["sign", "--mode", "public", "--dist", str(tmp_path), "--version", "1", "--image", image],
        tools,
    )
    assert rc == 1  # the SBOMs of the fake syft were never written
    rc = chain.main(["verify", "--mode", "private", "--dist", str(tmp_path)], tools)
    assert rc == 1
