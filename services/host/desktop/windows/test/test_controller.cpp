// SPDX-License-Identifier: Apache-2.0

// Tests of the Controller against the fakes (brief CU-02 items 15 to 21; CU-T13 part, CU-T14,
// CU-T15, CU-T16 part).

#include <string>
#include <vector>

#include "bothy/helper/controller.hpp"
#include "check.hpp"
#include "fakes.hpp"

using namespace bothy::helper;
using fakes::Rig;
using fakes::str;
using json::Value;
using Type = InputEvent::Type;

namespace {

Value do_frame(std::int64_t seq, const char* action, std::initializer_list<Value::Member> extra = {}) {
  Value v = Value::object({{"op", "do"}, {"seq", seq}, {"action", action}});
  for (const auto& m : extra) v.set(m.first, m.second);
  return v;
}

Value capture_frame(std::int64_t seq, const char* format = "jpeg") {
  return Value::object(
      {{"op", "capture"}, {"seq", seq}, {"max_edge", 1280}, {"format", format}, {"quality", 80}});
}

std::string code_of(const Value& done) { return str(done, "code"); }

InputEvent key_event(Type t, Key key) {
  InputEvent e;
  e.type = t;
  e.key = key;
  return e;
}

}  // namespace

CHECK_TEST(enablement_is_off_after_install_and_rises_per_switch_on) {
  Rig rig(false);
  CHECK(!rig.controller.enablement().enabled);
  CHECK_EQ(rig.controller.enablement().generation, 0);
  rig.controller.on_switch_clicked();
  CHECK(rig.controller.enablement() == (EnablementState{true, 1}));
  CHECK(rig.store.stored == (EnablementState{true, 1}));
  CHECK(rig.indicator.enabled);
  Value st = rig.channel.last();
  CHECK_EQ(str(st, "op"), std::string("state"));
  CHECK(st.find("enabled")->as_bool());
  CHECK_EQ(st.find("generation")->as_integer(), 1);

  rig.controller.on_switch_clicked();  // off keeps the generation
  CHECK(rig.store.stored == (EnablementState{false, 1}));
  CHECK(!rig.indicator.enabled);
  CHECK(!rig.channel.last().find("enabled")->as_bool());
  rig.controller.on_switch_clicked();
  CHECK(rig.store.stored == (EnablementState{true, 2}));
}

CHECK_TEST(enablement_switch_on_needs_a_saved_state) {
  Rig rig(false);
  rig.store.fail_save = true;
  rig.controller.on_switch_clicked();
  CHECK(!rig.controller.enablement().enabled);
  CHECK(!rig.indicator.enabled);
}

CHECK_TEST(enablement_is_read_at_start_and_reported_in_hello) {
  Rig rig(false);
  rig.store.stored = EnablementState{true, 5};
  rig.controller.start();
  rig.controller.on_connected();
  Value hello = rig.channel.objects.front();
  CHECK_EQ(str(hello, "op"), std::string("hello"));
  CHECK(hello.find("enabled")->as_bool());
  CHECK_EQ(hello.find("generation")->as_integer(), 5);
  CHECK_EQ(str(hello, "os_account"), std::string("alex"));
  CHECK_EQ(str(hello, "helper_version"), std::string("1.2.3"));
  CHECK_EQ(str(hello, "signature"), std::string("development"));
  CHECK_EQ(str(*hello.find("permissions"), "capture"), std::string("granted"));
  CHECK_EQ(str(*hello.find("permissions"), "input"), std::string("granted"));
  CHECK(!hello.find("locked")->as_bool());
  CHECK(hello.find("secure_desktop") == nullptr);
  CHECK(rig.indicator.enabled);
}

CHECK_TEST(hello_is_followed_by_secure_desktop_state) {
  Rig rig;
  rig.lock.state.secure_desktop = true;
  rig.controller.on_connected();
  REQUIRE(rig.channel.objects.size() == 2);
  CHECK_EQ(str(rig.channel.objects[1], "op"), std::string("state"));
  CHECK(rig.channel.objects[1].find("secure_desktop")->as_bool());
}

CHECK_TEST(show_starts_indicator_tap_and_waker_hide_undoes_them) {
  Rig rig;
  rig.show();
  CHECK(rig.controller.session_shown());
  CHECK(rig.indicator.shown);
  CHECK_EQ(rig.indicator.agent, std::string("Builder"));
  CHECK(rig.tap.started);
  CHECK(rig.waker.awake);
  CHECK(rig.channel.objects.empty());

  // A hide for another session is stale and changes nothing.
  CHECK(rig.host(Value::object({{"op", "hide"}, {"session_id", "other"}})));
  CHECK(rig.indicator.shown);

  int released = rig.injector.release_all_calls;
  CHECK(rig.host(Value::object({{"op", "hide"}, {"session_id", "s1"}})));
  CHECK(!rig.controller.session_shown());
  CHECK(!rig.indicator.shown);
  CHECK(!rig.tap.started);
  CHECK(!rig.waker.awake);
  CHECK(rig.injector.release_all_calls > released);
}

