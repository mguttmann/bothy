// SPDX-License-Identifier: Apache-2.0
#pragma once

/// @file
/// The Win32 implementations of the platform interfaces (docs/spec/27-computer-use.md section 9.2)
/// and the small Win32 services they share. Everything here runs on the helper's UI thread unless a
/// comment says otherwise; work on other threads (the keyboard hook, the pipe reader) reaches the
/// UI thread only through posted window messages.

#include <windows.h>

#include <atomic>
#include <cstdint>
#include <deque>
#include <functional>
#include <map>
#include <mutex>
#include <optional>
#include <string>
#include <thread>
#include <vector>

#include "bothy/helper/codec.hpp"
#include "bothy/helper/controller.hpp"
#include "bothy/helper/escape.hpp"
#include "bothy/helper/platform.hpp"

namespace bothy::helper::win32 {

/// Window messages the helper posts to its main window.
enum : UINT {
  kMsgEscape = WM_APP + 1,   ///< the Esc tap saw a real Esc (posted by the hook thread)
  kMsgChannel = WM_APP + 2,  ///< the pipe channel has events in its queue (posted by its thread)
  kMsgTray = WM_APP + 3,     ///< the tray icon's callback message
  kMsgStop = WM_APP + 4,     ///< the stop button was clicked (posted by the indicator thread)
  /// open the settings window; posted by a second start with `--settings` (Start menu shortcut)
  kMsgOpenSettings = WM_APP + 5,
  kMsgRing = WM_APP + 6,   ///< to the ring window: wParam 1 shows and follows the pointer, 0 hides
  kMsgPulse = WM_APP + 7,  ///< to the pulse window: a click at wParam (x), lParam (y)
};

/// Converts UTF-8 to UTF-16 for Win32; invalid input yields an empty string.
/// @param text UTF-8 text.
/// @return the wide string.
std::wstring to_wide(const std::string& text);

/// Converts UTF-16 from Win32 to UTF-8; a lone surrogate yields an empty string.
/// @param text the wide string.
/// @return UTF-8 text.
std::string to_utf8(const std::wstring& text);

/// @return the full path of the running executable, or an empty string.
std::wstring own_executable_path();

/// Checks the Authenticode signature of a file with WinVerifyTrust, without network access
/// (revocation is not checked: the helper must answer quickly and works offline).
/// @param path the file.
/// @return true when the embedded signature, or the signature of a system catalog that lists the
///         file, verifies and chains to a trusted root.
bool signature_verifies(const std::wstring& path);

/// Returns the subject (simple display name) of the certificate that signed a file, embedded or
/// through a system catalog, but only when signature_verifies() holds for it.
/// @param path the file.
/// @return the subject, or an empty string when the file is unsigned or does not verify.
std::string signer_subject(const std::wstring& path);

/// @return the operating system account the helper runs as (the plain user name, as `os_account`
///         of `hello`), or an empty string.
std::string current_account();

/// Reads `host_service.computer_frame_max_bytes` from the host service's configuration file
/// `%ProgramFiles%\bothy-host\config.yaml` (D-CU-02-8).
/// @return the value, or the default of 27 section 18 when the file is not readable.
std::uint64_t read_frame_max();

/// Capturer over DXGI Desktop Duplication with a GDI fallback, encoding with WIC in memory.
/// The fallback covers what Desktop Duplication cannot do: an output owned by another adapter
/// (hybrid graphics), a rotated output, a desktop format other than BGRA, and the moments after a
/// mode change when duplication is lost.
class Win32Capturer final : public Capturer {
 public:
  std::optional<ScreenInfo> screen() override;
  std::optional<std::vector<std::uint8_t>> capture(const CaptureRequest& request) override;
};

/// Encodes a BGRA pixel buffer (top-down rows) with WIC into memory: the requested part, scaled to
/// the output size, JPEG with the given quality or PNG. Never touches the disk.
/// @param pixels BGRA pixels, `width * 4` bytes per row.
/// @param width image width.
/// @param height image height.
/// @param request the part, output size, format and quality.
/// @return the encoded bytes, or std::nullopt when WIC refused.
std::optional<std::vector<std::uint8_t>> encode_image_wic(const std::vector<std::uint8_t>& pixels,
                                                          int width, int height,
                                                          const CaptureRequest& request);

/// Injector over SendInput: physical pixels with per-monitor DPI awareness v2, absolute
/// coordinates over the virtual desktop, `KEYEVENTF_UNICODE` for text, and kInjectedMarker in
/// `dwExtraInfo` of every event.
class Win32Injector final : public Injector {
 public:
  bool supports(const Key& key) override;
  std::size_t send(const std::vector<InputEvent>& events) override;
  void release_all() override;
  std::optional<Point> cursor() override;
};

/// AppInspector over the foreground window: process, executable name, version resource and
/// signer. The signer is cached per file path, size and write time, because verifying a large
/// executable on every action would delay each answer.
class Win32AppInspector final : public AppInspector {
 public:
  AppIdentity frontmost() override;

