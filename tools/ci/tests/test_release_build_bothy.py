# SPDX-License-Identifier: Apache-2.0
"""`make release-build` of the component `bothy` (tools/ci/product_release.py, brief M7-29).

Every case of docs/spec/18-release.md section 2.1 rules 1 to 4 against a fixture repository with
a committed bundle template and a fake `docker` on PATH as the registry stub: the bundle takes
`BOTHY_VERSION` from the commit and sets `BOTHY_REGISTRY` from the owner in lower case, every
other file equals the commit, `install.sh` gets version, checksum and download address, and each
listed problem aborts without any output and names the value. The last test runs the rendered
`install.sh` with a download stub and checks the address it fetches the bundle from.
"""

from __future__ import annotations

import hashlib
import os
import shutil
import stat
import subprocess
import sys
import tarfile
from pathlib import Path

import pytest

REPO = Path(__file__).resolve().parents[3]
TOOL = REPO / "tools" / "ci" / "product_release.py"
PRODUCT = "1.0.0"
SERVER = "1.4.0"
# The tracked paths a product bundle is made of (docs/spec/18-release.md section 2), plus the
# source of install.sh and the test overlays it must leave out.
FIXTURE_PATHS = (
    "infra/compose",
    "infra/otel",
    "infra/observability",
    "LICENSE",
    "NOTICE",
    "CHANGELOG.md",
    "VERSION",
)
_GIT_ENV = {
    "GIT_AUTHOR_NAME": "Alex",
    "GIT_AUTHOR_EMAIL": "alex@example.com",
    "GIT_COMMITTER_NAME": "Alex",
    "GIT_COMMITTER_EMAIL": "alex@example.com",
    "GIT_CONFIG_GLOBAL": os.devnull,
    "GIT_CONFIG_NOSYSTEM": "1",
}
_STUB_DOCKER = """#!/bin/sh
# Registry stub: `docker manifest inspect <ref>` fails for refs listed in STUB_MISSING.
printf '%s\\n' "$*" >> "$STUB_LOG"
case "$1 $2" in
"manifest inspect")
  for ref in "$@"; do last="$ref"; done
  for gone in $STUB_MISSING; do [ "$last" = "$gone" ] && exit 1; done
  echo '{}'; exit 0 ;;
esac
exit 0
"""


def _git(repo: Path, *args: str) -> str:
    env = {**os.environ, **_GIT_ENV}
    for key in ("GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR"):
        env.pop(key, None)
    return subprocess.run(
        ["git", "-C", str(repo), *args], env=env, check=True, capture_output=True, text=True
    ).stdout


def _tracked(path: str) -> list[str]:
    return _git(REPO, "ls-files", "--", path).split()


def make_fixture(root: Path, server: str = SERVER, product: str = PRODUCT) -> Path:
    """Copy the tracked bundle files of this repository into a fresh git repository.

    The fixture pins `server` in `infra/compose/.env.example`, sets `VERSION` to `product` and
    commits; the working tree then equals the commit.
    """
    repo = root / "fixture"
    for top in FIXTURE_PATHS:
        for name in _tracked(top):
            dest = repo / name
            dest.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(REPO / name, dest)
    # The source of install.sh, also before it is committed in this repository.
    shutil.copy2(REPO / "infra/compose/install.sh", repo / "infra/compose/install.sh")
    (repo / "VERSION").write_text(product + "\n", encoding="utf-8")
    env = repo / "infra" / "compose" / ".env.example"
    lines = env.read_text(encoding="utf-8").splitlines(keepends=True)
    env.write_text(
        "".join(
            f"BOTHY_VERSION={server}\n" if ln.startswith("BOTHY_VERSION=") else ln for ln in lines
        ),
        encoding="utf-8",
    )
    _git(repo, "init", "-q", "-b", "main")
    _git(repo, "add", "-A")
    _git(repo, "commit", "-q", "-m", "chore(release): bothy fixture")
    return repo


