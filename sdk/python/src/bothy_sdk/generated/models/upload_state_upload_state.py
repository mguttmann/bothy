from enum import StrEnum


class UploadStateUploadState(StrEnum):
    COMPLETE = "complete"
    PENDING = "pending"
    UPLOADING = "uploading"

    def __str__(self) -> str:
        return str(self.value)
