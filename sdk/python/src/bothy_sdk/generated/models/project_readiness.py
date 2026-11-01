from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.project_trash_check_report import ProjectTrashCheckReport


T = TypeVar("T", bound="ProjectReadiness")


@_attrs_define
class ProjectReadiness:
    """
    Attributes:
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        ready (bool):
        reason (str | Unset): Why the forum is not ready, a code of 09 18.12 (for example `provisioning`,
            `workspace_exists`, `clone_failed`); only with `ready` false. Stored as `project_bindings.ready_reason`.
        trash_check (ProjectTrashCheckReport | Unset): The folder check a runner reports in its readiness entry (09
            18.15).
    """

    channel_id: UUID
    ready: bool
    reason: str | Unset = UNSET
    trash_check: ProjectTrashCheckReport | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        channel_id = str(self.channel_id)

        ready = self.ready

        reason = self.reason

        trash_check: dict[str, Any] | Unset = UNSET
        if not isinstance(self.trash_check, Unset):
            trash_check = self.trash_check.to_dict()

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "channel_id": channel_id,
                "ready": ready,
            }
        )
        if reason is not UNSET:
            field_dict["reason"] = reason
        if trash_check is not UNSET:
            field_dict["trash_check"] = trash_check

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.project_trash_check_report import ProjectTrashCheckReport

        d = dict(src_dict)
        channel_id = UUID(d.pop("channel_id"))

        ready = d.pop("ready")

        reason = d.pop("reason", UNSET)

        _trash_check = d.pop("trash_check", UNSET)
        trash_check: ProjectTrashCheckReport | Unset
        if isinstance(_trash_check, Unset):
            trash_check = UNSET
        else:
            trash_check = ProjectTrashCheckReport.from_dict(_trash_check)

        project_readiness = cls(
            channel_id=channel_id,
            ready=ready,
            reason=reason,
            trash_check=trash_check,
        )

        return project_readiness
