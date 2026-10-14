from enum import StrEnum


class TerminalCloseDetail(StrEnum):
    HANDSHAKE_FAILED = "handshake_failed"
    HELPER_UNTRUSTED = "helper_untrusted"
    LOCAL_CONFIRM_DENIED = "local_confirm_denied"
    OPEN_TIMEOUT = "open_timeout"
    PROFILE_UNKNOWN = "profile_unknown"
    PTY_FAILED = "pty_failed"
    SERVER_HOST_OWNER_ONLY = "server_host_owner_only"
    SESSION_USER_MISMATCH = "session_user_mismatch"
    SESSION_USER_UNKNOWN = "session_user_unknown"
    SHARED_LOCATION = "shared_location"
    SUBSCRIPTION_LOGIN_DETECTED = "subscription_login_detected"
    TOKEN_EXPIRED = "token_expired"
    TOKEN_INVALID = "token_invalid"
    TOKEN_REPLAYED = "token_replayed"

    def __str__(self) -> str:
        return str(self.value)
