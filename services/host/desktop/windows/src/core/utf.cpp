// SPDX-License-Identifier: Apache-2.0

#include "bothy/helper/utf.hpp"

#include <cstdint>

namespace bothy::helper {

namespace {

// Decodes one UTF-8 sequence at `i`; returns false for anything that is not the shortest form of a
// Unicode scalar value, so overlong forms cannot smuggle characters past a check.
bool decode_one(std::string_view s, std::size_t& i, char32_t& out) {
  auto b0 = static_cast<unsigned char>(s[i]);
  if (b0 < 0x80) {
    out = b0;
    ++i;
    return true;
  }
  int extra = 0;
  char32_t cp = 0;
  char32_t min = 0;
  if ((b0 & 0xE0) == 0xC0) {
    extra = 1;
    cp = b0 & 0x1F;
    min = 0x80;
  } else if ((b0 & 0xF0) == 0xE0) {
    extra = 2;
    cp = b0 & 0x0F;
    min = 0x800;
  } else if ((b0 & 0xF8) == 0xF0) {
    extra = 3;
    cp = b0 & 0x07;
    min = 0x10000;
  } else {
    return false;
  }
  if (s.size() - i <= static_cast<std::size_t>(extra)) return false;  // truncated sequence
  for (int k = 1; k <= extra; ++k) {
    auto b = static_cast<unsigned char>(s[i + static_cast<std::size_t>(k)]);
    if ((b & 0xC0) != 0x80) return false;
    cp = (cp << 6) | (b & 0x3F);
  }
  if (cp < min || cp > 0x10FFFF || (cp >= 0xD800 && cp <= 0xDFFF)) return false;
  out = cp;
  i += static_cast<std::size_t>(extra) + 1;
  return true;
}

}  // namespace

bool append_utf8(std::string& out, char32_t cp) {
  if (cp > 0x10FFFF || (cp >= 0xD800 && cp <= 0xDFFF)) return false;
  if (cp < 0x80) {
    out.push_back(static_cast<char>(cp));
  } else if (cp < 0x800) {
    out.push_back(static_cast<char>(0xC0 | (cp >> 6)));
    out.push_back(static_cast<char>(0x80 | (cp & 0x3F)));
  } else if (cp < 0x10000) {
    out.push_back(static_cast<char>(0xE0 | (cp >> 12)));
    out.push_back(static_cast<char>(0x80 | ((cp >> 6) & 0x3F)));
    out.push_back(static_cast<char>(0x80 | (cp & 0x3F)));
  } else {
    out.push_back(static_cast<char>(0xF0 | (cp >> 18)));
    out.push_back(static_cast<char>(0x80 | ((cp >> 12) & 0x3F)));
    out.push_back(static_cast<char>(0x80 | ((cp >> 6) & 0x3F)));
    out.push_back(static_cast<char>(0x80 | (cp & 0x3F)));
  }
  return true;
}

std::optional<std::u16string> utf8_to_utf16(std::string_view text) {
  std::u16string out;
  out.reserve(text.size());
  std::size_t i = 0;
  while (i < text.size()) {
    char32_t cp = 0;
    if (!decode_one(text, i, cp)) return std::nullopt;
    if (cp >= 0x10000) {
      cp -= 0x10000;
      out.push_back(static_cast<char16_t>(0xD800 + (cp >> 10)));
      out.push_back(static_cast<char16_t>(0xDC00 + (cp & 0x3FF)));
    } else {
      out.push_back(static_cast<char16_t>(cp));
    }
  }
  return out;
}

std::optional<std::string> utf16_to_utf8(std::u16string_view text) {
  std::string out;
  out.reserve(text.size());
  for (std::size_t i = 0; i < text.size(); ++i) {
    char32_t cp = text[i];
    if (cp >= 0xD800 && cp <= 0xDBFF) {
      if (i + 1 >= text.size()) return std::nullopt;
      char32_t lo = text[i + 1];
      if (lo < 0xDC00 || lo > 0xDFFF) return std::nullopt;
      cp = 0x10000 + ((cp - 0xD800) << 10) + (lo - 0xDC00);
      ++i;
    } else if (cp >= 0xDC00 && cp <= 0xDFFF) {
      return std::nullopt;
    }
    append_utf8(out, cp);
  }
  return out;
}

bool is_valid_utf8(std::string_view text) {
  std::size_t i = 0;
  char32_t cp = 0;
  while (i < text.size())
    if (!decode_one(text, i, cp)) return false;
  return true;
}

}  // namespace bothy::helper