CHECK_TEST(show_while_disabled_shows_nothing_and_reports_state) {
  Rig rig(false);
  rig.show();
  CHECK(!rig.controller.session_shown());
  CHECK(!rig.indicator.shown);
  CHECK(!rig.tap.started);
  CHECK(!rig.waker.awake);
  Value st = rig.channel.last();
  CHECK_EQ(str(st, "op"), std::string("state"));
  CHECK(!st.find("enabled")->as_bool());
}

CHECK_TEST(real_escape_ends_with_local_escape) {
  Rig rig;
  rig.show();
  rig.controller.on_escape();
  CHECK(!rig.controller.session_shown());
  CHECK(!rig.indicator.shown);
  CHECK(!rig.tap.started);
  CHECK(!rig.waker.awake);
  Value stop = rig.channel.last();
  CHECK_EQ(str(stop, "op"), std::string("stop"));
  CHECK_EQ(str(stop, "session_id"), std::string("s1"));
  CHECK_EQ(str(stop, "detail"), std::string("local_escape"));
  // A second Esc without a session sends nothing more.
  std::size_t sent = rig.channel.objects.size();
  rig.controller.on_escape();
  CHECK_EQ(rig.channel.objects.size(), sent);
}

CHECK_TEST(injected_escape_passes_and_stops_nothing) {
  Rig rig;
  rig.show();
  CHECK(rig.host(do_frame(1, "key", {{"keys", "Escape"}})));
  CHECK(rig.controller.session_shown());
  CHECK_EQ(code_of(rig.channel.last()), std::string("ok"));
  std::vector<InputEvent> want = {key_event(Type::KeyDown, Key{KeyCode::Escape, 0}),
                                  key_event(Type::KeyUp, Key{KeyCode::Escape, 0})};
  CHECK(rig.injector.events == want);
}

CHECK_TEST(stop_button_ends_with_indicator_stop) {
  Rig rig;
  rig.show();
  rig.controller.on_stop_button();
  CHECK(!rig.controller.session_shown());
  CHECK(!rig.indicator.shown);
  CHECK_EQ(str(rig.channel.last(), "detail"), std::string("indicator_stop"));
}

CHECK_TEST(local_switch_off_ends_the_session_before_state) {
  Rig rig;
  rig.show();
  rig.controller.on_switch_clicked();
  CHECK(!rig.controller.session_shown());
  CHECK(!rig.indicator.shown);
  CHECK(!rig.waker.awake);
  Value st = rig.channel.last();
  CHECK_EQ(str(st, "op"), std::string("state"));
  CHECK(!st.find("enabled")->as_bool());
}

CHECK_TEST(disable_switches_off_and_ends_the_session) {
  Rig rig;
  rig.show();
  CHECK(rig.host(Value::object({{"op", "disable"}, {"generation", 1}})));
  CHECK(!rig.controller.enablement().enabled);
  CHECK(rig.store.stored == (EnablementState{false, 1}));
  CHECK(!rig.controller.session_shown());
  CHECK(!rig.indicator.shown);
  CHECK(!rig.channel.last().find("enabled")->as_bool());
}

CHECK_TEST(every_disable_switches_off) {
  // D-CU-02-14: a disable switches off whatever its generation; the server's generation check
  // keeps the host unlisted, the helper only obeys.
  Rig rig;
  rig.controller.on_switch_clicked();
  rig.controller.on_switch_clicked();  // generation 2, on
  CHECK(rig.host(Value::object({{"op", "disable"}, {"generation", 1}})));
  CHECK(rig.controller.enablement() == (EnablementState{false, 2}));
}

CHECK_TEST(release_on_hide_broken_channel_and_start) {
  // Every end leaves nothing pressed: the controller releases what it holds and asks the platform
  // to release everything (CU-T16 part).
  {
    Rig rig;
    rig.show();
    rig.injector.partial_once = 1;  // the OS takes the first key down, then refuses
    CHECK(rig.host(do_frame(1, "key", {{"keys", "shift+a"}})));
    CHECK_EQ(code_of(rig.channel.last()), std::string("os_refused"));
    CHECK(rig.injector.nothing_held());
  }
  {
    Rig rig;
    rig.show();
    int before = rig.injector.release_all_calls;
    rig.controller.on_channel_broken();
    CHECK(!rig.controller.session_shown());
    CHECK(!rig.indicator.shown);
    CHECK(!rig.tap.started);
    CHECK(!rig.waker.awake);
    CHECK(rig.injector.release_all_calls > before);
    CHECK(rig.channel.objects.empty());  // no frame on a broken channel
  }
  {
    Rig rig(false);
    rig.indicator.shown = true;
    rig.tap.start();
    rig.waker.awake = true;
    int before = rig.injector.release_all_calls;
    rig.controller.start();
    CHECK(rig.injector.release_all_calls > before);
    CHECK(!rig.indicator.shown);
    CHECK(!rig.tap.started);
    CHECK(!rig.waker.awake);
  }
  {
    Rig rig;
    rig.show();
    int before = rig.injector.release_all_calls;
    CHECK(rig.host(Value::object({{"op", "release"}})));
    CHECK(rig.injector.release_all_calls > before);
    CHECK(rig.controller.session_shown());
  }
}

