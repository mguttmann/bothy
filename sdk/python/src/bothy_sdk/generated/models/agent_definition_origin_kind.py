from enum import StrEnum


class AgentDefinitionOriginKind(StrEnum):
    INTERVIEW = "interview"
    MANUAL = "manual"
    TEMPLATE = "template"

    def __str__(self) -> str:
        return str(self.value)
