// SPDX-License-Identifier: Apache-2.0

// Tests of the key parser, the geometry, the settings files, the Esc decision and UTF conversion.

#include <cmath>
#include <string>
#include <vector>

#include "bothy/helper/codec.hpp"
#include "bothy/helper/duplication.hpp"
#include "bothy/helper/escape.hpp"
#include "bothy/helper/geometry.hpp"
#include "bothy/helper/keys.hpp"
#include "bothy/helper/settings.hpp"
#include "bothy/helper/trust.hpp"
#include "bothy/helper/utf.hpp"
#include "check.hpp"

using namespace bothy::helper;

namespace {
Key k(KeyCode c, std::uint32_t v = 0) { return Key{c, v}; }
Key ch(char c) { return Key{KeyCode::Character, static_cast<std::uint32_t>(c)}; }
}  // namespace

CHECK_TEST(keys_parse_names_and_aliases) {
  struct Row {
    const char* name;
    Key key;
  } rows[] = {
      {"shift", k(KeyCode::Shift)},       {"ctrl", k(KeyCode::Ctrl)},
      {"control", k(KeyCode::Ctrl)},      {"alt", k(KeyCode::Alt)},
      {"option", k(KeyCode::Alt)},        {"cmd", k(KeyCode::Cmd)},
      {"command", k(KeyCode::Cmd)},       {"super", k(KeyCode::Cmd)},
      {"meta", k(KeyCode::Cmd)},          {"win", k(KeyCode::Cmd)},
      {"windows", k(KeyCode::Cmd)},       {"Return", k(KeyCode::Return)},
      {"enter", k(KeyCode::Return)},      {"Tab", k(KeyCode::Tab)},
      {"Escape", k(KeyCode::Escape)},     {"esc", k(KeyCode::Escape)},
      {"BackSpace", k(KeyCode::BackSpace)}, {"Delete", k(KeyCode::Delete)},
      {"Insert", k(KeyCode::Insert)},     {"space", k(KeyCode::Space)},
      {"Up", k(KeyCode::Up)},             {"Down", k(KeyCode::Down)},
      {"Left", k(KeyCode::Left)},         {"Right", k(KeyCode::Right)},
      {"Home", k(KeyCode::Home)},         {"End", k(KeyCode::End)},
      {"Page_Up", k(KeyCode::PageUp)},    {"prior", k(KeyCode::PageUp)},
      {"Page_Down", k(KeyCode::PageDown)}, {"Next", k(KeyCode::PageDown)},
      {"CapsLock", k(KeyCode::CapsLock)}, {"F1", k(KeyCode::Function, 1)},
      {"f12", k(KeyCode::Function, 12)},  {"F24", k(KeyCode::Function, 24)},
      {"a", ch('a')},                     {"A", ch('a')},
      {"1", ch('1')},                     {"/", ch('/')},
      {"+", ch('+')},
  };
  for (const Row& r : rows) {
    auto got = parse_key(r.name);
    if (!got || !(*got == r.key)) check::fail(__FILE__, __LINE__, std::string("key ") + r.name);
  }
  auto e = parse_key("\xC3\xA9");  // a single non-ASCII character
  REQUIRE(e.has_value());
  CHECK_EQ(e->value, 0xE9u);
}

CHECK_TEST(keys_refuse_unknown_names) {
  const char* bad[] = {"", "F0", "F25", "Fa", "foo", "ctrl+", "+ctrl+a", "ctrl++a", "ab"};
  for (const char* name : bad) {
    if (parse_keys(name).has_value()) check::fail(__FILE__, __LINE__, std::string("accepted ") + name);
  }
  CHECK(!parse_modifier("a").has_value());
  CHECK(!parse_modifier("Return").has_value());
  CHECK(parse_modifier("WIN").has_value());
}

CHECK_TEST(keys_parse_combinations) {
  auto c = parse_keys("CTRL+Alt+del");  // D-CU-02-14: Del is an alias of Delete (CU-T15)
  REQUIRE(c && c->size() == 3);
  CHECK((*c)[2] == k(KeyCode::Delete));
  c = parse_keys("ctrl+alt+Delete");
  REQUIRE(c && c->size() == 3);
  CHECK((*c)[2] == k(KeyCode::Delete));
  c = parse_keys("ctrl++");
  REQUIRE(c && c->size() == 2);
  CHECK((*c)[1] == ch('+'));
  c = parse_keys("+");
  REQUIRE(c && c->size() == 1);
  c = parse_keys("a+b");
  REQUIRE(c && c->size() == 2);
  CHECK_EQ(key_name(k(KeyCode::Function, 5)), std::string("F5"));
  CHECK_EQ(key_name(k(KeyCode::Cmd)), std::string("cmd"));
  CHECK_EQ(key_name(ch('q')), std::string("q"));
}

