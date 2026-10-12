from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar, cast

from attrs import define as _attrs_define

from ..models.app_scope import AppScope
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.command_register import CommandRegister
    from ..models.webhook_subscription_create import WebhookSubscriptionCreate


T = TypeVar("T", bound="AppManifest")


@_attrs_define
class AppManifest:
    """Schema `app_manifest.v1`.

    Attributes:
        schema_version (int):
        slug (str):
        name (str):
        version (str): SemVer of the manifest.
        scopes (list[AppScope]):
        description (str | Unset):
        publisher (str | Unset):
        homepage_url (str | Unset):
        event_subscriptions (list[str] | Unset):
        commands (list[CommandRegister] | Unset):
        webhooks (list[WebhookSubscriptionCreate] | Unset):
    """

    schema_version: int
    slug: str
    name: str
    version: str
    scopes: list[AppScope]
    description: str | Unset = UNSET
    publisher: str | Unset = UNSET
    homepage_url: str | Unset = UNSET
    event_subscriptions: list[str] | Unset = UNSET
    commands: list[CommandRegister] | Unset = UNSET
    webhooks: list[WebhookSubscriptionCreate] | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        schema_version = self.schema_version

        slug = self.slug

        name = self.name

        version = self.version

        scopes = []
        for scopes_item_data in self.scopes:
            scopes_item = scopes_item_data.value
            scopes.append(scopes_item)

        description = self.description

        publisher = self.publisher

        homepage_url = self.homepage_url

        event_subscriptions: list[str] | Unset = UNSET
        if not isinstance(self.event_subscriptions, Unset):
            event_subscriptions = self.event_subscriptions

        commands: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.commands, Unset):
            commands = []
            for commands_item_data in self.commands:
                commands_item = commands_item_data.to_dict()
                commands.append(commands_item)

        webhooks: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.webhooks, Unset):
            webhooks = []
            for webhooks_item_data in self.webhooks:
                webhooks_item = webhooks_item_data.to_dict()
                webhooks.append(webhooks_item)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "schema_version": schema_version,
                "slug": slug,
                "name": name,
                "version": version,
                "scopes": scopes,
            }
        )
        if description is not UNSET:
            field_dict["description"] = description
        if publisher is not UNSET:
            field_dict["publisher"] = publisher
        if homepage_url is not UNSET:
            field_dict["homepage_url"] = homepage_url
        if event_subscriptions is not UNSET:
            field_dict["event_subscriptions"] = event_subscriptions
        if commands is not UNSET:
            field_dict["commands"] = commands
        if webhooks is not UNSET:
            field_dict["webhooks"] = webhooks

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.command_register import CommandRegister
        from ..models.webhook_subscription_create import WebhookSubscriptionCreate

        d = dict(src_dict)
        schema_version = d.pop("schema_version")

        slug = d.pop("slug")

        name = d.pop("name")

        version = d.pop("version")

        scopes = []
        _scopes = d.pop("scopes")
        for scopes_item_data in _scopes:
            scopes_item = AppScope(scopes_item_data)

            scopes.append(scopes_item)

        description = d.pop("description", UNSET)

        publisher = d.pop("publisher", UNSET)

        homepage_url = d.pop("homepage_url", UNSET)

        event_subscriptions = cast(list[str], d.pop("event_subscriptions", UNSET))

        _commands = d.pop("commands", UNSET)
        commands: list[CommandRegister] | Unset = UNSET
        if _commands is not UNSET:
            commands = []
            for commands_item_data in _commands:
                commands_item = CommandRegister.from_dict(commands_item_data)

                commands.append(commands_item)

        _webhooks = d.pop("webhooks", UNSET)
        webhooks: list[WebhookSubscriptionCreate] | Unset = UNSET
        if _webhooks is not UNSET:
            webhooks = []
            for webhooks_item_data in _webhooks:
                webhooks_item = WebhookSubscriptionCreate.from_dict(webhooks_item_data)

                webhooks.append(webhooks_item)

        app_manifest = cls(
            schema_version=schema_version,
            slug=slug,
            name=name,
            version=version,
            scopes=scopes,
            description=description,
            publisher=publisher,
            homepage_url=homepage_url,
            event_subscriptions=event_subscriptions,
            commands=commands,
            webhooks=webhooks,
        )

        return app_manifest
