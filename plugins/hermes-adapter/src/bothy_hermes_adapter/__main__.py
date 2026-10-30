# SPDX-License-Identifier: Apache-2.0
"""Command line of the installer: ``python -m bothy_hermes_adapter install|switch-mode|check``.

Run it with the Python of the Hermes profile (the one that runs the gateway), so Hermes's own
reader and writer see the profile (09 17.2, 17.5, 17.7). It prints a summary without values and
names the restart the operator must do; it never restarts a process (Design note 17).

Examples:
    >>> build_parser().parse_args(["check", "--profile-home", "/srv/hermes"]).command
    'check'
"""

from __future__ import annotations

import argparse
import importlib.util
import shutil
import sys
from collections.abc import Sequence
from pathlib import Path
from typing import Any
from uuid import UUID

from bothy_hermes_adapter.installer import (
    InstallError,
    InstallRequest,
    check,
    install,
    loadable,
    locate_packages,
    switch_mode,
)

__all__ = ["build_parser", "copy_package", "locate", "main", "place_file"]


def build_parser() -> argparse.ArgumentParser:
    """The argument parser of the three commands.

    Returns:
        The parser.
    """
    parser = argparse.ArgumentParser(prog="python -m bothy_hermes_adapter")
    sub = parser.add_subparsers(dest="command", required=True)
    inst = sub.add_parser("install", help="install the adapter into a Hermes profile")
    inst.add_argument("--profile-home", type=Path, required=True)
    inst.add_argument("--server-url", required=True)
    inst.add_argument("--instance-id", type=UUID, required=True)
    inst.add_argument("--token-file", type=Path, required=True)
    inst.add_argument("--dpop-key-file", type=Path)
    inst.add_argument("--create-dpop-key", action="store_true")
    inst.add_argument("--busy-input-mode", required=True)
    inst.add_argument("--platform-key", default="bothy")
    inst.add_argument("--consumer", default="default")
    inst.add_argument("--report-channel", type=UUID)
    inst.add_argument("--report-thread", type=UUID)
    switch = sub.add_parser("switch-mode", help="switch the billing mode of the profile")
    switch.add_argument("mode", choices=("organization", "personal"))
    switch.add_argument("--profile-home", type=Path, required=True)
    switch.add_argument("--credential-holder", type=UUID)
    chk = sub.add_parser("check", help="show the profile's state and the predicted mode")
    chk.add_argument("--profile-home", type=Path, required=True)
    return parser


class _Bridge:
    """`bothy_hermes_adapter.installer.ProfileBridge` over ``hermes_config``."""

    def __init__(self, home: Path) -> None:
        from bothy_hermes_adapter import hermes_config  # noqa: PLC0415 - Hermes only here

        self.home = home
        self._hc = hermes_config

    def read_config(self) -> dict[str, Any]:
        with self._hc.profile_home(self.home):
            return self._hc.read_config()

    def write_config(self, config: Any) -> None:
        with self._hc.profile_home(self.home):
            self._hc.write_config(config)

    def env_defines(self, name: str) -> bool:
        return self._hc.env_defines(self.home, name)

    def set_env(self, name: str, value: str) -> None:
        with self._hc.profile_home(self.home):
            self._hc.set_env(name, value)

    def remove_env(self, name: str) -> None:
        with self._hc.profile_home(self.home):
            self._hc.remove_env(name)

    def predict(self) -> Any:
        return self._hc.predict(self.home)

    def account_home(self) -> Path:
        return Path.home()

    def api_key_variable(self) -> str:
        return str(self._hc.API_KEY_VARIABLE)

    def oauth_variables(self) -> tuple[str, ...]:
        return tuple(self._hc.OAUTH_VARIABLES)


def place_file(path: Path, text: str) -> None:
    """Write one file of the plugin entry, creating its directory."""
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text, encoding="utf-8")


def copy_package(source: Path, target: Path) -> None:
    """Replace ``target`` with a copy of the package directory ``source`` (no byte code)."""
    if target.exists():
        shutil.rmtree(target)
    target.parent.mkdir(parents=True, exist_ok=True)
    shutil.copytree(source, target, ignore=shutil.ignore_patterns("__pycache__", "*.pyc"))


def locate(name: str) -> Path | None:
    """The directory of an import package in this Python, or ``None``."""
    spec = importlib.util.find_spec(name)
    if spec is None or not spec.submodule_search_locations:
        return None
    return Path(next(iter(spec.submodule_search_locations)))


def main(argv: Sequence[str] | None = None) -> int:
    """Run one command.

    Args:
        argv: The arguments (default ``sys.argv[1:]``).

    Returns:
        ``0`` on success, ``1`` when ``check`` finds an adapter the gateway cannot load,
        ``2`` on a refusal.
    """
    args = build_parser().parse_args(argv)
    home = args.profile_home.resolve()
    bridge = _Bridge(home)
    try:
        if args.command == "install":
            if args.create_dpop_key:
                if args.dpop_key_file is None:
                    raise InstallError("--create-dpop-key needs --dpop-key-file")
                if not args.dpop_key_file.exists():
                    from bothy_sdk.auth import FileDpopKey  # noqa: PLC0415

                    FileDpopKey.generate(args.dpop_key_file)
            request = InstallRequest(
                server_url=args.server_url,
                instance_id=args.instance_id,
                token_file=args.token_file,
                busy_input_mode=args.busy_input_mode,
                platform_key=args.platform_key,
                consumer=args.consumer,
                dpop_key_file=args.dpop_key_file,
                report_channel=args.report_channel,
                report_thread=args.report_thread,
            )
            packages = locate_packages(locate, Path(sys.prefix))
            lines = install(bridge, request, place_file, packages, copy_package)
        elif args.command == "switch-mode":
            lines = switch_mode(bridge, args.mode, args.credential_holder)
        else:
            lines = check(bridge)
    except (InstallError, ValueError) as exc:
        sys.stderr.write(f"refused: {exc}\n")
        return 2
    sys.stdout.write("\n".join(lines) + "\n")
    return 0 if loadable(lines) else 1


if __name__ == "__main__":
    raise SystemExit(main())
