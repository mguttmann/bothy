// SPDX-License-Identifier: Apache-2.0
//
// The clipboard of the block copies of messages on Windows (docs/spec/01-product.md 6.12 item
// 4, brief M4-48): the plain text as CF_UNICODETEXT and the HTML form as the registered format
// "HTML Format" (CF_HTML) in one clipboard change, so a rich editor pastes the formatting and
// every other app the text. Dart builds the CF_HTML document with its byte offsets
// (`windowsClipboardHtml` in lib/app/rich_clipboard.dart); the runner stores its bytes as they
// are. The window serves both over the method channel `bothy/clipboard`.
#ifndef RUNNER_RICH_CLIPBOARD_H_
#define RUNNER_RICH_CLIPBOARD_H_

#include <windows.h>

#include <optional>
#include <string>

namespace rich_clipboard {

// Replaces the clipboard with |text| (UTF-8) and the CF_HTML document |cf_html| (UTF-8);
// false when the clipboard could not be opened or a format was not stored.
bool Write(HWND owner, const std::string& text, const std::string& cf_html);

// The CF_HTML document on the clipboard, or nothing; for the platform test.
std::optional<std::string> ReadHtml(HWND owner);

}  // namespace rich_clipboard

#endif  // RUNNER_RICH_CLIPBOARD_H_
