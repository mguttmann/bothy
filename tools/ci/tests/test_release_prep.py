# SPDX-License-Identifier: Apache-2.0
"""Tests for `make release-prep` (tools/ci/release_prep.py, ADR 0060 "Confirmation").

Every case builds a fixture repository in a temporary directory with commits in several
components and runs the pinned git-cliff against it, so the version rule is the real one.
"""

from __future__ import annotations

import re
import subprocess
from pathlib import Path

import fetch_tools
import pytest
import release_prep

REPO = Path(__file__).resolve().parents[3]
CLIFF_CONFIG = REPO / "cliff.toml"
DATE = "2026-10-15"
CHANGELOG = """# Changelog

Intro text.

## [Unreleased]

### Added

- Something for users.

## [0.1.0] - 2026-01-01

### Added

- The first thing.
"""


@pytest.fixture(scope="session")
def git_cliff() -> Path:
    """The pinned git-cliff binary (fetched once into the tool cache)."""
    lock = fetch_tools.load_lock(fetch_tools.DEFAULT_LOCK)
    return fetch_tools.ensure(
        lock, "git-cliff", fetch_tools.platform_key(), fetch_tools.DEFAULT_CACHE
    )


def git(repo: Path, *args: str) -> str:
    return subprocess.run(
        ["git", "-C", str(repo), *args], check=True, capture_output=True, text=True
    ).stdout


class Fixture:
    """A git repository with the component directories of the monorepo."""

    def __init__(self, root: Path) -> None:
        self.root = root
        git(root, "init", "-q", "-b", "main")
        git(root, "config", "user.name", "Alex")
        git(root, "config", "user.email", "alex@example.com")
        git(root, "config", "commit.gpgsign", "false")
        git(root, "config", "tag.gpgsign", "false")
        for directory in ("services/server", "sdk/python", "."):
            (root / directory).mkdir(parents=True, exist_ok=True)
            (root / directory / "VERSION").write_text("0.1.0\n", encoding="utf-8")
            (root / directory / "CHANGELOG.md").write_text(CHANGELOG, encoding="utf-8")
        self.commit("chore: initial state", "VERSION")

    def commit(self, message: str, *files: str) -> str:
        for name in files:
            path = self.root / name
            path.parent.mkdir(parents=True, exist_ok=True)
            with path.open("a", encoding="utf-8") as fh:
                fh.write(f"{message}\n")
        git(self.root, "add", "-A")
        git(self.root, "commit", "-q", "-m", message)
        return git(self.root, "rev-parse", "HEAD").strip()

    def tag(self, name: str) -> None:
        git(self.root, "tag", name)

    def read(self, name: str) -> str:
        return (self.root / name).read_text(encoding="utf-8")


@pytest.fixture
def fx(tmp_path: Path) -> Fixture:
    return Fixture(tmp_path)


def prep(
    fx: Fixture,
    git_cliff: Path,
    component: str = "server",
    channel: str = "stable",
    server_version: str | None = None,
):
    return release_prep.prepare(
        component,
        channel=channel,
        repo=fx.root,
        git_cliff=git_cliff,
        config=CLIFF_CONFIG,
        date=DATE,
        server_version=server_version,
    )


ENV_EXAMPLE = "infra/compose/.env.example"
README = "README.md"
INSTALL_DOC = "docs/operations/install-compose.md"
PRODUCT_FILES = ("VERSION", "CHANGELOG.md", ENV_EXAMPLE, README, INSTALL_DOC)


def install_url(version: str) -> str:
    return f"https://github.com/OWNER/bothy/releases/download/v{version}/install.sh"


def env_example(pinned: str) -> str:
    return (
        "# Template of infra/compose/.env.\n"
        "BOTHY_REGISTRY=ghcr.io/OWNER\n"
        f"BOTHY_VERSION={pinned}\n"
        "BOTHY_PUBLIC_URL=https://chat.example.com\n"
    )


