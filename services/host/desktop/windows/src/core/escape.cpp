// SPDX-License-Identifier: Apache-2.0

#include "bothy/helper/escape.hpp"

namespace bothy::helper {

TapDecision EscFilter::decide(const TapEvent& event, bool active) {
  if (!event.is_escape) return TapDecision::Pass;
  // Injected events never stop: neither the helper's own Esc for `key` (flag and marker) nor one of
  // another program. The marker also covers a platform that loses the injected flag.
  if (event.injected || event.extra_info == marker_) return TapDecision::Pass;
  if (event.down) {
    if (holding_) return TapDecision::Swallow;  // auto-repeat of a press already swallowed
    if (!active) return TapDecision::Pass;
    holding_ = true;
    return TapDecision::SwallowAndStop;
  }
  if (holding_) {
    holding_ = false;
    return TapDecision::Swallow;
  }
  return TapDecision::Pass;
}

}  // namespace bothy::helper

namespace bothy::helper {

std::uint64_t TapGeneration::begin() {
  std::lock_guard<std::mutex> lock(mutex_);
  ++generation_;
  live_ = true;
  return generation_;
}

bool TapGeneration::current(std::uint64_t generation) {
  std::lock_guard<std::mutex> lock(mutex_);
  return live_ && generation == generation_;
}

bool TapGeneration::end(std::uint64_t generation, const std::function<void()>& teardown) {
  std::lock_guard<std::mutex> lock(mutex_);
  if (!live_ || generation != generation_) return false;
  live_ = false;
  teardown();
  return true;
}

bool TapGeneration::run_if_current(std::uint64_t generation, const std::function<void()>& action) {
  std::lock_guard<std::mutex> lock(mutex_);
  if (!live_ || generation != generation_) return false;
  action();
  return true;
}

}  // namespace bothy::helper

namespace bothy::helper {

TapDecision tap_decide(TapGeneration& generations, std::uint64_t generation, EscFilter& filter,
                       const TapEvent& event, bool active, const std::function<void()>& on_stop) {
  TapDecision d = TapDecision::Pass;
  // A stale tap never reaches the filter: its event passes and the held state stays as it was.
  generations.run_if_current(generation, [&] {
    d = filter.decide(event, active);
    if (d == TapDecision::SwallowAndStop && on_stop) on_stop();
  });
  return d;
}

}  // namespace bothy::helper
