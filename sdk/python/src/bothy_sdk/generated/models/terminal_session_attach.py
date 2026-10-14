from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="TerminalSessionAttach")


@_attrs_define
class TerminalSessionAttach:
    """
    Attributes:
        cols (int):
        rows (int):
        client_ephemeral_key_hash (str):
        pin_changed (bool | Unset): Same as in `TerminalSessionOpen`; recorded in the audit entry of `terminal.attach`.
            Default: False.
    """

    cols: int
    rows: int
    client_ephemeral_key_hash: str
    pin_changed: bool | Unset = False

    def to_dict(self) -> dict[str, Any]:
        cols = self.cols

        rows = self.rows

        client_ephemeral_key_hash = self.client_ephemeral_key_hash

        pin_changed = self.pin_changed

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "cols": cols,
                "rows": rows,
                "client_ephemeral_key_hash": client_ephemeral_key_hash,
            }
        )
        if pin_changed is not UNSET:
            field_dict["pin_changed"] = pin_changed

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        cols = d.pop("cols")

        rows = d.pop("rows")

        client_ephemeral_key_hash = d.pop("client_ephemeral_key_hash")

        pin_changed = d.pop("pin_changed", UNSET)

        terminal_session_attach = cls(
            cols=cols,
            rows=rows,
            client_ephemeral_key_hash=client_ephemeral_key_hash,
            pin_changed=pin_changed,
        )

        return terminal_session_attach
