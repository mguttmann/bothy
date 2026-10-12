from enum import StrEnum


class DraftUpdatedEventType(StrEnum):
    DRAFT_UPDATED = "draft.updated"

    def __str__(self) -> str:
        return str(self.value)
