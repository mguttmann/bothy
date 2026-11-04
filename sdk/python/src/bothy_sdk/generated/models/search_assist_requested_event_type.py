from enum import StrEnum


class SearchAssistRequestedEventType(StrEnum):
    SEARCH_ASSIST_REQUESTED = "search.assist_requested"

    def __str__(self) -> str:
        return str(self.value)
