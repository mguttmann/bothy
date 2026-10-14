from enum import StrEnum


class JsonPatchOperationOp(StrEnum):
    ADD = "add"
    REMOVE = "remove"
    REPLACE = "replace"

    def __str__(self) -> str:
        return str(self.value)