def commit_file(repo: Path, name: str, text: str) -> None:
    """Change one file of the fixture and commit it."""
    (repo / name).write_text(text, encoding="utf-8")
    _git(repo, "add", name)
    _git(repo, "commit", "-q", "-m", f"test: change {name}")


def stub_bin(root: Path) -> Path:
    """A directory with the registry stub `docker`."""
    bindir = root / "bin"
    bindir.mkdir(exist_ok=True)
    docker = bindir / "docker"
    docker.write_text(_STUB_DOCKER, encoding="utf-8")
    docker.chmod(docker.stat().st_mode | stat.S_IXUSR)
    return bindir


def build(
    root: Path,
    repo: Path,
    *,
    version: str = PRODUCT,
    env: dict[str, str | None] | None = None,
    missing: str = "",
) -> subprocess.CompletedProcess[str]:
    """Run `product_release.py build` like `make release-build VERSION=<v> OUT=<root>/out`."""
    base = {k: v for k, v in os.environ.items() if not k.startswith(("GITHUB_", "RELEASE_TRIAL"))}
    base.update(
        {
            "GITHUB_REPOSITORY_OWNER": "Example-Org",
            "GITHUB_REPOSITORY": "Example-Org/bothy",
            "STUB_LOG": str(root / "docker.log"),
            "STUB_MISSING": missing,
            "PATH": f"{stub_bin(root)}{os.pathsep}{os.environ['PATH']}",
        }
    )
    for key, value in (env or {}).items():
        if value is None:
            base.pop(key, None)
        else:
            base[key] = value
    return subprocess.run(
        [
            sys.executable,
            str(TOOL),
            "build",
            "--repo",
            str(repo),
            "--version",
            version,
            "--out",
            str(root / "out"),
        ],
        env=base,
        capture_output=True,
        text=True,
        check=False,
        timeout=120,
    )


def outputs(root: Path) -> list[str]:
    out = root / "out"
    return sorted(p.name for p in out.iterdir()) if out.is_dir() else []


def bundle_members(path: Path) -> dict[str, bytes]:
    with tarfile.open(path) as archive:
        return {
            m.name: archive.extractfile(m).read()  # type: ignore[union-attr]
            for m in archive.getmembers()
            if m.isfile()
        }


# ---- the bundle and install.sh ----------------------------------------------------------------


def test_bundle_pins_commit_version_and_owner_registry(tmp_path: Path) -> None:
    repo = make_fixture(tmp_path)
    result = build(tmp_path, repo)
    assert result.returncode == 0, result.stderr
    bundle = tmp_path / "out" / f"bothy-compose-{PRODUCT}.tar.gz"
    assert outputs(tmp_path) == [bundle.name, "install.sh"]
    members = bundle_members(bundle)
    prefix = f"bothy-{PRODUCT}/"
    env = members[prefix + "infra/compose/.env.example"].decode()
    assert f"\nBOTHY_VERSION={SERVER}\n" in env
    assert "\nBOTHY_REGISTRY=ghcr.io/example-org\n" in env
    # Every other file and line equals the commit.
    committed = _git(repo, "show", "HEAD:infra/compose/.env.example").splitlines()
    assert [ln for ln in committed if not ln.startswith("BOTHY_REGISTRY=")] == [
        ln for ln in env.splitlines() if not ln.startswith("BOTHY_REGISTRY=")
    ]
    for name, data in members.items():
        rel = name.removeprefix(prefix)
        if rel != "infra/compose/.env.example":
            assert data == (repo / rel).read_bytes(), rel
    expected = {
        n
        for top in FIXTURE_PATHS
        for n in _git(repo, "ls-files", "--", top).split()
        if not n.startswith("infra/compose/test/") and n != "infra/compose/install.sh"
    }
    assert {n.removeprefix(prefix) for n in members} == expected


