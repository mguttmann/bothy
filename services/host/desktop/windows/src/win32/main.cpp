// SPDX-License-Identifier: Apache-2.0

// bothy-computer-helper.exe: wires the Win32 implementations to the Controller and runs the UI
// thread's message loop. `--version` prints HELPER_VERSION and exits. `--settings` (the Start menu
// shortcut "Computer Helper") opens the settings window: in the running helper of this session when
// there is one, else in this one once it runs. A start without it never shows a window, so the
// login and the installer start the helper silently (decisions.d/FIX-CU-HELPER-NO-ICON.md).

#include "win32.hpp"  // first: windows.h must precede the other Windows headers

#include <objbase.h>
#include <shellapi.h>

#include <memory>
#include <string>

#include "bothy/helper/controller.hpp"

using namespace bothy::helper;
using namespace bothy::helper::win32;

namespace {

constexpr wchar_t kWindowClass[] = L"bothy-computer-helper-main";
constexpr wchar_t kDefaultPipe[] = L"\\\\.\\pipe\\bothy-host-computer";
constexpr wchar_t kChannelVariable[] = L"BOTHY_COMPUTER_HELPER_CHANNEL";

// Everything the helper owns; created on the UI thread after the main window exists.
struct App {
  App(HINSTANCE instance, HWND window, const std::wstring& pipe, bool check_server, HelperInfo info)
      : lock(window, [this] { controller.on_lock_changed(); }),
        esc_tap(window, &interrupt),
        indicator(instance, window, &interrupt, [this] { controller.on_stop_button(); },
                  [this] { controller.on_switch_clicked(); }, about(info)),
        channel(window, pipe, check_server, info.frame_max_bytes, &interrupt),
        controller(Platform{capturer, injector, apps, lock, waker, esc_tap, indicator, store, channel},
                   info, &interrupt) {}

  Interrupt interrupt;
  Win32Capturer capturer;
  Win32Injector injector;
  Win32AppInspector apps;
  Win32LockMonitor lock;
  Win32Waker waker;
  Win32EscTap esc_tap;
  Win32Indicator indicator;
  FileEnablementStore store;
  PipeChannel channel;
  Controller controller;
  std::uint64_t connection = 0;  // the connection whose frames the Controller takes

  /// @return the version line of the settings window.
  static std::wstring about(const HelperInfo& info) {
    return L"Version " + to_wide(info.helper_version) +
           (info.signature == "verified" ? L", signed" : L", development build");
  }

  void drain_channel() {
    for (PipeChannel::Event& e : channel.drain()) {
      switch (e.kind) {
        case PipeChannel::Event::Kind::Connected:
          connection = e.connection;
          controller.on_connected();
          break;
        case PipeChannel::Event::Kind::Frame:
          // Frames of an earlier connection arrive after its break; they belong to nobody now.
          if (e.connection == connection && !controller.on_frame(e.frame)) {
            channel.close();
            connection = 0;  // the rest of this connection's frames are dropped
          }
          break;
        case PipeChannel::Event::Kind::Broken:
          if (e.connection == connection) {
            connection = 0;
            controller.on_channel_broken();
          }
          break;
      }
    }
  }
};

LRESULT CALLBACK window_proc(HWND hwnd, UINT msg, WPARAM wp, LPARAM lp) {
  auto* app = reinterpret_cast<App*>(GetWindowLongPtrW(hwnd, GWLP_USERDATA));
  if (app != nullptr) {
    LRESULT result = 0;
    // The flag tells whether the stop is still open: a running action may have handled it already,
    // and a later session must not end for an earlier Esc or click.
    if (msg == kMsgEscape) {
      if (app->interrupt.escape_pending()) app->controller.on_escape();
      return 0;
    }
    if (msg == kMsgStop) {
      if (app->interrupt.peek() == Interrupt::Kind::StopButton) app->controller.on_stop_button();
      return 0;
    }
    if (msg == kMsgChannel) {
      app->drain_channel();
      return 0;
    }
    if (app->indicator.handle(msg, wp, lp, result)) return result;
    if (app->lock.handle(msg, wp, lp, result)) return result;
    if (msg == WM_ENDSESSION && wp) {
      // Logoff or shutdown: nothing the helper pressed may outlive it.
      app->controller.on_channel_broken();
      return 0;
    }
  }
  if (msg == WM_DESTROY) {
    PostQuitMessage(0);
    return 0;
  }
  return DefWindowProcW(hwnd, msg, wp, lp);
}

// Writes to the console of the starting process; a GUI program has none of its own.
void print_line(const std::string& text) {
  HANDLE out = GetStdHandle(STD_OUTPUT_HANDLE);
  bool attached = false;
  if (out == nullptr || out == INVALID_HANDLE_VALUE) {
    if (AttachConsole(ATTACH_PARENT_PROCESS)) {
      attached = true;
      out = CreateFileW(L"CONOUT$", GENERIC_WRITE, FILE_SHARE_WRITE, nullptr, OPEN_EXISTING, 0, nullptr);
    }
  }
  if (out != nullptr && out != INVALID_HANDLE_VALUE) {
    std::string line = text + "\r\n";
    DWORD written = 0;
    WriteFile(out, line.data(), static_cast<DWORD>(line.size()), &written, nullptr);
    if (attached) CloseHandle(out);
  }
}

std::wstring environment(const wchar_t* name) {
  DWORD n = GetEnvironmentVariableW(name, nullptr, 0);
  if (n == 0) return {};
  std::wstring value(n, L'\0');
  n = GetEnvironmentVariableW(name, value.data(), n);
  value.resize(n);
  return value;
}

}  // namespace

