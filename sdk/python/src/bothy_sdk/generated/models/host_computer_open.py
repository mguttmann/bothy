from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

if TYPE_CHECKING:
    from ..models.host_computer_settings import HostComputerSettings


T = TypeVar("T", bound="HostComputerOpen")


@_attrs_define
class HostComputerOpen:
    """The server asks the host service to open the computer data connection of a session (27 5.1 step 7); `user_id` is the
    triggering owner (`sub` of the attach token).

        Attributes:
            session_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            run_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            agent_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            agent_name (str): Name of the agent that opens the session.
            user_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            data_path (str): Path of the computer data connection of the host service.
            expires_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
            settings (HostComputerSettings): The server values of a computer session (27 18), sent with `computer.open`.
    """

    session_id: UUID
    run_id: UUID
    agent_id: UUID
    agent_name: str
    user_id: UUID
    data_path: str
    expires_at: datetime.datetime
    settings: HostComputerSettings
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        session_id = str(self.session_id)

        run_id = str(self.run_id)

        agent_id = str(self.agent_id)

        agent_name = self.agent_name

        user_id = str(self.user_id)

        data_path = self.data_path

        expires_at = self.expires_at.isoformat()

        settings = self.settings.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "session_id": session_id,
                "run_id": run_id,
                "agent_id": agent_id,
                "agent_name": agent_name,
                "user_id": user_id,
                "data_path": data_path,
                "expires_at": expires_at,
                "settings": settings,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.host_computer_settings import HostComputerSettings

        d = dict(src_dict)
        session_id = UUID(d.pop("session_id"))

        run_id = UUID(d.pop("run_id"))

        agent_id = UUID(d.pop("agent_id"))

        agent_name = d.pop("agent_name")

        user_id = UUID(d.pop("user_id"))

        data_path = d.pop("data_path")

        expires_at = datetime.datetime.fromisoformat(d.pop("expires_at"))

        settings = HostComputerSettings.from_dict(d.pop("settings"))

        host_computer_open = cls(
            session_id=session_id,
            run_id=run_id,
            agent_id=agent_id,
            agent_name=agent_name,
            user_id=user_id,
            data_path=data_path,
            expires_at=expires_at,
            settings=settings,
        )

        host_computer_open.additional_properties = d
        return host_computer_open

    @property
    def additional_keys(self) -> list[str]:
        return list(self.additional_properties.keys())

    def __getitem__(self, key: str) -> Any:
        return self.additional_properties[key]

    def __setitem__(self, key: str, value: Any) -> None:
        self.additional_properties[key] = value

    def __delitem__(self, key: str) -> None:
        del self.additional_properties[key]

    def __contains__(self, key: str) -> bool:
        return key in self.additional_properties
