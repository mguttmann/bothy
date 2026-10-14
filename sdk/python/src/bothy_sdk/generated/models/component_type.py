from enum import StrEnum


class ComponentType(StrEnum):
    BUTTON = "button"
    FORM = "form"
    SELECT = "select"

    def __str__(self) -> str:
        return str(self.value)
