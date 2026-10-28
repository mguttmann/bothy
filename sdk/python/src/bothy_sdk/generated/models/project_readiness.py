from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="ProjectReadiness")


@_attrs_define
class ProjectReadiness:
    """
    Attributes:
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        ready (bool):
        reason (str | Unset): Why the forum is not ready, a code of 09 18.12 (for example `provisioning`,
            `workspace_exists`, `clone_failed`); only with `ready` false. Stored as `project_bindings.ready_reason`.
    """

    channel_id: UUID
    ready: bool
    reason: str | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        channel_id = str(self.channel_id)

        ready = self.ready

        reason = self.reason

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "channel_id": channel_id,
                "ready": ready,
            }
        )
        if reason is not UNSET:
            field_dict["reason"] = reason

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        channel_id = UUID(d.pop("channel_id"))

        ready = d.pop("ready")

        reason = d.pop("reason", UNSET)

        project_readiness = cls(
            channel_id=channel_id,
            ready=ready,
            reason=reason,
        )

        return project_readiness
