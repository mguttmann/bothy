from enum import StrEnum


class GroupChangedPayloadChange(StrEnum):
    CREATED = "created"
    DELETED = "deleted"
    MEMBER_ADDED = "member_added"
    MEMBER_EXPIRED = "member_expired"
    MEMBER_REMOVED = "member_removed"
    UPDATED = "updated"

    def __str__(self) -> str:
        return str(self.value)
