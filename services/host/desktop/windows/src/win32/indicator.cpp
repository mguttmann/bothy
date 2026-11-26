// SPDX-License-Identifier: Apache-2.0

// The indicator (27 9.2, brief item 16): border, stop button and tray icon. The border and the stop
// button are excluded from capture, so the agent never sees them and the native CI jobs can check
// that no screenshot contains the indicator color (D-CU-02-9).

#include "win32.hpp"  // first: windows.h must precede the other Windows headers

#include <shellapi.h>


#ifndef WDA_EXCLUDEFROMCAPTURE
#define WDA_EXCLUDEFROMCAPTURE 0x00000011
#endif

namespace bothy::helper::win32 {

namespace {

// D-CU-02-9: sRGB #FF6A00.
constexpr COLORREF kIndicatorColor = RGB(0xFF, 0x6A, 0x00);
// The transparent inside of the border window; any color other than the indicator color works.
constexpr COLORREF kColorKey = RGB(0x00, 0x00, 0x00);
constexpr UINT kTrayId = 1;
constexpr UINT_PTR kMenuSwitch = 1;
constexpr UINT_PTR kMenuStop = 2;
constexpr int kStopButtonId = 100;
constexpr wchar_t kBorderClass[] = L"bothy-computer-helper-border";
constexpr wchar_t kButtonClass[] = L"bothy-computer-helper-stop";
constexpr wchar_t kIdleTip[] = L"Computer helper";

// Sizes in pixels at 96 DPI, scaled to the monitor's DPI.
constexpr int kBorderWidth96 = 6;
constexpr int kButtonWidth96 = 220;
constexpr int kButtonHeight96 = 36;

int scaled(int px96, UINT dpi) { return MulDiv(px96, static_cast<int>(dpi), 96); }

}  // namespace

Win32Indicator::Win32Indicator(HINSTANCE instance, HWND window, std::function<void()> on_stop,
                               std::function<void()> on_switch)
    : instance_(instance), window_(window), on_stop_(std::move(on_stop)), on_switch_(std::move(on_switch)) {
  WNDCLASSEXW wc{};
  wc.cbSize = sizeof wc;
  wc.lpfnWndProc = border_proc;
  wc.hInstance = instance_;
  wc.hCursor = LoadCursorW(nullptr, IDC_ARROW);
  wc.lpszClassName = kBorderClass;
  RegisterClassExW(&wc);
  wc.lpfnWndProc = button_proc;
  wc.hbrBackground = CreateSolidBrush(kIndicatorColor);
  wc.lpszClassName = kButtonClass;
  RegisterClassExW(&wc);

  // Layered and transparent: the border never takes a click, so it hides nothing from the agent
  // or the person; the inside is the color key and shows the screen.
  border_ = CreateWindowExW(WS_EX_LAYERED | WS_EX_TRANSPARENT | WS_EX_TOPMOST | WS_EX_TOOLWINDOW |
                                WS_EX_NOACTIVATE,
                            kBorderClass, L"", WS_POPUP, 0, 0, 0, 0, nullptr, nullptr, instance_, this);
  if (border_ != nullptr) {
    SetLayeredWindowAttributes(border_, kColorKey, 255, LWA_COLORKEY);
    SetWindowDisplayAffinity(border_, WDA_EXCLUDEFROMCAPTURE);
  }
  button_ = CreateWindowExW(WS_EX_TOPMOST | WS_EX_TOOLWINDOW | WS_EX_NOACTIVATE, kButtonClass, L"",
                            WS_POPUP, 0, 0, 0, 0, nullptr, nullptr, instance_, this);
  if (button_ != nullptr) {
    CreateWindowExW(0, L"BUTTON", L"Stop the agent (Esc)", WS_CHILD | WS_VISIBLE | BS_PUSHBUTTON, 0, 0,
                    0, 0, button_, reinterpret_cast<HMENU>(static_cast<INT_PTR>(kStopButtonId)),
                    instance_, nullptr);
    SetWindowDisplayAffinity(button_, WDA_EXCLUDEFROMCAPTURE);
  }
  taskbar_created_ = RegisterWindowMessageW(L"TaskbarCreated");
  add_tray();
}

Win32Indicator::~Win32Indicator() {
  NOTIFYICONDATAW nid{};
  nid.cbSize = sizeof nid;
  nid.hWnd = window_;
  nid.uID = kTrayId;
  Shell_NotifyIconW(NIM_DELETE, &nid);
  if (button_ != nullptr) DestroyWindow(button_);
  if (border_ != nullptr) DestroyWindow(border_);
}

void Win32Indicator::add_tray() {
  NOTIFYICONDATAW nid{};
  nid.cbSize = sizeof nid;
  nid.hWnd = window_;
  nid.uID = kTrayId;
  nid.uFlags = NIF_MESSAGE | NIF_ICON | NIF_TIP | NIF_SHOWTIP;
  nid.uCallbackMessage = kMsgTray;
  nid.hIcon = LoadIconW(nullptr, IDI_APPLICATION);
  lstrcpynW(nid.szTip, kIdleTip, static_cast<int>(sizeof nid.szTip / sizeof nid.szTip[0]));
  Shell_NotifyIconW(NIM_ADD, &nid);
  nid.uVersion = NOTIFYICON_VERSION_4;
  Shell_NotifyIconW(NIM_SETVERSION, &nid);
  update_tray_text();
}

void Win32Indicator::update_tray_text() {
  NOTIFYICONDATAW nid{};
  nid.cbSize = sizeof nid;
  nid.hWnd = window_;
  nid.uID = kTrayId;
  nid.uFlags = NIF_TIP | NIF_SHOWTIP;
  std::wstring tip = shown_ ? L"Agent " + agent_ + L" controls this computer, Esc to stop" : kIdleTip;
  lstrcpynW(nid.szTip, tip.c_str(), static_cast<int>(sizeof nid.szTip / sizeof nid.szTip[0]));
  Shell_NotifyIconW(NIM_MODIFY, &nid);
}

void Win32Indicator::place() {
  // The screen the agent works on: the one with the foreground window (27 6.5 item 1).
  HWND fg = GetForegroundWindow();
  HMONITOR monitor = fg != nullptr ? MonitorFromWindow(fg, MONITOR_DEFAULTTOPRIMARY)
                                   : MonitorFromPoint(POINT{0, 0}, MONITOR_DEFAULTTOPRIMARY);
  MONITORINFO mi{};
  mi.cbSize = sizeof mi;
  if (!GetMonitorInfoW(monitor, &mi)) return;
  const RECT& r = mi.rcMonitor;
  UINT dpi = border_ != nullptr ? GetDpiForWindow(border_) : 96;
  if (dpi == 0) dpi = 96;
  if (border_ != nullptr) {
    SetWindowPos(border_, HWND_TOPMOST, r.left, r.top, r.right - r.left, r.bottom - r.top,
                 SWP_NOACTIVATE | SWP_SHOWWINDOW);
    InvalidateRect(border_, nullptr, TRUE);
  }
  if (button_ != nullptr) {
    int w = scaled(kButtonWidth96, dpi);
    int h = scaled(kButtonHeight96, dpi);
    int b = scaled(kBorderWidth96, dpi);
    int x = r.left + (r.right - r.left - w) / 2;
    SetWindowPos(button_, HWND_TOPMOST, x, r.top + b, w, h, SWP_NOACTIVATE | SWP_SHOWWINDOW);
    HWND child = GetDlgItem(button_, kStopButtonId);
    if (child != nullptr) SetWindowPos(child, nullptr, b / 2, b / 2, w - b, h - b, SWP_NOZORDER | SWP_NOACTIVATE);
  }
}

void Win32Indicator::show(const std::string& agent_name) {
  agent_ = to_wide(agent_name);
  shown_ = true;
  place();
  update_tray_text();
}

void Win32Indicator::hide() {
  shown_ = false;
  agent_.clear();
  if (border_ != nullptr) ShowWindow(border_, SW_HIDE);
  if (button_ != nullptr) ShowWindow(button_, SW_HIDE);
  update_tray_text();
}

void Win32Indicator::set_enabled(bool enabled) { enabled_ = enabled; }

void Win32Indicator::show_menu() {
  HMENU menu = CreatePopupMenu();
  if (menu == nullptr) return;
  AppendMenuW(menu, MF_STRING | (enabled_ ? MF_CHECKED : MF_UNCHECKED), kMenuSwitch,
              L"Allow computer control");
  AppendMenuW(menu, MF_STRING | (shown_ ? MF_ENABLED : MF_GRAYED), kMenuStop, L"Stop the agent");
  POINT at{};
  GetCursorPos(&at);
  // Without the foreground the menu would not close when the person clicks elsewhere.
  SetForegroundWindow(window_);
  UINT cmd = static_cast<UINT>(TrackPopupMenu(menu, TPM_RETURNCMD | TPM_NONOTIFY | TPM_RIGHTBUTTON,
                                              at.x, at.y, 0, window_, nullptr));
  DestroyMenu(menu);
  if (cmd == kMenuSwitch) on_switch_();
  if (cmd == kMenuStop && shown_) on_stop_();
}

bool Win32Indicator::handle(UINT msg, WPARAM wp, LPARAM lp, LRESULT& result) {
  (void)wp;
  if (msg == kMsgTray) {
    UINT event = LOWORD(lp);
    if (event == WM_CONTEXTMENU || event == NIN_SELECT || event == NIN_KEYSELECT) show_menu();
    result = 0;
    return true;
  }
  if (taskbar_created_ != 0 && msg == taskbar_created_) {
    add_tray();  // Explorer restarted and lost the icon
    result = 0;
    return true;
  }
  if (msg == WM_DISPLAYCHANGE || msg == WM_DPICHANGED) {
    if (shown_) place();
    return false;
  }
  return false;
}

LRESULT CALLBACK Win32Indicator::border_proc(HWND hwnd, UINT msg, WPARAM wp, LPARAM lp) {
  if (msg == WM_PAINT) {
    PAINTSTRUCT ps;
    HDC dc = BeginPaint(hwnd, &ps);
    RECT r;
    GetClientRect(hwnd, &r);
    HBRUSH key = CreateSolidBrush(kColorKey);
    FillRect(dc, &r, key);
    DeleteObject(key);
    int b = scaled(kBorderWidth96, GetDpiForWindow(hwnd) ? GetDpiForWindow(hwnd) : 96);
    HBRUSH color = CreateSolidBrush(kIndicatorColor);
    RECT edges[4] = {{r.left, r.top, r.right, r.top + b},
                     {r.left, r.bottom - b, r.right, r.bottom},
                     {r.left, r.top, r.left + b, r.bottom},
                     {r.right - b, r.top, r.right, r.bottom}};
    for (const RECT& e : edges) FillRect(dc, &e, color);
    DeleteObject(color);
    EndPaint(hwnd, &ps);
    return 0;
  }
  if (msg == WM_NCHITTEST) return HTTRANSPARENT;
  return DefWindowProcW(hwnd, msg, wp, lp);
}

LRESULT CALLBACK Win32Indicator::button_proc(HWND hwnd, UINT msg, WPARAM wp, LPARAM lp) {
  if (msg == WM_NCCREATE) {
    auto* cs = reinterpret_cast<CREATESTRUCTW*>(lp);
    SetWindowLongPtrW(hwnd, GWLP_USERDATA, reinterpret_cast<LONG_PTR>(cs->lpCreateParams));
  }
  if (msg == WM_MOUSEACTIVATE) return MA_NOACTIVATE;  // a click must not steal the agent's focus
  if (msg == WM_COMMAND && LOWORD(wp) == kStopButtonId && HIWORD(wp) == BN_CLICKED) {
    auto* self = reinterpret_cast<Win32Indicator*>(GetWindowLongPtrW(hwnd, GWLP_USERDATA));
    if (self != nullptr && self->shown_) self->on_stop_();
    return 0;
  }
  return DefWindowProcW(hwnd, msg, wp, lp);
}

}  // namespace bothy::helper::win32
