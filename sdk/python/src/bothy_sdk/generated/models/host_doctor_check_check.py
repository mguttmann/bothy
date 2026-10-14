from enum import StrEnum


class HostDoctorCheckCheck(StrEnum):
    HD10_HELPER_INTEGRITY = "hd10_helper_integrity"
    HD11_SHARED_LOCATIONS = "hd11_shared_locations"
    HD1_USER = "hd1_user"
    HD2_GROUPS = "hd2_groups"
    HD3_SUDO = "hd3_sudo"
    HD4_STATE_DIR = "hd4_state_dir"
    HD5_SERVER_HOST = "hd5_server_host"
    HD6_LLM_LOGIN = "hd6_llm_login"
    HD7_PROFILES = "hd7_profiles"
    HD8_SEPARATION = "hd8_separation"
    HD9_FOREIGN_HOMES = "hd9_foreign_homes"

    def __str__(self) -> str:
        return str(self.value)