int WINAPI wWinMain(HINSTANCE instance, HINSTANCE, PWSTR, int) {
  int argc = 0;
  LPWSTR* argv = CommandLineToArgvW(GetCommandLineW(), &argc);
  std::wstring arg = argc > 1 && argv != nullptr ? argv[1] : L"";
  if (argv != nullptr) LocalFree(argv);
  if (argc == 2 && arg == L"--version") {
    print_line(HELPER_VERSION);
    return 0;
  }
  bool settings = argc == 2 && arg == L"--settings";
  if (argc > 1 && !settings) {
    print_line("usage: bothy-computer-helper [--version | --settings]");
    return 2;
  }

  SetProcessDpiAwarenessContext(DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2);  // the manifest also says so
  // One helper per desktop session: a second one would fight over the tray and the channel.
  HANDLE single = CreateMutexW(nullptr, TRUE, L"Local\\bothy-computer-helper");
  if (single == nullptr || GetLastError() == ERROR_ALREADY_EXISTS) {
    // The person started the helper again from the Start menu: the running one opens its settings
    // window. A second start without --settings (installer, login) stays silent.
    HWND running = settings ? FindWindowW(kWindowClass, nullptr) : nullptr;
    if (running != nullptr) {
      DWORD pid = 0;
      GetWindowThreadProcessId(running, &pid);
      if (pid != 0) AllowSetForegroundWindow(pid);
      PostMessageW(running, kMsgOpenSettings, 0, 0);
    }
    if (single != nullptr) CloseHandle(single);
    return 0;
  }
  if (FAILED(CoInitializeEx(nullptr, COINIT_APARTMENTTHREADED))) return 1;

  WNDCLASSEXW wc{};
  wc.cbSize = sizeof wc;
  wc.lpfnWndProc = window_proc;
  wc.hInstance = instance;
  wc.lpszClassName = kWindowClass;
  if (RegisterClassExW(&wc) == 0) return 1;
  // A hidden top-level window: it receives the session, power, tray and taskbar broadcasts, which
  // a message-only window would not.
  HWND window = CreateWindowExW(WS_EX_TOOLWINDOW, kWindowClass, L"bothy-computer-helper", WS_POPUP,
                                0, 0, 0, 0, nullptr, nullptr, instance, nullptr);
  if (window == nullptr) return 1;

  HelperInfo info;
  info.helper_version = HELPER_VERSION;
  info.os_account = current_account();
  info.signature = signature_verifies(own_executable_path()) ? "verified" : "development";
  info.frame_max_bytes = read_frame_max();

  // D-CU-02-5: only a development build takes another endpoint, and then skips the server check
  // (the CI jobs run a fake host on their own pipe); a verified build ignores the variable.
  std::wstring pipe = kDefaultPipe;
  bool check_server = true;
  if (info.signature == "development") {
    std::wstring override_pipe = environment(kChannelVariable);
    if (!override_pipe.empty()) {
      pipe = override_pipe;
      check_server = false;
    }
  }

  auto app = std::make_unique<App>(instance, window, pipe, check_server, info);
  SetWindowLongPtrW(window, GWLP_USERDATA, reinterpret_cast<LONG_PTR>(app.get()));
  app->controller.start();
  app->channel.start();
  // The settings window opens by itself only while the switch was never switched on, so the person
  // finds it after installing; otherwise the helper shows nothing until a session.
  if (settings || needs_setup(app->store.load())) app->indicator.open_settings();

  MSG msg;
  while (GetMessageW(&msg, nullptr, 0, 0) > 0) {
    TranslateMessage(&msg);
    DispatchMessageW(&msg);
  }
  app->controller.on_channel_broken();
  SetWindowLongPtrW(window, GWLP_USERDATA, 0);
  app.reset();
  CoUninitialize();
  CloseHandle(single);
  return 0;
}
