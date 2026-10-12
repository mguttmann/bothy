from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="TranscriptSubmit")


@_attrs_define
class TranscriptSubmit:
    """
    Attributes:
        text (str):
        language (str | Unset):
    """

    text: str
    language: str | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        text = self.text

        language = self.language

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "text": text,
            }
        )
        if language is not UNSET:
            field_dict["language"] = language

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        text = d.pop("text")

        language = d.pop("language", UNSET)

        transcript_submit = cls(
            text=text,
            language=language,
        )

        return transcript_submit
