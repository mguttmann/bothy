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
