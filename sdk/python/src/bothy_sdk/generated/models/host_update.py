from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.host_session_user_assign import HostSessionUserAssign


T = TypeVar("T", bound="HostUpdate")


@_attrs_define
class HostUpdate:
    """Members that are absent stay unchanged. To remove the assignee send `clear_assigned_user` (no JSON null, ADR 0053).

    Attributes:
        reason (str):
        name (str | Unset):
        assigned_user_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        clear_assigned_user (bool | Unset):
        add_session_users (list[HostSessionUserAssign] | Unset): New session user assignments (ruling T12).
        revoke_session_users (list[UUID] | Unset): People whose session user assignment ends; their names stay taken on
            this host.
    """

    reason: str
    name: str | Unset = UNSET
    assigned_user_id: UUID | Unset = UNSET
    clear_assigned_user: bool | Unset = UNSET
    add_session_users: list[HostSessionUserAssign] | Unset = UNSET
    revoke_session_users: list[UUID] | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        reason = self.reason

        name = self.name

        assigned_user_id: str | Unset = UNSET
        if not isinstance(self.assigned_user_id, Unset):
            assigned_user_id = str(self.assigned_user_id)

        clear_assigned_user = self.clear_assigned_user

        add_session_users: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.add_session_users, Unset):
            add_session_users = []
            for add_session_users_item_data in self.add_session_users:
                add_session_users_item = add_session_users_item_data.to_dict()
                add_session_users.append(add_session_users_item)

        revoke_session_users: list[str] | Unset = UNSET
        if not isinstance(self.revoke_session_users, Unset):
            revoke_session_users = []
            for revoke_session_users_item_data in self.revoke_session_users:
                revoke_session_users_item = str(revoke_session_users_item_data)
                revoke_session_users.append(revoke_session_users_item)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "reason": reason,
            }
        )
        if name is not UNSET:
            field_dict["name"] = name
        if assigned_user_id is not UNSET:
            field_dict["assigned_user_id"] = assigned_user_id
        if clear_assigned_user is not UNSET:
            field_dict["clear_assigned_user"] = clear_assigned_user
        if add_session_users is not UNSET:
            field_dict["add_session_users"] = add_session_users
        if revoke_session_users is not UNSET:
            field_dict["revoke_session_users"] = revoke_session_users

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.host_session_user_assign import HostSessionUserAssign

        d = dict(src_dict)
        reason = d.pop("reason")

        name = d.pop("name", UNSET)

        _assigned_user_id = d.pop("assigned_user_id", UNSET)
        assigned_user_id: UUID | Unset
        if isinstance(_assigned_user_id, Unset):
            assigned_user_id = UNSET
        else:
            assigned_user_id = UUID(_assigned_user_id)

        clear_assigned_user = d.pop("clear_assigned_user", UNSET)

        _add_session_users = d.pop("add_session_users", UNSET)
        add_session_users: list[HostSessionUserAssign] | Unset = UNSET
        if _add_session_users is not UNSET:
            add_session_users = []
            for add_session_users_item_data in _add_session_users:
                add_session_users_item = HostSessionUserAssign.from_dict(
                    add_session_users_item_data
                )

                add_session_users.append(add_session_users_item)

        _revoke_session_users = d.pop("revoke_session_users", UNSET)
        revoke_session_users: list[UUID] | Unset = UNSET
        if _revoke_session_users is not UNSET:
            revoke_session_users = []
            for revoke_session_users_item_data in _revoke_session_users:
                revoke_session_users_item = UUID(revoke_session_users_item_data)

                revoke_session_users.append(revoke_session_users_item)

        host_update = cls(
            reason=reason,
            name=name,
            assigned_user_id=assigned_user_id,
            clear_assigned_user=clear_assigned_user,
            add_session_users=add_session_users,
            revoke_session_users=revoke_session_users,
        )

        return host_update
