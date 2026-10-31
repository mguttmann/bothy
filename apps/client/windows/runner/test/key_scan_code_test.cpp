// SPDX-License-Identifier: Apache-2.0
//
// Tests of key_scan_code.h with the host C++ compiler (run_tests.sh, `make runner-test`;
// docs/decisions.d/FIX-windows-dictation.md). The header has no Windows dependency, so the
// test runs on any host; the map of virtual keys to scan codes is a fake.
#include "../key_scan_code.h"

#include <cstdint>
#include <cstdio>

namespace {

int failures = 0;

// A fake MapVirtualKeyW(vk, MAPVK_VK_TO_VSC_EX): Ctrl, V and the extended Delete key.
uint32_t FakeMap(uint32_t vk) {
  switch (vk) {
    case 0x11:
      return 0x1D;
    case 0x56:
      return 0x2F;
    case 0x2E:
      return 0xE053;
    default:
      return 0;
  }
}

void Expect(const char* name, int64_t got, int64_t want) {
  if (got != want) {
    ++failures;
    std::printf("FAIL %s: got 0x%08llX, want 0x%08llX\n", name,
                static_cast<unsigned long long>(got),
                static_cast<unsigned long long>(want));
  } else {
    std::printf("ok   %s\n", name);
  }
}

constexpr uint32_t kKeyDown = 0x100;
constexpr uint32_t kKeyUp = 0x101;
constexpr uint32_t kSysKeyDown = 0x104;
constexpr uint32_t kSysKeyUp = 0x105;
constexpr uint32_t kChar = 0x102;
constexpr uint32_t kMouseMove = 0x200;

}  // namespace

int main() {
  using key_scan_code::WithScanCode;
  // The Ctrl+V of a tool that sends only virtual keys (scan code 0).
  Expect("ctrl down gets its scan code",
         WithScanCode(kKeyDown, 0x11, 0x00000001, FakeMap), 0x001D0001);
  Expect("v down gets its scan code",
         WithScanCode(kKeyDown, 0x56, 0x00000001, FakeMap), 0x002F0001);
  Expect("v up keeps the transition bits",
         WithScanCode(kKeyUp, 0x56, 0xC0000001, FakeMap), 0xC02F0001);
  Expect("ctrl up gets its scan code",
         WithScanCode(kKeyUp, 0x11, 0xC0000001, FakeMap), 0xC01D0001);
  Expect("syskeydown gets its scan code",
         WithScanCode(kSysKeyDown, 0x56, 0x20000001, FakeMap), 0x202F0001);
  Expect("syskeyup gets its scan code",
         WithScanCode(kSysKeyUp, 0x56, 0xE0000001, FakeMap), 0xE02F0001);
  // A real keyboard already sends a scan code.
  Expect("a real scan code stays",
         WithScanCode(kKeyDown, 0x56, 0x002F0001, FakeMap), 0x002F0001);
  Expect("a different real scan code stays",
         WithScanCode(kKeyDown, 0x56, 0x00110001, FakeMap), 0x00110001);
  // KEYEVENTF_UNICODE input: the character travels in the scan code field and is left alone.
  Expect("vk_packet stays",
         WithScanCode(kKeyDown, 0xE7, 0x00000001, FakeMap), 0x00000001);
  // An extended key gets the extended bit 24.
  Expect("delete gets the extended bit",
         WithScanCode(kKeyDown, 0x2E, 0x00000001, FakeMap), 0x01530001);
  // A virtual key without a scan code in the layout stays as it is.
  Expect("an unknown key stays",
         WithScanCode(kKeyDown, 0xFF, 0x00000001, FakeMap), 0x00000001);
  // Other messages are never touched.
  Expect("wm_char stays", WithScanCode(kChar, 0x76, 0x00000001, FakeMap),
         0x00000001);
  Expect("wm_mousemove stays",
         WithScanCode(kMouseMove, 0x11, 0x00000001, FakeMap), 0x00000001);
  if (failures != 0) {
    std::printf("%d failed\n", failures);
    return 1;
  }
  std::printf("all passed\n");
  return 0;
}