CHECK_TEST(shortcuts_pass_unchanged) {
  // CU-T15: every key and combination of the table reaches the injector unchanged; nothing is
  // refused.
  struct Row {
    const char* keys;
    std::vector<Key> press;
  } rows[] = {
      {"cmd+q", {Key{KeyCode::Cmd, 0}, Key{KeyCode::Character, 'q'}}},
      {"ctrl+alt+Delete", {Key{KeyCode::Ctrl, 0}, Key{KeyCode::Alt, 0}, Key{KeyCode::Delete, 0}}},
      {"ctrl+alt+del", {Key{KeyCode::Ctrl, 0}, Key{KeyCode::Alt, 0}, Key{KeyCode::Delete, 0}}},
      {"cmd+space", {Key{KeyCode::Cmd, 0}, Key{KeyCode::Space, 0}}},
      {"win+r", {Key{KeyCode::Cmd, 0}, Key{KeyCode::Character, 'r'}}},
      {"F5", {Key{KeyCode::Function, 5}}},
      {"Return", {Key{KeyCode::Return, 0}}},
      {"alt+F4", {Key{KeyCode::Alt, 0}, Key{KeyCode::Function, 4}}},
      {"ctrl+shift+Escape", {Key{KeyCode::Ctrl, 0}, Key{KeyCode::Shift, 0}, Key{KeyCode::Escape, 0}}},
      {"cmd+shift+t", {Key{KeyCode::Cmd, 0}, Key{KeyCode::Shift, 0}, Key{KeyCode::Character, 't'}}},
      {"win+l", {Key{KeyCode::Cmd, 0}, Key{KeyCode::Character, 'l'}}},
  };
  std::int64_t seq = 1;
  for (const Row& r : rows) {
    Rig rig;
    rig.show();
    CHECK(rig.host(do_frame(seq, "key", {{"keys", r.keys}})));
    if (code_of(rig.channel.last()) != "ok") {
      check::fail(__FILE__, __LINE__, std::string("refused: ") + r.keys);
      continue;
    }
    std::vector<InputEvent> want;
    for (const Key& k : r.press) want.push_back(key_event(Type::KeyDown, k));
    for (auto it = r.press.rbegin(); it != r.press.rend(); ++it)
      want.push_back(key_event(Type::KeyUp, *it));
    if (!(rig.injector.events == want)) check::fail(__FILE__, __LINE__, std::string("changed: ") + r.keys);
    ++seq;
  }
}

CHECK_TEST(key_repeat_presses_the_combination_again) {
  Rig rig;
  rig.show();
  CHECK(rig.host(do_frame(1, "key", {{"keys", "Down"}, {"repeat", 3}})));
  CHECK_EQ(rig.injector.events.size(), 6u);
  CHECK(rig.injector.nothing_held());
  CHECK(rig.host(do_frame(2, "key", {{"keys", "Down"}, {"repeat", 0}})));
  CHECK_EQ(code_of(rig.channel.last()), std::string("invalid_action"));
}

CHECK_TEST(unknown_key_names_are_invalid_and_inject_nothing) {
  Rig rig;
  rig.show();
  rig.injector.unsupported = {0xE9};
  const char* bad[] = {"ctrl+foo", "F25", "", "ctrl+", "\xC3\xA9"};
  std::int64_t seq = 1;
  for (const char* keys : bad) {
    CHECK(rig.host(do_frame(seq++, "key", {{"keys", keys}})));
    CHECK_EQ(code_of(rig.channel.last()), std::string("invalid_action"));
  }
  CHECK(rig.host(do_frame(seq++, "key", {})));
  CHECK_EQ(code_of(rig.channel.last()), std::string("invalid_action"));
  CHECK(rig.host(do_frame(seq++, "fly", {})));
  CHECK_EQ(code_of(rig.channel.last()), std::string("invalid_action"));
  CHECK(rig.host(do_frame(seq++, "screenshot", {})));
  CHECK_EQ(code_of(rig.channel.last()), std::string("invalid_action"));
  CHECK(rig.injector.events.empty());
}

