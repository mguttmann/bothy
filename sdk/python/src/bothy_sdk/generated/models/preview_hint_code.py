from enum import StrEnum


class PreviewHintCode(StrEnum):
    EXPIRY_MISSING = "expiry_missing"
    GROUP_REACH = "group_reach"
    READ_HORIZON = "read_horizon"
    TERMINAL_NOT_EFFECTIVE = "terminal_not_effective"
    TRIGGER_BLOCKED_TOOL_CLASS = "trigger_blocked_tool_class"

    def __str__(self) -> str:
        return str(self.value)
