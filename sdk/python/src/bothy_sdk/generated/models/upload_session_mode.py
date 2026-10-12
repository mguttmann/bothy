from enum import StrEnum


class UploadSessionMode(StrEnum):
    DIRECT = "direct"
    PRESIGNED = "presigned"

    def __str__(self) -> str:
        return str(self.value)