CHECK_TEST(actions_outside_a_session_are_invalid) {
  Rig rig;
  CHECK(rig.host(do_frame(1, "left_click", {{"x", 1}, {"y", 2}})));
  Value done = rig.channel.last();
  CHECK_EQ(str(done, "op"), std::string("done"));
  CHECK_EQ(done.find("seq")->as_integer(), 1);
  CHECK_EQ(code_of(done), std::string("invalid_action"));
  CHECK(rig.host(capture_frame(2)));
  CHECK_EQ(code_of(rig.channel.last()), std::string("invalid_action"));
  CHECK(rig.host(Value::object({{"op", "position"}, {"seq", 3}})));
  CHECK_EQ(code_of(rig.channel.last()), std::string("invalid_action"));
  CHECK(rig.injector.events.empty());
  CHECK(rig.capturer.requests.empty());
}

CHECK_TEST(locked_and_secure_desktop_inject_nothing) {
  Rig rig;
  rig.show();
  rig.lock.state.locked = true;
  rig.controller.on_lock_changed();
  Value st = rig.channel.last();
  CHECK_EQ(str(st, "op"), std::string("state"));
  CHECK(st.find("locked")->as_bool());
  CHECK(rig.controller.session_shown());  // a lock pauses, it does not end
  CHECK(rig.indicator.shown);

  CHECK(rig.host(do_frame(1, "left_click", {{"x", 1}, {"y", 2}})));
  Value done = rig.channel.last();
  CHECK_EQ(code_of(done), std::string("screen_locked"));
  CHECK(done.find("app") != nullptr && done.find("screen") != nullptr);
  CHECK(rig.host(do_frame(2, "type", {{"text", "x"}})));
  CHECK_EQ(code_of(rig.channel.last()), std::string("screen_locked"));
  CHECK(rig.host(capture_frame(3)));
  CHECK_EQ(code_of(rig.channel.last()), std::string("screen_locked"));
  CHECK(rig.host(Value::object({{"op", "position"}, {"seq", 4}})));
  CHECK_EQ(code_of(rig.channel.last()), std::string("screen_locked"));

  rig.lock.state = LockState{false, true};
  CHECK(rig.host(do_frame(5, "key", {{"keys", "Return"}})));
  // The changed state goes out before the answer, because the action asked the monitor again.
  std::size_t n = rig.channel.objects.size();
  CHECK_EQ(str(rig.channel.objects[n - 2], "op"), std::string("state"));
  CHECK_EQ(code_of(rig.channel.last()), std::string("secure_desktop"));
  CHECK(rig.injector.events.empty());
  CHECK(rig.capturer.requests.empty());

  rig.lock.state = LockState{};
  rig.controller.on_lock_changed();
  REQUIRE(rig.channel.last().find("secure_desktop") != nullptr);
  CHECK(!rig.channel.last().find("secure_desktop")->as_bool());
  CHECK(rig.host(do_frame(6, "key", {{"keys", "Return"}})));
  CHECK_EQ(code_of(rig.channel.last()), std::string("ok"));
}

CHECK_TEST(lock_releases_what_is_held) {
  Rig rig;
  rig.show();
  int before = rig.injector.release_all_calls;
  rig.lock.state.locked = true;
  rig.controller.on_lock_changed();
  CHECK(rig.injector.release_all_calls > before);
}

CHECK_TEST(capture_scales_into_max_edge_and_reports_the_screen) {
  struct Row {
    ScreenInfo screen;
    PixelRect source;
    int w, h;
  } rows[] = {
      {{0, 0, 1920, 1080, 1}, {0, 0, 1920, 1080}, 1280, 720},
      {{0, 0, 1536, 864, 1.25}, {0, 0, 1920, 1080}, 1280, 720},
      {{-1920, 0, 1280, 720, 1.5}, {0, 0, 1920, 1080}, 1280, 720},
      {{100, -50, 1000, 500, 1}, {0, 0, 1000, 500}, 1000, 500},
  };
  for (const Row& r : rows) {
    Rig rig;
    rig.show();
    rig.capturer.screen_info = r.screen;
    CHECK(rig.host(capture_frame(9)));
    REQUIRE(rig.capturer.requests.size() == 1);
    const CaptureRequest& req = rig.capturer.requests[0];
    CHECK(req.source == r.source);
    CHECK_EQ(req.out_width, r.w);
    CHECK_EQ(req.out_height, r.h);
    CHECK(req.format == ImageFormat::Jpeg);
    CHECK_EQ(req.quality, 80);
    Value done = rig.channel.last();
    CHECK_EQ(code_of(done), std::string("ok"));
    CHECK_EQ(done.find("seq")->as_integer(), 9);
    const Value* screen = done.find("screen");
    REQUIRE(screen);
    CHECK(*screen == Value::object({{"x", r.screen.x},
                                    {"y", r.screen.y},
                                    {"width", r.screen.width},
                                    {"height", r.screen.height},
                                    {"scale", r.screen.scale}}));
    const Value* image = done.find("image");
    REQUIRE(image);
    CHECK(*image == Value::object({{"format", "jpeg"}, {"width", r.w}, {"height", r.h}, {"bytes", 4}}));
    CHECK(*done.find("app") ==
          Value::object({{"id", "editor.exe"}, {"name", "Editor"}, {"signer", "Example Corp"}}));
    // The image frame follows its done directly.
    REQUIRE(rig.channel.order.size() >= 2);
    CHECK_EQ(rig.channel.order[rig.channel.order.size() - 2], 1);
    CHECK_EQ(rig.channel.order.back(), 2);
    CHECK(rig.channel.images.back() == rig.capturer.image);
  }
}

