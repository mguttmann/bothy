# SPDX-License-Identifier: Apache-2.0
"""A recording command runner for the tests of tools/distribution (no external tool runs)."""

from __future__ import annotations

import subprocess
from collections.abc import Callable, Sequence
from pathlib import Path
from typing import Any

import common

MACHO = b"\xcf\xfa\xed\xfe" + bytes(12)

Handler = Callable[[list[str], dict[str, Any]], "subprocess.CompletedProcess[str] | None"]


class FakeRun(common.Runner):
    """Records every command; `handlers` answer by program name (and may change files)."""

    def __init__(self, handlers: dict[str, Handler] | None = None) -> None:
        self.calls: list[list[str]] = []
        self.stdin: list[str | None] = []
        self.handlers = handlers or {}

    def __call__(
        self,
        cmd: Sequence[str | Path],
        *,
        cwd: Path | None = None,
        env: Any = None,
        stdin: str | None = None,
        capture: bool = False,
        check: bool = True,
        redact: Sequence[str] = (),
        patience: float = 0,
    ) -> subprocess.CompletedProcess[str]:
        args = [str(c) for c in cmd]
        self.calls.append(args)
        self.stdin.append(stdin)
        name = Path(args[0]).name
        handler = self.handlers.get(f"{name} {args[1]}" if len(args) > 1 else name)
        handler = handler or self.handlers.get(name)
        done = handler(args, {"cwd": cwd, "env": env}) if handler else None
        done = done or subprocess.CompletedProcess(args, 0, "", "")
        if check and done.returncode != 0:
            raise common.StepError(f"{' '.join(args)} exited with {done.returncode}")
        return done

    def programs(self) -> list[str]:
        return [f"{Path(c[0]).name} {c[1]}" if len(c) > 1 else c[0] for c in self.calls]


def ok(stdout: str = "", code: int = 0) -> subprocess.CompletedProcess[str]:
    return subprocess.CompletedProcess([], code, stdout, "")
