// SPDX-License-Identifier: Apache-2.0

// Lock and secure desktop (27 8.2 and 9.2). The state is computed fresh in current(); the
// notifications only make the Controller ask at once instead of at the next action.

#include "win32.hpp"  // first: windows.h must precede the other Windows headers

#include <wtsapi32.h>

namespace bothy::helper::win32 {

namespace {

Win32LockMonitor* g_monitor = nullptr;  // the WinEvent callback has no context pointer

void CALLBACK desktop_switched(HWINEVENTHOOK, DWORD, HWND, LONG, LONG, DWORD, DWORD) {
  if (g_monitor != nullptr) {
    LRESULT ignored = 0;
    g_monitor->handle(WM_NULL, 0, 0, ignored);
  }
}

bool session_locked() {
  DWORD session = 0;
  if (!ProcessIdToSessionId(GetCurrentProcessId(), &session)) return false;
  // Another session at the console (a user switch) counts as locked for this one.
  if (WTSGetActiveConsoleSessionId() != session) return true;
  LPWSTR buffer = nullptr;
  DWORD bytes = 0;
  bool locked = false;
  if (WTSQuerySessionInformationW(WTS_CURRENT_SERVER_HANDLE, session, WTSSessionInfoEx, &buffer, &bytes) &&
      buffer != nullptr && bytes >= sizeof(WTSINFOEXW)) {
    auto* info = reinterpret_cast<WTSINFOEXW*>(buffer);
    if (info->Level == 1) locked = info->Data.WTSInfoExLevel1.SessionFlags == WTS_SESSIONSTATE_LOCK;
  }
  if (buffer != nullptr) WTSFreeMemory(buffer);
  return locked;
}

bool screen_saver_running() {
  BOOL running = FALSE;
  SystemParametersInfoW(SPI_GETSCREENSAVERRUNNING, 0, &running, 0);
  return running != FALSE;
}

// The input desktop is not the user's `Default` desktop: the UAC prompt, the secure attention
// screen or the logon desktop has the input, and this session's input cannot reach it.
bool foreign_input_desktop() {
  HDESK desk = OpenInputDesktop(0, FALSE, DESKTOP_READOBJECTS);
  if (desk == nullptr) return true;
  wchar_t name[256] = {};
  DWORD needed = 0;
  bool other = !GetUserObjectInformationW(desk, UOI_NAME, name, sizeof name, &needed) ||
               lstrcmpiW(name, L"Default") != 0;
  CloseDesktop(desk);
  return other;
}

}  // namespace

Win32LockMonitor::Win32LockMonitor(HWND window, std::function<void()> on_change)
    : window_(window), on_change_(std::move(on_change)) {
  registered_ = WTSRegisterSessionNotification(window_, NOTIFY_FOR_THIS_SESSION) != FALSE;
  power_ = RegisterPowerSettingNotification(window_, &GUID_CONSOLE_DISPLAY_STATE,
                                            DEVICE_NOTIFY_WINDOW_HANDLE);
  g_monitor = this;
  desktop_hook_ = SetWinEventHook(EVENT_SYSTEM_DESKTOPSWITCH, EVENT_SYSTEM_DESKTOPSWITCH, nullptr,
                                  desktop_switched, 0, 0, WINEVENT_OUTOFCONTEXT);
}

Win32LockMonitor::~Win32LockMonitor() {
  if (desktop_hook_ != nullptr) UnhookWinEvent(desktop_hook_);
  g_monitor = nullptr;
  if (power_ != nullptr) UnregisterPowerSettingNotification(power_);
  if (registered_) WTSUnRegisterSessionNotification(window_);
}

LockState Win32LockMonitor::current() {
  LockState s;
  s.locked = display_off_ || session_locked() || screen_saver_running();
  // While locked the logon desktop has the input too; the lock is the state that explains it.
  s.secure_desktop = !s.locked && foreign_input_desktop();
  return s;
}

bool Win32LockMonitor::handle(UINT msg, WPARAM wp, LPARAM lp, LRESULT& result) {
  if (msg == WM_NULL) {  // from the desktop switch hook
    on_change_();
    return true;
  }
  if (msg == WM_WTSSESSION_CHANGE) {
    on_change_();
    result = 0;
    return true;
  }
  if (msg == WM_POWERBROADCAST && wp == PBT_POWERSETTINGCHANGE) {
    auto* setting = reinterpret_cast<POWERBROADCAST_SETTING*>(lp);
    if (setting != nullptr && IsEqualGUID(setting->PowerSetting, GUID_CONSOLE_DISPLAY_STATE) &&
        setting->DataLength >= sizeof(DWORD)) {
      DWORD state = *reinterpret_cast<const DWORD*>(setting->Data);
      display_off_ = state == 0;  // 0 off, 1 on, 2 dimmed (dimmed still shows the screen)
      on_change_();
    }
    result = TRUE;
    return true;
  }
  return false;
}

}  // namespace bothy::helper::win32
