// SPDX-License-Identifier: Apache-2.0

#include "win32.hpp"

namespace bothy::helper::win32 {

std::wstring to_wide(const std::string& text) {
  if (text.empty()) return {};
  int n = MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, text.data(),
                              static_cast<int>(text.size()), nullptr, 0);
  if (n <= 0) return {};
  std::wstring out(static_cast<std::size_t>(n), L'\0');
  MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, text.data(), static_cast<int>(text.size()),
                      out.data(), n);
  return out;
}

std::string to_utf8(const std::wstring& text) {
  if (text.empty()) return {};
  int n = WideCharToMultiByte(CP_UTF8, WC_ERR_INVALID_CHARS, text.data(),
                              static_cast<int>(text.size()), nullptr, 0, nullptr, nullptr);
  if (n <= 0) return {};
  std::string out(static_cast<std::size_t>(n), '\0');
  WideCharToMultiByte(CP_UTF8, WC_ERR_INVALID_CHARS, text.data(), static_cast<int>(text.size()),
                      out.data(), n, nullptr, nullptr);
  return out;
}

}  // namespace bothy::helper::win32
