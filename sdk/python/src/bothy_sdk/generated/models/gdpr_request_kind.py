from enum import StrEnum


class GdprRequestKind(StrEnum):
    DELETION = "deletion"
    EXPORT = "export"
    RECTIFICATION = "rectification"

    def __str__(self) -> str:
        return str(self.value)
