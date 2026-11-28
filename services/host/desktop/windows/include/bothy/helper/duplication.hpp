// SPDX-License-Identifier: Apache-2.0
#pragma once

/// @file
/// Which frame of a DXGI Desktop Duplication carries the desktop image (docs/spec/27-computer-use.md
/// section 9.2). The helper opens a new duplication for every capture. Its first frame can be a
/// pointer-only update (`LastPresentTime` 0, `AccumulatedFrames` 0) whose texture was never written
/// and is all black; on a desktop with four monitors every capture of a still desktop got exactly
/// that frame. Such a frame must be released and the next one taken; it is never an image.

#include <functional>

namespace bothy::helper {

/// What one `AcquireNextFrame` delivered.
enum class DuplicationFrame {
  Image,    ///< a frame with `LastPresentTime` other than 0; it stays acquired for the copy
  NoImage,  ///< a pointer-only frame (`LastPresentTime` 0); already released by the callback
  Failed,   ///< the call failed or timed out (a still desktop, a lost duplication)
};

/// The most frames looked at before giving up; each wait is bounded by the caller's timeout.
inline constexpr int kMaxDuplicationFrames = 8;

/// Acquires frames until one carries the desktop image.
/// @param acquire_next acquires the next frame and classifies it; it releases a NoImage frame itself.
/// @param max_frames the most frames to look at (at least 1).
/// @return true when the frame acquired last is an Image and still held; false when a call failed
///         or only pointer-only frames came, so the caller takes the GDI path instead.
/// Example: frames NoImage, Image yield true after two calls.
bool acquire_desktop_image(const std::function<DuplicationFrame()>& acquire_next,
                           int max_frames = kMaxDuplicationFrames);

}  // namespace bothy::helper
