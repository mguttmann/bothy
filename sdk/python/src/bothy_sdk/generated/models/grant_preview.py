from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.grant_preview_text_locale import GrantPreviewTextLocale
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.access_effect import AccessEffect
    from ..models.invariant_violation import InvariantViolation
    from ..models.preview_hint import PreviewHint


T = TypeVar("T", bound="GrantPreview")


@_attrs_define
class GrantPreview:
    """Effect of a grant before it is created. No state changes (07, 9.3).

    Attributes:
        allowed (bool): False if a delegation limit of the caller (`denial_reason`) or an invariant would reject the
            grant.
        effects (list[AccessEffect]):
        invariant_violations (list[InvariantViolation]):
        hints (list[PreviewHint]):
        text (str): Plain text from templates in `text_locale`, lines separated by a line feed, no trailing line feed.
        text_locale (GrantPreviewTextLocale):
        denial_reason (str | Unset): Reason code of 07 section 4.1 when a delegation limit rejects the grant, for
            example `delegation_exceeds_own_level`.
    """

    allowed: bool
    effects: list[AccessEffect]
    invariant_violations: list[InvariantViolation]
    hints: list[PreviewHint]
    text: str
    text_locale: GrantPreviewTextLocale
    denial_reason: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        allowed = self.allowed

        effects = []
        for effects_item_data in self.effects:
            effects_item = effects_item_data.to_dict()
            effects.append(effects_item)

        invariant_violations = []
        for invariant_violations_item_data in self.invariant_violations:
            invariant_violations_item = invariant_violations_item_data.to_dict()
            invariant_violations.append(invariant_violations_item)

        hints = []
        for hints_item_data in self.hints:
            hints_item = hints_item_data.to_dict()
            hints.append(hints_item)

        text = self.text

        text_locale = self.text_locale.value

        denial_reason = self.denial_reason

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "allowed": allowed,
                "effects": effects,
                "invariant_violations": invariant_violations,
                "hints": hints,
                "text": text,
                "text_locale": text_locale,
            }
        )
        if denial_reason is not UNSET:
            field_dict["denial_reason"] = denial_reason

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.access_effect import AccessEffect
        from ..models.invariant_violation import InvariantViolation
        from ..models.preview_hint import PreviewHint

        d = dict(src_dict)
        allowed = d.pop("allowed")

        effects = []
        _effects = d.pop("effects")
        for effects_item_data in _effects:
            effects_item = AccessEffect.from_dict(effects_item_data)

            effects.append(effects_item)

        invariant_violations = []
        _invariant_violations = d.pop("invariant_violations")
        for invariant_violations_item_data in _invariant_violations:
            invariant_violations_item = InvariantViolation.from_dict(
                invariant_violations_item_data
            )

            invariant_violations.append(invariant_violations_item)

        hints = []
        _hints = d.pop("hints")
        for hints_item_data in _hints:
            hints_item = PreviewHint.from_dict(hints_item_data)

            hints.append(hints_item)

        text = d.pop("text")

        text_locale = GrantPreviewTextLocale(d.pop("text_locale"))

        denial_reason = d.pop("denial_reason", UNSET)

        grant_preview = cls(
            allowed=allowed,
            effects=effects,
            invariant_violations=invariant_violations,
            hints=hints,
            text=text,
            text_locale=text_locale,
            denial_reason=denial_reason,
        )

        grant_preview.additional_properties = d
        return grant_preview

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
