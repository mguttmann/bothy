# SPDX-License-Identifier: Apache-2.0
"""The probes of docs/spec/27-computer-use.md 16 against the real CLI (CU-06).

Collected only with ``CLAUDE_CODE_RUNNER_REAL_CLI=1`` (``tests/conftest.py``); free: no login and
no model call. The CLI runs against the local stand-in for the API (``stand_in.py``) with a
``HOME`` and a ``CLAUDE_CONFIG_DIR`` of its own; the MCP server is ``image_mcp.py``, a ``stdio``
server whose tool returns a PNG with a known byte pattern. The runner's own code writes the
per-turn MCP file (`claude_code_runner.computer.mcp_document`) and the argument list
(`build_argv`, `background_argv`), and deletes the session's files at the end
(`remove_session_files`).

Probes:

1. Print mode: the CLI starts the ``stdio`` server of the per-turn file, the server gets exactly
   the five variables of 27 10.1, the model's call of ``mcp__bothy_computer__screenshot`` reaches
   it, and the image lands in the transcript ``<config>/projects/<folder>/<session>.jsonl`` and as
   a file under ``<config>/projects/<folder>/<session>/tool-results/``. After
   `remove_session_files` no file under the CLI's folders holds the image.
2. Background mode (the CI job of ``make runner-real-cli``, ``ci_cli.py``): the same in a
   background session under ``host`` launched with the runner's argument list; after
   ``claude stop`` and ``claude rm`` and `remove_session_files` no file holds the image.
"""

from __future__ import annotations

import asyncio
import base64
import contextlib
import json
import shutil
import subprocess
import sys
import tempfile
import uuid
from collections.abc import AsyncIterator, Mapping
from pathlib import Path
from typing import Any
from uuid import UUID

from real_cli import image_mcp
from real_cli.background import MODEL, World, new_root, turn_ends
from real_cli.stand_in import Answer, cli_env, private_ancestors, stand_in, text, user_line
from real_cli.test_real_cli import claude_bin

from claude_code_runner.commandline import SessionChoice, build_argv
from claude_code_runner.computer import (
    ENV_NAMES,
    ComputerPaths,
    mcp_document,
    remove_session_files,
    session_files,
    write_turn_file,
)

TOOL = "mcp__bothy_computer__screenshot"
WAIT = 180.0
PROBE_WORD = "TAKE-A-SCREENSHOT"
SERVER = "https://chat.example.com"


class _Script:
    """The stand-in: a request of the conversation with the probe word gets one call of the
    tool, the request after its result a text; every other request (the CLI's own) a text."""

    def __init__(self) -> None:
        self.bodies: list[Mapping[str, Any]] = []
        self.results: list[Any] = []

    def __call__(self, body: Mapping[str, Any]) -> Answer:
        self.bodies.append(body)
        messages = body.get("messages") or []
        if not any(PROBE_WORD in json.dumps(m) for m in messages):
            return [text("Ok.")]
        last = messages[-1]
        content = last.get("content") if isinstance(last, Mapping) else None
        results = [
            block
            for block in (content if isinstance(content, list) else [])
            if isinstance(block, Mapping) and block.get("type") == "tool_result"
        ]
        if results:
            self.results.extend(results)
            return [text("Done.")]
        return [{"type": "tool_use", "id": "toolu_cu06", "name": TOOL, "input": {}}]


def _base() -> Path:
    base = Path(tempfile.mkdtemp(prefix="ccr"))
    found = private_ancestors(base / "x" / "work")
    assert not found, f"the temporary folder lies below {found[0].parent}: set TMPDIR elsewhere"
    return base


def _binary(folder: Path) -> Path:
    """An executable that starts ``image_mcp.py``: the file of ``runner.computer_mcp_bin``."""
    path = folder / "bothy-computer-mcp"
    path.write_text(f'#!/bin/sh\nexec "{sys.executable}" -I "{image_mcp.__file__}"\n')
    path.chmod(0o755)
    return path


def _turn_file(state: Path, run_id: UUID, folder: Path) -> ComputerPaths:
    paths = ComputerPaths(state, run_id)
    write_turn_file(paths, mcp_document(_binary(folder), paths, "probe-nonce", 30.0, SERVER))
    return paths


def _holders(*roots: Path) -> list[Path]:
    """Every file under ``roots`` that holds the image: its bytes or its base64 text."""
    data = image_mcp.png()
    needles = (image_mcp.SENTINEL, base64.b64encode(data)[:64])
    found = []
    for root in roots:
        for path in root.rglob("*"):
            if path.is_file() and not path.is_symlink():
                content = path.read_bytes()
                if any(n in content for n in needles):
                    found.append(path)
    return found


def _check_env(paths: ComputerPaths) -> None:
    received = json.loads((paths.request_dir / image_mcp.ENV_FILE).read_text())
    assert set(received) == set(ENV_NAMES)
    assert received["BOTHY_COMPUTER_DIR"] == str(paths.request_dir)
    assert received["BOTHY_COMPUTER_NONCE"] == "probe-nonce"
    assert received["BOTHY_COMPUTER_TIMEOUT_MS"] == "30000"
    assert received["BOTHY_COMPUTER_PINS"] == str(paths.pin_file)
    assert received["BOTHY_COMPUTER_SERVER"] == SERVER


