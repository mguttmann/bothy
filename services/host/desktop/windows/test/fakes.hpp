// SPDX-License-Identifier: Apache-2.0
#pragma once

/// @file
/// Fakes of the platform interfaces for the Controller tests. They record what the Controller asks
/// of the operating system, so the tests can prove what was injected, shown and sent.

#include <cstddef>
#include <cstdint>
#include <functional>
#include <optional>
#include <string>
#include <vector>

#include "bothy/helper/controller.hpp"
#include "bothy/helper/escape.hpp"
#include "bothy/helper/platform.hpp"

namespace fakes {

using namespace bothy::helper;

/// Capturer that returns a configurable screen and records requests.
struct FakeCapturer : Capturer {
  std::optional<ScreenInfo> screen_info = ScreenInfo{0, 0, 1920, 1080, 1};  ///< what screen() returns
  bool refuse = false;                       ///< capture() fails when true
  std::vector<CaptureRequest> requests;      ///< every capture() call
  std::vector<std::uint8_t> image = {0xff, 0xd8, 0xff, 0xd9};  ///< what capture() returns

  std::optional<ScreenInfo> screen() override { return screen_info; }
  std::optional<std::vector<std::uint8_t>> capture(const CaptureRequest& r) override {
    requests.push_back(r);
    if (refuse) return std::nullopt;
    return image;
  }
};

/// Injector that records every event and tracks what is held.
struct FakeInjector : Injector {
  std::vector<InputEvent> events;     ///< every event accepted, in order
  std::vector<std::vector<InputEvent>> batches;  ///< every send() call's accepted events
  std::optional<std::size_t> partial_once;  ///< the next send() accepts only this many events
  int sends = 0;                            ///< calls of send()
  std::function<void(int)> after_send;      ///< called with the call count after each send()
  int release_all_calls = 0;          ///< calls of release_all()
  std::vector<std::uint32_t> unsupported;  ///< characters supports() refuses
  Point cursor_at{12, 34};            ///< what cursor() returns
  bool refuse_cursor = false;         ///< cursor() fails when true

  bool supports(const Key& key) override {
    if (key.code != KeyCode::Character) return true;
    for (std::uint32_t c : unsupported)
      if (c == key.value) return false;
    return true;
  }
  std::size_t send(const std::vector<InputEvent>& batch) override {
    std::size_t limit = partial_once.value_or(SIZE_MAX);
    partial_once.reset();
    std::size_t n = 0;
    for (const InputEvent& e : batch) {
      if (n >= limit) break;
      events.push_back(e);
      ++n;
    }
    batches.emplace_back(batch.begin(), batch.begin() + static_cast<std::ptrdiff_t>(n));
    ++sends;
    if (after_send) after_send(sends);
    return n;
  }
  void release_all() override { ++release_all_calls; }
  std::optional<Point> cursor() override {
    if (refuse_cursor) return std::nullopt;
    return cursor_at;
  }

