from enum import StrEnum


class ProjectFolderDeletionTrashState(StrEnum):
    REFUSED = "refused"
    REQUESTED = "requested"
    TRASHED = "trashed"

    def __str__(self) -> str:
        return str(self.value)
