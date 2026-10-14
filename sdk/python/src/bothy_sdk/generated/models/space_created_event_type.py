from enum import StrEnum


class SpaceCreatedEventType(StrEnum):
    SPACE_CREATED = "space.created"

    def __str__(self) -> str:
        return str(self.value)