CHECK_TEST(geometry_native_rect_with_scales_and_origins) {
  auto r = native_rect(ScreenInfo{0, 0, 1512, 982, 2}, std::nullopt);
  REQUIRE(r);
  CHECK(*r == (PixelRect{0, 0, 3024, 1964}));
  r = native_rect(ScreenInfo{-1920, 0, 1536, 864, 1.25}, std::nullopt);
  REQUIRE(r);
  CHECK(*r == (PixelRect{0, 0, 1920, 1080}));
  r = native_rect(ScreenInfo{2560, -200, 1280, 720, 1.5}, std::nullopt);
  REQUIRE(r);
  CHECK(*r == (PixelRect{0, 0, 1920, 1080}));
  // A region in screen space, relative to a screen whose origin is not 0.
  r = native_rect(ScreenInfo{2560, -200, 1280, 720, 1.5}, Rect{2660, 0, 320, 240});
  REQUIRE(r);
  CHECK(*r == (PixelRect{150, 300, 480, 360}));
  // Fractional edges widen to whole pixels.
  r = native_rect(ScreenInfo{0, 0, 1536, 864, 1.25}, Rect{10.1, 10.1, 1, 1});
  REQUIRE(r);
  CHECK(*r == (PixelRect{12, 12, 2, 2}));
  // D-CU-02-14: a region must lie inside the screen; one that crosses its edge is refused.
  CHECK(!native_rect(ScreenInfo{0, 0, 100, 100, 1}, Rect{90, 90, 50, 50}));
  r = native_rect(ScreenInfo{0, 0, 100, 100, 1}, Rect{90, 90, 10, 10});
  REQUIRE(r);
  CHECK(*r == (PixelRect{90, 90, 10, 10}));
  // Outside, empty or not finite.
  CHECK(!native_rect(ScreenInfo{0, 0, 100, 100, 1}, Rect{100, 0, 10, 10}));
  CHECK(!native_rect(ScreenInfo{0, 0, 100, 100, 1}, Rect{0, 0, 0, 10}));
  CHECK(!native_rect(ScreenInfo{0, 0, 100, 100, 1}, Rect{NAN, 0, 10, 10}));
  CHECK(!native_rect(ScreenInfo{0, 0, 0, 100, 1}, std::nullopt));
}

CHECK_TEST(geometry_fits_into_max_edge) {
  CHECK(fit_into({3024, 1964}, 1280) == (PixelSize{1280, 831}));
  CHECK(fit_into({1920, 1080}, 1280) == (PixelSize{1280, 720}));
  CHECK(fit_into({1080, 1920}, 1280) == (PixelSize{720, 1280}));
  CHECK(fit_into({800, 600}, 1280) == (PixelSize{800, 600}));
  CHECK(fit_into({100000, 10}, 1000) == (PixelSize{1000, 1}));
}

CHECK_TEST(geometry_absolute_points_land_on_the_pixel) {
  PixelRect desktops[] = {{0, 0, 1920, 1080}, {-1920, -200, 4480, 1440}, {0, 0, 7680, 4320}};
  for (const PixelRect& d : desktops) {
    for (std::int64_t px = d.x; px < d.x + d.width; px += 7) {
      AbsolutePoint a = to_absolute(Point{static_cast<double>(px), static_cast<double>(d.y)}, d);
      std::int64_t back = d.x + (static_cast<std::int64_t>(a.x) * d.width) / 65536;
      if (back != px) {
        check::fail(__FILE__, __LINE__, "pixel " + std::to_string(px) + " maps to " + std::to_string(back));
        break;
      }
      CHECK(a.x >= 0 && a.x <= 65535);
    }
  }
  // Rounded and clamped.
  AbsolutePoint a = to_absolute(Point{-5000, 99999}, PixelRect{0, 0, 1920, 1080});
  CHECK_EQ(a.x, 0);
  CHECK(a.y <= 65535 && (static_cast<std::int64_t>(a.y) * 1080) / 65536 == 1079);
  CHECK_EQ(to_absolute(Point{3, 3}, PixelRect{0, 0, 0, 0}).x, 0);
}

CHECK_TEST(settings_enablement_file) {
  EnablementState s = parse_enablement(R"({"v": 1, "enabled": true, "generation": 7})");
  CHECK(s.enabled);
  CHECK_EQ(s.generation, 7);
  EnablementState back = parse_enablement(serialize_enablement(EnablementState{true, 12}));
  CHECK(back == (EnablementState{true, 12}));
  const char* bad[] = {"", "{", "[]", R"({"v": 2, "enabled": true, "generation": 1})",
                       R"({"v": 1, "enabled": true, "generation": -1})",
                       R"({"v": 1, "enabled": "yes", "generation": 1})",
                       R"({"v": 1, "enabled": true, "generation": 1, "extra": 0})",
                       R"({"v": 1, "enabled": true})"};
  for (const char* text : bad) CHECK(parse_enablement(text) == (EnablementState{false, 0}));
}