 private:
  struct Cached {
    std::uint64_t size = 0;
    std::uint64_t write_time = 0;
    AppIdentity identity;
  };
  std::map<std::wstring, Cached> cache_;
};

/// LockMonitor: session lock and console switches (WTSRegisterSessionNotification), display power
/// (GUID_CONSOLE_DISPLAY_STATE), the screen saver, and the input desktop (OpenInputDesktop; a
/// desktop other than `Default` is the secure desktop).
class Win32LockMonitor final : public LockMonitor {
 public:
  /// Registers for the notifications on `window`.
  /// @param window the helper's main window.
  /// @param on_change called on the UI thread when a notification arrives.
  Win32LockMonitor(HWND window, std::function<void()> on_change);
  ~Win32LockMonitor() override;
  Win32LockMonitor(const Win32LockMonitor&) = delete;
  Win32LockMonitor& operator=(const Win32LockMonitor&) = delete;

  LockState current() override;

  /// Lets the monitor see a message of the main window.
  /// @return true when the message was one of the monitor's; `result` is then the answer.
  bool handle(UINT msg, WPARAM wp, LPARAM lp, LRESULT& result);

 private:
  HWND window_;
  std::function<void()> on_change_;
  HPOWERNOTIFY power_ = nullptr;
  HWINEVENTHOOK desktop_hook_ = nullptr;
  bool display_off_ = false;
  bool registered_ = false;
};

/// Waker over SetThreadExecutionState. The state belongs to the calling thread, which is the UI
/// thread for every Controller call.
class Win32Waker final : public Waker {
 public:
  void keep_awake() override;
  void allow_sleep() override;
};

/// EscTap over a WH_KEYBOARD_LL hook on its own thread, so a busy UI thread (a capture, a large
/// write) never delays the person's Esc past the system's hook timeout.
class Win32EscTap final : public EscTap {
 public:
  /// @param window the main window that receives kMsgEscape.
  /// @param interrupt raised from the hook thread on a real Esc, so an action that keeps the UI
  ///        thread busy stops between two steps.
  Win32EscTap(HWND window, Interrupt* interrupt);
  ~Win32EscTap() override;
  Win32EscTap(const Win32EscTap&) = delete;
  Win32EscTap& operator=(const Win32EscTap&) = delete;

  void start() override;
  void stop() override;

 private:
  static LRESULT CALLBACK hook_proc(int code, WPARAM wp, LPARAM lp);
  void run();

  HWND window_;
  Interrupt* interrupt_;
  std::thread thread_;
  std::atomic<DWORD> thread_id_{0};
  std::atomic<bool> active_{false};
  EscFilter filter_{kInjectedMarker};  // used on the hook thread only
  HHOOK hook_ = nullptr;
  TapGeneration generations_;          // 27 9.6 item 5: only the current tap acts or ends
  std::atomic<std::uint64_t> started_generation_{0};  // set by start(), read by stop()
  std::uint64_t hook_generation_ = 0;  // hook thread only: the start the hook serves
  std::uint64_t remove_pending_ = 0;   // hook thread only: end this generation once Esc is up
};

/// Indicator (decisions.d/FIX-CU-HELPER-NO-ICON.md): while a session is shown, a click-through,
/// topmost, layered border window in the indicator color with a separate stop button window, the
/// pointer ring that follows the pointer and the pulse window of a click, all excluded from capture
/// with WDA_EXCLUDEFROMCAPTURE, and a tray icon with the session text and the menu item "Stop the
/// agent". Outside a session nothing shows: no tray icon, no window. The settings window with the
/// switch "Allow computer control" opens on a second start with `--settings` (the Start menu
/// shortcut) and by itself at start while the switch was never switched on.
/// The border, the stop button, the ring and the pulse live on their own thread with their own
/// message loop, so a click on the stop button raises the interrupt while the UI thread is busy
/// with an action (27 9.6 item 1); the tray and the settings window stay on the UI thread.
class Win32Indicator final : public Indicator {
 public:
  /// @param instance the module instance.
  /// @param window the main window that receives the tray messages and kMsgStop.
  /// @param interrupt raised with Kind::StopButton by a click on the stop button.
  /// @param on_stop called on the UI thread when the person picks the tray's stop item.
  /// @param on_switch called on the UI thread when the person clicks "Allow computer control".
  /// @param about the version line of the settings window.
  Win32Indicator(HINSTANCE instance, HWND window, Interrupt* interrupt, std::function<void()> on_stop,
                 std::function<void()> on_switch, std::wstring about);
  ~Win32Indicator() override;
  Win32Indicator(const Win32Indicator&) = delete;
  Win32Indicator& operator=(const Win32Indicator&) = delete;

  void show(const std::string& agent_name) override;
  void hide() override;
  void set_enabled(bool enabled) override;
  void mark_click(const Point& at) override;

  /// Opens the settings window with the switch "Allow computer control" and brings it to the
  /// front. UI thread only.
  void open_settings();

