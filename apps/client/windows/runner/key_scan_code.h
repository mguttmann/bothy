// SPDX-License-Identifier: Apache-2.0
//
// Scan codes for key messages that arrive without one (docs/decisions.d/
// FIX-windows-dictation.md). Tools that insert text by sending Ctrl+V through SendInput with a
// virtual key only (dictation tools, text expanders, remote input) produce WM_KEYDOWN and
// WM_KEYUP with scan code 0 in lParam bits 16-23. The Flutter embedder derives the physical key
// from the scan code, so Ctrl and V both become the same unknown physical key: the engine sends
// a synthesized Ctrl up before V down, the app sees a plain "v" and the paste shortcut never
// fires. The runner fills the scan code from the virtual key before TranslateMessage and the
// engine see the message. Header-only and without <windows.h>, so the host compiler can test it.
#ifndef RUNNER_KEY_SCAN_CODE_H_
#define RUNNER_KEY_SCAN_CODE_H_

#include <cstdint>

namespace key_scan_code {

// The lParam of |message| with the scan code of |wparam| filled in: only for WM_KEYDOWN,
// WM_KEYUP, WM_SYSKEYDOWN and WM_SYSKEYUP whose scan code is 0 and whose virtual key is not
// VK_PACKET (KEYEVENTF_UNICODE input, which the engine leaves to WM_CHAR). |vk_to_vsc_ex| is
// MapVirtualKeyW(vk, MAPVK_VK_TO_VSC_EX) on Windows: its high byte 0xE0 marks an extended key,
// which sets bit 24. Every other lParam, and one whose key has no scan code (0), is returned
// unchanged.
inline int64_t WithScanCode(uint32_t message, uint64_t wparam, int64_t lparam,
                            uint32_t (*vk_to_vsc_ex)(uint32_t)) {
  constexpr uint32_t kKeyDown = 0x100;
  constexpr uint32_t kKeyUp = 0x101;
  constexpr uint32_t kSysKeyDown = 0x104;
  constexpr uint32_t kSysKeyUp = 0x105;
  constexpr uint64_t kVkPacket = 0xE7;
  if (message != kKeyDown && message != kKeyUp && message != kSysKeyDown &&
      message != kSysKeyUp) {
    return lparam;
  }
  if (((lparam >> 16) & 0xFF) != 0 || wparam == kVkPacket) {
    return lparam;
  }
  const uint32_t scan = vk_to_vsc_ex(static_cast<uint32_t>(wparam));
  if ((scan & 0xFF) == 0) {
    return lparam;
  }
  int64_t result = lparam | (static_cast<int64_t>(scan & 0xFF) << 16);
  if ((scan & 0xFF00) == 0xE000) {
    result |= static_cast<int64_t>(1) << 24;
  }
  return result;
}

}  // namespace key_scan_code

#endif  // RUNNER_KEY_SCAN_CODE_H_
