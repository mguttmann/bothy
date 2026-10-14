from enum import StrEnum


class BillingMode(StrEnum):
    ORGANIZATION = "organization"
    PERSONAL = "personal"

    def __str__(self) -> str:
        return str(self.value)
