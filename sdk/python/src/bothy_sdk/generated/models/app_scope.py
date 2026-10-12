from enum import StrEnum


class AppScope(StrEnum):
    COMMANDSWRITE = "commands:write"
    COMPONENTSWRITE = "components:write"
    COSTSRECONCILE = "costs:reconcile"
    EVENTSSUBSCRIBE = "events:subscribe"
    FILESREAD = "files:read"
    FILESWRITE = "files:write"
    MESSAGESREAD = "messages:read"
    MESSAGESWRITE = "messages:write"
    USERSREAD = "users:read"

    def __str__(self) -> str:
        return str(self.value)
