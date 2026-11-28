// SPDX-License-Identifier: Apache-2.0
#pragma once

/// @file
/// The interfaces between the helper's logic and the operating system (brief CU-02 item 13):
/// Capturer, Injector, AppInspector, LockMonitor, Waker, EscTap, Indicator, EnablementStore and
/// HostChannel. Every platform call sits behind one of them, so the Controller runs against fakes in
/// unit tests and the Win32 layer stays thin. All calls happen on the helper's UI thread; an
/// implementation that works on other threads hands its results back to that thread itself.

#include <cstddef>
#include <cstdint>
#include <optional>
#include <string>
#include <vector>

#include "bothy/helper/json.hpp"
#include "bothy/helper/keys.hpp"

namespace bothy::helper {

/// A point in the platform's screen space (Windows: physical pixels of the virtual desktop with
/// per-monitor DPI awareness; docs/spec/27-computer-use.md section 9.4).
struct Point {
  double x = 0;  ///< horizontal coordinate
  double y = 0;  ///< vertical coordinate
};

/// A rectangle in screen space.
struct Rect {
  double x = 0;       ///< left edge
  double y = 0;       ///< top edge
  double width = 0;   ///< width
  double height = 0;  ///< height
};

/// A rectangle in whole pixels of a captured image or of the virtual desktop.
struct PixelRect {
  std::int64_t x = 0;       ///< left edge
  std::int64_t y = 0;       ///< top edge
  std::int64_t width = 0;   ///< width
  std::int64_t height = 0;  ///< height
  /// @return true when all members are equal.
  bool operator==(const PixelRect& o) const {
    return x == o.x && y == o.y && width == o.width && height == o.height;
  }
};

/// The screen a capture or action refers to: origin and size in screen space and `scale` native
/// pixels per screen unit. The `screen` member of `done` (27 9.4). The Win32 layer reports physical
/// pixels and therefore scale 1; the core handles any factor because the same logic serves the
/// specification's other screen spaces and the tests.
struct ScreenInfo {
  double x = 0;       ///< left edge in screen space
  double y = 0;       ///< top edge in screen space
  double width = 0;   ///< width in screen space
  double height = 0;  ///< height in screen space
  double scale = 1;   ///< native pixels per screen unit
};

/// Identity of the frontmost app for the `app` member of `done`: on Windows the executable's file
/// name, a display name, and the subject of its verified Authenticode signer (empty when unsigned).
struct AppIdentity {
  std::string id;      ///< executable file name, for example `editor.exe`
  std::string name;    ///< display name
  std::string signer;  ///< signer subject, empty when there is no verified signature
};

/// Image formats of `capture`.
enum class ImageFormat { Jpeg, Png };

/// What the Capturer is asked to produce: the pixels `source` of `screen`'s native image, scaled to
/// `out_width` x `out_height` and encoded in memory.
struct CaptureRequest {
  ScreenInfo screen;          ///< the screen returned by Capturer::screen()
  PixelRect source;           ///< the part of the native image, in its pixels
  int out_width = 0;          ///< width of the encoded image
  int out_height = 0;         ///< height of the encoded image
  ImageFormat format = ImageFormat::Jpeg;  ///< encoding
  int quality = 0;            ///< JPEG quality from the request (1 to 100); ignored for PNG
};

/// Captures the screen. Implementations exclude the indicator from the image and never write an
/// image to disk (27 section 6.5, brief "Definition of done").
class Capturer {
 public:
  virtual ~Capturer() = default;
  /// @return the screen of the frontmost window, else the main screen (27 6.5 item 1), or
  ///         std::nullopt when the system does not tell (answered `os_refused`).
  virtual std::optional<ScreenInfo> screen() = 0;
  /// Captures and encodes.
  /// @param request what to capture.
  /// @return the encoded bytes, or std::nullopt when the operating system refused (answered
  ///         `os_refused`).
  virtual std::optional<std::vector<std::uint8_t>> capture(const CaptureRequest& request) = 0;
};

/// Mouse buttons.
enum class MouseButton { Left, Right, Middle };
/// Scroll directions of `do scroll`.
enum class ScrollDirection { Up, Down, Left, Right };

/// One step of synthetic input. The Controller composes every action from these steps so that the
/// tests see exactly what reaches the operating system and the release logic knows what is held.
struct InputEvent {
  /// What the step does.
  enum class Type {
    KeyDown,      ///< press `key`
    KeyUp,        ///< release `key`
    ButtonDown,   ///< press `button` at the current pointer position
    ButtonUp,     ///< release `button`
    Move,         ///< move the pointer to `point`
    Scroll,       ///< scroll `amount` lines towards `direction`
    UnicodeDown,  ///< press the UTF-16 code unit `unit` as text input
    UnicodeUp,    ///< release the UTF-16 code unit `unit`
  };
  Type type = Type::Move;                              ///< the step
  Key key;                                             ///< for KeyDown and KeyUp
  MouseButton button = MouseButton::Left;              ///< for ButtonDown and ButtonUp
  Point point;                                         ///< for Move
  ScrollDirection direction = ScrollDirection::Down;   ///< for Scroll
  int amount = 0;                                      ///< lines, for Scroll
  char16_t unit = 0;                                   ///< for UnicodeDown and UnicodeUp

