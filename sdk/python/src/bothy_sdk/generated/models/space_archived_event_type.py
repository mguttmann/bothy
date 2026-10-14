from enum import StrEnum


class SpaceArchivedEventType(StrEnum):
    SPACE_ARCHIVED = "space.archived"

    def __str__(self) -> str:
        return str(self.value)
