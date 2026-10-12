from enum import StrEnum


class RunContextMessageTrust(StrEnum):
    AUTHORIZED = "authorized"
    SELF = "self"
    SYSTEM = "system"
    TRIGGER = "trigger"
    UNVERIFIED = "unverified"

    def __str__(self) -> str:
        return str(self.value)
