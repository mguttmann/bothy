from enum import StrEnum


class StatsSloReportMeasuredBy(StrEnum):
    OWN = "own"

    def __str__(self) -> str:
        return str(self.value)
