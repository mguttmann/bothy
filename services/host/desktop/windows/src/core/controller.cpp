// SPDX-License-Identifier: Apache-2.0

#include "bothy/helper/controller.hpp"

#include <cmath>
#include <string_view>

#include "bothy/helper/geometry.hpp"
#include "bothy/helper/utf.hpp"

namespace bothy::helper {

using json::Value;

namespace {

constexpr const char* kOk = "ok";
constexpr const char* kInvalid = "invalid_action";
constexpr const char* kLocked = "screen_locked";
constexpr const char* kSecure = "secure_desktop";
constexpr const char* kRefused = "os_refused";
// Not a code: the channel broke, so nothing is answered (27 9.6 item 2).
constexpr const char* kSilent = "";

const std::string* member_string(const Value& o, const char* name) {
  const Value* v = o.find(name);
  return v != nullptr && v->is_string() ? &v->as_string() : nullptr;
}

std::optional<std::int64_t> member_int(const Value& o, const char* name) {
  const Value* v = o.find(name);
  if (v == nullptr || !v->is_integer()) return std::nullopt;
  return v->as_integer();
}

std::optional<double> member_number(const Value& o, const char* name) {
  const Value* v = o.find(name);
  if (v == nullptr || !v->is_number() || !std::isfinite(v->as_number())) return std::nullopt;
  return v->as_number();
}

std::optional<Point> member_point(const Value& o) {
  std::optional<double> x = member_number(o, "x");
  std::optional<double> y = member_number(o, "y");
  if (!x || !y) return std::nullopt;
  return Point{*x, *y};
}

// The modifiers of a click or scroll; an absent member means none.
std::optional<std::vector<Key>> member_modifiers(const Value& o) {
  const Value* v = o.find("modifiers");
  std::vector<Key> out;
  if (v == nullptr || v->is_null()) return out;
  if (!v->is_array()) return std::nullopt;
  for (const Value& item : v->as_array()) {
    if (!item.is_string()) return std::nullopt;
    std::optional<Key> k = parse_modifier(item.as_string());
    if (!k) return std::nullopt;
    out.push_back(*k);
  }
  return out;
}

InputEvent key_event(InputEvent::Type type, const Key& key) {
  InputEvent e;
  e.type = type;
  e.key = key;
  return e;
}

InputEvent button_event(InputEvent::Type type, MouseButton button) {
  InputEvent e;
  e.type = type;
  e.button = button;
  return e;
}

InputEvent move_event(Point p) {
  InputEvent e;
  e.type = InputEvent::Type::Move;
  e.point = p;
  return e;
}

void press(std::vector<InputEvent>& out, const std::vector<Key>& keys) {
  for (const Key& k : keys) out.push_back(key_event(InputEvent::Type::KeyDown, k));
}

void unpress(std::vector<InputEvent>& out, const std::vector<Key>& keys) {
  for (auto it = keys.rbegin(); it != keys.rend(); ++it)
    out.push_back(key_event(InputEvent::Type::KeyUp, *it));
}

Value app_json(const AppIdentity& app) {
  return Value::object({{"id", app.id}, {"name", app.name}, {"signer", app.signer}});
}

}  // namespace

bool InputEvent::operator==(const InputEvent& o) const {
  if (type != o.type) return false;
  switch (type) {
    case Type::KeyDown:
    case Type::KeyUp: return key == o.key;
    case Type::ButtonDown:
    case Type::ButtonUp: return button == o.button;
    case Type::Move: return point.x == o.point.x && point.y == o.point.y;
    case Type::Scroll: return direction == o.direction && amount == o.amount;
    case Type::UnicodeDown:
    case Type::UnicodeUp: return unit == o.unit;
  }
  return false;
}

Controller::Controller(Platform platform, HelperInfo info, Interrupt* interrupt)
    : p_(platform), info_(std::move(info)), interrupt_(interrupt) {}

void Controller::start() {
  // A helper that crashed may have left keys or buttons down; nothing it pressed may outlive it.
  held_.clear();
  p_.injector.release_all();
  p_.indicator.hide();
  p_.esc_tap.stop();
  p_.waker.allow_sleep();
  session_.reset();
  enablement_ = p_.store.load();
  p_.indicator.set_enabled(enablement_.enabled);
  if (interrupt_ != nullptr) {
    interrupt_->set_shown(std::string());
    interrupt_->clear();
  }
  lock_ = p_.lock.current();
}

void Controller::send(const Value& object) {
  if (connected_) p_.channel.send(object);
}

void Controller::on_connected() {
  // A break raised for the previous connection belongs to it; it must not silence this one.
  if (interrupt_ != nullptr) interrupt_->clear(Interrupt::Kind::ChannelBroken);
  connected_ = true;
  lock_ = p_.lock.current();
  send(Value::object({{"op", "hello"},
                      {"enabled", enablement_.enabled},
                      {"generation", enablement_.generation},
                      {"os_account", info_.os_account},
                      {"permissions", Value::object({{"capture", info_.capture_permission},
                                                     {"input", info_.input_permission}})},
                      {"helper_version", info_.helper_version},
                      {"signature", info_.signature},
                      {"locked", lock_.locked}}));
  // hello has no secure_desktop member (27 9.4); a state right after it carries it.
  if (lock_.secure_desktop) send(Value::object({{"op", "state"}, {"secure_desktop", true}}));
}

void Controller::on_channel_broken() {
  if (interrupt_ != nullptr) interrupt_->clear(Interrupt::Kind::ChannelBroken);
  connected_ = false;
  end_session();
}

void Controller::release_pressed() {
  if (!held_.empty()) {
    std::vector<InputEvent> ups;
    for (auto it = held_.rbegin(); it != held_.rend(); ++it) {
      ups.push_back(it->is_button ? button_event(InputEvent::Type::ButtonUp, it->button)
                                  : key_event(InputEvent::Type::KeyUp, it->key));
    }
    held_.clear();
    p_.injector.send(ups);
  }
  // The platform call also covers what the tracking cannot know (a refused key up, a crash).
  p_.injector.release_all();
}

void Controller::end_session() {
  // Order matters to the person at the machine: input stops first, then the indicator goes.
  release_pressed();
  p_.esc_tap.stop();
  p_.indicator.hide();
  p_.waker.allow_sleep();
  session_.reset();
  if (interrupt_ != nullptr) interrupt_->set_shown(std::string());
}

std::size_t Controller::send_input(const std::vector<InputEvent>& events) {
  std::size_t accepted = p_.injector.send(events);
  // Track what the accepted part left pressed, so a partial batch is undone.
  for (std::size_t i = 0; i < accepted && i < events.size(); ++i) {
    const InputEvent& e = events[i];
    switch (e.type) {
      case InputEvent::Type::KeyDown: held_.push_back(Held{false, e.key, MouseButton::Left}); break;
      case InputEvent::Type::ButtonDown: held_.push_back(Held{true, Key{}, e.button}); break;
      case InputEvent::Type::KeyUp:
      case InputEvent::Type::ButtonUp:
        for (auto it = held_.rbegin(); it != held_.rend(); ++it) {
          bool same = e.type == InputEvent::Type::KeyUp ? (!it->is_button && it->key == e.key)
                                                        : (it->is_button && it->button == e.button);
          if (same) {
            held_.erase(std::next(it).base());
            break;
          }
        }
        break;
      default: break;
    }
  }
  if (accepted < events.size()) release_pressed();
  return accepted;
}

void Controller::on_escape() {
  if (interrupt_ != nullptr) interrupt_->clear();
  if (!session_) return;
  std::string id = session_->id;
  end_session();
  send(Value::object({{"op", "stop"}, {"session_id", id}, {"detail", "local_escape"}}));
}

void Controller::on_stop_button() {
  if (interrupt_ != nullptr) interrupt_->clear(Interrupt::Kind::StopButton);
  if (!session_) return;
  std::string id = session_->id;
  end_session();
  send(Value::object({{"op", "stop"}, {"session_id", id}, {"detail", "indicator_stop"}}));
}

void Controller::on_switch_clicked() {
  if (!enablement_.enabled) {
    EnablementState next{true, enablement_.generation + 1};
    // Switching on without a persisted generation could reuse a generation after a restart and
    // undo a remote disable (D-CU-01-25), so a failed save keeps the switch off.
    if (!p_.store.save(next)) return;
    enablement_ = next;
  } else {
    end_session();
    enablement_.enabled = false;
    // Off holds in memory even when the file cannot be written; off is the safe direction.
    p_.store.save(enablement_);
  }
  p_.indicator.set_enabled(enablement_.enabled);
  send(Value::object({{"op", "state"},
                      {"enabled", enablement_.enabled},
                      {"generation", enablement_.generation}}));
}

LockState Controller::refresh_lock() {
  LockState now = p_.lock.current();
  if (!(now == lock_)) {
    Value st = Value::object({{"op", "state"}});
    if (now.locked != lock_.locked) st.set("locked", now.locked);
    if (now.secure_desktop != lock_.secure_desktop) st.set("secure_desktop", now.secure_desktop);
    bool newly_blocked = (now.locked && !lock_.locked) || (now.secure_desktop && !lock_.secure_desktop);
    lock_ = now;
    // A lock pauses the session: nothing may stay pressed while the person is away (27 8.2).
    if (newly_blocked) release_pressed();
    send(st);
  }
  return now;
}

void Controller::on_lock_changed() { refresh_lock(); }

const char* Controller::take_interrupt() {
  if (interrupt_ == nullptr) return nullptr;
  switch (interrupt_->take()) {
    case Interrupt::Kind::None:
    case Interrupt::Kind::Lock: return nullptr;  // the caller asks the lock monitor next
    case Interrupt::Kind::Hide:
    case Interrupt::Kind::Disable:
      // The frame itself follows in the queue and does the rest (a disable switches off there).
      if (session_) end_session();
      return kInvalid;
    case Interrupt::Kind::ChannelBroken:
      on_channel_broken();
      return kSilent;
    case Interrupt::Kind::StopButton:
      on_stop_button();
      return kInvalid;
    case Interrupt::Kind::Escape:
      on_escape();
      return kInvalid;
  }
  return nullptr;
}

const char* Controller::gate() {
  if (const char* code = take_interrupt()) return code;
  if (!session_) return kInvalid;
  LockState now = refresh_lock();
  if (now.locked) return kLocked;
  if (now.secure_desktop) return kSecure;
  return nullptr;
}

Value Controller::screen_json(const std::optional<ScreenInfo>& s) const {
  ScreenInfo v = s.value_or(ScreenInfo{0, 0, 0, 0, 1});
  return Value::object({{"x", v.x}, {"y", v.y}, {"width", v.width}, {"height", v.height}, {"scale", v.scale}});
}

void Controller::answer(std::int64_t seq, const char* code, const AppIdentity& app,
                        const std::optional<ScreenInfo>& screen) {
  send(Value::object({{"op", "done"},
                      {"seq", seq},
                      {"code", code},
                      {"app", app_json(app)},
                      {"screen", screen_json(screen)}}));
}

bool Controller::on_frame(const Frame& frame) {
  if (frame.type != kFrameObject) return false;  // the host never sends images
  const std::string* op = member_string(frame.object, "op");
  if (op == nullptr) return false;
  if (*op == "show") return handle_show(frame.object);
  if (*op == "hide") return handle_hide(frame.object);
  if (*op == "disable") return handle_disable(frame.object);
  if (*op == "capture") return handle_capture(frame.object);
  if (*op == "do") return handle_do(frame.object);
  if (*op == "position") return handle_position(frame.object);
  if (*op == "release") {
    release_pressed();
    return true;
  }
  return false;
}

bool Controller::handle_show(const Value& o) {
  const std::string* id = member_string(o, "session_id");
  const std::string* name = member_string(o, "agent_name");
  if (id == nullptr || id->empty() || name == nullptr) return false;
  if (!enablement_.enabled) {
    // Nothing shows while the person has not allowed control; the state makes the host service end
    // the session with device_disabled (D-CU-02-6).
    send(Value::object({{"op", "state"}, {"enabled", false}}));
    return true;
  }
  if (interrupt_ != nullptr) {
    Interrupt::Kind pending = interrupt_->peek();
    if (pending == Interrupt::Kind::Escape || pending == Interrupt::Kind::StopButton) {
      // The person stopped just before this show (27 9.6 item 6): their stop is never cleared by a
      // new session; it refuses this one, which the host learns by a stop with the person's detail.
      interrupt_->take();
      if (session_) end_session();
      send(Value::object({{"op", "stop"},
                          {"session_id", *id},
                          {"detail", pending == Interrupt::Kind::Escape ? "local_escape" : "indicator_stop"}}));
      return true;
    }
  }
  if (session_ && session_->id == *id && session_->agent_name == *name) return true;
  if (session_ && session_->id != *id) release_pressed();  // the old session's input ends here
  session_ = Session{*id, *name};
  if (interrupt_ != nullptr) {
    // Only weaker kinds belong to the earlier session (hide, disable, a broken channel, an ended
    // lock). Each is cleared only if it is still the pending kind, so an Esc raised on the hook
    // thread a moment ago survives (27 9.6 item 6).
    for (Interrupt::Kind k : {Interrupt::Kind::Lock, Interrupt::Kind::Hide, Interrupt::Kind::Disable,
                              Interrupt::Kind::ChannelBroken})
      interrupt_->clear(k);
    interrupt_->set_shown(*id);
  }
  p_.indicator.show(*name);
  p_.esc_tap.start();
  p_.waker.keep_awake();
  return true;
}

bool Controller::handle_hide(const Value& o) {
  const std::string* id = member_string(o, "session_id");
  if (id == nullptr) return false;
  if (interrupt_ != nullptr) interrupt_->clear(Interrupt::Kind::Hide);
  if (session_ && session_->id == *id) end_session();
  return true;  // a hide for a session that is no longer shown is stale, not an error
}

bool Controller::handle_disable(const Value& o) {
  std::optional<std::int64_t> generation = member_int(o, "generation");
  if (!generation) return false;
  if (interrupt_ != nullptr) interrupt_->clear(Interrupt::Kind::Disable);
  // D-CU-02-14: every disable switches off, whatever its generation; the server's generation keeps
  // the host unlisted (D-CU-01-25), the helper obeys.
  end_session();
  if (enablement_.enabled) {
    enablement_.enabled = false;
    p_.store.save(enablement_);
    p_.indicator.set_enabled(false);
  }
  send(Value::object({{"op", "state"}, {"enabled", false}, {"generation", enablement_.generation}}));
  return true;
}

bool Controller::handle_capture(const Value& o) {
  std::optional<std::int64_t> seq = member_int(o, "seq");
  if (!seq) return false;
  if (const char* refused = gate()) {
    if (refused != kSilent) answer(*seq, refused, AppIdentity{}, session_ ? p_.capturer.screen() : std::nullopt);
    return true;
  }
  std::optional<ScreenInfo> screen = p_.capturer.screen();
  if (!screen) {
    answer(*seq, kRefused, AppIdentity{}, std::nullopt);
    return true;
  }
  std::optional<std::int64_t> max_edge = member_int(o, "max_edge");
  const std::string* format = member_string(o, "format");
  std::optional<std::int64_t> quality = member_int(o, "quality");
  std::optional<Rect> region;
  if (const Value* r = o.find("region"); r != nullptr && !r->is_null()) {
    std::optional<double> x = member_number(*r, "x"), y = member_number(*r, "y"),
                          w = member_number(*r, "width"), h = member_number(*r, "height");
    if (!r->is_object() || !x || !y || !w || !h) {
      answer(*seq, kInvalid, AppIdentity{}, screen);
      return true;
    }
    region = Rect{*x, *y, *w, *h};
  }
  bool format_ok = format != nullptr && (*format == "jpeg" || *format == "png");
  std::optional<PixelRect> source = native_rect(*screen, region);
  if (!max_edge || *max_edge < 1 || !format_ok || !quality || *quality < 1 || *quality > 100 ||
      !source) {
    answer(*seq, kInvalid, AppIdentity{}, screen);
    return true;
  }
  PixelSize out = fit_into(PixelSize{source->width, source->height}, *max_edge);
  CaptureRequest req;
  req.screen = *screen;
  req.source = *source;
  req.out_width = static_cast<int>(out.width);
  req.out_height = static_cast<int>(out.height);
  req.format = *format == "png" ? ImageFormat::Png : ImageFormat::Jpeg;
  req.quality = static_cast<int>(*quality);
  std::optional<std::vector<std::uint8_t>> bytes = p_.capturer.capture(req);
  AppIdentity app = p_.apps.frontmost();
  if (!bytes) {
    answer(*seq, kRefused, app, screen);
    return true;
  }
  if (!frame_fits(bytes->size(), info_.frame_max_bytes)) {
    // The host service would close the channel on a longer frame
    // (host_service.computer_frame_max_bytes); D-CU-02-14 answers os_refused, as on macOS.
    answer(*seq, kRefused, app, screen);
    return true;
  }
  send(Value::object({{"op", "done"},
                      {"seq", *seq},
                      {"code", kOk},
                      {"app", app_json(app)},
                      {"screen", screen_json(screen)},
                      {"image", Value::object({{"format", *format},
                                               {"width", out.width},
                                               {"height", out.height},
                                               {"bytes", static_cast<std::int64_t>(bytes->size())}})}}));
  if (connected_) p_.channel.send_image(*bytes);
  return true;
}

bool Controller::handle_position(const Value& o) {
  std::optional<std::int64_t> seq = member_int(o, "seq");
  if (!seq) return false;
  if (const char* refused = gate()) {
    if (refused != kSilent) answer(*seq, refused, AppIdentity{}, session_ ? p_.capturer.screen() : std::nullopt);
    return true;
  }
  std::optional<Point> at = p_.injector.cursor();
  AppIdentity app = p_.apps.frontmost();
  std::optional<ScreenInfo> screen = p_.capturer.screen();
  if (!at) {
    answer(*seq, kRefused, app, screen);
    return true;
  }
  send(Value::object({{"op", "done"},
                      {"seq", *seq},
                      {"code", kOk},
                      {"app", app_json(app)},
                      {"screen", screen_json(screen)},
                      {"cursor", Value::object({{"x", at->x}, {"y", at->y}})}}));
  return true;
}

bool Controller::handle_do(const Value& o) {
  std::optional<std::int64_t> seq = member_int(o, "seq");
  if (!seq) return false;
  if (const char* refused = gate()) {
    if (refused != kSilent) answer(*seq, refused, AppIdentity{}, session_ ? p_.capturer.screen() : std::nullopt);
    return true;
  }
  const std::string* action = member_string(o, "action");
  // The action as steps: each step is one balanced unit (a click with its modifiers, one character,
  // one press of a combination), and the Controller checks for an Esc and a lock between steps.
  std::vector<std::vector<InputEvent>> steps;
  std::vector<InputEvent> events;
  std::optional<Point> click_at;
  bool valid = action != nullptr;
  std::string_view a = valid ? std::string_view(*action) : std::string_view();

  if (a == "left_click" || a == "right_click" || a == "middle_click" || a == "double_click" ||
      a == "triple_click") {
    std::optional<Point> at = member_point(o);
    std::optional<std::vector<Key>> mods = member_modifiers(o);
    valid = at && mods;
    if (valid) {
      MouseButton button = a == "right_click"    ? MouseButton::Right
                           : a == "middle_click" ? MouseButton::Middle
                                                 : MouseButton::Left;
      int count = a == "double_click" ? 2 : a == "triple_click" ? 3 : 1;
      click_at = at;
      // Modifiers go down before and up after the click (D-CU-02-7). One step per click; the
      // first carries the move and the modifiers, the last releases them.
      for (int i = 0; i < count; ++i) {
        std::vector<InputEvent> step;
        if (i == 0) {
          step.push_back(move_event(*at));
          press(step, *mods);
        }
        step.push_back(button_event(InputEvent::Type::ButtonDown, button));
        step.push_back(button_event(InputEvent::Type::ButtonUp, button));
        if (i == count - 1) unpress(step, *mods);
        steps.push_back(std::move(step));
      }
    }
  } else if (a == "mouse_move") {
    std::optional<Point> at = member_point(o);
    valid = at.has_value();
    if (valid) events.push_back(move_event(*at));
  } else if (a == "scroll") {
    std::optional<Point> at = member_point(o);
    std::optional<std::vector<Key>> mods = member_modifiers(o);
    const std::string* dir = member_string(o, "direction");
    std::optional<std::int64_t> amount = member_int(o, "amount");
    std::optional<ScrollDirection> d;
    if (dir != nullptr) {
      if (*dir == "up") d = ScrollDirection::Up;
      else if (*dir == "down") d = ScrollDirection::Down;
      else if (*dir == "left") d = ScrollDirection::Left;
      else if (*dir == "right") d = ScrollDirection::Right;
    }
    // A line is one step; computer.scroll_amount_max bounds the count here as on the host (27 9.6).
    valid = at && mods && d && amount && *amount >= 1 && *amount <= info_.scroll_amount_max;
    if (valid) {
      // One step per line, so an Esc or a lock stops a long scroll between two lines.
      for (std::int64_t i = 0; i < *amount; ++i) {
        std::vector<InputEvent> step;
        if (i == 0) {
          step.push_back(move_event(*at));
          press(step, *mods);
        }
        InputEvent line;
        line.type = InputEvent::Type::Scroll;
        line.direction = *d;
        line.amount = 1;
        step.push_back(line);
        if (i == *amount - 1) unpress(step, *mods);
        steps.push_back(std::move(step));
      }
    }
  } else if (a == "type") {
    const std::string* text = member_string(o, "text");
    // A text that does not fit one frame of host_service.computer_frame_max_bytes cannot have come
    // over the channel; refusing it keeps one action as short as the host service allows.
    std::optional<std::u16string> units =
        text && frame_fits(text->size(), info_.frame_max_bytes) ? utf8_to_utf16(*text) : std::nullopt;
    valid = units.has_value();
    if (valid) {
      const Key ret{KeyCode::Return, 0};
      for (std::size_t i = 0; i < units->size(); ++i) {
        char16_t u = (*units)[i];
        if (u == u'\r' || u == u'\n') {
          // A line break is Return (D-CU-02-7); CR LF is one break.
          if (u == u'\r' && i + 1 < units->size() && (*units)[i + 1] == u'\n') ++i;
          events.push_back(key_event(InputEvent::Type::KeyDown, ret));
          events.push_back(key_event(InputEvent::Type::KeyUp, ret));
        } else {
          InputEvent down;
          down.type = InputEvent::Type::UnicodeDown;
          down.unit = u;
          InputEvent up = down;
          up.type = InputEvent::Type::UnicodeUp;
          events.push_back(down);
          events.push_back(up);
        }
        // One step per character; a surrogate pair stays together in its step.
        bool high = u >= 0xD800 && u <= 0xDBFF;
        if (!high) {
          steps.push_back(std::move(events));
          events.clear();
        }
      }
      if (!events.empty()) steps.push_back(std::move(events));
      events.clear();
    }
  } else if (a == "key") {
    const std::string* keys = member_string(o, "keys");
    std::optional<std::vector<Key>> combo = keys ? parse_keys(*keys) : std::nullopt;
    std::int64_t repeat = 1;
    if (const Value* r = o.find("repeat"); r != nullptr && !r->is_null()) {
      repeat = r->is_integer() ? r->as_integer() : 0;
    }
    // computer.key_repeat_max bounds repeat here as on the host service.
    valid = combo && repeat >= 1 && repeat <= info_.key_repeat_max;
    if (valid) {
      for (const Key& k : *combo) valid = valid && p_.injector.supports(k);
    }
    if (valid) {
      for (std::int64_t i = 0; i < repeat; ++i) {
        std::vector<InputEvent> step;
        press(step, *combo);
        unpress(step, *combo);
        steps.push_back(std::move(step));
      }
    }
  } else {
    // screenshot, zoom, wait and cursor_position are answered by the host service (27 9.4), so they
    // are as unknown here as any other name.
    valid = false;
  }

  if (!events.empty()) steps.push_back(std::move(events));
  std::optional<ScreenInfo> screen = p_.capturer.screen();
  if (!valid) {
    answer(*seq, kInvalid, AppIdentity{}, screen);
    return true;
  }
  const char* code = run_steps(steps, click_at);
  if (code == kSilent) return true;
  answer(*seq, code, session_ ? p_.apps.frontmost() : AppIdentity{}, screen);
  return true;
}

const char* Controller::run_steps(const std::vector<std::vector<InputEvent>>& steps,
                                  std::optional<Point> click_at) {
  for (const std::vector<InputEvent>& step : steps) {
    // Before every step, the first included (27 9.6 item 1): an Esc, the stop button, a hide, a
    // disable or a broken channel raised on another thread ends the action here, and a lock or the
    // secure desktop pauses it; both paths release every key and button first.
    if (const char* code = take_interrupt()) return code;
    if (!session_) return kInvalid;
    LockState now = refresh_lock();
    if (now.locked || now.secure_desktop) {
      release_pressed();
      return now.locked ? kLocked : kSecure;
    }
    if (send_input(step) < step.size()) return kRefused;
    if (click_at) {
      for (const InputEvent& e : step)
        if (e.type == InputEvent::Type::ButtonDown) p_.indicator.mark_click(*click_at);
    }
  }
  return kOk;
}

}  // namespace bothy::helper
