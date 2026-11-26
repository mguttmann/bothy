// SPDX-License-Identifier: Apache-2.0

#include "bothy/helper/keys.hpp"

#include <cctype>

#include "bothy/helper/utf.hpp"

namespace bothy::helper {

namespace {

struct Name {
  const char* name;
  KeyCode code;
};

// D-CU-02-7, aliases included. Compared without regard to ASCII case.
constexpr Name kNames[] = {
    {"shift", KeyCode::Shift},         {"ctrl", KeyCode::Ctrl},
    {"control", KeyCode::Ctrl},        {"alt", KeyCode::Alt},
    {"option", KeyCode::Alt},          {"cmd", KeyCode::Cmd},
    {"command", KeyCode::Cmd},         {"super", KeyCode::Cmd},
    {"meta", KeyCode::Cmd},            {"win", KeyCode::Cmd},
    {"windows", KeyCode::Cmd},         {"return", KeyCode::Return},
    {"enter", KeyCode::Return},        {"tab", KeyCode::Tab},
    {"escape", KeyCode::Escape},       {"esc", KeyCode::Escape},
    {"backspace", KeyCode::BackSpace}, {"delete", KeyCode::Delete},
    {"del", KeyCode::Delete},  // D-CU-02-14: CU-T15 names ctrl+alt+del
    {"insert", KeyCode::Insert},       {"space", KeyCode::Space},
    {"up", KeyCode::Up},               {"down", KeyCode::Down},
    {"left", KeyCode::Left},           {"right", KeyCode::Right},
    {"home", KeyCode::Home},           {"end", KeyCode::End},
    {"page_up", KeyCode::PageUp},      {"prior", KeyCode::PageUp},
    {"page_down", KeyCode::PageDown},  {"next", KeyCode::PageDown},
    {"capslock", KeyCode::CapsLock},
};

bool equal_ascii_nocase(std::string_view a, std::string_view b) {
  if (a.size() != b.size()) return false;
  for (std::size_t i = 0; i < a.size(); ++i) {
    if (std::tolower(static_cast<unsigned char>(a[i])) != std::tolower(static_cast<unsigned char>(b[i])))
      return false;
  }
  return true;
}

}  // namespace

bool Key::is_modifier() const {
  return code == KeyCode::Shift || code == KeyCode::Ctrl || code == KeyCode::Alt ||
         code == KeyCode::Cmd;
}

std::optional<Key> parse_key(std::string_view name) {
  if (name.empty()) return std::nullopt;
  for (const Name& n : kNames)
    if (equal_ascii_nocase(name, n.name)) return Key{n.code, 0};
  if (name.size() >= 2 && (name[0] == 'F' || name[0] == 'f')) {
    std::uint32_t number = 0;
    bool digits = name.size() <= 3;
    for (std::size_t i = 1; digits && i < name.size(); ++i) {
      if (name[i] < '0' || name[i] > '9') digits = false;
      else number = number * 10 + static_cast<std::uint32_t>(name[i] - '0');
    }
    if (digits && name[1] != '0' && number >= 1 && number <= 24) return Key{KeyCode::Function, number};
    if (digits) return std::nullopt;  // F0, F25 and the like: no such key
  }
  // One character, whatever its UTF-8 length.
  std::optional<std::u16string> units = utf8_to_utf16(name);
  if (!units || units->empty() || units->size() > 2) return std::nullopt;
  char32_t cp = (*units)[0];
  if (units->size() == 2) {
    if (cp < 0xD800 || cp > 0xDBFF) return std::nullopt;
    cp = 0x10000 + ((cp - 0xD800) << 10) + ((*units)[1] - 0xDC00);
  }
  if (cp >= 'A' && cp <= 'Z') cp = cp - 'A' + 'a';  // case does not matter (D-CU-02-7)
  return Key{KeyCode::Character, static_cast<std::uint32_t>(cp)};
}

std::optional<std::vector<Key>> parse_keys(std::string_view keys) {
  if (keys == "+") return std::vector<Key>{Key{KeyCode::Character, '+'}};
  bool plus_last = keys.size() >= 2 && keys.substr(keys.size() - 2) == "++";
  if (plus_last) keys.remove_suffix(2);
  std::vector<Key> out;
  if (!keys.empty() || !plus_last) {
    std::size_t start = 0;
    while (true) {
      std::size_t end = keys.find('+', start);
      std::string_view part = keys.substr(start, end == std::string_view::npos ? std::string_view::npos : end - start);
      std::optional<Key> k = parse_key(part);
      if (!k) return std::nullopt;
      out.push_back(*k);
      if (end == std::string_view::npos) break;
      start = end + 1;
    }
  }
  if (plus_last) out.push_back(Key{KeyCode::Character, '+'});
  if (out.empty()) return std::nullopt;
  return out;
}

std::optional<Key> parse_modifier(std::string_view name) {
  std::optional<Key> k = parse_key(name);
  if (!k || !k->is_modifier()) return std::nullopt;
  return k;
}

std::string key_name(const Key& key) {
  switch (key.code) {
    case KeyCode::Shift: return "shift";
    case KeyCode::Ctrl: return "ctrl";
    case KeyCode::Alt: return "alt";
    case KeyCode::Cmd: return "cmd";
    case KeyCode::Return: return "Return";
    case KeyCode::Tab: return "Tab";
    case KeyCode::Escape: return "Escape";
    case KeyCode::BackSpace: return "BackSpace";
    case KeyCode::Delete: return "Delete";
    case KeyCode::Insert: return "Insert";
    case KeyCode::Space: return "space";
    case KeyCode::Up: return "Up";
    case KeyCode::Down: return "Down";
    case KeyCode::Left: return "Left";
    case KeyCode::Right: return "Right";
    case KeyCode::Home: return "Home";
    case KeyCode::End: return "End";
    case KeyCode::PageUp: return "Page_Up";
    case KeyCode::PageDown: return "Page_Down";
    case KeyCode::CapsLock: return "CapsLock";
    case KeyCode::Function: return "F" + std::to_string(key.value);
    case KeyCode::Character: {
      std::string out;
      if (!append_utf8(out, static_cast<char32_t>(key.value))) return "?";
      return out;
    }
  }
  return "?";
}

}  // namespace bothy::helper