CHECK_TEST(capture_region_for_zoom) {
  Rig rig;
  rig.show();
  rig.capturer.screen_info = ScreenInfo{-1920, 0, 1536, 864, 1.25};
  Value f = capture_frame(2, "png");
  f.set("region", Value::object({{"x", -1820}, {"y", 200}, {"width", 320}, {"height", 240}}));
  CHECK(rig.host(f));
  REQUIRE(rig.capturer.requests.size() == 1);
  const CaptureRequest& req = rig.capturer.requests[0];
  CHECK(req.source == (PixelRect{125, 250, 400, 300}));
  CHECK_EQ(req.out_width, 400);
  CHECK_EQ(req.out_height, 300);
  CHECK(req.format == ImageFormat::Png);
  CHECK_EQ(str(*rig.channel.last().find("image"), "format"), std::string("png"));

  Value outside = capture_frame(3, "png");
  outside.set("region", Value::object({{"x", 5000}, {"y", 0}, {"width", 10}, {"height", 10}}));
  CHECK(rig.host(outside));
  CHECK_EQ(code_of(rig.channel.last()), std::string("invalid_action"));
}

CHECK_TEST(capture_refusals) {
  Rig rig;
  rig.show();
  rig.capturer.refuse = true;
  CHECK(rig.host(capture_frame(1)));
  CHECK_EQ(code_of(rig.channel.last()), std::string("os_refused"));
  CHECK(rig.channel.images.empty());
  rig.capturer.refuse = false;
  rig.capturer.screen_info.reset();
  CHECK(rig.host(capture_frame(2)));
  CHECK_EQ(code_of(rig.channel.last()), std::string("os_refused"));
  rig.capturer.screen_info = ScreenInfo{0, 0, 100, 100, 1};
  Value bad = capture_frame(3, "gif");
  CHECK(rig.host(bad));
  CHECK_EQ(code_of(rig.channel.last()), std::string("invalid_action"));
  Value zero = capture_frame(4);
  zero.set("max_edge", 0);
  CHECK(rig.host(zero));
  CHECK_EQ(code_of(rig.channel.last()), std::string("invalid_action"));
}

CHECK_TEST(capture_too_large_for_a_frame_sends_no_image) {
  Rig rig;
  rig.show();
  rig.capturer.image.assign(64, 0xAB);
  // A controller whose frame limit is below the image.
  fakes::FakeChannel channel;
  HelperInfo info{"1", "alex", "development"};
  info.frame_max_bytes = 32;
  Controller small(Platform{rig.capturer, rig.injector, rig.apps, rig.lock, rig.waker, rig.tap,
                            rig.indicator, rig.store, channel},
                   info);
  small.start();
  if (!small.enablement().enabled) small.on_switch_clicked();  // the store is shared with rig
  REQUIRE(small.enablement().enabled);
  small.on_connected();
  Frame show;
  show.object = Value::object({{"op", "show"}, {"session_id", "s"}, {"agent_name", "A"}});
  CHECK(small.on_frame(show));
  REQUIRE(small.session_shown());
  Frame cap;
  cap.object = capture_frame(1);
  CHECK(small.on_frame(cap));
  CHECK_EQ(code_of(channel.last()), std::string("os_refused"));  // D-CU-02-14
  CHECK(channel.images.empty());
}

