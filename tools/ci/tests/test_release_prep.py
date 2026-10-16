# SPDX-License-Identifier: Apache-2.0
"""Tests for `make release-prep` (tools/ci/release_prep.py, ADR 0060 "Confirmation").

Every case builds a fixture repository in a temporary directory with commits in several
components and runs the pinned git-cliff against it, so the version rule is the real one.
"""

from __future__ import annotations

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


def prep(fx: Fixture, git_cliff: Path, component: str = "server", channel: str = "stable"):
    return release_prep.prepare(
        component,
        channel=channel,
        repo=fx.root,
        git_cliff=git_cliff,
        config=CLIFF_CONFIG,
        date=DATE,
    )


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
    assert "## [0.2.0] - 2026-10-15\n\nStable release of 0.2.0-beta.1" in text
    assert text.index("## [0.2.0]") < text.index("## [0.2.0-beta.1]")


def test_changelog_section_is_renamed(fx: Fixture, git_cliff: Path) -> None:
    released(fx).commit("fix(server): repair it", "services/server/a.go")
    prep(fx, git_cliff)
    text = fx.read("services/server/CHANGELOG.md")
    assert "## [Unreleased]\n\n## [0.1.1] - 2026-10-15\n\n### Added\n\n- Something for users." in (
        text
    )
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
    fx.commit("chore(release): v0.1.0", "VERSION")
    fx.tag("v0.1.0")
    fx.commit("fix(compose): repair", "infra/compose/compose.yaml")
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
