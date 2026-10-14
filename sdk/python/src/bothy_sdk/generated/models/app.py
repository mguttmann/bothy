from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

if TYPE_CHECKING:
    from ..models.app_installation import AppInstallation
    from ..models.app_manifest import AppManifest


T = TypeVar("T", bound="App")


@_attrs_define
class App:
    """
    Attributes:
        id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        slug (str):
        display_name (str):
        manifest (AppManifest): Schema `app_manifest.v1` (11, 3.1). Unknown members are an error. Beyond the schema the
            server applies the check rules of 11, 3.2: scopes from the catalogue, only subscribable events with the scope of
            their receive action and `events:subscribe`, `https` delivery addresses whose host is in
            `webhook.allowed_hosts`, `requested_grants` at most `write` and at most the capability `upload` and never on a
            host, commands without reserved names and at most `command.max_per_principal`, and at most
            `app.manifest_max_bytes`.
        manifest_version (str):
        installation (AppInstallation):
    """

    id: UUID
    slug: str
    display_name: str
    manifest: AppManifest
    manifest_version: str
    installation: AppInstallation
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        slug = self.slug

        display_name = self.display_name

        manifest = self.manifest.to_dict()

        manifest_version = self.manifest_version

        installation = self.installation.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "slug": slug,
                "display_name": display_name,
                "manifest": manifest,
                "manifest_version": manifest_version,
                "installation": installation,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.app_installation import AppInstallation
        from ..models.app_manifest import AppManifest

        d = dict(src_dict)
        id = UUID(d.pop("id"))

        slug = d.pop("slug")

        display_name = d.pop("display_name")

        manifest = AppManifest.from_dict(d.pop("manifest"))

        manifest_version = d.pop("manifest_version")

        installation = AppInstallation.from_dict(d.pop("installation"))

        app = cls(
            id=id,
            slug=slug,
            display_name=display_name,
            manifest=manifest,
            manifest_version=manifest_version,
            installation=installation,
        )

        app.additional_properties = d
        return app

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