CHECK_TEST(settings_frame_max_from_host_config) {
  CHECK_EQ(frame_max_from_host_config("host_service:\n  computer_frame_max_bytes: 1048576\n"),
           1048576u);
  CHECK_EQ(frame_max_from_host_config("# c\nserver: x\nhost_service:\n  session_users: [a]\n"
                                      "  computer_frame_max_bytes: 2048 # comment\n  other: 1\n"),
           2048u);
  // Nested deeper, in another block, absent, invalid or flow style: the default.
  CHECK_EQ(frame_max_from_host_config("host_service:\n  x:\n    computer_frame_max_bytes: 5\n"),
           kDefaultFrameMaxBytes);
  CHECK_EQ(frame_max_from_host_config("other:\n  computer_frame_max_bytes: 5\n"),
           kDefaultFrameMaxBytes);
  CHECK_EQ(frame_max_from_host_config(""), kDefaultFrameMaxBytes);
  CHECK_EQ(frame_max_from_host_config("host_service:\n  computer_frame_max_bytes: 0\n"),
           kDefaultFrameMaxBytes);
  CHECK_EQ(frame_max_from_host_config("host_service:\n  computer_frame_max_bytes: lots\n"),
           kDefaultFrameMaxBytes);
  CHECK_EQ(frame_max_from_host_config("host_service: {computer_frame_max_bytes: 5}\n"),
           kDefaultFrameMaxBytes);
  CHECK_EQ(frame_max_from_host_config("host_service:\r\n  computer_frame_max_bytes: 4096\r\n"),
           4096u);
}

CHECK_TEST(escape_real_is_swallowed_and_stops_injected_passes) {
  EscFilter f(kInjectedMarker);
  // CU-T14: a real Esc while a session is shown stops it and is swallowed, press and release.
  CHECK(f.decide({true, true, false, 0}, true) == TapDecision::SwallowAndStop);
  CHECK(f.decide({true, true, false, 0}, false) == TapDecision::Swallow);  // auto-repeat
  CHECK(f.decide({true, false, false, 0}, false) == TapDecision::Swallow);
  CHECK(f.decide({true, false, false, 0}, false) == TapDecision::Pass);
  // The helper's own Esc passes and stops nothing, by flag and by marker.
  CHECK(f.decide({true, true, true, kInjectedMarker}, true) == TapDecision::Pass);
  CHECK(f.decide({true, false, true, kInjectedMarker}, true) == TapDecision::Pass);
  CHECK(f.decide({true, true, false, kInjectedMarker}, true) == TapDecision::Pass);
  // Another program's injected Esc passes too: only the real keyboard stops.
  CHECK(f.decide({true, true, true, 0}, true) == TapDecision::Pass);
  // Other keys and an inactive tap pass.
  CHECK(f.decide({false, true, false, 0}, true) == TapDecision::Pass);
  CHECK(f.decide({true, true, false, 0}, false) == TapDecision::Pass);
}

CHECK_TEST(json_refuses_numbers_out_of_range) {
  CHECK(!bothy::helper::json::parse("1e999").has_value());
  CHECK(!bothy::helper::json::parse("-1e999").has_value());
}

CHECK_TEST(utf_conversion) {
  auto u = utf8_to_utf16("a\xC3\xA9\xE4\xB8\x96\xF0\x9F\x98\x80");
  REQUIRE(u);
  CHECK(*u == (std::u16string{u'a', 0x00E9, 0x4E16, 0xD83D, 0xDE00}));
  auto back = utf16_to_utf8(*u);
  REQUIRE(back);
  CHECK_EQ(*back, std::string("a\xC3\xA9\xE4\xB8\x96\xF0\x9F\x98\x80"));
  CHECK(!utf8_to_utf16("\xED\xA0\x80"));  // encoded surrogate
  CHECK(!utf8_to_utf16("\xF4\x90\x80\x80"));  // above U+10FFFF
  CHECK(!utf16_to_utf8(std::u16string{0xD800}));
  CHECK(is_valid_utf8("plain"));
  CHECK(!is_valid_utf8("\xFF"));
}

