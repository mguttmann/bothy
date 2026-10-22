from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="TerminalSessionOpen")


@_attrs_define
class TerminalSessionOpen:
    """
    Attributes:
        profile (str):
        cols (int):
        rows (int):
        client_ephemeral_key_hash (str): SHA-256 of the ephemeral X25519 public key the device sends in Noise message 1,
            base64url without padding; becomes the claim `cek`.
        workdir_index (int | Unset): Required exactly with a `claude` profile: index into `workdirs` of the caller's own
            entry in `reported_session_users` (ruling T13). A `claude` profile without a valid index, or an index with
            another profile, gives 403 `terminal_denied` with `reason: profile_unknown`; other profiles omit it. The token
            carries it as `wdi` (24, 5.3, 11.1).
        pin_changed (bool | Unset): `true` when the person confirmed a host key that differs from the key this device
            pinned earlier (docs/spec/24-terminal.md section 5.5, docs/spec/15-client-architecture.md section 23.3);
            recorded in the audit entry of `terminal.open`. Default: False.
    """

    profile: str
    cols: int
    rows: int
    client_ephemeral_key_hash: str
    workdir_index: int | Unset = UNSET
    pin_changed: bool | Unset = False

    def to_dict(self) -> dict[str, Any]:
        profile = self.profile

        cols = self.cols

        rows = self.rows

        client_ephemeral_key_hash = self.client_ephemeral_key_hash

        workdir_index = self.workdir_index

        pin_changed = self.pin_changed

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "profile": profile,
                "cols": cols,
                "rows": rows,
                "client_ephemeral_key_hash": client_ephemeral_key_hash,
            }
        )
        if workdir_index is not UNSET:
            field_dict["workdir_index"] = workdir_index
        if pin_changed is not UNSET:
            field_dict["pin_changed"] = pin_changed

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        profile = d.pop("profile")

        cols = d.pop("cols")

        rows = d.pop("rows")

        client_ephemeral_key_hash = d.pop("client_ephemeral_key_hash")

        workdir_index = d.pop("workdir_index", UNSET)

        pin_changed = d.pop("pin_changed", UNSET)

        terminal_session_open = cls(
            profile=profile,
            cols=cols,
            rows=rows,
            client_ephemeral_key_hash=client_ephemeral_key_hash,
            workdir_index=workdir_index,
            pin_changed=pin_changed,
        )

        return terminal_session_open
