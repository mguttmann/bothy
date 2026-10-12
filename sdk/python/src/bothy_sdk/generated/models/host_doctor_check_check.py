from enum import StrEnum


class HostDoctorCheckCheck(StrEnum):
    HD1_USER = "hd1_user"
    HD2_GROUPS = "hd2_groups"
    HD3_SUDO = "hd3_sudo"
    HD4_STATE_DIR = "hd4_state_dir"
    HD5_SERVER_HOST = "hd5_server_host"
    HD6_LLM_LOGIN = "hd6_llm_login"
    HD7_PROFILES = "hd7_profiles"
    HD8_SEPARATION = "hd8_separation"

    def __str__(self) -> str:
        return str(self.value)
