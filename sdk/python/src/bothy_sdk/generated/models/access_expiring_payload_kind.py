from enum import StrEnum


class AccessExpiringPayloadKind(StrEnum):
    GRANT = "grant"
    GROUP_MEMBERSHIP = "group_membership"

    def __str__(self) -> str:
        return str(self.value)
