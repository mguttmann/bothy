from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.agent_definition_skill_file import AgentDefinitionSkillFile


T = TypeVar("T", bound="AgentDefinitionSkill")


@_attrs_define
class AgentDefinitionSkill:
    """One skill. The runner writes the frontmatter of `SKILL.md` from `name` and `description` only (26 5.2).

    Attributes:
        name (str): `^[a-z0-9][a-z0-9-]{0,63}$`, unique among the skills (`invalid_name`, `duplicate_name`).
        description (str): One line, not blank, at most `agents.definition.skill_description_max_chars` characters.
        body (str): Markdown, at most `agents.definition.skill_body_max_bytes` bytes; its first line is never `---`
            (`skill_frontmatter`).
        files (list[AgentDefinitionSkillFile] | Unset): At most `agents.definition.max_skill_files` extra files, written
            flat next to `SKILL.md`.
    """

    name: str
    description: str
    body: str
    files: list[AgentDefinitionSkillFile] | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        name = self.name

        description = self.description

        body = self.body

        files: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.files, Unset):
            files = []
            for files_item_data in self.files:
                files_item = files_item_data.to_dict()
                files.append(files_item)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "name": name,
                "description": description,
                "body": body,
            }
        )
        if files is not UNSET:
            field_dict["files"] = files

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.agent_definition_skill_file import AgentDefinitionSkillFile

        d = dict(src_dict)
        name = d.pop("name")

        description = d.pop("description")

        body = d.pop("body")

        _files = d.pop("files", UNSET)
        files: list[AgentDefinitionSkillFile] | Unset = UNSET
        if _files is not UNSET:
            files = []
            for files_item_data in _files:
                files_item = AgentDefinitionSkillFile.from_dict(files_item_data)

                files.append(files_item)

        agent_definition_skill = cls(
            name=name,
            description=description,
            body=body,
            files=files,
        )

        return agent_definition_skill
