from enum import StrEnum


class PermissionHintRulesItem(StrEnum):
    SR2 = "sr2"
    SR3 = "sr3"

    def __str__(self) -> str:
        return str(self.value)
