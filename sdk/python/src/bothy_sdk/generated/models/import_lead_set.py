from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.import_lead_set_lead import ImportLeadSetLead

T = TypeVar("T", bound="ImportLeadSet")


@_attrs_define
class ImportLeadSet:
    """
    Attributes:
        source_channel (str): ID of the channel in the source.
        lead (ImportLeadSetLead): `discord`: the source leads, the channel here is read-only and mirrored; `bothy`: this
            server leads (22, 12.1).
    """

    source_channel: str
    lead: ImportLeadSetLead

    def to_dict(self) -> dict[str, Any]:
        source_channel = self.source_channel

        lead = self.lead.value

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "source_channel": source_channel,
                "lead": lead,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        source_channel = d.pop("source_channel")

        lead = ImportLeadSetLead(d.pop("lead"))

        import_lead_set = cls(
            source_channel=source_channel,
            lead=lead,
        )

        return import_lead_set