def product(fx: Fixture, pinned: str = "0.0.0-placeholder") -> Fixture:
    """Release the product as v0.1.0 with its bundle files, then add a fix to release.

    The server tags `server/v1.4.0` and `server/v2.0.0-beta.1` exist. README.md and the
    installation guide name the documented command for v0.1.0, next to a bundle URL of the
    same release that release-prep must leave alone.
    """
    (fx.root / "infra/compose").mkdir(parents=True, exist_ok=True)
    (fx.root / "docs/operations").mkdir(parents=True, exist_ok=True)
    (fx.root / ENV_EXAMPLE).write_text(env_example(pinned), encoding="utf-8")
    (fx.root / README).write_text(
        f"# Product\n\n```sh\ncurl -fsSL {install_url('0.1.0')} | sh\n```\n\n"
        "Bundle: https://github.com/OWNER/bothy/releases/download/v0.1.0/bothy-compose.tar.gz\n",
        encoding="utf-8",
    )
    (fx.root / INSTALL_DOC).write_text(
        f"# Install\n\n```sh\ncurl -fsSL {install_url('0.1.0')} \\\n  | sh -s -- --yes\n```\n\n"
        f"Again: {install_url('0.1.0')}\n",
        encoding="utf-8",
    )
    fx.commit("chore(release): v0.1.0", "VERSION")
    fx.tag("v0.1.0")
    fx.tag("server/v1.4.0")
    fx.tag("server/v2.0.0-beta.1")
    fx.commit("fix(compose): repair", "infra/compose/compose.yaml")
    return fx


def snapshot(fx: Fixture) -> dict[str, bytes | None]:
    """The bytes of every file a product release-prep may write (None when missing)."""
    return {
        name: (fx.root / name).read_bytes() if (fx.root / name).is_file() else None
        for name in PRODUCT_FILES
    }


def released(fx: Fixture, component: str = "server") -> Fixture:
    """Tag the current state of `component` as 0.1.0 on a commit inside its directory."""
    directory = {"server": "services/server", "sdk-python": "sdk/python"}[component]
    fx.commit(f"chore(release): {component} v0.1.0", f"{directory}/release.txt")
    fx.tag(f"{component}/v0.1.0")
    return fx


def test_fix_bumps_the_patch(fx: Fixture, git_cliff: Path) -> None:
    released(fx).commit("fix(server): repair it", "services/server/a.go")
    result = prep(fx, git_cliff)
    assert result.version == "0.1.1"
    assert fx.read("services/server/VERSION") == "0.1.1\n"


def test_feat_bumps_the_minor(fx: Fixture, git_cliff: Path) -> None:
    released(fx).commit("feat(server): new thing", "services/server/a.go")
    assert prep(fx, git_cliff).version == "0.2.0"


def test_break_before_1_0_bumps_the_minor(fx: Fixture, git_cliff: Path) -> None:
    released(fx).commit("feat(server)!: drop the old thing", "services/server/a.go")
    assert prep(fx, git_cliff).version == "0.2.0"


def test_break_after_1_0_bumps_the_major(fx: Fixture, git_cliff: Path) -> None:
    fx.commit("chore(release): sdk-python v1.2.3", "sdk/python/VERSION")
    fx.tag("sdk-python/v1.2.3")
    fx.commit("fix(sdk-python)!: change the client signature", "sdk/python/b.py")
    assert prep(fx, git_cliff, "sdk-python").version == "2.0.0"


def test_feat_after_1_0_bumps_the_minor(fx: Fixture, git_cliff: Path) -> None:
    fx.commit("chore(release): sdk-python v1.2.3", "sdk/python/VERSION")
    fx.tag("sdk-python/v1.2.3")
    fx.commit("feat(sdk-python): add a call", "sdk/python/b.py")
    assert prep(fx, git_cliff, "sdk-python").version == "1.3.0"