CHECK_TEST(clicks_with_modifiers_and_counts) {
  Rig rig;
  rig.show();
  CHECK(rig.host(do_frame(1, "left_click", {{"x", 640.5}, {"y", 400}, {"modifiers", Value::Array{"cmd", "shift"}}})));
  CHECK_EQ(code_of(rig.channel.last()), std::string("ok"));
  const auto& e = rig.injector.events;
  REQUIRE(e.size() == 7);
  CHECK(e[0].type == Type::Move && e[0].point.x == 640.5 && e[0].point.y == 400);
  CHECK(e[1] == key_event(Type::KeyDown, Key{KeyCode::Cmd, 0}));
  CHECK(e[2] == key_event(Type::KeyDown, Key{KeyCode::Shift, 0}));
  CHECK(e[3].type == Type::ButtonDown && e[3].button == MouseButton::Left);
  CHECK(e[4].type == Type::ButtonUp && e[4].button == MouseButton::Left);
  CHECK(e[5] == key_event(Type::KeyUp, Key{KeyCode::Shift, 0}));
  CHECK(e[6] == key_event(Type::KeyUp, Key{KeyCode::Cmd, 0}));

  struct Row {
    const char* action;
    MouseButton button;
    int count;
  } rows[] = {{"right_click", MouseButton::Right, 1},
              {"middle_click", MouseButton::Middle, 1},
              {"double_click", MouseButton::Left, 2},
              {"triple_click", MouseButton::Left, 3}};
  for (const Row& r : rows) {
    rig.injector.events.clear();
    CHECK(rig.host(do_frame(2, r.action, {{"x", 1}, {"y", 2}})));
    int downs = 0;
    for (const InputEvent& ev : rig.injector.events)
      if (ev.type == Type::ButtonDown && ev.button == r.button) ++downs;
    CHECK_EQ(downs, r.count);
    CHECK(rig.injector.nothing_held());
  }

  rig.injector.events.clear();
  CHECK(rig.host(do_frame(3, "left_click", {{"x", 1}, {"y", 2}, {"modifiers", Value::Array{"Return"}}})));
  CHECK_EQ(code_of(rig.channel.last()), std::string("invalid_action"));
  CHECK(rig.host(do_frame(4, "left_click", {{"x", 1}})));
  CHECK_EQ(code_of(rig.channel.last()), std::string("invalid_action"));
  CHECK(rig.injector.events.empty());
}

CHECK_TEST(move_scroll_and_position) {
  Rig rig;
  rig.show();
  CHECK(rig.host(do_frame(1, "mouse_move", {{"x", -100}, {"y", 20}})));
  REQUIRE(rig.injector.events.size() == 1);
  CHECK(rig.injector.events[0].type == Type::Move && rig.injector.events[0].point.x == -100);

  rig.injector.events.clear();
  CHECK(rig.host(do_frame(2, "scroll", {{"x", 10}, {"y", 20}, {"direction", "down"}, {"amount", 3}})));
  // One step per line (27 9.6 item 1).
  REQUIRE(rig.injector.events.size() == 4);
  for (std::size_t i = 1; i < 4; ++i) {
    CHECK(rig.injector.events[i].type == Type::Scroll);
    CHECK(rig.injector.events[i].direction == ScrollDirection::Down);
    CHECK_EQ(rig.injector.events[i].amount, 1);
  }
  CHECK(rig.host(do_frame(3, "scroll", {{"x", 10}, {"y", 20}, {"direction", "sideways"}, {"amount", 3}})));
  CHECK_EQ(code_of(rig.channel.last()), std::string("invalid_action"));
  CHECK(rig.host(do_frame(4, "scroll", {{"x", 10}, {"y", 20}, {"direction", "up"}, {"amount", 0}})));
  CHECK_EQ(code_of(rig.channel.last()), std::string("invalid_action"));

  CHECK(rig.host(Value::object({{"op", "position"}, {"seq", 7}})));
  Value done = rig.channel.last();
  CHECK_EQ(code_of(done), std::string("ok"));
  CHECK(*done.find("cursor") == Value::object({{"x", 12}, {"y", 34}}));
  rig.injector.refuse_cursor = true;
  CHECK(rig.host(Value::object({{"op", "position"}, {"seq", 8}})));
  CHECK_EQ(code_of(rig.channel.last()), std::string("os_refused"));
}

CHECK_TEST(type_sends_unicode_units_and_return_for_line_breaks) {
  Rig rig;
  rig.show();
  CHECK(rig.host(do_frame(1, "type", {{"text", "a\xF0\x9F\x98\x80\r\nb\n"}})));
  CHECK_EQ(code_of(rig.channel.last()), std::string("ok"));
  std::vector<InputEvent> want;
  auto unit = [&](char16_t u) {
    InputEvent d;
    d.type = Type::UnicodeDown;
    d.unit = u;
    InputEvent up = d;
    up.type = Type::UnicodeUp;
    want.push_back(d);
    want.push_back(up);
  };
  auto ret = [&] {
    want.push_back(key_event(Type::KeyDown, Key{KeyCode::Return, 0}));
    want.push_back(key_event(Type::KeyUp, Key{KeyCode::Return, 0}));
  };
  unit(u'a');
  unit(0xD83D);
  unit(0xDE00);
  ret();
  unit(u'b');
  ret();
  CHECK(rig.injector.events == want);
}