  /// @return true when every member that matters for `type` is equal.
  bool operator==(const InputEvent& o) const;
};

/// Sends synthetic input. Implementations mark their own events so the Esc tap can tell them from
/// the person's keyboard (27 8.4 item 4).
class Injector {
 public:
  virtual ~Injector() = default;
  /// @param key a parsed key.
  /// @return true when the keyboard layout has a key for it; false makes `do key` answer
  ///         `invalid_action` (D-CU-02-7: a name the platform has no key for).
  virtual bool supports(const Key& key) = 0;
  /// Sends the steps in order, as one batch where the platform allows it.
  /// @param events the steps.
  /// @return how many steps the operating system accepted; fewer than events.size() means it
  ///         refused the rest (answered `os_refused`).
  virtual std::size_t send(const std::vector<InputEvent>& events) = 0;
  /// Releases every key and mouse button that is down, whoever pressed it, so that nothing stays
  /// pressed after any end, also after a crash of an earlier helper process (27 8.4 item 2).
  virtual void release_all() = 0;
  /// @return the pointer position in screen space, or std::nullopt when the system refused.
  virtual std::optional<Point> cursor() = 0;
};

/// Identifies the frontmost app.
class AppInspector {
 public:
  virtual ~AppInspector() = default;
  /// @return the identity of the app of the foreground window; empty members when unknown.
  virtual AppIdentity frontmost() = 0;
};

/// Lock and secure desktop state (27 section 8.2, brief item 20).
struct LockState {
  /// Lock, screen saver, a switch of the console user or display sleep.
  bool locked = false;
  /// The secure desktop (UAC prompt, secure attention sequence) has the input.
  bool secure_desktop = false;
  /// @return true when both members are equal.
  bool operator==(const LockState& o) const {
    return locked == o.locked && secure_desktop == o.secure_desktop;
  }
};

/// Reports the lock state. Implementations call Controller::on_lock_changed() on the UI thread
/// when they learn of a change; current() is asked again before every action, so a missed signal
/// never lets input through.
class LockMonitor {
 public:
  virtual ~LockMonitor() = default;
  /// @return the state now.
  virtual LockState current() = 0;
};

/// Keeps the display awake while a session is shown (27 8.2 item 5).
class Waker {
 public:
  virtual ~Waker() = default;
  /// Starts keeping the display awake; repeated calls are harmless.
  virtual void keep_awake() = 0;
  /// Stops keeping the display awake; repeated calls are harmless.
  virtual void allow_sleep() = 0;
};

/// The Esc tap (27 8.4 item 4). While started, a real Esc of the person's keyboard is swallowed
/// and reported through Controller::on_escape(); an Esc the helper injects passes unchanged.
class EscTap {
 public:
  virtual ~EscTap() = default;
  /// Starts watching; repeated calls are harmless.
  virtual void start() = 0;
  /// Stops watching; repeated calls are harmless.
  virtual void stop() = 0;
};

/// The indicator (27 9.2, brief item 16, decisions.d/FIX-CU-HELPER-NO-ICON.md): while a session is
/// shown, the border around the screen with its stop button, the pointer mark (a ring that follows
/// the pointer and a pulse at every click the helper sends), all excluded from capture, and a tray
/// icon with the session text and a stop item. Nothing of it shows outside a session. The switch
/// "Allow computer control" lives in the helper's settings window, which opens when the person
/// starts the helper again. A click on the stop button reaches Controller::on_stop_button(), a click
/// on the switch Controller::on_switch_clicked().
class Indicator {
 public:
  virtual ~Indicator() = default;
  /// Shows the border, the stop button, the pointer ring and the tray icon with the text
  /// "Agent <name> controls this computer, Esc to stop".
  /// @param agent_name the agent's display name from `show`.
  virtual void show(const std::string& agent_name) = 0;
  /// Hides all of it and removes the tray icon; repeated calls are harmless.
  virtual void hide() = 0;
  /// Sets the check mark of the switch "Allow computer control" in the settings window.
  /// @param enabled the enablement.
  virtual void set_enabled(bool enabled) = 0;
  /// Pulses at the point of a click the helper just sent for the agent; returns at once.
  /// @param at the click's point in screen space.
  virtual void mark_click(const Point& at) = 0;
};

/// The persisted enablement (D-CU-02-4).
struct EnablementState {
  bool enabled = false;         ///< the switch
  std::int64_t generation = 0;  ///< raised by one on every switch-on
  /// @return true when both members are equal.
  bool operator==(const EnablementState& o) const {
    return enabled == o.enabled && generation == o.generation;
  }
};

/// @return true while the switch was never switched on (generation 0): the helper then opens its
///         settings window by itself at start, so the person finds the switch after installing.
inline bool needs_setup(const EnablementState& state) { return !state.enabled && state.generation == 0; }

/// Persists the enablement in the helper's own state file (D-CU-02-4). The helper reads it at
/// start only and never treats a change of the file as an order.
class EnablementStore {
 public:
  virtual ~EnablementStore() = default;
  /// @return the stored state; off with generation 0 when the file is missing or unreadable.
  virtual EnablementState load() = 0;
  /// Writes the state atomically.
  /// @param state the state.
  /// @return false when it could not be written.
  virtual bool save(const EnablementState& state) = 0;
};

/// The helper's side of the channel to the host service. The transport encodes frames, checks the
/// server (D-CU-02-5), reconnects, and reports connects, frames and breaks to the Controller on the
/// UI thread.
class HostChannel {
 public:
  virtual ~HostChannel() = default;
  /// Sends one object frame.
  /// @param object the object.
  /// @return false when the channel is not connected or the write failed; the transport then
  ///         reports a break.
  virtual bool send(const json::Value& object) = 0;
  /// Sends one image frame.
  /// @param bytes the encoded image.
  /// @return as send().
  virtual bool send_image(const std::vector<std::uint8_t>& bytes) = 0;
};

}  // namespace bothy::helper