def test_commits_of_other_components_do_not_count(fx: Fixture, git_cliff: Path) -> None:
    released(fx).commit("fix(server): small", "services/server/a.go")
    fx.commit("feat(sdk-python)!: big change elsewhere", "sdk/python/b.py")
    fx.commit("feat(host): unrelated", "services/host/c.go")
    assert prep(fx, git_cliff).version == "0.1.1"


def test_api_contract_counts_for_the_server(fx: Fixture, git_cliff: Path) -> None:
    released(fx).commit("feat(api): new operation", "docs/api/openapi.yaml")
    assert prep(fx, git_cliff).version == "0.2.0"


def test_first_release_without_a_tag(fx: Fixture, git_cliff: Path) -> None:
    (fx.root / "services/server/CHANGELOG.md").write_text(
        "# Changelog\n\n## [Unreleased]\n\n- First.\n", encoding="utf-8"
    )
    fx.commit("feat(server): first feature", "services/server/a.go")
    assert prep(fx, git_cliff).version == "0.1.0"


def test_breaking_footer_before_1_0_bumps_the_minor(fx: Fixture, git_cliff: Path) -> None:
    message = "docs(server): x\n\nBREAKING CHANGE: the old flag is gone"
    released(fx).commit(message, "services/server/a.go")
    assert prep(fx, git_cliff).version == "0.2.0"


def test_breaking_chore_counts(fx: Fixture, git_cliff: Path) -> None:
    released(fx).commit("chore(server)!: drop the old layout", "services/server/a.go")
    assert prep(fx, git_cliff).version == "0.2.0"


def test_next_stable_rule() -> None:
    change = release_prep.Change
    assert release_prep.next_stable("1.4.2", [change("a", "perf", False)]) == "1.4.3"
    assert release_prep.next_stable("1.4.2", [change("a", "docs", False)]) is None
    assert release_prep.next_stable(None, [change("a", "fix", False)]) == "0.1.0"
    assert release_prep.next_stable(None, [change("a", "chore", False)]) is None


def test_beta_counter_counts_existing_betas(fx: Fixture, git_cliff: Path) -> None:
    released(fx).commit("feat(server): new thing", "services/server/a.go")
    first = prep(fx, git_cliff, channel="beta")
    assert first.version == "0.2.0-beta.1"
    fx.commit("chore(release): server v0.2.0-beta.1", "services/server/x")
    fx.tag("server/v0.2.0-beta.1")
    fx.tag("server/v0.2.0-beta.2")
    (fx.root / "services/server/CHANGELOG.md").write_text(
        fx.read("services/server/CHANGELOG.md").replace(
            "## [Unreleased]\n", "## [Unreleased]\n\n### Fixed\n\n- More.\n", 1
        ),
        encoding="utf-8",
    )
    fx.commit("fix(server): follow-up", "services/server/a.go")
    assert prep(fx, git_cliff, channel="beta").version == "0.2.0-beta.3"


def test_stable_after_beta_keeps_the_version(fx: Fixture, git_cliff: Path) -> None:
    released(fx).commit("feat(server): new thing", "services/server/a.go")
    prep(fx, git_cliff, channel="beta")
    fx.commit("chore(release): server v0.2.0-beta.1", "services/server/x")
    fx.tag("server/v0.2.0-beta.1")
    result = prep(fx, git_cliff)
    assert result.version == "0.2.0"
    text = fx.read("services/server/CHANGELOG.md")
    assert (
        f"## [0.2.0] - 2026-10-15\n\n{release_prep.SUMMARY_PLACEHOLDER}\n\n"
        "Stable release of 0.2.0-beta.1"
    ) in text
    assert text.index("## [0.2.0]") < text.index("## [0.2.0-beta.1]")


def test_changelog_section_is_renamed(fx: Fixture, git_cliff: Path) -> None:
    released(fx).commit("fix(server): repair it", "services/server/a.go")
    prep(fx, git_cliff)
    text = fx.read("services/server/CHANGELOG.md")
    assert (
        f"## [Unreleased]\n\n## [0.1.1] - 2026-10-15\n\n{release_prep.SUMMARY_PLACEHOLDER}\n\n"
        "### Added\n\n- Something for users."
    ) in text
    assert text.count("## [Unreleased]") == 1


