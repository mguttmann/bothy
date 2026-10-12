from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define

if TYPE_CHECKING:
    from ..models.json_patch_operation import JsonPatchOperation


T = TypeVar("T", bound="RunPatch")


@_attrs_define
class RunPatch:
    """
    Attributes:
        base_version (int):
        ops (list[JsonPatchOperation]):
    """

    base_version: int
    ops: list[JsonPatchOperation]

    def to_dict(self) -> dict[str, Any]:
        base_version = self.base_version

        ops = []
        for ops_item_data in self.ops:
            ops_item = ops_item_data.to_dict()
            ops.append(ops_item)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "base_version": base_version,
                "ops": ops,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.json_patch_operation import JsonPatchOperation

        d = dict(src_dict)
        base_version = d.pop("base_version")

        ops = []
        _ops = d.pop("ops")
        for ops_item_data in _ops:
            ops_item = JsonPatchOperation.from_dict(ops_item_data)

            ops.append(ops_item)

        run_patch = cls(
            base_version=base_version,
            ops=ops,
        )

        return run_patch