def test_install_sh_gets_version_checksum_and_address(tmp_path: Path) -> None:
    repo = make_fixture(tmp_path)
    result = build(tmp_path, repo)
    assert result.returncode == 0, result.stderr
    bundle = tmp_path / "out" / f"bothy-compose-{PRODUCT}.tar.gz"
    script = (tmp_path / "out" / "install.sh").read_text(encoding="utf-8")
    digest = hashlib.sha256(bundle.read_bytes()).hexdigest()
    assert f"\nBOTHY_INSTALL_VERSION='{PRODUCT}'\n" in script
    assert f"\nBOTHY_INSTALL_BUNDLE_SHA256='{digest}'\n" in script
    assert (
        "\nBOTHY_INSTALL_BASE_URL="
        f"'https://github.com/Example-Org/bothy/releases/download/v{PRODUCT}/'\n" in script
    )
    assert "OWNER" not in script
    assert "placeholder" not in script.lower()
    assert os.access(tmp_path / "out" / "install.sh", os.X_OK)


def test_images_of_compose_are_checked_in_the_registry(tmp_path: Path) -> None:
    repo = make_fixture(tmp_path)
    assert build(tmp_path, repo).returncode == 0
    calls = (tmp_path / "docker.log").read_text(encoding="utf-8").splitlines()
    refs = sorted(c.split()[-1] for c in calls if c.startswith("manifest inspect"))
    assert refs == [
        f"ghcr.io/example-org/bothy-postgres:pg18-{SERVER}",
        f"ghcr.io/example-org/bothy-server:{SERVER}",
    ]


def test_bundle_is_reproducible(tmp_path: Path) -> None:
    repo = make_fixture(tmp_path)
    assert build(tmp_path, repo).returncode == 0
    first = (tmp_path / "out" / f"bothy-compose-{PRODUCT}.tar.gz").read_bytes()
    shutil.rmtree(tmp_path / "out")
    assert build(tmp_path, repo).returncode == 0
    assert (tmp_path / "out" / f"bothy-compose-{PRODUCT}.tar.gz").read_bytes() == first


# ---- aborts: no output, the value named --------------------------------------------------------


@pytest.mark.parametrize(
    ("server", "product", "named"),
    [
        ("0.0.0-placeholder", PRODUCT, "0.0.0-placeholder"),
        ("v1.4", PRODUCT, "v1.4"),
        ("latest", PRODUCT, "latest"),
        ("1.5.0-beta.1", PRODUCT, "1.5.0-beta.1"),
    ],
)
def test_bad_server_version_aborts(tmp_path: Path, server: str, product: str, named: str) -> None:
    repo = make_fixture(tmp_path, server=server, product=product)
    result = build(tmp_path, repo, version=product)
    assert result.returncode != 0
    assert named in result.stderr
    assert outputs(tmp_path) == []


def test_server_beta_in_a_product_beta_passes(tmp_path: Path) -> None:
    repo = make_fixture(tmp_path, server="1.5.0-beta.1", product="1.1.0-beta.1")
    result = build(tmp_path, repo, version="1.1.0-beta.1")
    assert result.returncode == 0, result.stderr


def test_version_other_than_the_file_aborts(tmp_path: Path) -> None:
    repo = make_fixture(tmp_path)
    result = build(tmp_path, repo, version="9.9.9")
    assert result.returncode != 0
    assert "9.9.9" in result.stderr
    assert outputs(tmp_path) == []


@pytest.mark.parametrize("variable", ["GITHUB_REPOSITORY_OWNER", "GITHUB_REPOSITORY"])
def test_missing_repository_variable_aborts(tmp_path: Path, variable: str) -> None:
    repo = make_fixture(tmp_path)
    result = build(tmp_path, repo, env={variable: None})
    assert result.returncode != 0
    assert variable in result.stderr
    assert outputs(tmp_path) == []


