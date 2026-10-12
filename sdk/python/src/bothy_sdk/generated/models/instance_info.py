from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

if TYPE_CHECKING:
    from ..models.branding import Branding
    from ..models.evaluated_flag import EvaluatedFlag
    from ..models.instance_limits import InstanceLimits


T = TypeVar("T", bound="InstanceInfo")


@_attrs_define
class InstanceInfo:
    """Instance view for an authenticated principal.

    Attributes:
        instance_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        branding (Branding):
        locale_default (str):
        spec_version (str):
        features (list[EvaluatedFlag]): Feature flags evaluated for the caller.
        limits (InstanceLimits): Effective values of limits that clients enforce before sending.
    """

    instance_id: UUID
    branding: Branding
    locale_default: str
    spec_version: str
    features: list[EvaluatedFlag]
    limits: InstanceLimits
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        instance_id = str(self.instance_id)

        branding = self.branding.to_dict()

        locale_default = self.locale_default

        spec_version = self.spec_version

        features = []
        for features_item_data in self.features:
            features_item = features_item_data.to_dict()
            features.append(features_item)

        limits = self.limits.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "instance_id": instance_id,
                "branding": branding,
                "locale_default": locale_default,
                "spec_version": spec_version,
                "features": features,
                "limits": limits,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.branding import Branding
        from ..models.evaluated_flag import EvaluatedFlag
        from ..models.instance_limits import InstanceLimits

        d = dict(src_dict)
        instance_id = UUID(d.pop("instance_id"))

        branding = Branding.from_dict(d.pop("branding"))

        locale_default = d.pop("locale_default")

        spec_version = d.pop("spec_version")

        features = []
        _features = d.pop("features")
        for features_item_data in _features:
            features_item = EvaluatedFlag.from_dict(features_item_data)

            features.append(features_item)

        limits = InstanceLimits.from_dict(d.pop("limits"))

        instance_info = cls(
            instance_id=instance_id,
            branding=branding,
            locale_default=locale_default,
            spec_version=spec_version,
            features=features,
            limits=limits,
        )

        instance_info.additional_properties = d
        return instance_info

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
