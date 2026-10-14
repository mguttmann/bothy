from enum import StrEnum


class ComponentRestriction(StrEnum):
    ANY = "any"
    OWNER = "owner"
    RUN_STARTER = "run_starter"

    def __str__(self) -> str:
        return str(self.value)