def test_owner_placeholder_in_registry_aborts(tmp_path: Path) -> None:
    repo = make_fixture(tmp_path)
    result = build(tmp_path, repo, env={"GITHUB_REPOSITORY_OWNER": "OWNER"})
    assert result.returncode != 0
    assert "OWNER" in result.stderr
    assert outputs(tmp_path) == []


def test_owner_placeholder_in_install_address_aborts(tmp_path: Path) -> None:
    repo = make_fixture(tmp_path)
    result = build(tmp_path, repo, env={"GITHUB_REPOSITORY": "OWNER/bothy"})
    assert result.returncode != 0
    assert "OWNER" in result.stderr
    assert "install.sh" in result.stderr
    assert outputs(tmp_path) == []


def test_placeholder_left_in_install_sh_aborts(tmp_path: Path) -> None:
    repo = make_fixture(tmp_path)
    source = (repo / "infra/compose/install.sh").read_text(encoding="utf-8")
    commit_file(repo, "infra/compose/install.sh", source + "# 0.0.0-placeholder\n")
    result = build(tmp_path, repo)
    assert result.returncode != 0
    assert "install.sh" in result.stderr
    assert "placeholder" in result.stderr
    assert outputs(tmp_path) == []


def test_missing_built_in_line_aborts(tmp_path: Path) -> None:
    repo = make_fixture(tmp_path)
    source = (repo / "infra/compose/install.sh").read_text(encoding="utf-8")
    kept = "".join(
        ln
        for ln in source.splitlines(keepends=True)
        if not ln.startswith("BOTHY_INSTALL_BUNDLE_SHA256=")
    )
    commit_file(repo, "infra/compose/install.sh", kept)
    result = build(tmp_path, repo)
    assert result.returncode != 0
    assert "BOTHY_INSTALL_BUNDLE_SHA256" in result.stderr
    assert outputs(tmp_path) == []


def test_image_missing_in_registry_aborts(tmp_path: Path) -> None:
    repo = make_fixture(tmp_path)
    gone = f"ghcr.io/example-org/bothy-postgres:pg18-{SERVER}"
    result = build(tmp_path, repo, missing=gone)
    assert result.returncode != 0
    assert gone in result.stderr
    assert outputs(tmp_path) == []


def test_the_commit_wins_over_the_working_tree(tmp_path: Path) -> None:
    repo = make_fixture(tmp_path)
    env = repo / "infra/compose/.env.example"
    env.write_text(env.read_text(encoding="utf-8").replace(SERVER, "7.7.7"), encoding="utf-8")
    result = build(tmp_path, repo)
    assert result.returncode == 0, result.stderr
    members = bundle_members(tmp_path / "out" / f"bothy-compose-{PRODUCT}.tar.gz")
    text = members[f"bothy-{PRODUCT}/infra/compose/.env.example"].decode()
    assert f"BOTHY_VERSION={SERVER}" in text


# ---- trial registry (D-W-127, Design note 3) ---------------------------------------------------


def test_trial_registry_and_address(tmp_path: Path) -> None:
    repo = make_fixture(tmp_path)
    result = build(
        tmp_path,
        repo,
        env={
            "GITHUB_ACTIONS": None,
            "RELEASE_TRIAL_REGISTRY": "127.0.0.1:45555",
            "RELEASE_TRIAL_BASE_URL": "http://127.0.0.1:45556/",
        },
    )
    assert result.returncode == 0, result.stderr
    members = bundle_members(tmp_path / "out" / f"bothy-compose-{PRODUCT}.tar.gz")
    env = members[f"bothy-{PRODUCT}/infra/compose/.env.example"].decode()
    assert "\nBOTHY_REGISTRY=127.0.0.1:45555\n" in env
    script = (tmp_path / "out" / "install.sh").read_text(encoding="utf-8")
    assert "\nBOTHY_INSTALL_BASE_URL='http://127.0.0.1:45556/'\n" in script
    calls = (tmp_path / "docker.log").read_text(encoding="utf-8")
    assert f"manifest inspect --insecure 127.0.0.1:45555/bothy-server:{SERVER}" in calls


