from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..types import UNSET, Unset

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
            owner_account (bool | Unset): Explicit opt-in for the owner account of the host (ADR 0073, 24 section 2.4): the
                existing account of the instance owner with administrator rights that the host service runs as. Must be `true`
                exactly when the host reports the name with `owner_account = true` (otherwise `400 validation_failed` with
                `errors[].code` `owner_account_opt_in_required` or `owner_account_not_on_host` at `owner_account`), and only an
                active owner may assign it, only to himself (otherwise `403 forbidden` with `reason: owner_account_owner_only`).
                Default: False.
    """

    user_id: UUID
    os_user: str
    owner_account: bool | Unset = False

    def to_dict(self) -> dict[str, Any]:
        user_id = str(self.user_id)

        os_user = self.os_user

        owner_account = self.owner_account

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "user_id": user_id,
                "os_user": os_user,
            }
        )
        if owner_account is not UNSET:
            field_dict["owner_account"] = owner_account

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        user_id = UUID(d.pop("user_id"))

        os_user = d.pop("os_user")

        owner_account = d.pop("owner_account", UNSET)

        host_session_user_assign = cls(
            user_id=user_id,
            os_user=os_user,
            owner_account=owner_account,
        )

        return host_session_user_assign
