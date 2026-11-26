// SPDX-License-Identifier: Apache-2.0

// The stop contract of the helpers (docs/spec/27-computer-use.md 9.6): every case of the shared
// conformance vectors runs against the Controller and its fakes.

#include <fstream>
#include <memory>
#include <sstream>
#include <string>

#include "bothy/helper/controller.hpp"
#include "bothy/helper/escape.hpp"
#include "check.hpp"
#include "fakes.hpp"

using namespace bothy::helper;
using json::Value;

#ifndef STOP_VECTORS_PATH
#error "CMake passes STOP_VECTORS_PATH, the path of stop-vectors.json"
#endif

namespace {

Value load_stop_vectors() {
  std::ifstream in(STOP_VECTORS_PATH, std::ios::binary);
  REQUIRE(in.good());
  std::stringstream buf;
  buf << in.rdbuf();
  auto v = json::parse(buf.str());
  REQUIRE(v.has_value());
  return *v;
}

// A unit is a send() with anything other than releases; the releases after an interrupt are not.
bool is_unit(const std::vector<InputEvent>& batch) {
  for (const InputEvent& e : batch) {
    if (e.type != InputEvent::Type::KeyUp && e.type != InputEvent::Type::ButtonUp &&
        e.type != InputEvent::Type::UnicodeUp)
      return true;
  }
  return false;
}

// Raises one interrupt kind from outside the Controller, as the tap, the stop button, the lock
// monitor or the channel reader do on their own threads.
void raise(fakes::Rig& rig, const std::string& kind) {
  if (kind == "escape") {
    rig.interrupt.raise_escape();
  } else if (kind == "stop_button") {
    rig.interrupt.raise(Interrupt::Kind::StopButton);
  } else if (kind == "lock") {
    rig.lock.state.locked = true;
    rig.interrupt.raise(Interrupt::Kind::Lock);
  } else if (kind == "hide") {
    rig.interrupt.raise_hide(rig.controller.session_id());
  } else if (kind == "channel_broken") {
    rig.interrupt.raise(Interrupt::Kind::ChannelBroken);
  } else {
    check::fail(__FILE__, __LINE__, "unknown interrupt kind " + kind);
  }
}

void run_case(const Value& c, std::int64_t key_repeat_max, std::int64_t scroll_amount_max) {
  const std::string name = c.find("name")->as_string();
  HelperInfo info{"1.2.3", "alex", "development"};
  info.key_repeat_max = key_repeat_max;
  info.scroll_amount_max = scroll_amount_max;
  if (const Value* fm = c.find("frame_max_bytes")) info.frame_max_bytes = static_cast<std::uint64_t>(fm->as_integer());
  fakes::Rig rig(c.find("enabled")->as_bool(), info);
  EscFilter filter(kInjectedMarker);
  rig.tap.filter = &filter;

  std::size_t sent_from = 0;
  std::size_t do_from = 0;
  std::optional<std::pair<std::string, std::int64_t>> armed;
  for (const Value& ev : c.find("events")->as_array()) {
    if (const Value* frame = ev.find("host")) {
      const std::string op = frame->find("op")->as_string();
      if (op == "show") sent_from = rig.channel.objects.size();
      if (op == "do") {
        do_from = rig.injector.batches.size();
        rig.injector.after_send = nullptr;
        if (armed) {
          std::string kind = armed->first;
          std::int64_t k = armed->second;
          armed.reset();
          if (k == 0) {
            raise(rig, kind);
          } else {
            auto fired = std::make_shared<int>(0);  // outlives this block with the callback
            rig.injector.after_send = [&rig, kind, k, do_from, fired](int) {
              std::int64_t units = 0;
              for (std::size_t i = do_from; i < rig.injector.batches.size(); ++i)
                if (is_unit(rig.injector.batches[i])) ++units;
              if (*fired == 0 && units == k) {
                *fired = 1;
                raise(rig, kind);
              }
            };
          }
        }
      }
      if (!rig.host(*frame)) check::fail(__FILE__, __LINE__, name + ": frame refused");
      rig.injector.after_send = nullptr;
    } else if (const Value* kind = ev.find("interrupt")) {
      armed = std::make_pair(kind->as_string(), ev.find("before_unit")->as_integer());
    } else if (const Value* r = ev.find("raise")) {
      raise(rig, r->as_string());  // another thread, before the next event (27 9.6 item 6)
    } else if (const Value* td = ev.find("tap_teardown")) {
      // The late teardown of the tap generation before the current one (27 9.6 item 5).
      if (td->as_string() != "previous") check::fail(__FILE__, __LINE__, name + ": unknown teardown");
      rig.tap.teardown(rig.tap.generation - 1);
    } else if (const Value* esc = ev.find("esc")) {
      TapEvent te;
      te.is_escape = true;
      te.down = esc->as_string() == "down";
      te.injected = ev.find("injected")->as_bool();
      te.extra_info = te.injected ? kInjectedMarker : 0;
      TapDecision d = filter.decide(te, rig.tap.started);
      bool swallowed = d != TapDecision::Pass;
      if (swallowed != ev.find("swallow")->as_bool())
        check::fail(__FILE__, __LINE__, name + ": Esc swallow differs");
      if (d == TapDecision::SwallowAndStop) {
        rig.interrupt.raise_escape();
        rig.controller.on_escape();  // the UI thread's answer to the tap's message
      }
    }
  }

  const Value& expect = *c.find("expect");
  std::int64_t units = 0;
  for (std::size_t i = do_from; i < rig.injector.batches.size(); ++i)
    if (is_unit(rig.injector.batches[i])) ++units;
  if (units != expect.find("units")->as_integer())
    check::fail(__FILE__, __LINE__, name + ": units " + std::to_string(units));
  if (const Value* shown = expect.find("shown"); shown != nullptr && shown->as_bool() != rig.indicator.shown)
    check::fail(__FILE__, __LINE__, name + ": indicator shown differs");
  if (expect.find("released_after")->as_bool() != rig.injector.nothing_held())
    check::fail(__FILE__, __LINE__, name + ": something stays pressed");

  std::vector<Value> sent;
  for (std::size_t i = sent_from; i < rig.channel.objects.size(); ++i) {
    std::string op = fakes::str(rig.channel.objects[i], "op");
    if (op == "stop" || op == "done") sent.push_back(rig.channel.objects[i]);
  }
  const auto& want = expect.find("sent")->as_array();
  if (sent.size() != want.size()) {
    check::fail(__FILE__, __LINE__, name + ": " + std::to_string(sent.size()) + " stop/done frames");
    return;
  }
  for (std::size_t i = 0; i < want.size(); ++i) {
    for (const auto& m : want[i].as_object()) {
      const Value* got = sent[i].find(m.first);
      if (got == nullptr || !(*got == m.second))
        check::fail(__FILE__, __LINE__, name + ": frame " + std::to_string(i) + " member " + m.first);
    }
  }
}

}  // namespace

CHECK_TEST(stop_vectors_hold) {
  Value doc = load_stop_vectors();
  std::int64_t key_repeat_max = doc.find("key_repeat_max")->as_integer();
  std::int64_t scroll_amount_max = doc.find("scroll_amount_max")->as_integer();
  const auto& cases = doc.find("cases")->as_array();
  REQUIRE(!cases.empty());
  for (const Value& c : cases) {
    int before = check::failures();
    run_case(c, key_repeat_max, scroll_amount_max);
    if (check::failures() != before) std::cerr << "  in case: " << c.find("name")->as_string() << "\n";
  }
}
