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
  std::uint64_t g = generations_.begin();
  started_generation_ = g;
  active_ = true;
  PostThreadMessageW(thread_id_.load(), kInstall, static_cast<WPARAM>(g), 0);
}

void Win32EscTap::stop() {
  active_ = false;
  PostThreadMessageW(thread_id_.load(), kRemove, static_cast<WPARAM>(started_generation_.load()), 0);
}

void Win32EscTap::run() {
  MSG msg;
  PeekMessageW(&msg, nullptr, WM_USER, WM_USER, PM_NOREMOVE);  // creates the queue
  thread_id_ = GetCurrentThreadId();
  while (GetMessageW(&msg, nullptr, 0, 0) > 0) {
    auto g = static_cast<std::uint64_t>(msg.wParam);
    if (msg.message == kInstall) {
      // A new session: its first real Esc must stop it, whatever an earlier one held (27 9.6 item 4).
      generations_.run_if_current(g, [this, g] {
        filter_.reset();
        remove_pending_ = 0;
        hook_generation_ = g;
        if (hook_ == nullptr) hook_ = SetWindowsHookExW(WH_KEYBOARD_LL, hook_proc, GetModuleHandleW(nullptr), 0);
      });
    } else if (msg.message == kRemove) {
      if (filter_.holding() && generations_.current(g)) {
        // The Esc that stopped is still down: keep the hook until its release, so its auto-repeat
        // and key up are swallowed and reach no app (27 9.6 item 4).
        remove_pending_ = g;
      } else {
        // Only the teardown of the current generation removes the hook; a late one of an older
        // tap finds a newer start and leaves the hook of the next session in place (item 5).
        generations_.end(g, [this] {
          if (hook_ != nullptr) UnhookWindowsHookEx(hook_);
          hook_ = nullptr;
          remove_pending_ = 0;
        });
      }
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
    // Only the tap of the current session decides and stops; a stale one passes the event and
    // leaves the filter alone (27 9.6 item 5). The hook raises the flag and returns at once; the UI
    // thread ends the session.
    Win32EscTap* tap = g_tap;
    TapDecision d = tap_decide(tap->generations_, tap->hook_generation_, tap->filter_, e,
                               tap->active_.load(), [tap] {
                                 tap->active_ = false;
                                 if (tap->interrupt_ != nullptr) tap->interrupt_->raise_escape();
                                 PostMessageW(tap->window_, kMsgEscape, 0, 0);
                               });
    if (g_tap->remove_pending_ != 0 && !g_tap->filter_.holding())
      PostThreadMessageW(g_tap->thread_id_.load(), kRemove, static_cast<WPARAM>(g_tap->remove_pending_), 0);
    if (d != TapDecision::Pass) return 1;
  }
  return CallNextHookEx(nullptr, code, wp, lp);
}

}  // namespace bothy::helper::win32
