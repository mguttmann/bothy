from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

T = TypeVar("T", bound="AgentDefinitionSkillFile")


@_attrs_define
class AgentDefinitionSkillFile:
    """One extra file of a skill.

    Attributes:
        name (str): A flat name matching `^[a-z0-9][a-z0-9._-]{0,63}\\.md$`, never `SKILL.md` in any case, unique within
            the skill.
        content (str): Markdown, at most `agents.definition.skill_body_max_bytes` bytes; its first line is never `---`.
    """

    name: str
    content: str

    def to_dict(self) -> dict[str, Any]:
        name = self.name

        content = self.content

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "name": name,
                "content": content,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        name = d.pop("name")

        content = d.pop("content")

        agent_definition_skill_file = cls(
            name=name,
            content=content,
        )

        return agent_definition_skill_file
