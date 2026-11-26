// SPDX-License-Identifier: Apache-2.0
#pragma once

/// @file
/// The decision of the server check of the helper channel (D-CU-02-5), kept apart from the Win32
/// calls that gather its inputs so the rule is tested without Windows.

#include <cstdint>
#include <string_view>

namespace bothy::helper {

/// The rights the helper asks for when it opens the host service's pipe: FILE_GENERIC_READ plus
/// FILE_WRITE_DATA, exactly what the pipe's DACL grants interactive users (`pipeSDDL` in
/// services/host/internal/confirm/peer_windows.go). GENERIC_WRITE would add FILE_APPEND_DATA (on a
/// pipe FILE_CREATE_PIPE_INSTANCE), FILE_WRITE_EA and FILE_WRITE_ATTRIBUTES, which the DACL does
/// not grant, so CreateFile would fail with ERROR_ACCESS_DENIED and the helper never connect.
inline constexpr std::uint32_t kPipeClientAccess = 0x0012008b;

/// The SID string of LocalSystem.
inline constexpr std::string_view kLocalSystemSid = "S-1-5-18";

/// Decides whether the process at the server end of the pipe is the host service.
/// @param server_sid the user SID of the server process's token as a string (`S-1-...`); empty
///        when the process or its token could not be opened, which refuses.
/// @param own_sid the user SID of the helper's own process; empty refuses.
/// @param service_sid the SID of `NT SERVICE\bothy-host` from LookupAccountName; empty when the
///        account does not exist, which leaves only LocalSystem.
/// @return true exactly when server_sid is LocalSystem or service_sid, and is not the helper's own
///         user (a helper must never trust a server of its own account, which any process of that
///         account could be). SID strings compare without regard to case.
/// Example: pipe_server_allowed("S-1-5-18", "S-1-5-21-1-2-3-1001", "") is true.
bool pipe_server_allowed(std::string_view server_sid, std::string_view own_sid,
                         std::string_view service_sid);

}  // namespace bothy::helper