CHECK_TEST(done_after_an_action_carries_app_and_screen) {
  Rig rig;
  rig.show();
  CHECK(rig.host(do_frame(3, "mouse_move", {{"x", 1}, {"y", 2}})));
  Value done = rig.channel.last();
  CHECK_EQ(done.find("seq")->as_integer(), 3);
  CHECK_EQ(str(*done.find("app"), "id"), std::string("editor.exe"));
  CHECK_EQ(done.find("screen")->find("width")->as_integer(), 1920);
  CHECK(done.find("image") == nullptr);
}

CHECK_TEST(protocol_errors_close_the_channel) {
  Rig rig;
  Frame image;
  image.type = kFrameImage;
  image.bytes = {1};
  CHECK(!rig.controller.on_frame(image));
  CHECK(!rig.host(Value::object({{"op", "nope"}})));
  CHECK(!rig.host(Value::object({{"seq", 1}})));
  CHECK(!rig.host(Value::object({{"op", "do"}, {"action", "type"}})));  // no seq to answer
  CHECK(!rig.host(Value::object({{"op", "show"}, {"agent_name", "x"}})));
  CHECK(!rig.host(Value::object({{"op", "disable"}})));
  CHECK(!rig.host(Value::object({{"op", "hello"}})));  // helper-to-host op
}

CHECK_TEST(a_new_show_replaces_the_session) {
  Rig rig;
  rig.show();
  CHECK(rig.host(Value::object({{"op", "show"}, {"session_id", "s2"}, {"agent_name", "Robin"}})));
  CHECK_EQ(rig.controller.session_id(), std::string("s2"));
  CHECK_EQ(rig.indicator.agent, std::string("Robin"));
  CHECK(rig.tap.started);
  CHECK(rig.waker.awake);
}

CHECK_TEST(a_real_escape_aborts_a_long_action_between_steps) {
  Rig rig;
  rig.show();
  // The Esc hook raises the interrupt from its own thread while the action runs.
  rig.injector.after_send = [&](int n) {
    if (n == 3) rig.interrupt.raise_escape();
  };
  const Value combo = Value("ctrl+shift+Down");  // kept out of the literal: secret scanners match it
  CHECK(rig.host(do_frame(1, "key", {{"keys", combo}, {"repeat", 50}})));
  CHECK(rig.injector.events.size() < 50u * 6u);
  CHECK(rig.injector.nothing_held());
  CHECK(!rig.controller.session_shown());
  CHECK(!rig.indicator.shown);
  CHECK(!rig.interrupt.escape_pending());
  bool stopped = false;
  for (const Value& o : rig.channel.objects)
    stopped = stopped || (str(o, "op") == "stop" && str(o, "detail") == "local_escape");
  CHECK(stopped);
  CHECK_EQ(code_of(rig.channel.last()), std::string("invalid_action"));
}

CHECK_TEST(a_lock_aborts_a_long_action_between_steps) {
  Rig rig;
  rig.show();
  rig.injector.after_send = [&](int n) {
    if (n == 2) rig.lock.state.locked = true;
  };
  CHECK(rig.host(do_frame(1, "type", {{"text", "a long text typed character by character"}})));
  CHECK(rig.injector.events.size() < 80u);
  CHECK(rig.injector.nothing_held());
  CHECK(rig.controller.session_shown());  // a lock pauses
  CHECK_EQ(code_of(rig.channel.last()), std::string("screen_locked"));

  Rig secure;
  secure.show();
  secure.injector.after_send = [&](int n) {
    if (n == 1) secure.lock.state.secure_desktop = true;
  };
  CHECK(secure.host(do_frame(1, "key", {{"keys", "Tab"}, {"repeat", 10}})));
  CHECK(secure.injector.nothing_held());
  CHECK_EQ(code_of(secure.channel.last()), std::string("secure_desktop"));
}

