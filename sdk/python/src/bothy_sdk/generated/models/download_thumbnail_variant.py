from enum import StrEnum


class DownloadThumbnailVariant(StrEnum):
    LARGE = "large"
    MEDIUM = "medium"
    POSTER = "poster"
    SMALL = "small"

    def __str__(self) -> str:
        return str(self.value)
