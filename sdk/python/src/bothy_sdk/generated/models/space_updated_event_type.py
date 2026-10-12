from enum import StrEnum


class SpaceUpdatedEventType(StrEnum):
    SPACE_UPDATED = "space.updated"

    def __str__(self) -> str:
        return str(self.value)
