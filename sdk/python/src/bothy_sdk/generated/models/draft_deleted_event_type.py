from enum import StrEnum


class DraftDeletedEventType(StrEnum):
    DRAFT_DELETED = "draft.deleted"

    def __str__(self) -> str:
        return str(self.value)