CHECK_TEST(repeat_and_text_over_their_caps_are_invalid) {
  Rig rig;
  rig.show();
  // computer.key_repeat_max (27 section 18) bounds repeat on the helper too.
  CHECK(rig.host(do_frame(1, "key", {{"keys", "Down"}, {"repeat", rig.info.key_repeat_max + 1}})));
  CHECK_EQ(code_of(rig.channel.last()), std::string("invalid_action"));
  CHECK(rig.injector.events.empty());
  CHECK(rig.host(do_frame(2, "key", {{"keys", "Down"}, {"repeat", rig.info.key_repeat_max}})));
  CHECK_EQ(code_of(rig.channel.last()), std::string("ok"));

  // A text longer than one frame of host_service.computer_frame_max_bytes cannot have come over
  // the channel; it is refused before anything is typed.
  fakes::FakeChannel channel;
  HelperInfo info{"1", "alex", "development"};
  info.frame_max_bytes = 32;
  Controller small(Platform{rig.capturer, rig.injector, rig.apps, rig.lock, rig.waker, rig.tap,
                            rig.indicator, rig.store, channel},
                   info);
  small.start();
  if (!small.enablement().enabled) small.on_switch_clicked();  // the store is shared with rig
  REQUIRE(small.enablement().enabled);
  small.on_connected();
  Frame show;
  show.object = Value::object({{"op", "show"}, {"session_id", "s"}, {"agent_name", "A"}});
  CHECK(small.on_frame(show));
  REQUIRE(small.session_shown());
  rig.injector.events.clear();
  Frame type;
  type.object = do_frame(1, "type", {{"text", std::string(64, 'x')}});
  CHECK(small.on_frame(type));
  CHECK_EQ(code_of(channel.last()), std::string("invalid_action"));
  CHECK(rig.injector.events.empty());
}

CHECK_TEST(scroll_amount_over_its_cap_is_invalid) {
  // computer.scroll_amount_max (27 section 18) bounds amount on the helper too (27 9.6 item 3).
  Rig rig;
  rig.show();
  CHECK(rig.host(do_frame(1, "scroll", {{"x", 1}, {"y", 2}, {"direction", "up"}, {"amount", rig.info.scroll_amount_max + 1}})));
  CHECK_EQ(code_of(rig.channel.last()), std::string("invalid_action"));
  CHECK(rig.injector.events.empty());
}

CHECK_TEST(a_new_connection_drops_a_stale_broken_channel) {
  // A break of the previous connection must not silence the next one.
  Rig rig;
  rig.interrupt.raise(Interrupt::Kind::ChannelBroken);
  rig.controller.on_connected();
  rig.channel.objects.clear();
  rig.show();
  rig.interrupt.raise(Interrupt::Kind::ChannelBroken);
  rig.controller.on_connected();
  CHECK(rig.host(do_frame(1, "key", {{"keys", "Return"}})));
  CHECK_EQ(code_of(rig.channel.last()), std::string("ok"));
}

// The pointer mark (27 9.2, decisions.d/FIX-CU-HELPER-NO-ICON.md): every click the helper sends for
// the agent pulses at its point, other actions pulse nothing, a stopped click pulses nothing.
CHECK_TEST(every_click_pulses_at_its_point) {
  Rig rig;
  rig.show();
  CHECK(rig.host(do_frame(1, "left_click", {{"x", 10}, {"y", 20}})));
  CHECK_EQ(code_of(rig.channel.last()), std::string("ok"));
  REQUIRE(rig.indicator.clicks.size() == 1);
  CHECK_EQ(rig.indicator.clicks[0].x, 10.0);
  CHECK_EQ(rig.indicator.clicks[0].y, 20.0);
  CHECK(rig.host(do_frame(2, "double_click", {{"x", 30}, {"y", 40}})));
  REQUIRE(rig.indicator.clicks.size() == 3);
  CHECK_EQ(rig.indicator.clicks[2].x, 30.0);
  CHECK_EQ(rig.indicator.clicks[2].y, 40.0);
}

CHECK_TEST(other_actions_pulse_nothing) {
  Rig rig;
  rig.show();
  CHECK(rig.host(do_frame(1, "mouse_move", {{"x", 10}, {"y", 20}})));
  CHECK_EQ(code_of(rig.channel.last()), std::string("ok"));
  CHECK(rig.host(do_frame(2, "type", {{"text", "hi"}})));
  CHECK_EQ(code_of(rig.channel.last()), std::string("ok"));
  CHECK(rig.host(do_frame(3, "key", {{"keys", "Return"}})));
  CHECK_EQ(code_of(rig.channel.last()), std::string("ok"));
  CHECK(rig.host(do_frame(4, "scroll", {{"x", 10}, {"y", 20}, {"direction", "down"}, {"amount", 2}})));
  CHECK_EQ(code_of(rig.channel.last()), std::string("ok"));
  CHECK(rig.indicator.clicks.empty());
}

CHECK_TEST(a_stopped_click_pulses_nothing) {
  Rig rig;
  rig.show();
  rig.interrupt.raise(Interrupt::Kind::StopButton);
  CHECK(rig.host(do_frame(1, "left_click", {{"x", 10}, {"y", 20}})));
  CHECK(rig.indicator.clicks.empty());
}

// The settings window opens by itself at start only while the switch was never switched on.
CHECK_TEST(settings_show_at_start_only_before_the_first_switch_on) {
  CHECK(needs_setup(EnablementState{}));
  CHECK(!needs_setup(EnablementState{true, 1}));
  CHECK(!needs_setup(EnablementState{false, 2}));
}
