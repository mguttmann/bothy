from enum import StrEnum


class AppManifestComponentsItem(StrEnum):
    BUTTON = "button"
    FORM = "form"
    SELECT = "select"

    def __str__(self) -> str:
        return str(self.value)
