from enum import StrEnum


class CommandAutocompleteRequestedEventType(StrEnum):
    COMMAND_AUTOCOMPLETE_REQUESTED = "command.autocomplete_requested"

    def __str__(self) -> str:
        return str(self.value)
