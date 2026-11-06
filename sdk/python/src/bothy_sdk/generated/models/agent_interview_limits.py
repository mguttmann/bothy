from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar, cast

from attrs import define as _attrs_define
from attrs import field as _attrs_field

T = TypeVar("T", bound="AgentInterviewLimits")


@_attrs_define
class AgentInterviewLimits:
    """The limits a draft must keep (the values of `agents.definition.*`, `agents.mcp_stdio_runners` and
    `agents.interview.answer_max_chars`).

        Attributes:
            instructions_max_bytes (int):
            max_skills (int):
            skill_description_max_chars (int):
            skill_body_max_bytes (int):
            max_skill_files (int):
            max_mcp_servers (int):
            max_allowed_tools (int):
            max_cost_usd_per_run (float):
            mcp_stdio_runners (list[str]):
            question_max_chars (int): The longest question of an answer.
    """

    instructions_max_bytes: int
    max_skills: int
    skill_description_max_chars: int
    skill_body_max_bytes: int
    max_skill_files: int
    max_mcp_servers: int
    max_allowed_tools: int
    max_cost_usd_per_run: float
    mcp_stdio_runners: list[str]
    question_max_chars: int
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        instructions_max_bytes = self.instructions_max_bytes

        max_skills = self.max_skills

        skill_description_max_chars = self.skill_description_max_chars

        skill_body_max_bytes = self.skill_body_max_bytes

        max_skill_files = self.max_skill_files

        max_mcp_servers = self.max_mcp_servers

        max_allowed_tools = self.max_allowed_tools

        max_cost_usd_per_run = self.max_cost_usd_per_run

        mcp_stdio_runners = self.mcp_stdio_runners

        question_max_chars = self.question_max_chars

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "instructions_max_bytes": instructions_max_bytes,
                "max_skills": max_skills,
                "skill_description_max_chars": skill_description_max_chars,
                "skill_body_max_bytes": skill_body_max_bytes,
                "max_skill_files": max_skill_files,
                "max_mcp_servers": max_mcp_servers,
                "max_allowed_tools": max_allowed_tools,
                "max_cost_usd_per_run": max_cost_usd_per_run,
                "mcp_stdio_runners": mcp_stdio_runners,
                "question_max_chars": question_max_chars,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        instructions_max_bytes = d.pop("instructions_max_bytes")

        max_skills = d.pop("max_skills")

        skill_description_max_chars = d.pop("skill_description_max_chars")

        skill_body_max_bytes = d.pop("skill_body_max_bytes")

        max_skill_files = d.pop("max_skill_files")

        max_mcp_servers = d.pop("max_mcp_servers")

        max_allowed_tools = d.pop("max_allowed_tools")

        max_cost_usd_per_run = d.pop("max_cost_usd_per_run")

        mcp_stdio_runners = cast(list[str], d.pop("mcp_stdio_runners"))

        question_max_chars = d.pop("question_max_chars")

        agent_interview_limits = cls(
            instructions_max_bytes=instructions_max_bytes,
            max_skills=max_skills,
            skill_description_max_chars=skill_description_max_chars,
            skill_body_max_bytes=skill_body_max_bytes,
            max_skill_files=max_skill_files,
            max_mcp_servers=max_mcp_servers,
            max_allowed_tools=max_allowed_tools,
            max_cost_usd_per_run=max_cost_usd_per_run,
            mcp_stdio_runners=mcp_stdio_runners,
            question_max_chars=question_max_chars,
        )

        agent_interview_limits.additional_properties = d
        return agent_interview_limits

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