def test_checklist_lists_commits_without_a_changelog_change(fx: Fixture, git_cliff: Path) -> None:
    released(fx)
    without = fx.commit("fix(server): forgot the changelog", "services/server/a.go")
    fx.commit("feat(server): with entry", "services/server/b.go", "services/server/CHANGELOG.md")
    fx.commit("fix(sdk-python): other component", "sdk/python/b.py")
    result = prep(fx, git_cliff)
    assert [c.sha for c in result.checklist] == [without]
    assert result.checklist[0].subject == "fix(server): forgot the changelog"


def test_nothing_to_release_writes_nothing(fx: Fixture, git_cliff: Path) -> None:
    released(fx).commit("docs(server): only words", "services/server/README.md")
    before = (fx.read("services/server/VERSION"), fx.read("services/server/CHANGELOG.md"))
    with pytest.raises(release_prep.ReleasePrepError, match="nothing to release"):
        prep(fx, git_cliff)
    assert (fx.read("services/server/VERSION"), fx.read("services/server/CHANGELOG.md")) == before


def test_missing_unreleased_section_writes_nothing(fx: Fixture, git_cliff: Path) -> None:
    released(fx).commit("fix(server): repair it", "services/server/a.go")
    (fx.root / "services/server/CHANGELOG.md").write_text("# Changelog\n", encoding="utf-8")
    with pytest.raises(release_prep.ReleasePrepError, match="Unreleased"):
        prep(fx, git_cliff)
    assert fx.read("services/server/VERSION") == "0.1.0\n"


def test_empty_unreleased_section_is_an_error(fx: Fixture, git_cliff: Path) -> None:
    released(fx).commit("fix(server): repair it", "services/server/a.go")
    (fx.root / "services/server/CHANGELOG.md").write_text(
        "# Changelog\n\n## [Unreleased]\n\n## [0.1.0] - 2026-01-01\n\n- x\n", encoding="utf-8"
    )
    with pytest.raises(release_prep.ReleasePrepError, match="no entries"):
        prep(fx, git_cliff)
    assert fx.read("services/server/VERSION") == "0.1.0\n"


def test_existing_section_is_an_error(fx: Fixture, git_cliff: Path) -> None:
    released(fx).commit("fix(server): repair it", "services/server/a.go")
    path = fx.root / "services/server/CHANGELOG.md"
    path.write_text(path.read_text() + "\n## [0.1.1] - 2026-02-02\n\n- y\n", encoding="utf-8")
    with pytest.raises(release_prep.ReleasePrepError, match=r"already has a section \[0.1.1\]"):
        prep(fx, git_cliff)


def test_tag_on_a_commit_outside_the_component_still_counts(fx: Fixture, git_cliff: Path) -> None:
    fx.commit("feat(server): before the tag", "services/server/a.go")
    fx.commit("chore: somewhere else", "README.md")
    fx.tag("server/v0.1.0")
    fx.commit("fix(server): repair it", "services/server/a.go")
    result = prep(fx, git_cliff)
    assert result.version == "0.1.1"
    assert [c.subject for c in result.checklist] == ["fix(server): repair it"]


