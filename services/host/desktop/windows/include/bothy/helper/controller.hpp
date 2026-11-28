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
#include <mutex>
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
  /// The highest `amount` of one `scroll`: the default of `computer.scroll_amount_max`
  /// (27 section 18, 9.6 item 3), held here for the same reason as key_repeat_max.
  std::int64_t scroll_amount_max = 50;
};

/// The interrupt flag of the stop contract (27 9.6): the Esc tap, the stop button, the lock
/// monitor and the channel reader raise it on their own threads, and the Controller checks it
/// before every step of an action, so a stop never waits for an action to finish. When several
/// kinds are raised, the strongest wins (an Esc over a lock), because it decides the answer.
///
/// Example:
/// @code
///   Interrupt interrupt;
///   interrupt.raise_escape();                      // from the keyboard hook thread
///   Controller c(platform, info, &interrupt);      // c stops the running action at its next step
/// @endcode
class Interrupt {
 public:
  /// The kinds, weakest first.
  enum class Kind : int {
    None = 0,           ///< nothing pending
    Lock = 1,           ///< the lock monitor saw a lock or the secure desktop
    Hide = 2,           ///< the host hid the shown session
    Disable = 3,        ///< the host switched the enablement off
    ChannelBroken = 4,  ///< the channel to the host broke
    StopButton = 5,     ///< the person clicked the stop button
    Escape = 6,         ///< the person pressed a real Esc
  };

  /// Raises a kind; safe from any thread. A weaker kind never replaces a stronger pending one.
  /// @param kind the kind.
  void raise(Kind kind) {
    int want = static_cast<int>(kind);
    int now = kind_.load();
    while (now < want && !kind_.compare_exchange_weak(now, want)) {
    }
  }
  /// Raises Kind::Escape; safe from any thread.
  void raise_escape() { raise(Kind::Escape); }
  /// Raises Kind::Hide when `session_id` is the shown session; a stale hide stops nothing.
  /// @param session_id the session of the host's `hide`.
  void raise_hide(const std::string& session_id) {
    std::lock_guard<std::mutex> lock(mutex_);
    if (!shown_.empty() && shown_ == session_id) raise(Kind::Hide);
  }
  /// @return the pending kind without clearing it.
  Kind peek() const { return static_cast<Kind>(kind_.load()); }
  /// @return the pending kind, and clears it.
  Kind take() { return static_cast<Kind>(kind_.exchange(0)); }
  /// @return true while a raised Esc has not been handled.
  bool escape_pending() const { return peek() == Kind::Escape; }
  /// Clears every pending kind.
  void clear() { kind_.store(0); }
  /// Clears `kind` if it is the pending one; the Controller calls it when it handles that frame.
  /// @param kind the kind.
  void clear(Kind kind) {
    int expected = static_cast<int>(kind);
    kind_.compare_exchange_strong(expected, 0);
  }
  /// Records the shown session for raise_hide(); empty when none (Controller only).
  /// @param session_id the session.
  void set_shown(const std::string& session_id) {
    std::lock_guard<std::mutex> lock(mutex_);
    shown_ = session_id;
  }

 private:
  std::atomic<int> kind_{0};
  std::mutex mutex_;
  std::string shown_;
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
  /// @param injected the click came from input the helper sent for the agent (its marker): it may
  ///        switch off, never on (R-CU-03, decisions.d/FIX-CU-HELPER-NO-ICON.md).
  void on_switch_clicked(bool injected = false);

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
  // `click_at` is the point of a click action: every step that presses a button there pulses the
  // indicator's pointer mark once it was sent.
  const char* run_steps(const std::vector<std::vector<InputEvent>>& steps,
                        std::optional<Point> click_at = std::nullopt);
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
  const char* take_interrupt();

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
