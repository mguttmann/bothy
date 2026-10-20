# SPDX-License-Identifier: Apache-2.0
"""Tests for tools/ci/install_options_check.py (docs/spec/16 sections 3.1 and 11.1).

Every case writes a fixture repository into a temporary directory: the spec chapter with
section 3.1, the options file of the portability test, the documented installation command
in README.md and the installation guide, and VERSION.
"""

from __future__ import annotations

from pathlib import Path

import install_options_check as ioc
import pytest

REPO = Path(__file__).resolve().parents[3]
OPTIONS = (
    "--yes --public-url https://chat.example.com --edge direct --scan on --storage s3 "
    "--owner-handle alex --owner-name Alex"
)
SECOND_BLOCK = """curl -fsSL https://github.com/OWNER/bothy/releases/download/v0.0.0/install.sh \\
  | sh -s -- --yes --public-url https://chat.example.com --edge direct --scan on --storage s3 \\
      --owner-handle alex --owner-name Alex"""
OPTIONS_FILE = f"""# Options the portability test passes to install.sh after `sh -s --`.
# make check holds them equal to the second block of docs/spec/16 section 3.1.
{OPTIONS}
"""


def spec(second_block: str = SECOND_BLOCK) -> str:
    return f"""# 16 Operations

## 3. Installation

### 3.1 Installation in one command (setup wizard)

```sh
curl -fsSL https://github.com/OWNER/bothy/releases/download/v0.0.0/install.sh | sh
```

Without a terminal:

```sh
{second_block}
```

### 3.2 Next section

```sh
curl -fsSL https://example.com/other.sh | sh -s -- --yes --edge tailscale
```
"""


def url(version: str) -> str:
    return f"https://github.com/OWNER/bothy/releases/download/v{version}/install.sh"


class Repo:
    """A fixture repository with every file the check reads."""

    def __init__(self, root: Path) -> None:
        self.root = root
        self.write("docs/spec/16-operations.md", spec())
        self.write("tests/portability/install-options", OPTIONS_FILE)
        self.write("VERSION", "0.3.0\n")
        self.write("README.md", f"# Product\n\n    curl -fsSL {url('0.3.0')} | sh\n")
        self.write(
            "docs/operations/install-compose.md",
            f"# Install\n\n```sh\ncurl -fsSL {url('0.3.0')} | sh\n```\n",
        )

    def write(self, name: str, text: str) -> None:
        path = self.root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text, encoding="utf-8")

    def options(self, line: str) -> None:
        self.write("tests/portability/install-options", f"# Options.\n{line}\n")


@pytest.fixture
def repo(tmp_path: Path) -> Repo:
    return Repo(tmp_path)


def test_equal_options_pass(repo: Repo) -> None:
    assert ioc.check(repo.root) == []


def test_changed_option_value_fails(repo: Repo) -> None:
    repo.options(OPTIONS.replace("--edge direct", "--edge tailscale"))
    findings = ioc.check(repo.root)
    assert len(findings) == 1
    assert "--edge" in findings[0]
    assert "tailscale" in findings[0]


def test_missing_option_fails(repo: Repo) -> None:
    repo.options(OPTIONS.replace(" --scan on", ""))
    findings = ioc.check(repo.root)
    assert len(findings) == 1
    assert "--scan" in findings[0]
    assert "missing" in findings[0]


def test_extra_option_fails(repo: Repo) -> None:
    repo.options(OPTIONS + " --observability on")
    findings = ioc.check(repo.root)
    assert len(findings) == 1
    assert "--observability" in findings[0]


def test_reordered_options_fail(repo: Repo) -> None:
    repo.options(OPTIONS.replace("--edge direct --scan on", "--scan on --edge direct"))
    findings = ioc.check(repo.root)
    assert len(findings) == 1
    assert "order" in findings[0]


def test_missing_flag_fails(repo: Repo) -> None:
    repo.options(OPTIONS.replace("--yes ", ""))
    findings = ioc.check(repo.root)
    assert any("--yes" in f for f in findings)


def test_option_without_value_fails(repo: Repo) -> None:
    repo.options(OPTIONS + " --dir")
    assert any("--dir" in f for f in ioc.check(repo.root))


def test_quoted_words_fail(repo: Repo) -> None:
    repo.options(OPTIONS.replace("--owner-name Alex", "--owner-name 'Alex'"))
    assert any("quote" in f for f in ioc.check(repo.root))