  /// Lets the indicator see a message of the main window (tray callbacks, menu commands, the
  /// taskbar restart, display changes).
  /// @return true when the message was handled; `result` is then the answer.
  bool handle(UINT msg, WPARAM wp, LPARAM lp, LRESULT& result);

 private:
  static LRESULT CALLBACK border_proc(HWND, UINT, WPARAM, LPARAM);
  static LRESULT CALLBACK button_proc(HWND, UINT, WPARAM, LPARAM);
  static LRESULT CALLBACK ring_proc(HWND, UINT, WPARAM, LPARAM);
  static LRESULT CALLBACK pulse_proc(HWND, UINT, WPARAM, LPARAM);
  static LRESULT CALLBACK settings_proc(HWND, UINT, WPARAM, LPARAM);
  void run_windows();
  void add_tray();
  void remove_tray();
  void place();
  void show_menu();
  HWND make_settings();
  void follow_pointer();

  HINSTANCE instance_;
  HWND window_;
  Interrupt* interrupt_;
  std::function<void()> on_stop_;
  std::function<void()> on_switch_;
  std::wstring about_;
  std::thread thread_;
  std::atomic<DWORD> thread_id_{0};
  std::atomic<bool> ready_{false};
  HWND border_ = nullptr;  // written by the indicator thread before ready_
  HWND button_ = nullptr;  // written by the indicator thread before ready_
  HWND ring_ = nullptr;    // written by the indicator thread before ready_
  HWND pulse_ = nullptr;   // written by the indicator thread before ready_
  HWND settings_ = nullptr;  // UI thread
  HFONT font_ = nullptr;     // UI thread, the settings window's font
  UINT taskbar_created_ = 0;
  std::atomic<bool> shown_{false};
  bool tray_added_ = false;  // UI thread
  bool enabled_ = false;
  std::wstring agent_;
  ULONGLONG pulse_start_ = 0;  // indicator thread
};

/// EnablementStore in `%LOCALAPPDATA%\bothy-computer-helper\state.json` (D-CU-02-4), written to a
/// temporary file in the same folder and renamed over the old one.
class FileEnablementStore final : public EnablementStore {
 public:
  EnablementState load() override;
  bool save(const EnablementState& state) override;

 private:
  std::wstring folder();
};

/// The client side of the named pipe to the host service with reconnect and the server check of
/// D-CU-02-5. A reader thread connects, checks, reads frames and queues them; the UI thread drains
/// the queue on kMsgChannel and writes with overlapped I/O, so a pending read never blocks a write.
class PipeChannel final : public HostChannel {
 public:
  /// What the reader thread reports.
  struct Event {
    enum class Kind { Connected, Frame, Broken } kind = Kind::Broken;  ///< what happened
    Frame frame;          ///< for Kind::Frame
    std::uint64_t connection = 0;  ///< the connection the event belongs to
  };

  /// @param window the main window that receives kMsgChannel.
  /// @param pipe_name the pipe to connect to.
  /// @param check_server false only for a development build with BOTHY_COMPUTER_HELPER_CHANNEL.
  /// @param frame_max `host_service.computer_frame_max_bytes`.
  /// @param interrupt raised by the reader thread for a `hide` of the shown session, a `disable`
  ///        and a broken channel, so a running action stops at its next step (27 9.6 item 1).
  PipeChannel(HWND window, std::wstring pipe_name, bool check_server, std::uint64_t frame_max,
              Interrupt* interrupt);
  ~PipeChannel() override;
  PipeChannel(const PipeChannel&) = delete;
  PipeChannel& operator=(const PipeChannel&) = delete;

  /// Starts the reader thread.
  void start();
  /// Takes the queued events (UI thread).
  /// @return the events in order.
  std::vector<Event> drain();
  /// Closes the current connection; the reader reports a break and reconnects (UI thread).
  void close();

  bool send(const json::Value& object) override;
  bool send_image(const std::vector<std::uint8_t>& bytes) override;

 private:
  void run();
  bool write_all(const std::vector<std::uint8_t>& bytes);
  void push(Event event);

  HWND window_;
  std::wstring pipe_name_;
  bool check_server_;
  std::uint64_t frame_max_;
  Interrupt* interrupt_;
  std::thread thread_;
  std::atomic<bool> quit_{false};
  HANDLE quit_event_ = nullptr;
  std::mutex mutex_;  // guards pipe_, connection_ and queue_
  HANDLE pipe_ = INVALID_HANDLE_VALUE;
  std::uint64_t connection_ = 0;
  std::deque<Event> queue_;
};

/// Checks that the server end of a connected pipe belongs to the host service (D-CU-02-5). The
/// server must run in session 0 (services only), and its account, read from the server process's
/// token or, when a standard user may not open that token, from the owner of the pipe object, must
/// be exactly LocalSystem or `NT SERVICE\bothy-host` and not the helper's own user
/// (pipe_server_allowed()). A user process cannot create a pipe owned by either account. Anything
/// that cannot be read refuses.
/// @param pipe the connected pipe.
/// @return true when the server is the host service.
bool pipe_server_trusted(HANDLE pipe);

}  // namespace bothy::helper::win32
