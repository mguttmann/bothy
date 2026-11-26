// SPDX-License-Identifier: Apache-2.0
#pragma once

/// @file
/// The two files the helper reads: its own enablement state file (D-CU-02-4) and the host
/// service's configuration file for `host_service.computer_frame_max_bytes` (D-CU-02-8). Only the
/// text handling lives here; the Win32 layer does the file access.

#include <cstdint>
#include <string>
#include <string_view>

#include "bothy/helper/platform.hpp"

namespace bothy::helper {

/// Parses the content of the enablement state file `{"v": 1, "enabled": <bool>, "generation": <int>}`.
/// @param text the file content.
/// @return the state; off with generation 0 when the text is not exactly that shape (wrong
///         version, a negative generation, unknown members), so a damaged file never switches the
///         helper on.
EnablementState parse_enablement(std::string_view text);

/// Writes the content of the enablement state file.
/// @param state the state.
/// @return the JSON text with a final line break.
std::string serialize_enablement(const EnablementState& state);

/// Reads `host_service.computer_frame_max_bytes` from the text of the host service's YAML
/// configuration file. The file is nested like the key names (`host_service:` with an indented
/// `computer_frame_max_bytes: <int>`); the helper has no YAML library, so this reads the block form
/// the host service writes and nothing else.
/// @param yaml the file content.
/// @return the value, or kDefaultFrameMaxBytes when the key is absent, not a positive integer, or
///         written in a form this reader does not know (flow style, anchors).
std::uint64_t frame_max_from_host_config(std::string_view yaml);

}  // namespace bothy::helper
