from enum import StrEnum


class AgentSessionEndReason(StrEnum):
    AGENT_CHANGED = "agent_changed"
    COMPUTER_USE = "computer_use"
    IMPORT = "import"
    REBIND = "rebind"
    RESET = "reset"

    def __str__(self) -> str:
        return str(self.value)
