// SPDX-License-Identifier: Apache-2.0

// The indicator (27 9.2, brief item 16, decisions.d/FIX-CU-HELPER-NO-ICON.md): border, stop button,
// pointer ring, click pulse and tray icon while a session is shown, and the settings window with
// the switch. The border, the stop button, the ring and the pulse are excluded from capture, so the
// agent never sees them and the native CI jobs can check that no screenshot contains the indicator
// color (D-CU-02-9). Outside a session nothing of it shows.

#include "win32.hpp"  // first: windows.h must precede the other Windows headers

#include <shellapi.h>

#ifndef WDA_EXCLUDEFROMCAPTURE
#define WDA_EXCLUDEFROMCAPTURE 0x00000011
#endif

namespace bothy::helper::win32 {

namespace {

// D-CU-02-9: sRGB #FF6A00.
constexpr COLORREF kIndicatorColor = RGB(0xFF, 0x6A, 0x00);
// The glow around the pointer ring, a lighter shade of the indicator color.
constexpr COLORREF kGlowColor = RGB(0xFF, 0xB0, 0x7A);
// The transparent inside of the overlay windows; any color other than the ones drawn works.
constexpr COLORREF kColorKey = RGB(0x00, 0x00, 0x00);
constexpr UINT kTrayId = 1;
constexpr UINT_PTR kMenuStop = 2;
constexpr int kStopButtonId = 100;
constexpr int kSwitchId = 201;
constexpr int kCloseId = 202;
constexpr UINT_PTR kRingTimer = 1;
constexpr UINT_PTR kPulseTimer = 2;
constexpr UINT kFrameMs = 16;
constexpr ULONGLONG kPulseMs = 400;
constexpr wchar_t kBorderClass[] = L"bothy-computer-helper-border";
constexpr wchar_t kButtonClass[] = L"bothy-computer-helper-stop";
constexpr wchar_t kRingClass[] = L"bothy-computer-helper-ring";
constexpr wchar_t kPulseClass[] = L"bothy-computer-helper-pulse";
constexpr wchar_t kSettingsClass[] = L"bothy-computer-helper-settings";
constexpr wchar_t kSettingsTitle[] = L"Computer Helper";

// Sizes in pixels at 96 DPI, scaled to the monitor's DPI.
constexpr int kBorderWidth96 = 6;
constexpr int kButtonWidth96 = 220;
constexpr int kButtonHeight96 = 36;
constexpr int kRingSize96 = 40;
constexpr int kPulseSize96 = 72;
constexpr int kSettingsWidth96 = 440;
constexpr int kSettingsHeight96 = 230;

int scaled(int px96, UINT dpi) { return MulDiv(px96, static_cast<int>(dpi), 96); }

UINT dpi_of(HWND hwnd) {
  UINT dpi = hwnd != nullptr ? GetDpiForWindow(hwnd) : 0;
  return dpi != 0 ? dpi : 96;
}

// A layered, click-through, topmost overlay window that takes no focus, excluded from capture.
HWND overlay(HINSTANCE instance, const wchar_t* cls, void* self, const wchar_t* what) {
  HWND hwnd = CreateWindowExW(WS_EX_LAYERED | WS_EX_TRANSPARENT | WS_EX_TOPMOST | WS_EX_TOOLWINDOW |
                                  WS_EX_NOACTIVATE,
                              cls, L"", WS_POPUP, 0, 0, 0, 0, nullptr, nullptr, instance, self);
  if (hwnd != nullptr) {
    SetLayeredWindowAttributes(hwnd, kColorKey, 255, LWA_COLORKEY);
    if (!SetWindowDisplayAffinity(hwnd, WDA_EXCLUDEFROMCAPTURE)) OutputDebugStringW(what);
  }
  return hwnd;
}

// Fills the client area with the color key and draws a circle of `radius` around its center.
void paint_ring(HWND hwnd, int radius, int width, COLORREF color, int glow) {
  PAINTSTRUCT ps;
  HDC dc = BeginPaint(hwnd, &ps);
  RECT r;
  GetClientRect(hwnd, &r);
  HBRUSH key = CreateSolidBrush(kColorKey);
  FillRect(dc, &r, key);
  DeleteObject(key);
  int cx = (r.left + r.right) / 2;
  int cy = (r.top + r.bottom) / 2;
  HGDIOBJ old_brush = SelectObject(dc, GetStockObject(NULL_BRUSH));
  if (glow > 0) {
    HPEN halo = CreatePen(PS_SOLID, glow, kGlowColor);
    HGDIOBJ old = SelectObject(dc, halo);
    int g = radius + width / 2 + glow / 2;
    Ellipse(dc, cx - g, cy - g, cx + g, cy + g);
    SelectObject(dc, old);
    DeleteObject(halo);
  }
  HPEN pen = CreatePen(PS_SOLID, width, color);
  HGDIOBJ old_pen = SelectObject(dc, pen);
  Ellipse(dc, cx - radius, cy - radius, cx + radius, cy + radius);
  SelectObject(dc, old_pen);
  SelectObject(dc, old_brush);
  DeleteObject(pen);
  EndPaint(hwnd, &ps);
}

template <typename T>
T* self_of(HWND hwnd, UINT msg, LPARAM lp) {
  if (msg == WM_NCCREATE) {
    auto* cs = reinterpret_cast<CREATESTRUCTW*>(lp);
    SetWindowLongPtrW(hwnd, GWLP_USERDATA, reinterpret_cast<LONG_PTR>(cs->lpCreateParams));
  }
  return reinterpret_cast<T*>(GetWindowLongPtrW(hwnd, GWLP_USERDATA));
}

}  // namespace

Win32Indicator::Win32Indicator(HINSTANCE instance, HWND window, Interrupt* interrupt,
                               std::function<void()> on_stop, std::function<void(bool)> on_switch,
                               std::wstring about)
    : instance_(instance),
      window_(window),
      interrupt_(interrupt),
      on_stop_(std::move(on_stop)),
      on_switch_(std::move(on_switch)),
      about_(std::move(about)) {
  thread_ = std::thread([this] { run_windows(); });
  // The windows must exist before show() can place them.
  while (!ready_.load()) Sleep(0);
  taskbar_created_ = RegisterWindowMessageW(L"TaskbarCreated");
}

Win32Indicator::~Win32Indicator() {
  remove_tray();
  if (settings_ != nullptr) DestroyWindow(settings_);
  if (font_ != nullptr) DeleteObject(font_);
  // The overlay windows belong to the indicator thread and go with it.
  PostThreadMessageW(thread_id_.load(), WM_QUIT, 0, 0);
  if (thread_.joinable()) thread_.join();
}

void Win32Indicator::run_windows() {
  MSG msg;
  PeekMessageW(&msg, nullptr, WM_USER, WM_USER, PM_NOREMOVE);  // creates the queue
  thread_id_ = GetCurrentThreadId();
  WNDCLASSEXW wc{};
  wc.cbSize = sizeof wc;
  wc.lpfnWndProc = border_proc;
  wc.hInstance = instance_;
  wc.hCursor = LoadCursorW(nullptr, IDC_ARROW);
  wc.lpszClassName = kBorderClass;
  RegisterClassExW(&wc);
  wc.lpfnWndProc = ring_proc;
  wc.lpszClassName = kRingClass;
  RegisterClassExW(&wc);
  wc.lpfnWndProc = pulse_proc;
  wc.lpszClassName = kPulseClass;
  RegisterClassExW(&wc);
  wc.lpfnWndProc = button_proc;
  wc.hbrBackground = CreateSolidBrush(kIndicatorColor);
  wc.lpszClassName = kButtonClass;
  RegisterClassExW(&wc);

  // Layered and transparent: the border, the ring and the pulse never take a click, so they hide
  // nothing from the agent or the person; their inside is the color key and shows the screen.
  border_ = overlay(instance_, kBorderClass, this,
                    L"bothy-computer-helper: the border cannot be excluded from capture\n");
  ring_ = overlay(instance_, kRingClass, this,
                  L"bothy-computer-helper: the pointer ring cannot be excluded from capture\n");
  pulse_ = overlay(instance_, kPulseClass, this,
                   L"bothy-computer-helper: the click pulse cannot be excluded from capture\n");
  button_ = CreateWindowExW(WS_EX_TOPMOST | WS_EX_TOOLWINDOW | WS_EX_NOACTIVATE, kButtonClass, L"",
                            WS_POPUP, 0, 0, 0, 0, nullptr, nullptr, instance_, this);
  if (button_ != nullptr) {
    CreateWindowExW(0, L"BUTTON", L"Stop the agent (Esc)", WS_CHILD | WS_VISIBLE | BS_PUSHBUTTON, 0, 0,
                    0, 0, button_, reinterpret_cast<HMENU>(static_cast<INT_PTR>(kStopButtonId)),
                    instance_, nullptr);
    if (!SetWindowDisplayAffinity(button_, WDA_EXCLUDEFROMCAPTURE))
      OutputDebugStringW(L"bothy-computer-helper: the stop button cannot be excluded from capture\n");
  }
  ready_ = true;
  while (GetMessageW(&msg, nullptr, 0, 0) > 0) {
    TranslateMessage(&msg);
    DispatchMessageW(&msg);
  }
}

void Win32Indicator::add_tray() {
  NOTIFYICONDATAW nid{};
  nid.cbSize = sizeof nid;
  nid.hWnd = window_;
  nid.uID = kTrayId;
  nid.uFlags = NIF_MESSAGE | NIF_ICON | NIF_TIP | NIF_SHOWTIP;
  nid.uCallbackMessage = kMsgTray;
  nid.hIcon = LoadIconW(nullptr, IDI_APPLICATION);
  std::wstring tip = L"Agent " + agent_ + L" controls this computer, Esc to stop";
  lstrcpynW(nid.szTip, tip.c_str(), static_cast<int>(sizeof nid.szTip / sizeof nid.szTip[0]));
  tray_added_ = Shell_NotifyIconW(NIM_ADD, &nid) != FALSE;
  nid.uVersion = NOTIFYICON_VERSION_4;
  Shell_NotifyIconW(NIM_SETVERSION, &nid);
}

void Win32Indicator::remove_tray() {
  if (!tray_added_) return;
  NOTIFYICONDATAW nid{};
  nid.cbSize = sizeof nid;
  nid.hWnd = window_;
  nid.uID = kTrayId;
  Shell_NotifyIconW(NIM_DELETE, &nid);
  tray_added_ = false;
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
  UINT dpi = dpi_of(border_);
  if (border_ != nullptr) {
    // Asynchronous: the windows belong to the indicator thread, and the UI thread must not wait.
    SetWindowPos(border_, HWND_TOPMOST, r.left, r.top, r.right - r.left, r.bottom - r.top,
                 SWP_NOACTIVATE | SWP_SHOWWINDOW | SWP_ASYNCWINDOWPOS);
    InvalidateRect(border_, nullptr, TRUE);
  }
  if (button_ != nullptr) {
    int w = scaled(kButtonWidth96, dpi);
    int h = scaled(kButtonHeight96, dpi);
    int b = scaled(kBorderWidth96, dpi);
    int x = r.left + (r.right - r.left - w) / 2;
    SetWindowPos(button_, HWND_TOPMOST, x, r.top + b, w, h,
                 SWP_NOACTIVATE | SWP_SHOWWINDOW | SWP_ASYNCWINDOWPOS);
    HWND child = GetDlgItem(button_, kStopButtonId);
    if (child != nullptr)
      SetWindowPos(child, nullptr, b / 2, b / 2, w - b, h - b,
                   SWP_NOZORDER | SWP_NOACTIVATE | SWP_ASYNCWINDOWPOS);
  }
}

void Win32Indicator::show(const std::string& agent_name) {
  agent_ = to_wide(agent_name);
  shown_ = true;
  place();
  if (ring_ != nullptr) PostMessageW(ring_, kMsgRing, 1, 0);
  remove_tray();
  add_tray();
}

void Win32Indicator::hide() {
  shown_ = false;
  agent_.clear();
  if (border_ != nullptr) ShowWindowAsync(border_, SW_HIDE);
  if (button_ != nullptr) ShowWindowAsync(button_, SW_HIDE);
  if (ring_ != nullptr) PostMessageW(ring_, kMsgRing, 0, 0);
  remove_tray();
}

void Win32Indicator::set_enabled(bool enabled) {
  enabled_ = enabled;
  if (settings_ != nullptr)
    SendDlgItemMessageW(settings_, kSwitchId, BM_SETCHECK, enabled ? BST_CHECKED : BST_UNCHECKED, 0);
}

void Win32Indicator::mark_click(const Point& at) {
  if (!shown_.load() || pulse_ == nullptr) return;
  // Screen space is physical pixels (27 9.4), so the point fits a LONG.
  auto x = static_cast<LONG_PTR>(at.x);
  auto y = static_cast<LONG_PTR>(at.y);
  PostMessageW(pulse_, kMsgPulse, static_cast<WPARAM>(x), static_cast<LPARAM>(y));
}

void Win32Indicator::follow_pointer() {
  POINT at{};
  if (ring_ == nullptr || !GetCursorPos(&at)) return;
  int s = scaled(kRingSize96, dpi_of(ring_));
  SetWindowPos(ring_, HWND_TOPMOST, at.x - s / 2, at.y - s / 2, s, s,
               SWP_NOACTIVATE | SWP_SHOWWINDOW);
}

void Win32Indicator::show_menu() {
  HMENU menu = CreatePopupMenu();
  if (menu == nullptr) return;
  AppendMenuW(menu, MF_STRING | (shown_ ? MF_ENABLED : MF_GRAYED), kMenuStop, L"Stop the agent");
  POINT at{};
  GetCursorPos(&at);
  // Without the foreground the menu would not close when the person clicks elsewhere.
  SetForegroundWindow(window_);
  UINT cmd = static_cast<UINT>(TrackPopupMenu(menu, TPM_RETURNCMD | TPM_NONOTIFY | TPM_RIGHTBUTTON,
                                              at.x, at.y, 0, window_, nullptr));
  DestroyMenu(menu);
  if (cmd == kMenuStop && shown_) on_stop_();
}

HWND Win32Indicator::make_settings() {
  WNDCLASSEXW wc{};
  wc.cbSize = sizeof wc;
  wc.lpfnWndProc = settings_proc;
  wc.hInstance = instance_;
  wc.hCursor = LoadCursorW(nullptr, IDC_ARROW);
  wc.hbrBackground = reinterpret_cast<HBRUSH>(COLOR_BTNFACE + 1);
  wc.lpszClassName = kSettingsClass;
  RegisterClassExW(&wc);  // a second registration fails harmlessly
  HWND hwnd = CreateWindowExW(WS_EX_CONTROLPARENT, kSettingsClass, kSettingsTitle, WS_OVERLAPPED | WS_CAPTION | WS_SYSMENU,
                              CW_USEDEFAULT, CW_USEDEFAULT, 0, 0, nullptr, nullptr, instance_, this);
  if (hwnd == nullptr) return nullptr;
  UINT dpi = dpi_of(hwnd);
  RECT frame{0, 0, scaled(kSettingsWidth96, dpi), scaled(kSettingsHeight96, dpi)};
  AdjustWindowRectExForDpi(&frame, WS_OVERLAPPED | WS_CAPTION | WS_SYSMENU, FALSE, 0, dpi);
  int w = frame.right - frame.left;
  int h = frame.bottom - frame.top;
  RECT work{};
  SystemParametersInfoW(SPI_GETWORKAREA, 0, &work, 0);
  SetWindowPos(hwnd, nullptr, work.left + (work.right - work.left - w) / 2,
               work.top + (work.bottom - work.top - h) / 2, w, h, SWP_NOZORDER | SWP_NOACTIVATE);

  NONCLIENTMETRICSW ncm{};
  ncm.cbSize = sizeof ncm;
  if (font_ == nullptr && SystemParametersInfoForDpi(SPI_GETNONCLIENTMETRICS, sizeof ncm, &ncm, 0, dpi))
    font_ = CreateFontIndirectW(&ncm.lfMessageFont);
  int m = scaled(20, dpi);
  int line = scaled(24, dpi);
  int width = scaled(kSettingsWidth96, dpi) - 2 * m;
  struct Child {
    const wchar_t* cls;
    const wchar_t* text;
    DWORD style;
    int id;
    int lines;
  };
  std::wstring note =
      L"Shows nothing while no agent controls this computer. Start Computer Helper from the Start menu "
      L"to come back here.";
  const Child children[] = {
      {L"BUTTON", L"Allow computer control", BS_CHECKBOX | WS_TABSTOP, kSwitchId, 1},
      {L"STATIC", L"Windows asks for no permission for screen capture and input.", SS_LEFT, 0, 1},
      {L"STATIC", about_.c_str(), SS_LEFT, 0, 1},
      {L"STATIC", note.c_str(), SS_LEFT, 0, 2},
  };
  int y = m;
  for (const Child& c : children) {
    HWND child = CreateWindowExW(0, c.cls, c.text, WS_CHILD | WS_VISIBLE | c.style, m, y, width,
                                 line * c.lines, hwnd,
                                 reinterpret_cast<HMENU>(static_cast<INT_PTR>(c.id)), instance_, nullptr);
    if (child != nullptr && font_ != nullptr) SendMessageW(child, WM_SETFONT, reinterpret_cast<WPARAM>(font_), TRUE);
    y += line * c.lines + scaled(6, dpi);
  }
  int bw = scaled(90, dpi);
  HWND close = CreateWindowExW(0, L"BUTTON", L"Close", WS_CHILD | WS_VISIBLE | WS_TABSTOP | BS_DEFPUSHBUTTON,
                               m + width - bw, y, bw, scaled(28, dpi), hwnd,
                               reinterpret_cast<HMENU>(static_cast<INT_PTR>(kCloseId)), instance_, nullptr);
  if (close != nullptr && font_ != nullptr) SendMessageW(close, WM_SETFONT, reinterpret_cast<WPARAM>(font_), TRUE);
  return hwnd;
}

void Win32Indicator::open_settings() {
  if (settings_ == nullptr) settings_ = make_settings();
  if (settings_ == nullptr) return;
  set_enabled(enabled_);
  ShowWindow(settings_, SW_SHOWNORMAL);
  SetForegroundWindow(settings_);
  if (HWND box = GetDlgItem(settings_, kSwitchId)) SetFocus(box);
}

bool Win32Indicator::dialog_message(MSG* msg) {
  return settings_ != nullptr && IsWindowVisible(settings_) && IsDialogMessageW(settings_, msg) != FALSE;
}

bool Win32Indicator::handle(UINT msg, WPARAM wp, LPARAM lp, LRESULT& result) {
  (void)wp;
  if (msg == kMsgTray) {
    UINT event = LOWORD(lp);
    if (event == WM_CONTEXTMENU || event == NIN_SELECT || event == NIN_KEYSELECT) show_menu();
    result = 0;
    return true;
  }
  if (msg == kMsgOpenSettings) {
    open_settings();
    result = 0;
    return true;
  }
  if (taskbar_created_ != 0 && msg == taskbar_created_) {
    // Explorer restarted and lost the icon; it exists only during a session.
    tray_added_ = false;
    if (shown_) add_tray();
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
    int b = scaled(kBorderWidth96, dpi_of(hwnd));
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

LRESULT CALLBACK Win32Indicator::ring_proc(HWND hwnd, UINT msg, WPARAM wp, LPARAM lp) {
  auto* self = self_of<Win32Indicator>(hwnd, msg, lp);
  if (msg == kMsgRing) {
    // On the indicator thread, which owns the window and so its timer.
    if (wp != 0) {
      if (self != nullptr) self->follow_pointer();
      SetTimer(hwnd, kRingTimer, kFrameMs, nullptr);
    } else {
      KillTimer(hwnd, kRingTimer);
      ShowWindow(hwnd, SW_HIDE);
    }
    return 0;
  }
  if (msg == WM_TIMER && wp == kRingTimer) {
    if (self != nullptr) self->follow_pointer();
    return 0;
  }
  if (msg == WM_PAINT) {
    UINT dpi = dpi_of(hwnd);
    paint_ring(hwnd, scaled(11, dpi), scaled(3, dpi), kIndicatorColor, scaled(3, dpi));
    return 0;
  }
  if (msg == WM_NCHITTEST) return HTTRANSPARENT;
  return DefWindowProcW(hwnd, msg, wp, lp);
}

LRESULT CALLBACK Win32Indicator::pulse_proc(HWND hwnd, UINT msg, WPARAM wp, LPARAM lp) {
  auto* self = self_of<Win32Indicator>(hwnd, msg, lp);
  if (msg == kMsgPulse) {
    if (self == nullptr || !self->shown_.load()) return 0;
    auto x = static_cast<int>(static_cast<LONG_PTR>(wp));
    auto y = static_cast<int>(static_cast<LONG_PTR>(lp));
    int s = scaled(kPulseSize96, dpi_of(hwnd));
    self->pulse_start_ = GetTickCount64();
    SetWindowPos(hwnd, HWND_TOPMOST, x - s / 2, y - s / 2, s, s, SWP_NOACTIVATE | SWP_SHOWWINDOW);
    InvalidateRect(hwnd, nullptr, TRUE);
    SetTimer(hwnd, kPulseTimer, kFrameMs, nullptr);
    return 0;
  }
  if (msg == WM_TIMER && wp == kPulseTimer) {
    ULONGLONG elapsed = self != nullptr ? GetTickCount64() - self->pulse_start_ : kPulseMs;
    if (elapsed >= kPulseMs || self == nullptr || !self->shown_.load()) {
      KillTimer(hwnd, kPulseTimer);
      ShowWindow(hwnd, SW_HIDE);
    } else {
      InvalidateRect(hwnd, nullptr, TRUE);
    }
    return 0;
  }
  if (msg == WM_PAINT) {
    // The ring grows from a quarter to the full window while the pulse lasts.
    UINT dpi = dpi_of(hwnd);
    ULONGLONG elapsed = self != nullptr ? GetTickCount64() - self->pulse_start_ : kPulseMs;
    if (elapsed > kPulseMs) elapsed = kPulseMs;
    int full = scaled(kPulseSize96, dpi) / 2 - scaled(4, dpi);
    int radius = full / 4 + static_cast<int>((full - full / 4) * elapsed / kPulseMs);
    paint_ring(hwnd, radius, scaled(4, dpi), kIndicatorColor, 0);
    return 0;
  }
  if (msg == WM_NCHITTEST) return HTTRANSPARENT;
  return DefWindowProcW(hwnd, msg, wp, lp);
}

LRESULT CALLBACK Win32Indicator::settings_proc(HWND hwnd, UINT msg, WPARAM wp, LPARAM lp) {
  auto* self = self_of<Win32Indicator>(hwnd, msg, lp);
  if (msg == WM_COMMAND && HIWORD(wp) == BN_CLICKED) {
    // On the UI thread, which owns the window: the switch reaches the Controller directly, and
    // set_enabled() then shows the stored state.
    // GetMessageExtraInfo() is the dwExtraInfo of the mouse or key message that made the click: the
    // helper's marker means the agent clicked, which may switch off but never on (R-CU-03).
    if (LOWORD(wp) == kSwitchId && self != nullptr)
      self->on_switch_(static_cast<std::uintptr_t>(GetMessageExtraInfo()) == kInjectedMarker);
    if (LOWORD(wp) == kCloseId) ShowWindow(hwnd, SW_HIDE);
    return 0;
  }
  if (msg == WM_CLOSE) {
    ShowWindow(hwnd, SW_HIDE);  // kept for the next open
    return 0;
  }
  return DefWindowProcW(hwnd, msg, wp, lp);
}

LRESULT CALLBACK Win32Indicator::button_proc(HWND hwnd, UINT msg, WPARAM wp, LPARAM lp) {
  auto* self = self_of<Win32Indicator>(hwnd, msg, lp);
  if (msg == WM_MOUSEACTIVATE) return MA_NOACTIVATE;  // a click must not steal the agent's focus
  if (msg == WM_COMMAND && LOWORD(wp) == kStopButtonId && HIWORD(wp) == BN_CLICKED) {
    if (self != nullptr && self->shown_) {
      // Runs on the indicator thread: raise the interrupt for a running action, then let the UI
      // thread end the session (it checks the flag, so a stop it already handled is not repeated).
      if (self->interrupt_ != nullptr) self->interrupt_->raise(Interrupt::Kind::StopButton);
      PostMessageW(self->window_, kMsgStop, 0, 0);
    }
    return 0;
  }
  return DefWindowProcW(hwnd, msg, wp, lp);
}

}  // namespace bothy::helper::win32
