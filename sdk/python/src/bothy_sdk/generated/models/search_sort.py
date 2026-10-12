from enum import StrEnum


class SearchSort(StrEnum):
    NEWEST = "newest"
    RELEVANCE = "relevance"

    def __str__(self) -> str:
        return str(self.value)
