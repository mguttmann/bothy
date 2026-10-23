from enum import StrEnum


class AgentInvocationPayloadSessionMode(StrEnum):
    BIND = "bind"
    NEW = "new"
    RESUME = "resume"

    def __str__(self) -> str:
        return str(self.value)
