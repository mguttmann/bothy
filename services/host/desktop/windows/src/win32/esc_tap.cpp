// SPDX-License-Identifier: Apache-2.0

// The Esc tap (27 8.4 item 4): a low-level keyboard hook on its own thread. The hook is installed
// only while a session is shown, so the helper watches no keystroke outside a session.

#include "win32.hpp"

namespace bothy::helper::win32 {

namespace {

constexpr UINT kInstall = WM_APP + 10;
constexpr UINT kRemove = WM_APP + 11;
Win32EscTap* g_tap = nullptr;  // a low-level hook procedure has no context pointer

}  // namespace

Win32EscTap::Win32EscTap(HWND window, Interrupt* interrupt) : window_(window), interrupt_(interrupt) {
  g_tap = this;
  thread_ = std::thread([this] { run(); });
  // Wait until the thread has a message queue, or the first PostThreadMessage would be lost.
  while (thread_id_.load() == 0) Sleep(0);
}

Win32EscTap::~Win32EscTap() {
  PostThreadMessageW(thread_id_.load(), WM_QUIT, 0, 0);
  if (thread_.joinable()) thread_.join();
  g_tap = nullptr;
}

void Win32EscTap::start() {
  active_ = true;
  PostThreadMessageW(thread_id_.load(), kInstall, 0, 0);
}

void Win32EscTap::stop() {
  active_ = false;
  PostThreadMessageW(thread_id_.load(), kRemove, 0, 0);
}

void Win32EscTap::run() {
  MSG msg;
  PeekMessageW(&msg, nullptr, WM_USER, WM_USER, PM_NOREMOVE);  // creates the queue
  thread_id_ = GetCurrentThreadId();
  while (GetMessageW(&msg, nullptr, 0, 0) > 0) {
    if (msg.message == kInstall && hook_ == nullptr) {
      hook_ = SetWindowsHookExW(WH_KEYBOARD_LL, hook_proc, GetModuleHandleW(nullptr), 0);
    } else if (msg.message == kRemove && hook_ != nullptr && !active_) {
      UnhookWindowsHookEx(hook_);
      hook_ = nullptr;
    }
  }
  if (hook_ != nullptr) UnhookWindowsHookEx(hook_);
  hook_ = nullptr;
}

LRESULT CALLBACK Win32EscTap::hook_proc(int code, WPARAM wp, LPARAM lp) {
  if (code == HC_ACTION && g_tap != nullptr) {
    const auto* k = reinterpret_cast<const KBDLLHOOKSTRUCT*>(lp);
    TapEvent e;
    e.is_escape = k->vkCode == VK_ESCAPE;
    e.down = wp == WM_KEYDOWN || wp == WM_SYSKEYDOWN;
    e.injected = (k->flags & LLKHF_INJECTED) != 0;
    e.extra_info = static_cast<std::uintptr_t>(k->dwExtraInfo);
    TapDecision d = g_tap->filter_.decide(e, g_tap->active_.load());
    if (d == TapDecision::SwallowAndStop) {
      // The hook only raises the flag; the UI thread ends the session, so the hook returns at once.
      g_tap->active_ = false;
      if (g_tap->interrupt_ != nullptr) g_tap->interrupt_->raise_escape();
      PostMessageW(g_tap->window_, kMsgEscape, 0, 0);
    }
    if (d != TapDecision::Pass) return 1;
  }
  return CallNextHookEx(nullptr, code, wp, lp);
}

}  // namespace bothy::helper::win32
