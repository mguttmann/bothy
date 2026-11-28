// SPDX-License-Identifier: Apache-2.0

#include "bothy/helper/duplication.hpp"

namespace bothy::helper {

bool acquire_desktop_image(const std::function<DuplicationFrame()>& acquire_next, int max_frames) {
  for (int i = 0; i < max_frames; ++i) {
    switch (acquire_next()) {
      case DuplicationFrame::Image:
        return true;
      case DuplicationFrame::Failed:
        return false;
      case DuplicationFrame::NoImage:
        break;
    }
  }
  return false;
}

}  // namespace bothy::helper
