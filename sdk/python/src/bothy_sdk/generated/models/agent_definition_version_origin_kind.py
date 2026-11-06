from enum import StrEnum


class AgentDefinitionVersionOriginKind(StrEnum):
    INTERVIEW = "interview"
    MANUAL = "manual"
    TEMPLATE = "template"

    def __str__(self) -> str:
        return str(self.value)
