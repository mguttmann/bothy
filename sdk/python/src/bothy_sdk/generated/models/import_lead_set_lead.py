from enum import StrEnum


class ImportLeadSetLead(StrEnum):
    BOTHY = "bothy"
    DISCORD = "discord"

    def __str__(self) -> str:
        return str(self.value)
