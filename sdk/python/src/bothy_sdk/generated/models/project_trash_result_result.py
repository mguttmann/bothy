from enum import StrEnum


class ProjectTrashResultResult(StrEnum):
    REFUSED = "refused"
    TRASHED = "trashed"

    def __str__(self) -> str:
        return str(self.value)
