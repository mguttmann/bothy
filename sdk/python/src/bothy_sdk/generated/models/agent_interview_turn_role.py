from enum import StrEnum


class AgentInterviewTurnRole(StrEnum):
    BUILDER = "builder"
    OWNER = "owner"

    def __str__(self) -> str:
        return str(self.value)