CHECK_TEST(pipe_server_rule) {
  const char* service = "S-1-5-80-1111111111-2222222222-3333333333-4444444444-555555555";
  const char* own = "S-1-5-21-1-2-3-1001";
  // D-CU-02-5: exactly LocalSystem or NT SERVICE\bothy-host.
  CHECK(pipe_server_allowed("S-1-5-18", own, service));
  CHECK(pipe_server_allowed(service, own, service));
  CHECK(pipe_server_allowed("s-1-5-18", own, service));  // SID strings compare without case
  // Everything else is refused: the helper's own user, another user, Administrators, a lookup or
  // token failure (empty strings), a prefix of an allowed SID.
  CHECK(!pipe_server_allowed(own, own, service));
  CHECK(!pipe_server_allowed("S-1-5-21-1-2-3-1002", own, service));
  CHECK(!pipe_server_allowed("S-1-5-32-544", own, service));
  CHECK(!pipe_server_allowed("", own, service));
  CHECK(!pipe_server_allowed(service, own, ""));
  CHECK(!pipe_server_allowed("S-1-5-1", own, service));
  CHECK(!pipe_server_allowed("S-1-5-18", "", service));
  // A helper that itself runs as LocalSystem would trust itself; that is refused too.
  CHECK(!pipe_server_allowed("S-1-5-18", "S-1-5-18", service));
  CHECK(!pipe_server_allowed(service, service, service));
}

CHECK_TEST(escape_filter_reset_forgets_a_held_escape) {
  EscFilter f(kInjectedMarker);
  CHECK(f.decide({true, true, false, 0}, true) == TapDecision::SwallowAndStop);
  CHECK(f.holding());
  f.reset();  // the tap starts again for a new session
  CHECK(!f.holding());
  CHECK(f.decide({true, true, false, 0}, true) == TapDecision::SwallowAndStop);
}

CHECK_TEST(tap_generation_lets_only_the_current_tap_end) {
  TapGeneration g;
  std::uint64_t first = g.begin();
  std::uint64_t second = g.begin();
  int ended = 0;
  CHECK(!g.end(first, [&] { ++ended; }));  // a late teardown of the older tap
  CHECK_EQ(ended, 0);
  CHECK(g.current(second));
  CHECK(!g.current(first));
  CHECK(g.end(second, [&] { ++ended; }));
  CHECK_EQ(ended, 1);
  CHECK(!g.current(second));  // ended: no tap is current
}

CHECK_TEST(a_stale_tap_never_changes_the_filter) {
  // A hook of an older tap generation decides nothing and leaves the held state alone, so the next
  // real Esc of the current tap still stops (27 9.6 item 5).
  TapGeneration gens;
  EscFilter filter(kInjectedMarker);
  std::uint64_t old_tap = gens.begin();
  std::uint64_t new_tap = gens.begin();
  TapEvent real{true, true, false, 0};
  CHECK(tap_decide(gens, old_tap, filter, real, true) == TapDecision::Pass);
  CHECK(!filter.holding());
  CHECK(tap_decide(gens, new_tap, filter, real, true) == TapDecision::SwallowAndStop);
  CHECK(filter.holding());
  // An ended tap decides nothing either.
  CHECK(gens.end(new_tap, [] {}));
  EscFilter fresh(kInjectedMarker);
  CHECK(tap_decide(gens, new_tap, fresh, real, true) == TapDecision::Pass);
  CHECK(!fresh.holding());
}

namespace {
// Plays a fixed sequence of frames and counts the AcquireNextFrame calls.
struct FrameScript {
  std::vector<DuplicationFrame> frames;
  int calls = 0;
  DuplicationFrame operator()() { return frames.at(static_cast<std::size_t>(calls++)); }
};
}  // namespace

// The first frame of a new duplication was pointer-only and all black, and the helper sent it.
CHECK_TEST(duplication_skips_pointer_only_first_frame) {
  FrameScript s{{DuplicationFrame::NoImage, DuplicationFrame::Image}};
  CHECK(acquire_desktop_image(std::ref(s)));
  CHECK_EQ(s.calls, 2);
}

CHECK_TEST(duplication_takes_an_image_at_once) {
  FrameScript s{{DuplicationFrame::Image}};
  CHECK(acquire_desktop_image(std::ref(s)));
  CHECK_EQ(s.calls, 1);
}

CHECK_TEST(duplication_failure_falls_back) {
  FrameScript s{{DuplicationFrame::NoImage, DuplicationFrame::Failed}};
  CHECK(!acquire_desktop_image(std::ref(s)));
  CHECK_EQ(s.calls, 2);
}

CHECK_TEST(duplication_only_pointer_frames_fall_back_after_the_bound) {
  FrameScript s{std::vector<DuplicationFrame>(kMaxDuplicationFrames, DuplicationFrame::NoImage)};
  CHECK(!acquire_desktop_image(std::ref(s)));
  CHECK_EQ(s.calls, kMaxDuplicationFrames);
}
