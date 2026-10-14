from enum import StrEnum


class GdprDecisionDecision(StrEnum):
    APPROVE = "approve"
    REJECT = "reject"

    def __str__(self) -> str:
        return str(self.value)
