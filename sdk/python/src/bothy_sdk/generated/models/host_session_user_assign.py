from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

T = TypeVar("T", bound="HostSessionUserAssign")


@_attrs_define
class HostSessionUserAssign:
    """Assigns a person their own session user on a host (ruling T12, 24 section 2.2). The name must be one the host
    reported (`reported_session_users`); any other name gives `400 validation_failed` with `errors[].code`
    `session_user_not_on_host`, and nothing is stored.

        Attributes:
            user_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            os_user (str): Operating system user name from `host_service.session_users`; at most `host.os_user_max_chars`
                characters; never a path.
    """

    user_id: UUID
    os_user: str

    def to_dict(self) -> dict[str, Any]:
        user_id = str(self.user_id)

        os_user = self.os_user

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "user_id": user_id,
                "os_user": os_user,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        user_id = UUID(d.pop("user_id"))

        os_user = d.pop("os_user")

        host_session_user_assign = cls(
            user_id=user_id,
            os_user=os_user,
        )

        return host_session_user_assign
