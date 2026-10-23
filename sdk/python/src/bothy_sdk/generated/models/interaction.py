from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.interaction_state import InteractionState
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.interaction_response import InteractionResponse
    from ..models.json_object import JsonObject


T = TypeVar("T", bound="Interaction")


@_attrs_define
class Interaction:
    """
    Attributes:
        id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        component_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        message_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        principal_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        client_interaction_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        values (JsonObject): Free-form JSON object whose schema is named in the field description.
        state (InteractionState): `received` when stored. For a component of an agent card the delivery sets
            `dispatched` when it hands `interaction.created` to the agent, or `failed` when the re-check denies (09, 11.2).
            For a component of an app card the job `runs.app_dispatch` acts after the commit: `failed` when the app has no
            active installation; for an app whose manifest declares `delivery.interactions_url` it re-checks with the rights
            of that moment (`failed` without a request on a denial), sets `dispatched` when it hands the event to that
            address, then `acknowledged` on a `200` answer whose reply message is written in the name of the app, and
            `failed` when the app does not answer within `app.interaction_timeout` or gives any other answer; a `202` keeps
            `dispatched` until the app calls `respondToInteraction`. For an app that receives its events over the gateway
            the interaction stays `received` until `respondToInteraction`. `respondToInteraction` moves the state from
            `received` or `dispatched` to `acknowledged`, `failed` or `rejected`; every change is a compare-and-swap, so the
            first of the job and the owner wins and the job never overwrites the answer of the owner.
        created_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        custom_id (str | Unset):
        thread_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        run_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        response (InteractionResponse | Unset):
    """

    id: UUID
    component_id: UUID
    message_id: UUID
    principal_id: UUID
    client_interaction_id: UUID
    values: JsonObject
    state: InteractionState
    created_at: datetime.datetime
    custom_id: str | Unset = UNSET
    thread_id: UUID | Unset = UNSET
    run_id: UUID | Unset = UNSET
    response: InteractionResponse | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        component_id = str(self.component_id)

        message_id = str(self.message_id)

        principal_id = str(self.principal_id)

        client_interaction_id = str(self.client_interaction_id)

        values = self.values.to_dict()

        state = self.state.value

        created_at = self.created_at.isoformat()

        custom_id = self.custom_id

        thread_id: str | Unset = UNSET
        if not isinstance(self.thread_id, Unset):
            thread_id = str(self.thread_id)

        run_id: str | Unset = UNSET
        if not isinstance(self.run_id, Unset):
            run_id = str(self.run_id)

        response: dict[str, Any] | Unset = UNSET
        if not isinstance(self.response, Unset):
            response = self.response.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "component_id": component_id,
                "message_id": message_id,
                "principal_id": principal_id,
                "client_interaction_id": client_interaction_id,
                "values": values,
                "state": state,
                "created_at": created_at,
            }
        )
        if custom_id is not UNSET:
            field_dict["custom_id"] = custom_id
        if thread_id is not UNSET:
            field_dict["thread_id"] = thread_id
        if run_id is not UNSET:
            field_dict["run_id"] = run_id
        if response is not UNSET:
            field_dict["response"] = response

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.interaction_response import InteractionResponse
        from ..models.json_object import JsonObject

        d = dict(src_dict)
        id = UUID(d.pop("id"))

        component_id = UUID(d.pop("component_id"))

        message_id = UUID(d.pop("message_id"))

        principal_id = UUID(d.pop("principal_id"))

        client_interaction_id = UUID(d.pop("client_interaction_id"))

        values = JsonObject.from_dict(d.pop("values"))

        state = InteractionState(d.pop("state"))

        created_at = datetime.datetime.fromisoformat(d.pop("created_at"))

        custom_id = d.pop("custom_id", UNSET)

        _thread_id = d.pop("thread_id", UNSET)
        thread_id: UUID | Unset
        if isinstance(_thread_id, Unset):
            thread_id = UNSET
        else:
            thread_id = UUID(_thread_id)

        _run_id = d.pop("run_id", UNSET)
        run_id: UUID | Unset
        if isinstance(_run_id, Unset):
            run_id = UNSET
        else:
            run_id = UUID(_run_id)

        _response = d.pop("response", UNSET)
        response: InteractionResponse | Unset
        if isinstance(_response, Unset):
            response = UNSET
        else:
            response = InteractionResponse.from_dict(_response)

        interaction = cls(
            id=id,
            component_id=component_id,
            message_id=message_id,
            principal_id=principal_id,
            client_interaction_id=client_interaction_id,
            values=values,
            state=state,
            created_at=created_at,
            custom_id=custom_id,
            thread_id=thread_id,
            run_id=run_id,
            response=response,
        )

        interaction.additional_properties = d
        return interaction

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
