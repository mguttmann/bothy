// SPDX-License-Identifier: Apache-2.0

// Keeping the display awake while a session is shown (27 8.2 item 5, brief design note 5).

#include "win32.hpp"

namespace bothy::helper::win32 {

void Win32Waker::keep_awake() { SetThreadExecutionState(ES_CONTINUOUS | ES_DISPLAY_REQUIRED); }

void Win32Waker::allow_sleep() { SetThreadExecutionState(ES_CONTINUOUS); }

}  // namespace bothy::helper::win32
