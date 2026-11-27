// SPDX-License-Identifier: Apache-2.0

// Synthetic input with SendInput (27 9.2). One call per action, so the steps of a combination reach
// the input stream together and no real input lands between them.

#include "win32.hpp"  // first: windows.h must precede the other Windows headers

#include "bothy/helper/geometry.hpp"

namespace bothy::helper::win32 {

namespace {

struct Vk {
  WORD vk = 0;
  bool extended = false;
  BYTE shift_state = 0;  // from VkKeyScanExW: 1 shift, 2 ctrl, 4 alt
};

HKL foreground_layout() {
  HWND w = GetForegroundWindow();
  DWORD thread = w != nullptr ? GetWindowThreadProcessId(w, nullptr) : 0;
  return GetKeyboardLayout(thread);
}

std::optional<Vk> map_key(const Key& key) {
  switch (key.code) {
    case KeyCode::Shift: return Vk{VK_SHIFT, false, 0};
    case KeyCode::Ctrl: return Vk{VK_CONTROL, false, 0};
    case KeyCode::Alt: return Vk{VK_MENU, false, 0};
    case KeyCode::Cmd: return Vk{VK_LWIN, true, 0};
    case KeyCode::Return: return Vk{VK_RETURN, false, 0};
    case KeyCode::Tab: return Vk{VK_TAB, false, 0};
    case KeyCode::Escape: return Vk{VK_ESCAPE, false, 0};
    case KeyCode::BackSpace: return Vk{VK_BACK, false, 0};
    case KeyCode::Delete: return Vk{VK_DELETE, true, 0};
    case KeyCode::Insert: return Vk{VK_INSERT, true, 0};
    case KeyCode::Space: return Vk{VK_SPACE, false, 0};
    case KeyCode::Up: return Vk{VK_UP, true, 0};
    case KeyCode::Down: return Vk{VK_DOWN, true, 0};
    case KeyCode::Left: return Vk{VK_LEFT, true, 0};
    case KeyCode::Right: return Vk{VK_RIGHT, true, 0};
    case KeyCode::Home: return Vk{VK_HOME, true, 0};
    case KeyCode::End: return Vk{VK_END, true, 0};
    case KeyCode::PageUp: return Vk{VK_PRIOR, true, 0};
    case KeyCode::PageDown: return Vk{VK_NEXT, true, 0};
    case KeyCode::CapsLock: return Vk{VK_CAPITAL, false, 0};
    case KeyCode::Function:
      if (key.value < 1 || key.value > 24) return std::nullopt;
      return Vk{static_cast<WORD>(VK_F1 + key.value - 1), false, 0};
    case KeyCode::Character: {
      if (key.value > 0xFFFF) return std::nullopt;  // no key of a layout produces it
      SHORT r = VkKeyScanExW(static_cast<WCHAR>(key.value), foreground_layout());
      if (r == -1) return std::nullopt;
      return Vk{static_cast<WORD>(r & 0xFF), false, static_cast<BYTE>((r >> 8) & 0xFF)};
    }
  }
  return std::nullopt;
}

INPUT key_input(WORD vk, bool extended, bool up) {
  INPUT in{};
  in.type = INPUT_KEYBOARD;
  in.ki.wVk = vk;
  in.ki.wScan = static_cast<WORD>(MapVirtualKeyW(vk, MAPVK_VK_TO_VSC));
  in.ki.dwFlags = (extended ? KEYEVENTF_EXTENDEDKEY : 0) | (up ? KEYEVENTF_KEYUP : 0);
  in.ki.dwExtraInfo = kInjectedMarker;
  return in;
}

// Appends the inputs of one key step. A character whose layout needs Shift, Ctrl or Alt gets them
// around it inside the same step, so the step stays balanced and the release logic stays simple.
void append_key(std::vector<INPUT>& out, const Key& key, bool up) {
  std::optional<Vk> vk = map_key(key);
  if (!vk) return;
  const WORD mods[3] = {VK_SHIFT, VK_CONTROL, VK_MENU};
  if (!up) {
    for (int i = 0; i < 3; ++i)
      if (vk->shift_state & (1 << i)) out.push_back(key_input(mods[i], false, false));
    out.push_back(key_input(vk->vk, vk->extended, false));
  } else {
    out.push_back(key_input(vk->vk, vk->extended, true));
    for (int i = 2; i >= 0; --i)
      if (vk->shift_state & (1 << i)) out.push_back(key_input(mods[i], false, true));
  }
}

INPUT mouse_input(DWORD flags, LONG x = 0, LONG y = 0, DWORD data = 0) {
  INPUT in{};
  in.type = INPUT_MOUSE;
  in.mi.dx = x;
  in.mi.dy = y;
  in.mi.mouseData = data;
  in.mi.dwFlags = flags;
  in.mi.dwExtraInfo = kInjectedMarker;
  return in;
}

PixelRect virtual_desktop() {
  return PixelRect{GetSystemMetrics(SM_XVIRTUALSCREEN), GetSystemMetrics(SM_YVIRTUALSCREEN),
                   GetSystemMetrics(SM_CXVIRTUALSCREEN), GetSystemMetrics(SM_CYVIRTUALSCREEN)};
}

// Wheel delta for `lines` lines: one notch (WHEEL_DELTA) scrolls the user's configured number of
// lines, so the delta is scaled by that setting (D-CU-02-7: `amount` is lines).
DWORD wheel_delta(int lines, bool horizontal) {
  UINT per_notch = 0;
  SystemParametersInfoW(horizontal ? SPI_GETWHEELSCROLLCHARS : SPI_GETWHEELSCROLLLINES, 0,
                        &per_notch, 0);
  long long delta = static_cast<long long>(lines) * WHEEL_DELTA;
  if (per_notch != 0 && per_notch != WHEEL_PAGESCROLL) delta /= per_notch;
  if (delta < 1) delta = 1;
  if (delta > 0x7FFFFFFF) delta = 0x7FFFFFFF;
  return static_cast<DWORD>(delta);
}

}  // namespace

bool Win32Injector::supports(const Key& key) { return map_key(key).has_value(); }

std::size_t Win32Injector::send(const std::vector<InputEvent>& events) {
  // Each step can become several INPUTs; remember which step each INPUT belongs to, so a partial
  // insertion reports the steps that fully arrived.
  std::vector<INPUT> inputs;
  std::vector<std::size_t> step_end;
  PixelRect desk = virtual_desktop();
  for (const InputEvent& e : events) {
    switch (e.type) {
      case InputEvent::Type::KeyDown: append_key(inputs, e.key, false); break;
      case InputEvent::Type::KeyUp: append_key(inputs, e.key, true); break;
      case InputEvent::Type::ButtonDown:
      case InputEvent::Type::ButtonUp: {
        bool up = e.type == InputEvent::Type::ButtonUp;
        DWORD flag = e.button == MouseButton::Right    ? (up ? MOUSEEVENTF_RIGHTUP : MOUSEEVENTF_RIGHTDOWN)
                     : e.button == MouseButton::Middle ? (up ? MOUSEEVENTF_MIDDLEUP : MOUSEEVENTF_MIDDLEDOWN)
                                                       : (up ? MOUSEEVENTF_LEFTUP : MOUSEEVENTF_LEFTDOWN);
        inputs.push_back(mouse_input(flag));
        break;
      }
      case InputEvent::Type::Move: {
        AbsolutePoint a = to_absolute(e.point, desk);
        inputs.push_back(mouse_input(MOUSEEVENTF_MOVE | MOUSEEVENTF_ABSOLUTE | MOUSEEVENTF_VIRTUALDESK,
                                     a.x, a.y));
        break;
      }
      case InputEvent::Type::Scroll: {
        bool horizontal = e.direction == ScrollDirection::Left || e.direction == ScrollDirection::Right;
        DWORD delta = wheel_delta(e.amount, horizontal);
        // Positive is away from the user (up) and to the right.
        bool negative = e.direction == ScrollDirection::Down || e.direction == ScrollDirection::Left;
        DWORD data = negative ? static_cast<DWORD>(-static_cast<LONG>(delta)) : delta;
        inputs.push_back(mouse_input(horizontal ? MOUSEEVENTF_HWHEEL : MOUSEEVENTF_WHEEL, 0, 0, data));
        break;
      }
      case InputEvent::Type::UnicodeDown:
      case InputEvent::Type::UnicodeUp: {
        INPUT in{};
        in.type = INPUT_KEYBOARD;
        in.ki.wVk = 0;
        in.ki.wScan = static_cast<WORD>(e.unit);
        in.ki.dwFlags = KEYEVENTF_UNICODE |
                        (e.type == InputEvent::Type::UnicodeUp ? KEYEVENTF_KEYUP : 0);
        in.ki.dwExtraInfo = kInjectedMarker;
        inputs.push_back(in);
        break;
      }
    }
    step_end.push_back(inputs.size());
  }
  if (inputs.empty()) return events.size();
  UINT sent = SendInput(static_cast<UINT>(inputs.size()), inputs.data(), sizeof(INPUT));
  // UIPI or the secure desktop block input silently or by a short count; both are os_refused.
  std::size_t steps = 0;
  while (steps < step_end.size() && step_end[steps] <= sent) ++steps;
  return steps;
}

void Win32Injector::release_all() {
  // Every key or button that is down gets an up, whoever pressed it: a crashed helper or a refused
  // key up must not leave a modifier stuck for the person.
  std::vector<INPUT> ups;
  for (int vk = 1; vk < 256; ++vk) {
    if ((GetAsyncKeyState(vk) & 0x8000) == 0) continue;
    switch (vk) {
      case VK_LBUTTON: ups.push_back(mouse_input(MOUSEEVENTF_LEFTUP)); break;
      case VK_RBUTTON: ups.push_back(mouse_input(MOUSEEVENTF_RIGHTUP)); break;
      case VK_MBUTTON: ups.push_back(mouse_input(MOUSEEVENTF_MIDDLEUP)); break;
      case VK_XBUTTON1: ups.push_back(mouse_input(MOUSEEVENTF_XUP, 0, 0, XBUTTON1)); break;
      case VK_XBUTTON2: ups.push_back(mouse_input(MOUSEEVENTF_XUP, 0, 0, XBUTTON2)); break;
      case VK_CANCEL: break;  // Ctrl+Break, not a key of its own
      default: {
        bool extended = vk == VK_RCONTROL || vk == VK_RMENU || vk == VK_LWIN || vk == VK_RWIN ||
                        vk == VK_INSERT || vk == VK_DELETE || vk == VK_HOME || vk == VK_END ||
                        vk == VK_PRIOR || vk == VK_NEXT || (vk >= VK_LEFT && vk <= VK_DOWN);
        ups.push_back(key_input(static_cast<WORD>(vk), extended, true));
      }
    }
  }
  if (!ups.empty()) SendInput(static_cast<UINT>(ups.size()), ups.data(), sizeof(INPUT));
}

std::optional<Point> Win32Injector::cursor() {
  POINT p{};
  if (!GetCursorPos(&p)) return std::nullopt;
  return Point{static_cast<double>(p.x), static_cast<double>(p.y)};
}

}  // namespace bothy::helper::win32
