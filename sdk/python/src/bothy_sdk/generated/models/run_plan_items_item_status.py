from enum import StrEnum


class RunPlanItemsItemStatus(StrEnum):
    COMPLETED = "completed"
    IN_PROGRESS = "in_progress"
    PENDING = "pending"

    def __str__(self) -> str:
        return str(self.value)
