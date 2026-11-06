from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar, cast

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.agent_template_source import AgentTemplateSource

if TYPE_CHECKING:
    from ..models.agent_definition import AgentDefinition


T = TypeVar("T", bound="AgentTemplate")


@_attrs_define
class AgentTemplate:
    """One agent template (26 7.1): its catalogue entry and the template as a definition draft. A shipped template comes
    from `templates/agents/<slug>/` of the release, a private one from the folder `agents.templates_dir` of the
    installation.

        Attributes:
            slug (str): The folder name of the template, `^[a-z0-9][a-z0-9-]{0,63}$`; unique across shipped and private
                templates.
            source (AgentTemplateSource): `shipped` for a template of the release, `private` for one of
                `agents.templates_dir`.
            name (str): One line for the template picker.
            description (str): One line on what an agent from this template does.
            version (str): The version of the template, `^[0-9A-Za-z][0-9A-Za-z.+-]{0,63}$`; also
                `definition.origin.template_version`.
            required_secrets (list[str]): The names of the secrets the template's MCP servers name as `env:<NAME>`; the host
                operator provides them to the hosting runner. Names only, never a value.
            definition (AgentDefinition): Schema `agent_definition.v1` of a built-in agent (26 4.2), closed at every level.
                Every string is valid UTF-8 without control or bidi characters, except line feed and carriage return in
                `instructions`, a skill `body` and a file `content`. The limits are configuration values: `instructions` at most
                `agents.definition.instructions_max_bytes` bytes; at most `agents.definition.max_skills` skills and
                `agents.definition.max_mcp_servers` MCP servers. The text rules of 26 4.3 (frontmatter, inline shell, `@`
                imports outside the agent folder) and the rules of docs/decisions.d/A1-02.md apply; a finding is a field error
                of `400 validation_failed`.
    """

    slug: str
    source: AgentTemplateSource
    name: str
    description: str
    version: str
    required_secrets: list[str]
    definition: AgentDefinition
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        slug = self.slug

        source = self.source.value

        name = self.name

        description = self.description

        version = self.version

        required_secrets = self.required_secrets

        definition = self.definition.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "slug": slug,
                "source": source,
                "name": name,
                "description": description,
                "version": version,
                "required_secrets": required_secrets,
                "definition": definition,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.agent_definition import AgentDefinition

        d = dict(src_dict)
        slug = d.pop("slug")

        source = AgentTemplateSource(d.pop("source"))

        name = d.pop("name")

        description = d.pop("description")

        version = d.pop("version")

        required_secrets = cast(list[str], d.pop("required_secrets"))

        definition = AgentDefinition.from_dict(d.pop("definition"))

        agent_template = cls(
            slug=slug,
            source=source,
            name=name,
            description=description,
            version=version,
            required_secrets=required_secrets,
            definition=definition,
        )

        agent_template.additional_properties = d
        return agent_template

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
