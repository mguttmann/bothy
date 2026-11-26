// SPDX-License-Identifier: Apache-2.0
#pragma once

/// @file
/// The decision of the Esc tap (docs/spec/27-computer-use.md section 8.4 item 4, brief items 17
/// and design note 4): a real Esc ends the session and is swallowed, an injected Esc passes. Kept
/// out of the Win32 hook so the rule is tested without a desktop (CU-T14).

#include <cstdint>

namespace bothy::helper {

/// The value the helper writes into `dwExtraInfo` of every event it injects. Together with
/// `LLKHF_INJECTED` it marks the helper's own events; the value spells "BOTH" so it is easy to spot
/// in an input trace.
inline constexpr std::uintptr_t kInjectedMarker = 0x424F5448u;

/// One keyboard event as the low-level hook sees it.
struct TapEvent {
  bool is_escape = false;        ///< the virtual key is Esc
  bool down = false;             ///< key down (including auto-repeat) rather than key up
  bool injected = false;         ///< `LLKHF_INJECTED` is set
  std::uintptr_t extra_info = 0; ///< `dwExtraInfo` of the event
};

/// What the hook does with an event.
enum class TapDecision {
  Pass,            ///< hand the event on unchanged
  Swallow,         ///< drop the event
  SwallowAndStop,  ///< drop the event and end the session with `local_escape`
};

/// The Esc decision with its memory of a swallowed press, so that the auto-repeat and the release
/// of the same press are swallowed too and the app never sees half of a real Esc.
///
/// Example:
/// @code
///   EscFilter filter(kInjectedMarker);
///   filter.decide({true, true, false, 0}, true);   // SwallowAndStop
///   filter.decide({true, false, false, 0}, false); // Swallow (the release of that press)
/// @endcode
class EscFilter {
 public:
  /// @param marker the `dwExtraInfo` value of the helper's own events.
  explicit EscFilter(std::uintptr_t marker) : marker_(marker) {}

  /// Decides one event.
  /// @param event the event.
  /// @param active true while a session is shown (the tap is started).
  /// @return the decision. Any injected event passes, whoever injected it: only the person's own
  ///         keyboard may stop the session, and injected text must never stop anything (R-CU-09).
  TapDecision decide(const TapEvent& event, bool active);

 private:
  std::uintptr_t marker_;
  bool holding_ = false;
};

}  // namespace bothy::helper
