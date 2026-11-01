from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..models.project_trash_result_result import ProjectTrashResultResult
from ..types import UNSET, Unset

T = TypeVar("T", bound="ProjectTrashResult")


@_attrs_define
class ProjectTrashResult:
    """
    Attributes:
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        result (ProjectTrashResultResult):
        reason (str | Unset): The refusal code with `result` `refused` (09 18.15).
    """

    channel_id: UUID
    result: ProjectTrashResultResult
    reason: str | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        channel_id = str(self.channel_id)

        result = self.result.value

        reason = self.reason

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "channel_id": channel_id,
                "result": result,
            }
        )
        if reason is not UNSET:
            field_dict["reason"] = reason

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        channel_id = UUID(d.pop("channel_id"))

        result = ProjectTrashResultResult(d.pop("result"))

        reason = d.pop("reason", UNSET)

        project_trash_result = cls(
            channel_id=channel_id,
            result=result,
            reason=reason,
        )

        return project_trash_result
