// SPDX-License-Identifier: Apache-2.0
#pragma once

/// @file
/// The decision of the Esc tap (docs/spec/27-computer-use.md section 8.4 item 4, brief items 17
/// and design note 4): a real Esc ends the session and is swallowed, an injected Esc passes. Kept
/// out of the Win32 hook so the rule is tested without a desktop (CU-T14).

#include <cstdint>
#include <functional>
#include <mutex>

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

  /// Forgets a held Esc. The tap calls it whenever it starts, so an Esc held during an earlier
  /// session can never make the first real Esc of a new one look like auto-repeat (27 9.6 item 4).
  void reset() { holding_ = false; }

  /// @return true while a swallowed Esc is still held (its release not seen yet).
  bool holding() const { return holding_; }

 private:
  std::uintptr_t marker_;
  bool holding_ = false;
};

/// The generation of the Esc tap (27 9.6 item 5): every start raises it, and the callback or the
/// teardown of a tap acts only while its generation is the current one, checked and done under one
/// lock. So the late teardown of an older tap (a queued removal, a callback that fires after the
/// next start) never removes the tap of the next session.
///
/// Example:
/// @code
///   TapGeneration g;
///   auto mine = g.begin();
///   g.end(mine, [&] { /* remove the hook */ });   // runs only if no newer start came
/// @endcode
class TapGeneration {
 public:
  /// Starts a new generation; safe from any thread.
  /// @return the new generation (never 0).
  std::uint64_t begin();
  /// @param generation a generation from begin().
  /// @return true while it is current (started and not ended).
  bool current(std::uint64_t generation);
  /// Ends `generation` and runs `teardown` under the lock, but only if it is current.
  /// @param generation the generation to end.
  /// @param teardown what removing the tap means for the platform.
  /// @return true when it was current and `teardown` ran.
  bool end(std::uint64_t generation, const std::function<void()>& teardown);
  /// Runs `action` under the lock if `generation` is current (a tap callback).
  /// @param generation the callback's generation.
  /// @param action what the callback does.
  /// @return true when it ran.
  bool run_if_current(std::uint64_t generation, const std::function<void()>& action);

 private:
  std::mutex mutex_;
  std::uint64_t generation_ = 0;
  bool live_ = false;
};

/// The decision of a tap callback of generation `generation` (27 9.6 items 4 and 5). Under the
/// generation lock: a callback of a tap that is not the current one passes the event and leaves
/// `filter` untouched, so a stale hook can neither stop nor swallow nor leave a held state behind
/// that would make the next real Esc look like auto-repeat; a current one decides with `filter`.
/// @param generations the tap generations.
/// @param generation the generation of the tap whose callback runs.
/// @param filter the Esc filter of the current tap.
/// @param event the keyboard event.
/// @param active true while a session is shown.
/// @param on_stop run under the lock when the decision is SwallowAndStop (raise the interrupt,
///        post the stop); may be empty.
/// @return the decision.
TapDecision tap_decide(TapGeneration& generations, std::uint64_t generation, EscFilter& filter,
                       const TapEvent& event, bool active, const std::function<void()>& on_stop = {});

}  // namespace bothy::helper
