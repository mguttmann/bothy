from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.transcript_source import TranscriptSource
from ..models.transcript_status import TranscriptStatus
from ..types import UNSET, Unset

T = TypeVar("T", bound="Transcript")


@_attrs_define
class Transcript:
    """
    Attributes:
        file_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        status (TranscriptStatus):
        text (str | Unset):
        language (str | Unset):
        source (TranscriptSource | Unset):
    """

    file_id: UUID
    status: TranscriptStatus
    text: str | Unset = UNSET
    language: str | Unset = UNSET
    source: TranscriptSource | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        file_id = str(self.file_id)

        status = self.status.value

        text = self.text

        language = self.language

        source: str | Unset = UNSET
        if not isinstance(self.source, Unset):
            source = self.source.value

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "file_id": file_id,
                "status": status,
            }
        )
        if text is not UNSET:
            field_dict["text"] = text
        if language is not UNSET:
            field_dict["language"] = language
        if source is not UNSET:
            field_dict["source"] = source

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        file_id = UUID(d.pop("file_id"))

        status = TranscriptStatus(d.pop("status"))

        text = d.pop("text", UNSET)

        language = d.pop("language", UNSET)

        _source = d.pop("source", UNSET)
        source: TranscriptSource | Unset
        if isinstance(_source, Unset):
            source = UNSET
        else:
            source = TranscriptSource(_source)

        transcript = cls(
            file_id=file_id,
            status=status,
            text=text,
            language=language,
            source=source,
        )

        transcript.additional_properties = d
        return transcript

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
