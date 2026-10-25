# SPDX-License-Identifier: Apache-2.0
"""The checks of answer attachments (brief M3-12 notes 2 to 8, 09 18.10, plan row M3-12)."""

from __future__ import annotations

import os
import shutil
import socket
import tempfile
from pathlib import Path

import pytest

from claude_code_runner.config import ConfigError, load_config
from claude_code_runner.reply_attachments import (
    DEFAULT_DENIED_PATTERNS,
    REASONS,
    ROOT_NAMES,
    AllowedRoots,
    Candidate,
    DeniedPatterns,
    Limits,
    Outcome,
    Rejection,
    allowed_roots,
    build_notice,
    check_candidate,
    check_set,
    display_name,
    split_attach_lines,
    with_notice,
)
from claude_code_runner.title import split_title

from .test_config import write

THREAD = "0192d3a0-0000-7000-8000-00000000a001"
OTHER_THREAD = "0192d3a0-0000-7000-8000-00000000a002"
RUN = "0192d3a0-0000-7000-8000-00000000b001"
OTHER_RUN = "0192d3a0-0000-7000-8000-00000000b002"
CANARY = b"canary-secret-0123"


# ---- note 2 --------------------------------------------------------------------------------


def test_split_attach_lines() -> None:
    assert split_attach_lines("Done.\nATTACH: report.md\n") == ("Done.", ["report.md"])
    # Not ATTACH lines: indented, lower case, no space after the colon.
    for text in (" ATTACH: a.md", "attach: a.md", "ATTACH:a.md"):
        assert split_attach_lines(text) == (text, [])
    assert split_attach_lines("ATTACH:   a b.md  \t") == ("", ["a b.md"])
    assert split_attach_lines("ATTACH:\ta.md") == ("", ["a.md"])
    fenced = [
        "```\nATTACH: x.txt\n```",
        "````\nATTACH: x.txt\n```\nATTACH: y.txt\n````",
        "~~~\nATTACH: x.txt\n~~~",
        "```python\nATTACH: x.txt",  # unclosed: runs to the end
    ]
    for text in fenced:
        assert split_attach_lines(text) == (text, [])
    # A tilde fence is not closed by backticks; the line after the real close counts again.
    text, paths = split_attach_lines("~~~\n```\nATTACH: x\n~~~\nATTACH: y")
    assert (text, paths) == ("~~~\n```\nATTACH: x\n~~~", ["y"])
    # Empty path: removed and reported as an empty entry (named "(empty)", reason not_found).
    assert split_attach_lines("Hi\nATTACH:  \nThere") == ("Hi\nThere", [""])
    text, paths = split_attach_lines("a\nATTACH: 2.md\nb\nATTACH: 1.md\n\n\n")
    assert (text, paths) == ("a\nb", ["2.md", "1.md"])
    assert split_attach_lines("a\r\nATTACH: x\r\nb") == ("a\r\nb", ["x"])
    # After the TITLE line was removed, both lines are gone.
    _, reply = split_title("TITLE: Report\nHere it is.\nATTACH: out/r.md", True)
    assert split_attach_lines(reply) == ("Here it is.", ["out/r.md"])


# ---- note 3 --------------------------------------------------------------------------------


def test_allowed_roots_config(tmp_path: Path) -> None:
    cfg = load_config(write(tmp_path))
    assert cfg.attach_allowed_roots == ROOT_NAMES
    assert cfg.attach_denied_patterns == DEFAULT_DENIED_PATTERNS
    cfg = load_config(write(tmp_path, 'attach_allowed_roots = ["project"]'))
    assert cfg.attach_allowed_roots == ("project",)
    for bad in ('["home"]', '["project", "project"]', '["/srv/secret-place"]'):
        with pytest.raises(ConfigError) as err:
            load_config(write(tmp_path, f"attach_allowed_roots = {bad}"))
        assert err.value.key == "runner.attach_allowed_roots"
        assert str(tmp_path / "runner.toml") in str(err.value)
        assert "secret-place" not in str(err.value)
        assert "home" not in err.value.problem


