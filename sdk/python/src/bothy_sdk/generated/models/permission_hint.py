from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.permission_hint_kind import PermissionHintKind
from ..models.permission_hint_rules_item import PermissionHintRulesItem
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.grant import Grant


T = TypeVar("T", bound="PermissionHint")


@_attrs_define
class PermissionHint:
    """One permission hint. `h1_read_horizon` names `agent_id`, `person_id`, `space_id` and `channel_id` (the person can
    learn content of the channel through the agent, 07 section 6). `grant_to_disabled` and `trigger_blocked` carry the
    `grant`; `trigger_blocked` names the blocking `rules` (SR2 tool class, SR3 dev isolation). `agent_personal` names
    `agent_id`, `credential_holder_id` (absent when the agent has none) and the `trigger_set` (I1 asks for exactly the
    holder).

        Attributes:
            kind (PermissionHintKind): Kind of a permission hint (07 section 9.5), in the order of the list.
            agent_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
            person_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
            space_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
            channel_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
            grant (Grant | Unset):
            rules (list[PermissionHintRulesItem] | Unset):
            credential_holder_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
            trigger_set (list[UUID] | Unset): The people with effective `trigger` on a target the agent serves, ascending.
    """

    kind: PermissionHintKind
    agent_id: UUID | Unset = UNSET
    person_id: UUID | Unset = UNSET
    space_id: UUID | Unset = UNSET
    channel_id: UUID | Unset = UNSET
    grant: Grant | Unset = UNSET
    rules: list[PermissionHintRulesItem] | Unset = UNSET
    credential_holder_id: UUID | Unset = UNSET
    trigger_set: list[UUID] | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        kind = self.kind.value

        agent_id: str | Unset = UNSET
        if not isinstance(self.agent_id, Unset):
            agent_id = str(self.agent_id)

        person_id: str | Unset = UNSET
        if not isinstance(self.person_id, Unset):
            person_id = str(self.person_id)

        space_id: str | Unset = UNSET
        if not isinstance(self.space_id, Unset):
            space_id = str(self.space_id)

        channel_id: str | Unset = UNSET
        if not isinstance(self.channel_id, Unset):
            channel_id = str(self.channel_id)

        grant: dict[str, Any] | Unset = UNSET
        if not isinstance(self.grant, Unset):
            grant = self.grant.to_dict()

        rules: list[str] | Unset = UNSET
        if not isinstance(self.rules, Unset):
            rules = []
            for rules_item_data in self.rules:
                rules_item = rules_item_data.value
                rules.append(rules_item)

        credential_holder_id: str | Unset = UNSET
        if not isinstance(self.credential_holder_id, Unset):
            credential_holder_id = str(self.credential_holder_id)

        trigger_set: list[str] | Unset = UNSET
        if not isinstance(self.trigger_set, Unset):
            trigger_set = []
            for trigger_set_item_data in self.trigger_set:
                trigger_set_item = str(trigger_set_item_data)
                trigger_set.append(trigger_set_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "kind": kind,
            }
        )
        if agent_id is not UNSET:
            field_dict["agent_id"] = agent_id
        if person_id is not UNSET:
            field_dict["person_id"] = person_id
        if space_id is not UNSET:
            field_dict["space_id"] = space_id
        if channel_id is not UNSET:
            field_dict["channel_id"] = channel_id
        if grant is not UNSET:
            field_dict["grant"] = grant
        if rules is not UNSET:
            field_dict["rules"] = rules
        if credential_holder_id is not UNSET:
            field_dict["credential_holder_id"] = credential_holder_id
        if trigger_set is not UNSET:
            field_dict["trigger_set"] = trigger_set

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.grant import Grant

        d = dict(src_dict)
        kind = PermissionHintKind(d.pop("kind"))

        _agent_id = d.pop("agent_id", UNSET)
        agent_id: UUID | Unset
        if isinstance(_agent_id, Unset):
            agent_id = UNSET
        else:
            agent_id = UUID(_agent_id)

        _person_id = d.pop("person_id", UNSET)
        person_id: UUID | Unset
        if isinstance(_person_id, Unset):
            person_id = UNSET
        else:
            person_id = UUID(_person_id)

        _space_id = d.pop("space_id", UNSET)
        space_id: UUID | Unset
        if isinstance(_space_id, Unset):
            space_id = UNSET
        else:
            space_id = UUID(_space_id)

        _channel_id = d.pop("channel_id", UNSET)
        channel_id: UUID | Unset
        if isinstance(_channel_id, Unset):
            channel_id = UNSET
        else:
            channel_id = UUID(_channel_id)

        _grant = d.pop("grant", UNSET)
        grant: Grant | Unset
        if isinstance(_grant, Unset):
            grant = UNSET
        else:
            grant = Grant.from_dict(_grant)

        _rules = d.pop("rules", UNSET)
        rules: list[PermissionHintRulesItem] | Unset = UNSET
        if _rules is not UNSET:
            rules = []
            for rules_item_data in _rules:
                rules_item = PermissionHintRulesItem(rules_item_data)

                rules.append(rules_item)

        _credential_holder_id = d.pop("credential_holder_id", UNSET)
        credential_holder_id: UUID | Unset
        if isinstance(_credential_holder_id, Unset):
            credential_holder_id = UNSET
        else:
            credential_holder_id = UUID(_credential_holder_id)

        _trigger_set = d.pop("trigger_set", UNSET)
        trigger_set: list[UUID] | Unset = UNSET
        if _trigger_set is not UNSET:
            trigger_set = []
            for trigger_set_item_data in _trigger_set:
                trigger_set_item = UUID(trigger_set_item_data)

                trigger_set.append(trigger_set_item)

        permission_hint = cls(
            kind=kind,
            agent_id=agent_id,
            person_id=person_id,
            space_id=space_id,
            channel_id=channel_id,
            grant=grant,
            rules=rules,
            credential_holder_id=credential_holder_id,
            trigger_set=trigger_set,
        )

        permission_hint.additional_properties = d
        return permission_hint

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
