from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar, cast
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.auth_provider_info import AuthProviderInfo
    from ..models.client_version_requirement import ClientVersionRequirement
    from ..models.gateway_info import GatewayInfo
    from ..models.public_branding import PublicBranding


T = TypeVar("T", bound="Discovery")


@_attrs_define
class Discovery:
    """Public instance metadata for login screens, SDKs and adapters.

    Attributes:
        instance_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        setup_required (bool): True until the first owner exists (08, 4); every other operation answers `503
            setup_required` until then.
        api_versions (list[str]):
        spec_version (str): SemVer of openapi.yaml served by this instance.
        gateway (GatewayInfo):
        branding (PublicBranding):
        auth_providers (list[AuthProviderInfo]):
        jwks_uri (str):
        webhook_keys_uri (str):
        features (list[str] | Unset): Feature keys this server implements beyond the base contract of its spec version.
        local_password_enabled (bool | Unset):
        min_client_versions (list[ClientVersionRequirement] | Unset):
    """

    instance_id: UUID
    setup_required: bool
    api_versions: list[str]
    spec_version: str
    gateway: GatewayInfo
    branding: PublicBranding
    auth_providers: list[AuthProviderInfo]
    jwks_uri: str
    webhook_keys_uri: str
    features: list[str] | Unset = UNSET
    local_password_enabled: bool | Unset = UNSET
    min_client_versions: list[ClientVersionRequirement] | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        instance_id = str(self.instance_id)

        setup_required = self.setup_required

        api_versions = self.api_versions

        spec_version = self.spec_version

        gateway = self.gateway.to_dict()

        branding = self.branding.to_dict()

        auth_providers = []
        for auth_providers_item_data in self.auth_providers:
            auth_providers_item = auth_providers_item_data.to_dict()
            auth_providers.append(auth_providers_item)

        jwks_uri = self.jwks_uri

        webhook_keys_uri = self.webhook_keys_uri

        features: list[str] | Unset = UNSET
        if not isinstance(self.features, Unset):
            features = self.features

        local_password_enabled = self.local_password_enabled

        min_client_versions: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.min_client_versions, Unset):
            min_client_versions = []
            for min_client_versions_item_data in self.min_client_versions:
                min_client_versions_item = min_client_versions_item_data.to_dict()
                min_client_versions.append(min_client_versions_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "instance_id": instance_id,
                "setup_required": setup_required,
                "api_versions": api_versions,
                "spec_version": spec_version,
                "gateway": gateway,
                "branding": branding,
                "auth_providers": auth_providers,
                "jwks_uri": jwks_uri,
                "webhook_keys_uri": webhook_keys_uri,
            }
        )
        if features is not UNSET:
            field_dict["features"] = features
        if local_password_enabled is not UNSET:
            field_dict["local_password_enabled"] = local_password_enabled
        if min_client_versions is not UNSET:
            field_dict["min_client_versions"] = min_client_versions

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.auth_provider_info import AuthProviderInfo
        from ..models.client_version_requirement import ClientVersionRequirement
        from ..models.gateway_info import GatewayInfo
        from ..models.public_branding import PublicBranding

        d = dict(src_dict)
        instance_id = UUID(d.pop("instance_id"))

        setup_required = d.pop("setup_required")

        api_versions = cast(list[str], d.pop("api_versions"))

        spec_version = d.pop("spec_version")

        gateway = GatewayInfo.from_dict(d.pop("gateway"))

        branding = PublicBranding.from_dict(d.pop("branding"))

        auth_providers = []
        _auth_providers = d.pop("auth_providers")
        for auth_providers_item_data in _auth_providers:
            auth_providers_item = AuthProviderInfo.from_dict(auth_providers_item_data)

            auth_providers.append(auth_providers_item)

        jwks_uri = d.pop("jwks_uri")

        webhook_keys_uri = d.pop("webhook_keys_uri")

        features = cast(list[str], d.pop("features", UNSET))

        local_password_enabled = d.pop("local_password_enabled", UNSET)

        _min_client_versions = d.pop("min_client_versions", UNSET)
        min_client_versions: list[ClientVersionRequirement] | Unset = UNSET
        if _min_client_versions is not UNSET:
            min_client_versions = []
            for min_client_versions_item_data in _min_client_versions:
                min_client_versions_item = ClientVersionRequirement.from_dict(
                    min_client_versions_item_data
                )

                min_client_versions.append(min_client_versions_item)

        discovery = cls(
            instance_id=instance_id,
            setup_required=setup_required,
            api_versions=api_versions,
            spec_version=spec_version,
            gateway=gateway,
            branding=branding,
            auth_providers=auth_providers,
            jwks_uri=jwks_uri,
            webhook_keys_uri=webhook_keys_uri,
            features=features,
            local_password_enabled=local_password_enabled,
            min_client_versions=min_client_versions,
        )

        discovery.additional_properties = d
        return discovery

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
