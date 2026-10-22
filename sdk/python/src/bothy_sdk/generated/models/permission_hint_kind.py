from enum import StrEnum


class PermissionHintKind(StrEnum):
    AGENT_PERSONAL = "agent_personal"
    GRANT_TO_DISABLED = "grant_to_disabled"
    H1_READ_HORIZON = "h1_read_horizon"
    TRIGGER_BLOCKED = "trigger_blocked"

    def __str__(self) -> str:
        return str(self.value)
