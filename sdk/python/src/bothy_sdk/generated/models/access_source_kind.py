from enum import StrEnum


class AccessSourceKind(StrEnum):
    DIRECT = "direct"
    DM_PARTICIPANT = "dm_participant"
    GROUP = "group"
    GUARD_BLOCKED = "guard_blocked"
    INHERITED = "inherited"
    OPERATOR = "operator"
    OWNER = "owner"
    VIA_AGENT = "via_agent"

    def __str__(self) -> str:
        return str(self.value)
