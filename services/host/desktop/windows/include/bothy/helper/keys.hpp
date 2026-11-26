// SPDX-License-Identifier: Apache-2.0
#pragma once

/// @file
/// Key names of `do key` and of click modifiers (docs/spec/27-computer-use.md section 6.4,
/// D-CU-02-7): `+` joins the parts and case does not matter. One table for both helpers, held by
/// the shortcut tests (CU-T15). The parser refuses only names it does not know; it never refuses a
/// known key or combination, because the helper adds no limit of its own (27 section 1 item 1).

#include <cstdint>
#include <optional>
#include <string>
#include <string_view>
#include <vector>

namespace bothy::helper {

/// The keys the names map to. `Cmd` is the Windows key on Windows (D-CU-02-7).
enum class KeyCode : std::uint8_t {
  Shift,
  Ctrl,
  Alt,
  Cmd,
  Return,
  Tab,
  Escape,
  BackSpace,
  Delete,
  Insert,
  Space,
  Up,
  Down,
  Left,
  Right,
  Home,
  End,
  PageUp,
  PageDown,
  CapsLock,
  Function,   ///< F1 to F24; the number is in Key::value
  Character,  ///< one character; the code point is in Key::value
};

/// One key.
struct Key {
  /// Which key.
  KeyCode code = KeyCode::Character;
  /// The function key number for KeyCode::Function, the code point for KeyCode::Character
  /// (letters lower-cased, because case does not matter), else 0.
  std::uint32_t value = 0;

  /// @return true for Shift, Ctrl, Alt and Cmd.
  bool is_modifier() const;
  /// @return true when code and value are equal.
  bool operator==(const Key& other) const { return code == other.code && value == other.value; }
  /// @return !(*this == other).
  bool operator!=(const Key& other) const { return !(*this == other); }
};

/// Parses one key name.
/// @param name a modifier, a named key or one character, any case; aliases of D-CU-02-7 included.
/// @return the key, or std::nullopt for an unknown name.
/// Example: parse_key("Page_Up") and parse_key("prior") both yield KeyCode::PageUp.
std::optional<Key> parse_key(std::string_view name);

/// Parses a combination such as `ctrl+alt+Delete`. Every part is pressed in order and released in
/// reverse order; a part need not be a modifier, so `a+b` presses a, then b. A `+` key is written as
/// the last part after a joining `+` (`ctrl++`) or alone (`+`).
/// @param keys the combination.
/// @return the keys in press order, or std::nullopt when the text is empty, a part is empty or a
///         part is unknown.
std::optional<std::vector<Key>> parse_keys(std::string_view keys);

/// Parses a modifier name of a click or scroll (`shift`, `ctrl`, `alt`, `cmd`, `win` and their
/// aliases).
/// @param name the name.
/// @return the key, or std::nullopt when the name is unknown or not a modifier.
std::optional<Key> parse_modifier(std::string_view name);

/// @return the canonical name of a key (`ctrl`, `Return`, `F5`, `q`), used in logs and tests.
std::string key_name(const Key& key);

}  // namespace bothy::helper
