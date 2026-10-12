from enum import StrEnum


class FileTicketCreateVariant(StrEnum):
    LARGE = "large"
    MEDIUM = "medium"
    ORIGINAL = "original"
    POSTER = "poster"
    SMALL = "small"

    def __str__(self) -> str:
        return str(self.value)
