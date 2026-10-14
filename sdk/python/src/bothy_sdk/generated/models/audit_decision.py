from enum import StrEnum


class AuditDecision(StrEnum):
    ALLOW = "allow"
    DENY = "deny"
    ERROR = "error"

    def __str__(self) -> str:
        return str(self.value)
