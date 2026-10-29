// SPDX-License-Identifier: Apache-2.0
#include "rich_clipboard.h"

#include <cstring>

namespace rich_clipboard {

namespace {

// The registered clipboard format of HTML.
UINT HtmlFormat() { return RegisterClipboardFormatW(L"HTML Format"); }

// |bytes| in a movable global block with a terminating zero of |zero_bytes| bytes; null when
// the allocation fails.
HGLOBAL Global(const void* bytes, size_t size, size_t zero_bytes) {
  HGLOBAL block = GlobalAlloc(GMEM_MOVEABLE, size + zero_bytes);
  if (block == nullptr) {
    return nullptr;
  }
  auto* target = static_cast<char*>(GlobalLock(block));
  if (target == nullptr) {
    GlobalFree(block);
    return nullptr;
  }
  if (size > 0) {
    std::memcpy(target, bytes, size);
  }
  std::memset(target + size, 0, zero_bytes);
  GlobalUnlock(block);
  return block;
}

}  // namespace

bool Write(HWND owner, const std::string& text, const std::string& cf_html) {
  std::wstring wide;
  if (!text.empty()) {
    const int length = MultiByteToWideChar(CP_UTF8, 0, text.data(),
                                           static_cast<int>(text.size()),
                                           nullptr, 0);
    if (length <= 0) {
      return false;
    }
    wide.resize(length);
    MultiByteToWideChar(CP_UTF8, 0, text.data(), static_cast<int>(text.size()),
                        wide.data(), length);
  }
  const UINT html_format = HtmlFormat();
  if (html_format == 0) {
    return false;
  }
  HGLOBAL text_block =
      Global(wide.data(), wide.size() * sizeof(wchar_t), sizeof(wchar_t));
  HGLOBAL html_block = Global(cf_html.data(), cf_html.size(), 1);
  if (text_block == nullptr || html_block == nullptr ||
      !OpenClipboard(owner)) {
    if (text_block != nullptr) GlobalFree(text_block);
    if (html_block != nullptr) GlobalFree(html_block);
    return false;
  }
  EmptyClipboard();
  // The clipboard owns a block once SetClipboardData succeeds.
  const bool text_stored = SetClipboardData(CF_UNICODETEXT, text_block) != nullptr;
  if (!text_stored) GlobalFree(text_block);
  const bool html_stored = SetClipboardData(html_format, html_block) != nullptr;
  if (!html_stored) GlobalFree(html_block);
  CloseClipboard();
  return text_stored && html_stored;
}

std::optional<std::string> ReadHtml(HWND owner) {
  const UINT html_format = HtmlFormat();
  if (html_format == 0 || !IsClipboardFormatAvailable(html_format) ||
      !OpenClipboard(owner)) {
    return std::nullopt;
  }
  std::optional<std::string> html;
  HANDLE block = GetClipboardData(html_format);
  if (block != nullptr) {
    const auto* bytes = static_cast<const char*>(GlobalLock(block));
    if (bytes != nullptr) {
      const size_t size = GlobalSize(block);
      html = std::string(bytes, strnlen(bytes, size));
      GlobalUnlock(block);
    }
  }
  CloseClipboard();
  return html;
}

}  // namespace rich_clipboard