def test_disagreeing_git_cliff_is_an_error(
    fx: Fixture, git_cliff: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    released(fx).commit("fix(server): repair it", "services/server/a.go")
    change = release_prep.Change("a" * 40, "fix", False)
    monkeypatch.setattr(release_prep, "unreleased_changes", lambda *a: (None, [change]))
    with pytest.raises(release_prep.ReleasePrepError, match=r"not from tag server/v0\.1\.0"):
        prep(fx, git_cliff)


def test_git_cliff_failure_is_an_error(fx: Fixture, tmp_path: Path) -> None:
    released(fx).commit("fix(server): repair it", "services/server/a.go")
    fake = tmp_path / "git-cliff"
    fake.write_text("#!/bin/sh\necho broken >&2\nexit 3\n", encoding="utf-8")
    fake.chmod(0o755)
    with pytest.raises(release_prep.ReleasePrepError, match="broken"):
        prep(fx, fake)


def test_unknown_component_and_channel(fx: Fixture, git_cliff: Path) -> None:
    with pytest.raises(release_prep.ReleasePrepError, match="unknown release component"):
        prep(fx, git_cliff, "nope")
    with pytest.raises(release_prep.ReleasePrepError, match="CHANNEL"):
        prep(fx, git_cliff, channel="nightly")


def test_root_component_uses_plain_tags(fx: Fixture, git_cliff: Path) -> None:
    product(fx, pinned="1.4.0")
    assert prep(fx, git_cliff, "bothy").version == "0.1.1"
    assert fx.read("VERSION") == "0.1.1\n"


def test_cli_prints_version_and_checklist(
    fx: Fixture, git_cliff: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    released(fx).commit("fix(server): no entry", "services/server/a.go")
    rc = release_prep.main(
        [
            "--component",
            "server",
            "--repo",
            str(fx.root),
            "--git-cliff",
            str(git_cliff),
            "--config",
            str(CLIFF_CONFIG),
            "--date",
            DATE,
        ]
    )
    out = capsys.readouterr().out
    assert rc == 0
    assert "server 0.1.1" in out
    assert "fix(server): no entry" in out
    assert "git tag -s server/v0.1.1" in out
    assert "replace the summary placeholder" in out


def test_cli_error_exit(fx: Fixture, git_cliff: Path, capsys: pytest.CaptureFixture[str]) -> None:
    rc = release_prep.main(
        ["--component", "server", "--repo", str(fx.root), "--git-cliff", str(git_cliff)]
    )
    assert rc == 1
    assert "release-prep:" in capsys.readouterr().err


def test_no_commit_since_the_tag_is_nothing_to_release(fx: Fixture, git_cliff: Path) -> None:
    released(fx)
    fx.commit("fix(sdk-python): elsewhere", "sdk/python/b.py")
    with pytest.raises(release_prep.ReleasePrepError, match="nothing to release: no commit"):
        prep(fx, git_cliff)


def test_server_version_writes_the_pin(fx: Fixture, git_cliff: Path) -> None:
    product(fx)
    before = fx.read(ENV_EXAMPLE)
    result = prep(fx, git_cliff, "bothy", server_version="1.4.0")
    assert result.version == "0.1.1"
    assert fx.read(ENV_EXAMPLE) == before.replace(
        "BOTHY_VERSION=0.0.0-placeholder\n", "BOTHY_VERSION=1.4.0\n"
    )
    assert fx.read("VERSION") == "0.1.1\n"
    assert "## [0.1.1] - 2026-10-15" in fx.read("CHANGELOG.md")
    assert ENV_EXAMPLE in result.other_files


def test_server_version_without_its_tag_writes_nothing(fx: Fixture, git_cliff: Path) -> None:
    product(fx)
    before = snapshot(fx)
    with pytest.raises(release_prep.ReleasePrepError, match=r"server/v1\.5\.0"):
        prep(fx, git_cliff, "bothy", server_version="1.5.0")
    assert snapshot(fx) == before


@pytest.mark.parametrize("value", ["v1.4.0", "1.4", "latest", ""])
def test_server_version_that_is_not_semver_writes_nothing(
    fx: Fixture, git_cliff: Path, value: str
) -> None:
    product(fx)
    before = snapshot(fx)
    with pytest.raises(release_prep.ReleasePrepError, match="SERVER_VERSION"):
        prep(fx, git_cliff, "bothy", server_version=value)
    assert snapshot(fx) == before


def test_stable_product_rejects_a_server_pre_release(fx: Fixture, git_cliff: Path) -> None:
    product(fx)
    before = snapshot(fx)
    with pytest.raises(release_prep.ReleasePrepError, match=r"2\.0\.0-beta\.1.*CHANNEL=beta"):
        prep(fx, git_cliff, "bothy", server_version="2.0.0-beta.1")
    assert snapshot(fx) == before


def test_product_beta_accepts_a_server_pre_release(fx: Fixture, git_cliff: Path) -> None:
    product(fx)
    result = prep(fx, git_cliff, "bothy", channel="beta", server_version="2.0.0-beta.1")
    assert result.version == "0.1.1-beta.1"
    assert "BOTHY_VERSION=2.0.0-beta.1\n" in fx.read(ENV_EXAMPLE)


def test_missing_server_version_with_the_placeholder_writes_nothing(
    fx: Fixture, git_cliff: Path
) -> None:
    product(fx)
    before = snapshot(fx)
    with pytest.raises(release_prep.ReleasePrepError, match="SERVER_VERSION"):
        prep(fx, git_cliff, "bothy")
    assert snapshot(fx) == before


def test_missing_server_version_keeps_a_pinned_line(fx: Fixture, git_cliff: Path) -> None:
    product(fx, pinned="1.4.0")
    before = fx.read(ENV_EXAMPLE)
    assert prep(fx, git_cliff, "bothy").version == "0.1.1"
    assert fx.read(ENV_EXAMPLE) == before


def test_server_version_for_another_component_writes_nothing(fx: Fixture, git_cliff: Path) -> None:
    released(fx).commit("fix(server): repair it", "services/server/a.go")
    before = (fx.read("services/server/VERSION"), fx.read("services/server/CHANGELOG.md"))
    with pytest.raises(release_prep.ReleasePrepError, match="only to the component"):
        prep(fx, git_cliff, "server", server_version="1.4.0")
    assert (fx.read("services/server/VERSION"), fx.read("services/server/CHANGELOG.md")) == before


def test_cli_rejects_server_version_for_another_component(
    fx: Fixture, git_cliff: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    with pytest.raises(SystemExit) as exc:
        release_prep.main(
            [
                "--component",
                "server",
                "--server-version",
                "1.4.0",
                "--repo",
                str(fx.root),
                "--git-cliff",
                str(git_cliff),
            ]
        )
    assert exc.value.code == 2
    assert "--server-version" in capsys.readouterr().err


def test_documented_install_command_follows_the_product_version(
    fx: Fixture, git_cliff: Path
) -> None:
    product(fx)
    readme, guide = fx.read(README), fx.read(INSTALL_DOC)
    result = prep(fx, git_cliff, "bothy", server_version="1.4.0")
    # Only URLs of the documented command change; the bundle URL of v0.1.0 stays.
    assert fx.read(README) == readme.replace(install_url("0.1.0"), install_url("0.1.1"))
    assert fx.read(INSTALL_DOC) == guide.replace(install_url("0.1.0"), install_url("0.1.1"))
    assert "bothy-compose.tar.gz" in fx.read(README)
    assert "v0.1.0/bothy-compose.tar.gz" in fx.read(README)
    assert {README, INSTALL_DOC} <= set(result.other_files)


def test_documented_command_of_a_beta_names_the_beta(fx: Fixture, git_cliff: Path) -> None:
    product(fx)
    prep(fx, git_cliff, "bothy", channel="beta", server_version="1.4.0")
    assert install_url("0.1.1-beta.1") in fx.read(README)
    assert install_url("0.1.0") not in fx.read(INSTALL_DOC)


@pytest.mark.parametrize("name", [README, INSTALL_DOC])
def test_missing_documented_command_writes_nothing(fx: Fixture, git_cliff: Path, name: str) -> None:
    product(fx)
    (fx.root / name).write_text("# No command here.\n", encoding="utf-8")
    before = snapshot(fx)
    with pytest.raises(release_prep.ReleasePrepError, match=re.escape(name)):
        prep(fx, git_cliff, "bothy", server_version="1.4.0")
    assert snapshot(fx) == before


@pytest.mark.parametrize("name", [README, INSTALL_DOC, ENV_EXAMPLE])
def test_missing_product_file_writes_nothing(fx: Fixture, git_cliff: Path, name: str) -> None:
    product(fx)
    (fx.root / name).unlink()
    before = snapshot(fx)
    with pytest.raises(release_prep.ReleasePrepError, match=re.escape(name)):
        prep(fx, git_cliff, "bothy", server_version="1.4.0")
    assert snapshot(fx) == before


def test_two_pin_lines_write_nothing(fx: Fixture, git_cliff: Path) -> None:
    product(fx)
    path = fx.root / ENV_EXAMPLE
    path.write_text(path.read_text(encoding="utf-8") + "BOTHY_VERSION=1.0.0\n", encoding="utf-8")
    before = snapshot(fx)
    with pytest.raises(release_prep.ReleasePrepError, match="exactly one line BOTHY_VERSION="):
        prep(fx, git_cliff, "bothy", server_version="1.4.0")
    assert snapshot(fx) == before


def test_later_changelog_error_leaves_the_product_files(fx: Fixture, git_cliff: Path) -> None:
    product(fx)
    (fx.root / "CHANGELOG.md").write_text("# Changelog\n", encoding="utf-8")
    before = snapshot(fx)
    with pytest.raises(release_prep.ReleasePrepError, match="Unreleased"):
        prep(fx, git_cliff, "bothy", server_version="1.4.0")
    assert snapshot(fx) == before


def test_cli_passes_the_server_version(
    fx: Fixture, git_cliff: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    product(fx)
    rc = release_prep.main(
        [
            "--component",
            "bothy",
            "--server-version",
            "1.4.0",
            "--repo",
            str(fx.root),
            "--git-cliff",
            str(git_cliff),
            "--config",
            str(CLIFF_CONFIG),
            "--date",
            DATE,
        ]
    )
    out = capsys.readouterr().out
    assert rc == 0
    assert "BOTHY_VERSION=1.4.0" in fx.read(ENV_EXAMPLE)
    assert ENV_EXAMPLE in out


def test_fragments_go_into_the_release_section_and_are_deleted(
    fx: Fixture, git_cliff: Path
) -> None:
    """The tickets' changelog fragments become part of the version section (PL/24)."""
    released(fx)
    folder = fx.root / "services/server/changelog.d"
    folder.mkdir()
    (folder / "M2-05.md").write_text("### Fixed\n\n- A fixed thing.\n", encoding="utf-8")
    (folder / "M2-04.md").write_text("### Added\n\n- A new thing.\n", encoding="utf-8")
    fx.commit("feat(server): with a fragment", "services/server/b.go")
    without = fx.commit("fix(server): without an entry", "services/server/c.go")
    result = prep(fx, git_cliff)
    text = fx.read("services/server/CHANGELOG.md")
    section = text.split("## [0.2.0] - 2026-10-15", 1)[1].split("\n## [", 1)[0]
    assert "- Something for users.\n- A new thing.\n" in section
    assert "### Fixed\n\n- A fixed thing.\n" in section
    assert not folder.exists() or not any(folder.iterdir())
    assert result.used_fragments == (
        "services/server/changelog.d/M2-04.md",
        "services/server/changelog.d/M2-05.md",
    )
    assert [c.sha for c in result.checklist] == [without]


def test_a_broken_fragment_writes_nothing(fx: Fixture, git_cliff: Path) -> None:
    released(fx)
    folder = fx.root / "services/server/changelog.d"
    folder.mkdir()
    (folder / "M2-05.md").write_text("### Improved\n\n- Words.\n", encoding="utf-8")
    fx.commit("fix(server): a fix", "services/server/b.go")
    before = fx.read("services/server/CHANGELOG.md")
    with pytest.raises(release_prep.ReleasePrepError, match="not one of"):
        prep(fx, git_cliff)
    assert fx.read("services/server/CHANGELOG.md") == before
    assert (folder / "M2-05.md").is_file()
