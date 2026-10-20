from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.alertmanager_alert_status import AlertmanagerAlertStatus
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.json_object import JsonObject


T = TypeVar("T", bound="AlertmanagerAlert")


@_attrs_define
class AlertmanagerAlert:
    """One alert of an Alertmanager webhook payload; `status` and `labels` are required (checked with `errors[].code`
    `required`).

        Attributes:
            status (AlertmanagerAlertStatus | Unset):
            labels (JsonObject | Unset): Free-form JSON object whose schema is named in the field description.
            annotations (JsonObject | Unset): Free-form JSON object whose schema is named in the field description.
            starts_at (datetime.datetime | Unset):
            ends_at (datetime.datetime | Unset):
            generator_url (str | Unset): Address of the rule's source; written as text, never fetched.
            fingerprint (str | Unset):
    """

    status: AlertmanagerAlertStatus | Unset = UNSET
    labels: JsonObject | Unset = UNSET
    annotations: JsonObject | Unset = UNSET
    starts_at: datetime.datetime | Unset = UNSET
    ends_at: datetime.datetime | Unset = UNSET
    generator_url: str | Unset = UNSET
    fingerprint: str | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        status: str | Unset = UNSET
        if not isinstance(self.status, Unset):
            status = self.status.value

        labels: dict[str, Any] | Unset = UNSET
        if not isinstance(self.labels, Unset):
            labels = self.labels.to_dict()

        annotations: dict[str, Any] | Unset = UNSET
        if not isinstance(self.annotations, Unset):
            annotations = self.annotations.to_dict()

        starts_at: str | Unset = UNSET
        if not isinstance(self.starts_at, Unset):
            starts_at = self.starts_at.isoformat()

        ends_at: str | Unset = UNSET
        if not isinstance(self.ends_at, Unset):
            ends_at = self.ends_at.isoformat()

        generator_url = self.generator_url

        fingerprint = self.fingerprint

        field_dict: dict[str, Any] = {}

        field_dict.update({})
        if status is not UNSET:
            field_dict["status"] = status
        if labels is not UNSET:
            field_dict["labels"] = labels
        if annotations is not UNSET:
            field_dict["annotations"] = annotations
        if starts_at is not UNSET:
            field_dict["startsAt"] = starts_at
        if ends_at is not UNSET:
            field_dict["endsAt"] = ends_at
        if generator_url is not UNSET:
            field_dict["generatorURL"] = generator_url
        if fingerprint is not UNSET:
            field_dict["fingerprint"] = fingerprint

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.json_object import JsonObject

        d = dict(src_dict)
        _status = d.pop("status", UNSET)
        status: AlertmanagerAlertStatus | Unset
        if isinstance(_status, Unset):
            status = UNSET
        else:
            status = AlertmanagerAlertStatus(_status)

        _labels = d.pop("labels", UNSET)
        labels: JsonObject | Unset
        if isinstance(_labels, Unset):
            labels = UNSET
        else:
            labels = JsonObject.from_dict(_labels)

        _annotations = d.pop("annotations", UNSET)
        annotations: JsonObject | Unset
        if isinstance(_annotations, Unset):
            annotations = UNSET
        else:
            annotations = JsonObject.from_dict(_annotations)

        _starts_at = d.pop("startsAt", UNSET)
        starts_at: datetime.datetime | Unset
        if isinstance(_starts_at, Unset):
            starts_at = UNSET
        else:
            starts_at = datetime.datetime.fromisoformat(_starts_at)

        _ends_at = d.pop("endsAt", UNSET)
        ends_at: datetime.datetime | Unset
        if isinstance(_ends_at, Unset):
            ends_at = UNSET
        else:
            ends_at = datetime.datetime.fromisoformat(_ends_at)

        generator_url = d.pop("generatorURL", UNSET)

        fingerprint = d.pop("fingerprint", UNSET)

        alertmanager_alert = cls(
            status=status,
            labels=labels,
            annotations=annotations,
            starts_at=starts_at,
            ends_at=ends_at,
            generator_url=generator_url,
            fingerprint=fingerprint,
        )

        return alertmanager_alert
