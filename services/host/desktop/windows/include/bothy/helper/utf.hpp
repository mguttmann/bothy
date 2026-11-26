// SPDX-License-Identifier: Apache-2.0
#pragma once

/// @file
/// UTF-8 and UTF-16 conversion. The channel carries UTF-8; Win32 and `KEYEVENTF_UNICODE` take
/// UTF-16 code units, so the conversion sits in the portable core where it is tested.

#include <optional>
#include <string>
#include <string_view>

namespace bothy::helper {

/// Converts UTF-8 to UTF-16.
/// @param text UTF-8 text.
/// @return the UTF-16 code units, characters above U+FFFF as surrogate pairs; std::nullopt when
///         the input is not valid UTF-8 (overlong forms, surrogates and values above U+10FFFF
///         included).
/// Example: utf8_to_utf16("\xF0\x9F\x98\x80") yields the two units 0xD83D 0xDE00.
std::optional<std::u16string> utf8_to_utf16(std::string_view text);

/// Converts UTF-16 to UTF-8.
/// @param text UTF-16 code units.
/// @return the UTF-8 text; std::nullopt for a lone surrogate.
std::optional<std::string> utf16_to_utf8(std::u16string_view text);

/// Appends the UTF-8 form of one code point.
/// @param out the string to append to.
/// @param code_point a Unicode scalar value (not a surrogate, at most U+10FFFF).
/// @return false, appending nothing, when code_point is not a scalar value.
bool append_utf8(std::string& out, char32_t code_point);

/// Checks UTF-8 validity with the same rules as utf8_to_utf16().
/// @param text the bytes.
/// @return true when the bytes are valid UTF-8.
bool is_valid_utf8(std::string_view text);

}  // namespace bothy::helper
