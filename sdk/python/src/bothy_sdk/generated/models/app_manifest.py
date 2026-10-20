from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar, cast

from attrs import define as _attrs_define

from ..models.app_manifest_components_item import AppManifestComponentsItem
from ..models.app_manifest_schema_version import AppManifestSchemaVersion
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.app_manifest_delivery import AppManifestDelivery
    from ..models.app_manifest_incoming_webhooks_item import (
        AppManifestIncomingWebhooksItem,
    )
    from ..models.app_manifest_requested_grants_item import (
        AppManifestRequestedGrantsItem,
    )
    from ..models.app_manifest_scopes import AppManifestScopes
    from ..models.command_register import CommandRegister


T = TypeVar("T", bound="AppManifest")


@_attrs_define
class AppManifest:
    """Schema `app_manifest.v1` (11, 3.1). Unknown members are an error. Beyond the schema the server applies the check
    rules of 11, 3.2: scopes from the catalogue, only subscribable events with the scope of their receive action and
    `events:subscribe`, `https` delivery addresses whose host is in `webhook.allowed_hosts`, `requested_grants` at most
    `write` and at most the capability `upload` and never on a host, commands without reserved names and at most
    `command.max_per_principal`, and at most `app.manifest_max_bytes`.

        Attributes:
            schema_version (AppManifestSchemaVersion):
            slug (str): Unique in the instance; becomes the handle of the app principal.
            name (str): Becomes the display name of the app principal.
            description (str):
            publisher (str):
            version (str): SemVer 2.0.0 of the manifest (`apps.manifest_version`).
            min_api_version (str): Lowest API version (major.minor) the app needs; not above the version of this server.
            scopes (AppManifestScopes):
            homepage_url (str | Unset): Absolute `https` address for display only; the server never loads it.
            events (list[str] | Unset): Durable event types (05, 6) the app may subscribe to; each needs the scope of its
                receive action and `events:subscribe`.
            delivery (AppManifestDelivery | Unset): Required with `events`, `commands` or `components`.
            commands (list[CommandRegister] | Unset): Commands after 09, 10, without reserved names and without a target.
            components (list[AppManifestComponentsItem] | Unset): Component types the app uses; only shown for the consent.
            requested_grants (list[AppManifestRequestedGrantsItem] | Unset): Grants the app asks for; the owner creates them
                per target with `createGrant`. Only `read` or `write`, at most the capability `upload`, never on a host.
            incoming_webhooks (list[AppManifestIncomingWebhooksItem] | Unset):
            data_egress (str | Unset): Plain text on which data leave the instance and where to; required with `events` or
                `interactions_url`.
    """

    schema_version: AppManifestSchemaVersion
    slug: str
    name: str
    description: str
    publisher: str
    version: str
    min_api_version: str
    scopes: AppManifestScopes
    homepage_url: str | Unset = UNSET
    events: list[str] | Unset = UNSET
    delivery: AppManifestDelivery | Unset = UNSET
    commands: list[CommandRegister] | Unset = UNSET
    components: list[AppManifestComponentsItem] | Unset = UNSET
    requested_grants: list[AppManifestRequestedGrantsItem] | Unset = UNSET
    incoming_webhooks: list[AppManifestIncomingWebhooksItem] | Unset = UNSET
    data_egress: str | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        schema_version = self.schema_version.value

        slug = self.slug

        name = self.name

        description = self.description

        publisher = self.publisher

        version = self.version

        min_api_version = self.min_api_version

        scopes = self.scopes.to_dict()

        homepage_url = self.homepage_url

        events: list[str] | Unset = UNSET
        if not isinstance(self.events, Unset):
            events = self.events

        delivery: dict[str, Any] | Unset = UNSET
        if not isinstance(self.delivery, Unset):
            delivery = self.delivery.to_dict()

        commands: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.commands, Unset):
            commands = []
            for commands_item_data in self.commands:
                commands_item = commands_item_data.to_dict()
                commands.append(commands_item)

        components: list[str] | Unset = UNSET
        if not isinstance(self.components, Unset):
            components = []
            for components_item_data in self.components:
                components_item = components_item_data.value
                components.append(components_item)

        requested_grants: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.requested_grants, Unset):
            requested_grants = []
            for requested_grants_item_data in self.requested_grants:
                requested_grants_item = requested_grants_item_data.to_dict()
                requested_grants.append(requested_grants_item)

        incoming_webhooks: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.incoming_webhooks, Unset):
            incoming_webhooks = []
            for incoming_webhooks_item_data in self.incoming_webhooks:
                incoming_webhooks_item = incoming_webhooks_item_data.to_dict()
                incoming_webhooks.append(incoming_webhooks_item)

        data_egress = self.data_egress

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "schema_version": schema_version,
                "slug": slug,
                "name": name,
                "description": description,
                "publisher": publisher,
                "version": version,
                "min_api_version": min_api_version,
                "scopes": scopes,
            }
        )
        if homepage_url is not UNSET:
            field_dict["homepage_url"] = homepage_url
        if events is not UNSET:
            field_dict["events"] = events
        if delivery is not UNSET:
            field_dict["delivery"] = delivery
        if commands is not UNSET:
            field_dict["commands"] = commands
        if components is not UNSET:
            field_dict["components"] = components
        if requested_grants is not UNSET:
            field_dict["requested_grants"] = requested_grants
        if incoming_webhooks is not UNSET:
            field_dict["incoming_webhooks"] = incoming_webhooks
        if data_egress is not UNSET:
            field_dict["data_egress"] = data_egress

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.app_manifest_delivery import AppManifestDelivery
        from ..models.app_manifest_incoming_webhooks_item import (
            AppManifestIncomingWebhooksItem,
        )
        from ..models.app_manifest_requested_grants_item import (
            AppManifestRequestedGrantsItem,
        )
        from ..models.app_manifest_scopes import AppManifestScopes
        from ..models.command_register import CommandRegister

        d = dict(src_dict)
        schema_version = AppManifestSchemaVersion(d.pop("schema_version"))

        slug = d.pop("slug")

        name = d.pop("name")

        description = d.pop("description")

        publisher = d.pop("publisher")

        version = d.pop("version")

        min_api_version = d.pop("min_api_version")

        scopes = AppManifestScopes.from_dict(d.pop("scopes"))

        homepage_url = d.pop("homepage_url", UNSET)

        events = cast(list[str], d.pop("events", UNSET))

        _delivery = d.pop("delivery", UNSET)
        delivery: AppManifestDelivery | Unset
        if isinstance(_delivery, Unset):
            delivery = UNSET
        else:
            delivery = AppManifestDelivery.from_dict(_delivery)

        _commands = d.pop("commands", UNSET)
        commands: list[CommandRegister] | Unset = UNSET
        if _commands is not UNSET:
            commands = []
            for commands_item_data in _commands:
                commands_item = CommandRegister.from_dict(commands_item_data)

                commands.append(commands_item)

        _components = d.pop("components", UNSET)
        components: list[AppManifestComponentsItem] | Unset = UNSET
        if _components is not UNSET:
            components = []
            for components_item_data in _components:
                components_item = AppManifestComponentsItem(components_item_data)

                components.append(components_item)

        _requested_grants = d.pop("requested_grants", UNSET)
        requested_grants: list[AppManifestRequestedGrantsItem] | Unset = UNSET
        if _requested_grants is not UNSET:
            requested_grants = []
            for requested_grants_item_data in _requested_grants:
                requested_grants_item = AppManifestRequestedGrantsItem.from_dict(
                    requested_grants_item_data
                )

                requested_grants.append(requested_grants_item)

        _incoming_webhooks = d.pop("incoming_webhooks", UNSET)
        incoming_webhooks: list[AppManifestIncomingWebhooksItem] | Unset = UNSET
        if _incoming_webhooks is not UNSET:
            incoming_webhooks = []
            for incoming_webhooks_item_data in _incoming_webhooks:
                incoming_webhooks_item = AppManifestIncomingWebhooksItem.from_dict(
                    incoming_webhooks_item_data
                )

                incoming_webhooks.append(incoming_webhooks_item)

        data_egress = d.pop("data_egress", UNSET)

        app_manifest = cls(
            schema_version=schema_version,
            slug=slug,
            name=name,
            description=description,
            publisher=publisher,
            version=version,
            min_api_version=min_api_version,
            scopes=scopes,
            homepage_url=homepage_url,
            events=events,
            delivery=delivery,
            commands=commands,
            components=components,
            requested_grants=requested_grants,
            incoming_webhooks=incoming_webhooks,
            data_egress=data_egress,
        )

        return app_manifest
