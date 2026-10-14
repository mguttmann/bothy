from enum import StrEnum


class Capability(StrEnum):
    APPROVE_ACTIONS = "approve_actions"
    EXPORT = "export"
    MANAGE = "manage"
    TERMINAL = "terminal"
    UPLOAD = "upload"

    def __str__(self) -> str:
        return str(self.value)