  /// @return true when every key and button pressed through events was released again.
  bool nothing_held() const {
    std::vector<InputEvent> down;
    for (const InputEvent& e : events) {
      if (e.type == InputEvent::Type::KeyDown || e.type == InputEvent::Type::ButtonDown ||
          e.type == InputEvent::Type::UnicodeDown) {
        down.push_back(e);
      } else if (e.type == InputEvent::Type::KeyUp || e.type == InputEvent::Type::ButtonUp ||
                 e.type == InputEvent::Type::UnicodeUp) {
        for (auto it = down.begin(); it != down.end(); ++it) {
          bool same = (e.type == InputEvent::Type::KeyUp && it->type == InputEvent::Type::KeyDown &&
                       it->key == e.key) ||
                      (e.type == InputEvent::Type::ButtonUp &&
                       it->type == InputEvent::Type::ButtonDown && it->button == e.button) ||
                      (e.type == InputEvent::Type::UnicodeUp &&
                       it->type == InputEvent::Type::UnicodeDown && it->unit == e.unit);
          if (same) {
            down.erase(it);
            break;
          }
        }
      }
    }
    return down.empty();
  }
};

/// AppInspector with a fixed identity.
struct FakeApps : AppInspector {
  AppIdentity app{"editor.exe", "Editor", "Example Corp"};  ///< the identity returned
  int calls = 0;                                             ///< calls of frontmost()
  AppIdentity frontmost() override {
    ++calls;
    return app;
  }
};

/// LockMonitor with a settable state.
struct FakeLock : LockMonitor {
  LockState state;  ///< what current() returns
  LockState current() override { return state; }
};

/// Waker that records whether it holds the display.
struct FakeWaker : Waker {
  bool awake = false;  ///< the display assertion
  int changes = 0;     ///< calls of either method
  void keep_awake() override {
    awake = true;
    ++changes;
  }
  void allow_sleep() override {
    awake = false;
    ++changes;
  }
};

/// EscTap that records whether it is started.
struct FakeTap : EscTap {
  bool started = false;           ///< the tap
  EscFilter* filter = nullptr;    ///< the filter the tap resets when it starts (27 9.6 item 4)
  TapGeneration generations;      ///< the generation rule of 27 9.6 item 5
  std::uint64_t generation = 0;   ///< the generation of the last start
  void start() override {
    generation = generations.begin();
    started = true;
    removal_pending = false;
    if (filter != nullptr) filter->reset();
  }
  bool removal_pending = false;   ///< the stop waits for a held Esc's release, as the Win32 hook does
  void stop() override {
    started = false;
    if (filter != nullptr && filter->holding()) removal_pending = true;
    else teardown(generation);
  }
  /// Called after every key event at the tap: ends the tap once the held Esc is released.
  void after_event() {
    if (removal_pending && filter != nullptr && !filter->holding()) {
      removal_pending = false;
      teardown(generation);
    }
  }
  /// The teardown of tap generation `g`; only the current generation may remove the tap.
  /// @param g the generation whose teardown runs.
  void teardown(std::uint64_t g) {
    generations.end(g, [this] { started = false; });
  }
};

/// Indicator that records what it shows.
struct FakeIndicator : Indicator {
  bool shown = false;      ///< border and stop button
  std::string agent;       ///< the name given to show()
  bool enabled = false;    ///< the menu check mark
  void show(const std::string& name) override {
    shown = true;
    agent = name;
  }
  void hide() override {
    shown = false;
    agent.clear();
  }
  void set_enabled(bool e) override { enabled = e; }
};

/// EnablementStore in memory.
struct FakeStore : EnablementStore {
  EnablementState stored;  ///< the "file"
  bool fail_save = false;  ///< save() fails when true
  int saves = 0;           ///< successful saves
  EnablementState load() override { return stored; }
  bool save(const EnablementState& s) override {
    if (fail_save) return false;
    stored = s;
    ++saves;
    return true;
  }
};

/// HostChannel that records the frames sent.
struct FakeChannel : HostChannel {
  std::vector<json::Value> objects;                ///< object frames
  std::vector<std::vector<std::uint8_t>> images;   ///< image frames
  std::vector<int> order;                          ///< 1 object, 2 image, in send order
  bool send(const json::Value& o) override {
    objects.push_back(o);
    order.push_back(1);
    return true;
  }
  bool send_image(const std::vector<std::uint8_t>& b) override {
    images.push_back(b);
    order.push_back(2);
    return true;
  }
  /// @return the last object sent, or null.
  json::Value last() const { return objects.empty() ? json::Value() : objects.back(); }
};

/// All fakes and a Controller wired to them.
struct Rig {
  FakeCapturer capturer;
  FakeInjector injector;
  FakeApps apps;
  FakeLock lock;
  FakeWaker waker;
  FakeTap tap;
  FakeIndicator indicator;
  FakeStore store;
  FakeChannel channel;
  HelperInfo info{"1.2.3", "alex", "development"};
  Interrupt interrupt;
  Controller controller{Platform{capturer, injector, apps, lock, waker, tap, indicator, store, channel},
                        info, &interrupt};

  /// Starts the controller, enables it by a click when asked, connects and clears the frames.
  /// @param enable switch on before connecting.
  explicit Rig(bool enable = true) { init(enable); }

  /// As Rig(bool) with other helper facts (frame limit, repeat cap).
  /// @param enable switch on before connecting.
  /// @param custom the facts.
  Rig(bool enable, const HelperInfo& custom) : info(custom) { init(enable); }

  /// Starts, enables when asked, connects and clears the frames.
  /// @param enable switch on before connecting.
  void init(bool enable) {
    controller.start();
    if (enable) controller.on_switch_clicked();
    controller.on_connected();
    channel.objects.clear();
    channel.order.clear();
  }

  /// Delivers one object frame from the host.
  /// @param object the object.
  /// @return the Controller's result.
  bool host(const json::Value& object) {
    Frame f;
    f.type = kFrameObject;
    f.object = object;
    return controller.on_frame(f);
  }

  /// Shows a session with the ID "s1" and agent "Builder".
  void show() {
    host(json::Value::object({{"op", "show"}, {"session_id", "s1"}, {"agent_name", "Builder"}}));
  }
};

/// @return the string member `name` of `o`, or "" when absent.
inline std::string str(const json::Value& o, const char* name) {
  const json::Value* v = o.find(name);
  return v && v->is_string() ? v->as_string() : std::string();
}

}  // namespace fakes
