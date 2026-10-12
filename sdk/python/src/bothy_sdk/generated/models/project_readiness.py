from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

T = TypeVar("T", bound="ProjectReadiness")


@_attrs_define
class ProjectReadiness:
    """
    Attributes:
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        ready (bool):
    """

    channel_id: UUID
    ready: bool

    def to_dict(self) -> dict[str, Any]:
        channel_id = str(self.channel_id)

        ready = self.ready

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "channel_id": channel_id,
                "ready": ready,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        channel_id = UUID(d.pop("channel_id"))

        ready = d.pop("ready")

        project_readiness = cls(
            channel_id=channel_id,
            ready=ready,
        )

        return project_readiness
