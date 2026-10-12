from enum import StrEnum


class AgentStatus(StrEnum):
    ACTIVE = "active"
    DELETED = "deleted"
    DISABLED = "disabled"

    def __str__(self) -> str:
        return str(self.value)
