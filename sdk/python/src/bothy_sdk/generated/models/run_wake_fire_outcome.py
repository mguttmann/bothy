from enum import StrEnum


class RunWakeFireOutcome(StrEnum):
    FAILED = "failed"
    FIRED = "fired"
    INPUT_SKIPPED = "input_skipped"

    def __str__(self) -> str:
        return str(self.value)
