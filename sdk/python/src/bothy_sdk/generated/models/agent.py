from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.agent_status import AgentStatus
from ..models.billing_mode import BillingMode
from ..models.tool_class import ToolClass
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.agent_runtime_status import AgentRuntimeStatus
    from ..models.json_object import JsonObject


T = TypeVar("T", bound="Agent")


@_attrs_define
class Agent:
    """
    Attributes:
        id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        handle (str):
        display_name (str):
        status (AgentStatus):
        adapter_kind (str):
        description (str):
        billing_mode (BillingMode): Backend policy of an agent. `personal` runs on the subscription of exactly one
            person.
        tool_class (ToolClass): Tool class of an agent runtime, ordered `chat` < `scoped` < `host` (07, 2.11).
        presence (AgentRuntimeStatus): Reported state of the agent runtime (heartbeat), not the configured state.
        operator_config (JsonObject): Free-form JSON object whose schema is named in the field description.
        version (int):
        avatar_file_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        credential_holder_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        billing_mode_changed_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and
            suffix `Z`.
    """

    id: UUID
    handle: str
    display_name: str
    status: AgentStatus
    adapter_kind: str
    description: str
    billing_mode: BillingMode
    tool_class: ToolClass
    presence: AgentRuntimeStatus
    operator_config: JsonObject
    version: int
    avatar_file_id: UUID | Unset = UNSET
    credential_holder_id: UUID | Unset = UNSET
    billing_mode_changed_at: datetime.datetime | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        handle = self.handle

        display_name = self.display_name

        status = self.status.value

        adapter_kind = self.adapter_kind

        description = self.description

        billing_mode = self.billing_mode.value

        tool_class = self.tool_class.value

        presence = self.presence.to_dict()

        operator_config = self.operator_config.to_dict()

        version = self.version

        avatar_file_id: str | Unset = UNSET
        if not isinstance(self.avatar_file_id, Unset):
            avatar_file_id = str(self.avatar_file_id)

        credential_holder_id: str | Unset = UNSET
        if not isinstance(self.credential_holder_id, Unset):
            credential_holder_id = str(self.credential_holder_id)

        billing_mode_changed_at: str | Unset = UNSET
        if not isinstance(self.billing_mode_changed_at, Unset):
            billing_mode_changed_at = self.billing_mode_changed_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "handle": handle,
                "display_name": display_name,
                "status": status,
                "adapter_kind": adapter_kind,
                "description": description,
                "billing_mode": billing_mode,
                "tool_class": tool_class,
                "presence": presence,
                "operator_config": operator_config,
                "version": version,
            }
        )
        if avatar_file_id is not UNSET:
            field_dict["avatar_file_id"] = avatar_file_id
        if credential_holder_id is not UNSET:
            field_dict["credential_holder_id"] = credential_holder_id
        if billing_mode_changed_at is not UNSET:
            field_dict["billing_mode_changed_at"] = billing_mode_changed_at

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.agent_runtime_status import AgentRuntimeStatus
        from ..models.json_object import JsonObject

        d = dict(src_dict)
        id = UUID(d.pop("id"))

        handle = d.pop("handle")

        display_name = d.pop("display_name")

        status = AgentStatus(d.pop("status"))

        adapter_kind = d.pop("adapter_kind")

        description = d.pop("description")

        billing_mode = BillingMode(d.pop("billing_mode"))

        tool_class = ToolClass(d.pop("tool_class"))

        presence = AgentRuntimeStatus.from_dict(d.pop("presence"))

        operator_config = JsonObject.from_dict(d.pop("operator_config"))

        version = d.pop("version")

        _avatar_file_id = d.pop("avatar_file_id", UNSET)
        avatar_file_id: UUID | Unset
        if isinstance(_avatar_file_id, Unset):
            avatar_file_id = UNSET
        else:
            avatar_file_id = UUID(_avatar_file_id)

        _credential_holder_id = d.pop("credential_holder_id", UNSET)
        credential_holder_id: UUID | Unset
        if isinstance(_credential_holder_id, Unset):
            credential_holder_id = UNSET
        else:
            credential_holder_id = UUID(_credential_holder_id)

        _billing_mode_changed_at = d.pop("billing_mode_changed_at", UNSET)
        billing_mode_changed_at: datetime.datetime | Unset
        if isinstance(_billing_mode_changed_at, Unset):
            billing_mode_changed_at = UNSET
        else:
            billing_mode_changed_at = datetime.datetime.fromisoformat(
                _billing_mode_changed_at
            )

        agent = cls(
            id=id,
            handle=handle,
            display_name=display_name,
            status=status,
            adapter_kind=adapter_kind,
            description=description,
            billing_mode=billing_mode,
            tool_class=tool_class,
            presence=presence,
            operator_config=operator_config,
            version=version,
            avatar_file_id=avatar_file_id,
            credential_holder_id=credential_holder_id,
            billing_mode_changed_at=billing_mode_changed_at,
        )

        agent.additional_properties = d
        return agent

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