class Layout:
    """The temporary layout of ``test_check_candidate_roots``."""

    def __init__(self, base: Path) -> None:
        self.base = base
        self.projects = base / "projects"
        self.website = self.projects / "website"
        self.other = self.projects / "other"
        self.state = base / "state"
        self.inbox = self.state / "files" / THREAD
        self.other_inbox = self.state / "files" / OTHER_THREAD
        self.temp = self.state / "tmp" / RUN
        self.other_temp = self.state / "tmp" / OTHER_RUN
        self.sessions = self.state / "sessions" / "ref"
        self.outside = base / "home-of-alex"
        for d in (
            self.website,
            self.other,
            self.inbox,
            self.other_inbox,
            self.temp,
            self.other_temp,
            self.sessions,
            self.outside,
        ):
            d.mkdir(parents=True)
        for d in (self.website, self.other, self.inbox, self.other_inbox, self.temp):
            (d / "file.txt").write_bytes(b"x")
        (self.other_temp / "file.txt").write_bytes(b"x")
        (self.sessions / f"{RUN}.input.jsonl").write_bytes(CANARY)
        (self.state / "inbox.sqlite3").write_bytes(CANARY)
        (self.projects / "top.txt").write_bytes(b"x")
        self.canary = self.outside / "secret.txt"
        self.canary.write_bytes(CANARY)

    def roots(self, names: tuple[str, ...] = ROOT_NAMES) -> AllowedRoots:
        return allowed_roots(
            names,
            project_dir=self.website,
            thread_inbox=self.inbox,
            run_temp=self.temp,
            state_dir=self.state,
            projects_root=self.projects,
        )

    def check(self, path: str | Path, patterns: tuple[str, ...] = DEFAULT_DENIED_PATTERNS) -> str:
        result = check_candidate(
            str(path), self.website, self.roots(), DeniedPatterns.of(patterns)
        )
        return "ok" if isinstance(result, Candidate) else result.code


@pytest.fixture
def lay(tmp_path: Path) -> Layout:
    return Layout(tmp_path)


def test_check_candidate_roots(lay: Layout) -> None:
    assert lay.check("file.txt") == "ok"
    assert lay.check(lay.inbox / "file.txt") == "ok"
    assert lay.check(lay.temp / "file.txt") == "ok"
    for path in (
        lay.other / "file.txt",
        lay.other_inbox / "file.txt",
        lay.other_temp / "file.txt",
        lay.sessions / f"{RUN}.input.jsonl",
        lay.state / "inbox.sqlite3",
        lay.projects / "top.txt",
        "../other/file.txt",
        lay.website / ".." / "other" / "file.txt",
        lay.canary,
    ):
        assert lay.check(path) == "outside_allowed_roots", path
    assert lay.check("missing.txt") == "not_found"
    assert lay.check("~/file.txt") == "not_found"  # never expanded
    (lay.website / "~").mkdir()
    (lay.website / "~" / "file.txt").write_bytes(b"x")
    assert lay.check("~/file.txt") == "ok"  # the literal directory of that name
    # A root itself is never a candidate; only the configured roots count.
    assert lay.check(lay.website) == "outside_allowed_roots"
    narrow = check_candidate(
        str(lay.temp / "file.txt"),
        lay.website,
        lay.roots(("project",)),
        DeniedPatterns.of(DEFAULT_DENIED_PATTERNS),
    )
    assert isinstance(narrow, Rejection)
    assert narrow.code == "outside_allowed_roots"


def test_allowed_roots_never_projects_root_or_state(lay: Layout) -> None:
    # A project that is the projects root (or above it) is never a root (note 3 b and d).
    roots = allowed_roots(
        ROOT_NAMES,
        project_dir=lay.projects,
        thread_inbox=lay.inbox,
        run_temp=lay.temp,
        state_dir=lay.state,
        projects_root=lay.projects,
    )
    assert str(lay.projects.resolve()) not in roots.roots
    above = allowed_roots(
        ("project",),
        project_dir=lay.base,
        thread_inbox=lay.inbox,
        run_temp=lay.temp,
        state_dir=lay.state,
        projects_root=lay.projects,
    )
    assert above.roots == ()
    in_state = allowed_roots(
        ("project",),
        project_dir=lay.sessions,
        thread_inbox=lay.inbox,
        run_temp=lay.temp,
        state_dir=lay.state,
        projects_root=lay.projects,
    )
    assert in_state.roots == ()
    missing = allowed_roots(
        ROOT_NAMES,
        project_dir=None,
        thread_inbox=lay.state / "files" / "none",
        run_temp=lay.temp,
        state_dir=lay.state,
        projects_root=lay.projects,
    )
    assert missing.roots == (str(lay.temp.resolve()),)


