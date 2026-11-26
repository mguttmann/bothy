// SPDX-License-Identifier: Apache-2.0
#pragma once

/// @file
/// The helper's logic (docs/spec/27-computer-use.md sections 4, 8.2 to 8.4 and 9.4; D-CU-02-6):
/// the enablement with its generation, showing and hiding a session, the Esc and stop button ends,
/// releasing every key and button on every end, lock and secure desktop gating, and the answers to
/// `capture`, `do` and `position`. It talks to the operating system only through the interfaces of
/// platform.hpp and is driven by one thread (the UI thread of the Win32 helper).

#include <atomic>
#include <cstdint>
#include <optional>
#include <string>
#include <vector>

#include "bothy/helper/codec.hpp"
#include "bothy/helper/platform.hpp"

namespace bothy::helper {

/// The platform implementations the Controller uses. All references must outlive it.
struct Platform {
  Capturer& capturer;          ///< screen capture
  Injector& injector;          ///< synthetic input
  AppInspector& apps;          ///< frontmost app
  LockMonitor& lock;           ///< lock and secure desktop
  Waker& waker;                ///< display assertion
  EscTap& esc_tap;             ///< Esc tap
  Indicator& indicator;        ///< border, stop button, tray
  EnablementStore& store;      ///< enablement state file
  HostChannel& channel;        ///< channel to the host service
};

/// Fixed facts the helper reports in `hello` (27 9.4) and the frame limit.
struct HelperInfo {
  std::string helper_version;           ///< HELPER_VERSION of the build
  std::string os_account;               ///< the operating system account the helper runs as
  std::string signature;                ///< `verified` or `development` (D-CU-01-29)
  std::string capture_permission = "granted";  ///< `permissions.capture`; Windows has no dialog
  std::string input_permission = "granted";    ///< `permissions.input`; Windows has no dialog
  std::uint64_t frame_max_bytes = kDefaultFrameMaxBytes;  ///< `host_service.computer_frame_max_bytes`
  /// The highest `repeat` of one `key` action: the default of `computer.key_repeat_max`
  /// (27 section 18). The host service enforces the server's value; the helper holds the default as
  /// well, so a faulty host cannot make one action run for long.
  std::int64_t key_repeat_max = 50;
};

/// A flag that another thread raises to stop a running action between two of its steps. The Win32
/// Esc hook runs on its own thread and raises it at once, while the UI thread may still be busy
/// with a long `key` or `type`; the Controller checks it after every step (R-CU-09: the person's
/// Esc always ends the session, also in the middle of an action).
class Interrupt {
 public:
  /// Marks a real Esc; safe from any thread.
  void raise_escape() { escape_.store(true); }
  /// @return true while a raised Esc has not been handled.
  bool escape_pending() const { return escape_.load(); }
  /// Clears the flag; the Controller calls it when it ends the session for the Esc.
  void clear() { escape_.store(false); }

 private:
  std::atomic<bool> escape_{false};
};

/// The helper's state machine. Not thread-safe: call every method from one thread.
///
/// Example:
/// @code
///   Controller c(platform, info);
///   c.start();               // release everything, read the enablement
///   c.on_connected();        // sends hello
///   if (!c.on_frame(frame)) { /* protocol error: close the channel, then c.on_channel_broken() */ }
/// @endcode
class Controller {
 public:
  /// @param platform the platform implementations.
  /// @param info the facts of `hello` and the frame limit.
  /// @param interrupt the flag the Esc hook raises; may be null (no abort between steps).
  Controller(Platform platform, HelperInfo info, Interrupt* interrupt = nullptr);

  /// Runs once at the helper's start: releases every key and button (an earlier process may have
  /// crashed with something pressed), hides the indicator, stops the tap and the display
  /// assertion, and reads the enablement (27 8.4 item 2, brief item 16).
  void start();

  /// The channel is connected and the server passed its check: sends `hello`, then `state` with
  /// `secure_desktop` when the secure desktop has the input.
  void on_connected();

  /// Handles one frame from the host service.
  /// @param frame the frame.
  /// @return false for a protocol error (an image frame, an unknown `op`, a frame without the
  ///         members it needs to be answered); the caller closes the channel and reports the break.
  bool on_frame(const Frame& frame);

  /// The channel broke or was closed: ends a shown session without a frame (27 8.4 item 2).
  void on_channel_broken();

  /// The Esc tap saw a real Esc: ends the session with `stop` `local_escape`.
  void on_escape();

  /// The person clicked the stop button or the tray's stop item: ends the session with `stop`
  /// `indicator_stop`.
  void on_stop_button();

  /// The person clicked "Allow computer control" (brief item 15): switches on with the generation
  /// raised by one, or off (ending a shown session first), persists the state and reports `state`.
  /// This is the only way to switch on; nothing the host sends can do it.
  void on_switch_clicked();

  /// The lock monitor learned of a change: asks it again and reports what changed.
  void on_lock_changed();

  /// @return true while a session is shown.
  bool session_shown() const { return session_.has_value(); }
  /// @return the session ID of the shown session, empty otherwise.
  std::string session_id() const { return session_ ? session_->id : std::string(); }
  /// @return the enablement in memory.
  EnablementState enablement() const { return enablement_; }

 private:
  struct Session {
    std::string id;
    std::string agent_name;
  };
  struct Held {
    bool is_button = false;
    Key key;
    MouseButton button = MouseButton::Left;
  };

  void send(const json::Value& object);
  void end_session();
  void release_pressed();
  std::size_t send_input(const std::vector<InputEvent>& events);
  const char* run_steps(const std::vector<std::vector<InputEvent>>& steps);
  LockState refresh_lock();
  json::Value screen_json(const std::optional<ScreenInfo>& screen) const;
  void answer(std::int64_t seq, const char* code, const AppIdentity& app,
              const std::optional<ScreenInfo>& screen);
  bool handle_show(const json::Value& object);
  bool handle_hide(const json::Value& object);
  bool handle_disable(const json::Value& object);
  bool handle_capture(const json::Value& object);
  bool handle_do(const json::Value& object);
  bool handle_position(const json::Value& object);
  const char* gate();

  Platform p_;
  HelperInfo info_;
  Interrupt* interrupt_;
  EnablementState enablement_;
  std::optional<Session> session_;
  LockState lock_;
  bool connected_ = false;
  std::vector<Held> held_;
};

}  // namespace bothy::helper