@pytest.mark.parametrize("variable", ["RELEASE_TRIAL_REGISTRY", "RELEASE_TRIAL_BASE_URL"])
def test_trial_variables_refused_under_github_actions(tmp_path: Path, variable: str) -> None:
    repo = make_fixture(tmp_path)
    value = "127.0.0.1:45555" if variable == "RELEASE_TRIAL_REGISTRY" else "http://127.0.0.1:1/"
    result = build(tmp_path, repo, env={"GITHUB_ACTIONS": "true", variable: value})
    assert result.returncode != 0
    assert variable in result.stderr
    assert "GITHUB_ACTIONS" in result.stderr
    assert outputs(tmp_path) == []


@pytest.mark.parametrize(
    ("variable", "value"),
    [
        ("RELEASE_TRIAL_REGISTRY", "ghcr.io/example-org"),
        ("RELEASE_TRIAL_BASE_URL", "https://example.com/"),
    ],
)
def test_trial_variables_only_loopback(tmp_path: Path, variable: str, value: str) -> None:
    repo = make_fixture(tmp_path)
    result = build(tmp_path, repo, env={"GITHUB_ACTIONS": None, variable: value})
    assert result.returncode != 0
    assert variable in result.stderr
    assert outputs(tmp_path) == []


# ---- the rendered install.sh fetches its bundle from the built-in address ----------------------

_STUB_INSTALL_DOCKER = """#!/bin/sh
case "$*" in
"version --format"*) echo 29.0.0 ;;
"compose version --short") echo 2.40.0 ;;
"info --format {{.Architecture}}") echo x86_64 ;;
"info --format {{.MemTotal}}") echo 8589934592 ;;
esac
exit 0
"""
_STUB_CURL = """#!/bin/sh
# Download stub: records the address and serves nothing, so install.sh stops after the fetch.
for arg in "$@"; do last="$arg"; done
printf '%s\\n' "$last" >> "$STUB_LOG"
exit 22
"""


def test_rendered_install_sh_downloads_from_the_built_in_address(tmp_path: Path) -> None:
    repo = make_fixture(tmp_path)
    assert build(tmp_path, repo).returncode == 0
    bindir = tmp_path / "install-bin"
    bindir.mkdir()
    for name, text in (("docker", _STUB_INSTALL_DOCKER), ("curl", _STUB_CURL)):
        (bindir / name).write_text(text, encoding="utf-8")
        (bindir / name).chmod(0o755)
    log = tmp_path / "curl.log"
    result = subprocess.run(
        [
            "sh",
            str(tmp_path / "out" / "install.sh"),
            "--yes",
            "--dir",
            str(tmp_path / "target"),
            "--public-url",
            "https://chat.example.com",
            "--owner-handle",
            "alex",
            "--owner-name",
            "Alex",
        ],
        env={
            "PATH": f"{bindir}{os.pathsep}/usr/bin{os.pathsep}/bin",
            "STUB_LOG": str(log),
            "HOME": str(tmp_path),
        },
        stdin=subprocess.DEVNULL,
        capture_output=True,
        text=True,
        check=False,
        timeout=60,
    )
    assert result.returncode != 0
    assert "install.sh: bundle:" in result.stderr
    assert log.read_text(encoding="utf-8").splitlines()[0] == (
        f"https://github.com/Example-Org/bothy/releases/download/v{PRODUCT}/"
        f"bothy-compose-{PRODUCT}.tar.gz"
    )


def test_cli_without_arguments_is_a_usage_error() -> None:
    result = subprocess.run(
        [sys.executable, str(TOOL)], capture_output=True, text=True, check=False, timeout=60
    )
    assert result.returncode == 2
