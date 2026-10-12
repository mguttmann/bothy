from enum import StrEnum


class InteractionCreatedEventType(StrEnum):
    INTERACTION_CREATED = "interaction.created"

    def __str__(self) -> str:
        return str(self.value)