def test_state_dir_under_project_stays_forbidden(tmp_path: Path) -> None:
    projects = tmp_path / "projects"
    website = projects / "website"
    state = website / "state"
    (state / "sessions").mkdir(parents=True)
    (state / "sessions" / "x.jsonl").write_bytes(CANARY)
    (website / "ok.txt").write_bytes(b"x")
    roots = allowed_roots(
        ROOT_NAMES,
        project_dir=website,
        thread_inbox=state / "files" / THREAD,
        run_temp=state / "tmp" / RUN,
        state_dir=state,
        projects_root=projects,
    )
    patterns = DeniedPatterns.of(DEFAULT_DENIED_PATTERNS)
    bad = check_candidate("state/sessions/x.jsonl", website, roots, patterns)
    assert isinstance(bad, Rejection)
    assert bad.code == "outside_allowed_roots"
    assert isinstance(check_candidate("ok.txt", website, roots, patterns), Candidate)


def test_symlink_escape_rejected(lay: Layout) -> None:
    w = lay.website
    (w / "home-link").symlink_to(lay.canary)
    outside_abs = lay.base / "loose.txt"
    outside_abs.write_bytes(CANARY)
    (w / "abs-link").symlink_to(outside_abs)
    (w / "docs").symlink_to(lay.outside, target_is_directory=True)
    (w / "other-link").symlink_to(lay.other / "file.txt")
    (w / "session-link").symlink_to(lay.sessions / f"{RUN}.input.jsonl")
    (w / "hop2").symlink_to(lay.canary)
    (w / "hop1").symlink_to(w / "hop2")
    (w / "loop-a").symlink_to(w / "loop-b")
    (w / "loop-b").symlink_to(w / "loop-a")
    (w / "inner").symlink_to(w / "file.txt")
    for name in ("home-link", "abs-link", "docs/secret.txt", "other-link", "session-link", "hop1"):
        assert lay.check(name) == "outside_allowed_roots", name
    assert lay.check("loop-a") == "not_found"
    result = check_candidate(
        "inner", lay.website, lay.roots(), DeniedPatterns.of(DEFAULT_DENIED_PATTERNS)
    )
    assert isinstance(result, Candidate)
    assert result.real_path == str((w / "file.txt").resolve())


def test_secret_patterns_rejected(lay: Layout) -> None:
    w = lay.website
    examples = [
        ".env",
        ".env.local",
        "server.pem",
        "tls.key",
        "id_ed25519",
        ".netrc",
        ".ssh/config",
        ".claude/settings.json",
        ".config/tool/hosts.yml",
        ".aws/credentials",
        ".azure/profile.json",
        ".gnupg/pubring.kbx",
        ".ENV",
        "Server.PEM",
        ".ssh/deep/er/key.txt",
    ]
    for rel in examples:
        (w / rel).parent.mkdir(parents=True, exist_ok=True)
        (w / rel).write_bytes(CANARY)
    for rel in examples:
        assert lay.check(rel) == "denied_pattern", rel
    (w / "notes.txt").symlink_to(w / ".env")
    assert lay.check("notes.txt") == "denied_pattern"  # the real name is denied
    (w / "README.md").write_bytes(b"read me")
    (w / "id_rsa").symlink_to(w / "README.md")
    assert lay.check("id_rsa") == "denied_pattern"  # the announced name is denied
    assert lay.check("README.md") == "ok"


def test_projects_root_inside_denied_directory(tmp_path: Path) -> None:
    lay = Layout(tmp_path / ".config")
    (lay.website / "README.md").write_bytes(b"x")
    assert lay.check("README.md") == "ok"


def test_denied_patterns_config(tmp_path: Path, lay: Layout) -> None:
    tmp_path = tmp_path / "cfg"
    tmp_path.mkdir()
    without = [p for p in DEFAULT_DENIED_PATTERNS if p != ".ssh"]
    with pytest.raises(ConfigError) as err:
        load_config(write(tmp_path, f"attach_denied_patterns = {without!r}".replace("'", '"')))
    assert err.value.key == "runner.attach_denied_patterns"
    assert ".ssh" in err.value.problem
    added = [*DEFAULT_DENIED_PATTERNS, "*.sqlite"]
    cfg = load_config(write(tmp_path, f"attach_denied_patterns = {added!r}".replace("'", '"')))
    (lay.website / "data.sqlite").write_bytes(b"x")
    assert lay.check("data.sqlite", cfg.attach_denied_patterns) == "denied_pattern"
    for bad in ("dir/*.txt", "a\\u0007b", "a\\u202eb", ""):
        values = ", ".join(f'"{p}"' for p in [*DEFAULT_DENIED_PATTERNS, bad])
        with pytest.raises(ConfigError) as err:
            load_config(write(tmp_path, f"attach_denied_patterns = [{values}]"))
        assert err.value.key == "runner.attach_denied_patterns"


