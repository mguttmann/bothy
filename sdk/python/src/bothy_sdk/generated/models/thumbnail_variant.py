from enum import StrEnum


class ThumbnailVariant(StrEnum):
    LARGE = "large"
    MEDIUM = "medium"
    POSTER = "poster"
    SMALL = "small"

    def __str__(self) -> str:
        return str(self.value)