def test_two_option_lines_fail(repo: Repo) -> None:
    repo.options(f"{OPTIONS}\n--yes")
    assert any("exactly one line" in f for f in ioc.check(repo.root))


def test_different_storage_passes(repo: Repo) -> None:
    repo.options(OPTIONS.replace("--storage s3", "--storage fs"))
    assert ioc.check(repo.root) == []


def test_bundle_is_ignored(repo: Repo) -> None:
    repo.options(OPTIONS + " --bundle bothy-compose-0.3.0.tar.gz")
    assert ioc.check(repo.root) == []
    repo.write(
        "docs/spec/16-operations.md",
        spec(SECOND_BLOCK.replace("--yes ", "--yes --bundle b.tar.gz ")),
    )
    repo.options(OPTIONS)
    assert ioc.check(repo.root) == []


def test_changed_script_source_alone_passes(repo: Repo) -> None:
    block = SECOND_BLOCK.replace(
        "curl -fsSL https://github.com/OWNER/bothy/releases/download/v0.0.0/install.sh",
        "curl --proto '=https' -sSf https://example.com/elsewhere/install.sh",
    )
    repo.write("docs/spec/16-operations.md", spec(block))
    assert ioc.check(repo.root) == []


def test_spec_change_is_seen(repo: Repo) -> None:
    repo.write(
        "docs/spec/16-operations.md",
        spec(SECOND_BLOCK.replace("--owner-handle alex", "--owner-handle sam")),
    )
    findings = ioc.check(repo.root)
    assert len(findings) == 1
    assert "--owner-handle" in findings[0]


def test_spec_chapter_must_be_unique(repo: Repo) -> None:
    repo.write("docs/spec/16-betrieb.md", spec())
    assert any("exactly one" in f for f in ioc.check(repo.root))


def test_spec_without_a_second_block_fails(repo: Repo) -> None:
    text = spec().replace("```sh\n" + SECOND_BLOCK + "\n```\n", "")
    repo.write("docs/spec/16-operations.md", text)
    assert any("second" in f for f in ioc.check(repo.root))


def test_missing_options_file_fails(repo: Repo) -> None:
    (repo.root / "tests/portability/install-options").unlink()
    assert any("tests/portability/install-options" in f for f in ioc.check(repo.root))


def test_documented_version_equal_to_version_passes(repo: Repo) -> None:
    assert ioc.documented_command_findings(repo.root) == []


def test_documented_version_different_from_version_fails(repo: Repo) -> None:
    repo.write("VERSION", "0.4.0\n")
    findings = ioc.check(repo.root)
    assert len(findings) == 2
    assert any("README.md" in f and "v0.3.0" in f and "0.4.0" in f for f in findings)
    assert any("docs/operations/install-compose.md" in f for f in findings)


def test_one_stale_url_among_current_ones_fails(repo: Repo) -> None:
    repo.write("README.md", f"curl -fsSL {url('0.3.0')} | sh\nold: {url('0.2.0')}\n")
    findings = ioc.check(repo.root)
    assert len(findings) == 1
    assert "README.md:2" in findings[0]


def test_file_without_the_command_fails(repo: Repo) -> None:
    repo.write("docs/operations/install-compose.md", "# Install\n\nNo command.\n")
    findings = ioc.check(repo.root)
    assert len(findings) == 1
    assert "docs/operations/install-compose.md" in findings[0]


def test_missing_documented_file_fails(repo: Repo) -> None:
    (repo.root / "README.md").unlink()
    findings = ioc.check(repo.root)
    assert len(findings) == 1
    assert "README.md" in findings[0]


def test_cli_exit_codes(repo: Repo, capsys: pytest.CaptureFixture[str]) -> None:
    assert ioc.main(["check", "--repo", str(repo.root)]) == 0
    repo.write("VERSION", "9.9.9\n")
    assert ioc.main(["check", "--repo", str(repo.root)]) == 1
    assert "install-options:" in capsys.readouterr().err
    with pytest.raises(SystemExit) as exc:
        ioc.main(["nonsense"])
    assert exc.value.code == 2


@pytest.mark.skipif(
    not ioc.INSTALL_URL.search((REPO / "README.md").read_text(encoding="utf-8")),
    reason="README.md does not name the documented installation command yet (M7-29 docs)",
)
def test_this_repository_passes() -> None:
    assert ioc.check(REPO) == []