def test_regular_file_and_hard_link(lay: Layout) -> None:
    w = lay.website
    (w / "folder").mkdir()
    os.mkfifo(w / "pipe")
    assert lay.check("folder") == "not_a_regular_file"
    assert lay.check("pipe") == "not_a_regular_file"  # never opened, so never blocks
    os.link(lay.canary, w / "hard.txt")
    assert lay.check("hard.txt") == "hard_link"
    (w / "empty.txt").write_bytes(b"")
    assert lay.check("empty.txt") == "empty_file"


def test_socket_inside_root() -> None:
    # Socket paths have a small length limit: a short directory of its own, removed at the end.
    root = Path(tempfile.mkdtemp(prefix="s", dir="/tmp"))
    try:
        (root / "p" / "w").mkdir(parents=True)
        sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        try:
            sock.bind(str(root / "p" / "w" / "s"))
            roots = allowed_roots(
                ("project",),
                project_dir=root / "p" / "w",
                thread_inbox=root / "x",
                run_temp=root / "y",
                state_dir=root / "state",
                projects_root=root / "p",
            )
            result = check_candidate("s", root / "p" / "w", roots, DeniedPatterns.of(()))
            assert isinstance(result, Rejection)
            assert result.code == "not_a_regular_file"
        finally:
            sock.close()
    finally:
        shutil.rmtree(root)


# ---- note 6 --------------------------------------------------------------------------------


def _cand(name: str, size: int = 10, real: str | None = None) -> tuple[str, Candidate]:
    return name, Candidate(name, real or f"/p/{name}", size, 1, hash(name) & 0xFFFF)


def test_check_set_all_or_none() -> None:
    limits = Limits(max_attachments=3, max_upload_bytes=100)
    verdict = check_set([_cand("a"), ("b/.env", Rejection("denied_pattern")), _cand("c")], limits)
    assert not verdict.ok
    assert verdict.accepted == ()
    assert [o.rejection.code for o in verdict.outcomes if o.rejection] == [
        "set_not_sent",
        "denied_pattern",
        "set_not_sent",
    ]
    many = check_set([_cand(n) for n in "abcd"], limits)
    assert {o.rejection.code for o in many.outcomes if o.rejection} == {"too_many"}
    big = check_set([_cand("a"), _cand("b", size=101), _cand("c")], limits)
    assert [o.rejection.code for o in big.outcomes if o.rejection] == [
        "set_not_sent",
        "too_large",
        "set_not_sent",
    ]
    twice = check_set([_cand("a"), _cand("b", real="/p/a"), _cand("c")], limits)
    assert twice.ok
    assert [c.announced for c in twice.accepted] == ["a", "c"]
    assert [o.announced for o in twice.outcomes] == ["a", "c"]
    # The duplicate does not count against the limit.
    dup4 = check_set([_cand("a"), _cand("b"), _cand("c"), _cand("d", real="/p/a")], limits)
    assert dup4.ok
    none = check_set([_cand("a"), _cand("b")], None)
    assert {o.rejection.code for o in none.outcomes if o.rejection} == {"limits_unavailable"}
    good = check_set([_cand("a"), _cand("b", size=100)], limits)
    assert good.ok
    assert [c.announced for c in good.accepted] == ["a", "b"]


# ---- note 8 --------------------------------------------------------------------------------


def test_build_notice() -> None:
    assert display_name("/srv/projects/website/out/report.md") == "report.md"
    assert display_name("../x/y.txt") == "y.txt"
    assert display_name("a/b\u202ec`d\x07e.txt") == "bcde.txt"
    assert display_name("") == "(empty)"
    long = "\u00e9" * 200  # 400 bytes in UTF-8: cut to 255 bytes, never inside a character
    assert display_name(long) == "\u00e9" * 127
    outcomes = [
        Outcome("/srv/projects/website/a.md", Rejection("set_not_sent")),
        Outcome("b.zip", Rejection("upload_refused", "conflict", "quota_exceeded")),
        Outcome("c.md", Rejection("not_attached", "feature_disabled")),
        Outcome("d.md", Rejection("outside_allowed_roots")),
    ]
    notice = build_notice(outcomes, 3)
    assert notice == (
        "Attachments not sent:\n"
        "- `a.md`: set_not_sent\n"
        "- `b.zip`: upload_refused (conflict, quota_exceeded)\n"
        "- `c.md`: not_attached (feature_disabled)\n"
        "- and 1 more"
    )
    assert "/srv" not in notice
    assert build_notice([Outcome("a.md"), Outcome("b.md")], 3) == ""
    assert with_notice("Done.", "") == "Done."
    assert with_notice("Done.", "N") == "Done.\n\nN"
    assert with_notice("", "N") == "N"
    assert set(REASONS) >= {o.rejection.code for o in outcomes if o.rejection}
