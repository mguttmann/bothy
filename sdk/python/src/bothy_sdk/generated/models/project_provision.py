from enum import StrEnum


class ProjectProvision(StrEnum):
    CLONE = "clone"
    INIT = "init"
    NONE = "none"

    def __str__(self) -> str:
        return str(self.value)
