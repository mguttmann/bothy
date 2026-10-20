from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.json_patch_operation import JsonPatchOperation


T = TypeVar("T", bound="GatewayRunUpdate")


@_attrs_define
class GatewayRunUpdate:
    """
    Attributes:
        run_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        base_version (int):
        ops (list[JsonPatchOperation]):
        ref (str | Unset): Client reference echoed in `error` frames for this op.
    """

    run_id: UUID
    base_version: int
    ops: list[JsonPatchOperation]
    ref: str | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        run_id = str(self.run_id)

        base_version = self.base_version

        ops = []
        for ops_item_data in self.ops:
            ops_item = ops_item_data.to_dict()
            ops.append(ops_item)

        ref = self.ref

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "run_id": run_id,
                "base_version": base_version,
                "ops": ops,
            }
        )
        if ref is not UNSET:
            field_dict["ref"] = ref

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.json_patch_operation import JsonPatchOperation

        d = dict(src_dict)
        run_id = UUID(d.pop("run_id"))

        base_version = d.pop("base_version")

        ops = []
        _ops = d.pop("ops")
        for ops_item_data in _ops:
            ops_item = JsonPatchOperation.from_dict(ops_item_data)

            ops.append(ops_item)

        ref = d.pop("ref", UNSET)

        gateway_run_update = cls(
            run_id=run_id,
            base_version=base_version,
            ops=ops,
            ref=ref,
        )

        return gateway_run_update
