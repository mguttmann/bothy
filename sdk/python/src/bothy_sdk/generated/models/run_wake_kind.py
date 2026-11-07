from enum import StrEnum


class RunWakeKind(StrEnum):
    AT = "at"
    COMMAND = "command"
    PR_CHECKS = "pr_checks"

    def __str__(self) -> str:
        return str(self.value)