def _check_layout(config: Path, session: str) -> None:
    """The layout of 27 16: the transcript and the session folder hold the image, nothing else
    of the configuration folder does."""
    found = set(session_files(config, session))
    assert {p.name for p in found} == {f"{session}.jsonl", session}
    holders = _holders(config)
    assert holders, "the image is in no file of the configuration folder"
    for path in holders:
        assert any(path == f or f in path.parents for f in found), path
    images = [p for p in holders if p.suffix == ".png"]
    assert images, "no tool result file of the image"
    assert all(p.parent.name == "tool-results" for p in images)


def test_print_mode_starts_the_server_and_its_files_go_with_the_session() -> None:
    base = _base()
    script = _Script()
    try:
        with stand_in(script) as (url, _):
            root = new_root(base)
            work = root / "work"
            work.mkdir(parents=True)
            state = root / "state"
            state.mkdir(mode=0o700)
            env = cli_env(root, url)
            paths = _turn_file(state, uuid.uuid4(), root)
            prompt = root / "system-prompt.md"
            prompt.write_text("")
            session = str(uuid.uuid4())
            argv = build_argv(
                claude_bin=claude_bin(),
                model=MODEL,
                session=SessionChoice(ref=session, resume=False, new=True),
                tool_class="host",
                tools=[],
                add_dir=None,
                system_prompt_file=prompt,
                computer_mcp=paths.turn_file,
            )
            _, line = user_line(PROBE_WORD)
            done = subprocess.run(
                argv,
                input=line,
                cwd=work,
                env=env,
                capture_output=True,
                timeout=WAIT,
                check=False,
            )
            assert done.returncode == 0, done.stderr.decode()[-2000:]
            results = [json.loads(x) for x in done.stdout.decode().splitlines() if x.strip()]
            assert any(r.get("type") == "result" and r.get("result") == "Done." for r in results)
            # The model's call reached the server and its image came back to the model.
            assert script.results, "the tool result never reached the API"
            assert "image" in json.dumps(script.results)
            _check_env(paths)
            config = root / "config"
            _check_layout(config, session)
            assert remove_session_files(config, session)
            assert _holders(root / "config", root / "home") == []
    finally:
        shutil.rmtree(base, ignore_errors=True)


@contextlib.asynccontextmanager
async def _background() -> AsyncIterator[tuple[World, _Script]]:
    base = _base()
    script = _Script()
    try:
        with stand_in(script) as (url, _):
            place = World(new_root(base), claude_bin(), url)
            try:
                yield place, script
            finally:
                await place.close()
    finally:
        shutil.rmtree(base, ignore_errors=True)


def test_background_session_starts_the_server_and_its_files_go_after_stop_and_rm() -> None:
    async def scenario() -> None:
        async with _background() as (place, script):
            place.accept_bypass()
            state = place.root / "state"
            state.mkdir(mode=0o700)
            paths = _turn_file(state, uuid.uuid4(), place.root)
            out = await place.launch(
                "computer", PROBE_WORD, tool_class="host", tools=(), computer_mcp=paths.turn_file
            )
            assert out.refusal is None, place.output("computer")
            assert out.short_id is not None
            entry = await place.until_entry(out.short_id)
            deadline = asyncio.get_running_loop().time() + WAIT
            while turn_ends(place.records(entry.session_id)) < 1:
                assert asyncio.get_running_loop().time() < deadline, "the turn never ended"
                await asyncio.sleep(0.5)
            assert script.results, "the tool result never reached the API"
            _check_env(paths)
            _check_layout(place.config, entry.session_id)
            assert await place.stop(out.short_id)
            await place.cli.remove(out.short_id)
            assert remove_session_files(place.config, entry.session_id)
            assert _holders(place.config, place.root / "home") == []

    asyncio.run(scenario())


def test_no_image_outside_the_configuration_folder_in_print_mode() -> None:
    """The MCP logs of the CLI under ``HOME`` name the server, never the image (27 16)."""
    base = _base()
    try:
        with stand_in(_Script()) as (url, _):
            root = new_root(base)
            (root / "work").mkdir(parents=True)
            state = root / "state"
            state.mkdir(mode=0o700)
            paths = _turn_file(state, uuid.uuid4(), root)
            prompt = root / "system-prompt.md"
            prompt.write_text("")
            argv = build_argv(
                claude_bin=claude_bin(),
                model=MODEL,
                session=SessionChoice(ref=str(uuid.uuid4()), resume=False, new=True),
                tool_class="host",
                tools=[],
                add_dir=None,
                system_prompt_file=prompt,
                computer_mcp=paths.turn_file,
            )
            _, line = user_line(PROBE_WORD)
            subprocess.run(
                argv,
                input=line,
                cwd=root / "work",
                env={**cli_env(root, url), "TMPDIR": str(root)},
                capture_output=True,
                timeout=WAIT,
                check=True,
            )
            assert _holders(root / "home") == []
            assert [p for p in _holders(root) if root / "config" not in p.parents] == []
    finally:
        shutil.rmtree(base, ignore_errors=True)
