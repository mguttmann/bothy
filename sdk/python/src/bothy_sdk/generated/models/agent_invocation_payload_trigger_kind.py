from enum import StrEnum


class AgentInvocationPayloadTriggerKind(StrEnum):
    COMMAND = "command"
    INTERACTION = "interaction"
    MESSAGE = "message"
    TEAM = "team"
    WAKE = "wake"

    def __str__(self) -> str:
        return str(self.value)
