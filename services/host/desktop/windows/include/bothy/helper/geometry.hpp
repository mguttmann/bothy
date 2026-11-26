// SPDX-License-Identifier: Apache-2.0
#pragma once

/// @file
/// Geometry of captures and pointer moves (docs/spec/27-computer-use.md section 6.5): the native
/// pixels of a screen or region, the size that fits `max_edge`, and the absolute coordinates of
/// `SendInput` over the virtual desktop.

#include <cstdint>
#include <optional>

#include "bothy/helper/platform.hpp"

namespace bothy::helper {

/// Size of an image in pixels.
struct PixelSize {
  std::int64_t width = 0;   ///< width
  std::int64_t height = 0;  ///< height
  /// @return true when both members are equal.
  bool operator==(const PixelSize& o) const { return width == o.width && height == o.height; }
};

/// The native pixels of a screen, or of a region of it for `zoom`.
/// @param screen the screen (origin and size in screen space, `scale` pixels per unit).
/// @param region an optional region in screen space; it is clipped to the screen, and its edges are
///        widened to whole pixels so that nothing the agent asked for is cut off.
/// @return the pixel rectangle relative to the screen's native image, or std::nullopt when the
///         screen is empty, the region has no positive size, a value is not finite, or the region
///         lies outside the screen.
/// Example: a 1512 x 982 screen at scale 2 yields {0, 0, 3024, 1964}.
std::optional<PixelRect> native_rect(const ScreenInfo& screen, const std::optional<Rect>& region);

/// Scales a size so that its long edge is at most `max_edge`, keeping the aspect ratio
/// (27 6.5 item 2); a size that fits already stays as it is, and no edge becomes smaller than one
/// pixel.
/// @param size the native size.
/// @param max_edge the longest edge the request allows (`computer.image_max_edge` of the server,
///        sent in `capture`); must be at least 1.
/// @return the output size.
/// Example: fit_into({3024, 1964}, 1280) yields {1280, 831}.
PixelSize fit_into(PixelSize size, std::int64_t max_edge);

/// Absolute coordinates of a `SendInput` mouse move over the virtual desktop
/// (`MOUSEEVENTF_ABSOLUTE | MOUSEEVENTF_VIRTUALDESK`), where 0 to 65535 span the desktop.
struct AbsolutePoint {
  std::int32_t x = 0;  ///< normalized horizontal coordinate
  std::int32_t y = 0;  ///< normalized vertical coordinate
};

/// Maps a point in physical pixels to the normalized coordinates of `SendInput`. The point is
/// rounded to the nearest pixel and clamped into the desktop; the result is the smallest
/// normalized value that the system maps back to that pixel (it computes
/// floor(normalized * size / 65536)), so the pointer lands exactly on the pixel the host asked for.
/// @param point the point in physical pixels of the virtual desktop.
/// @param desktop the virtual desktop (`SM_XVIRTUALSCREEN` and friends).
/// @return the normalized coordinates; {0, 0} for an empty desktop.
AbsolutePoint to_absolute(Point point, const PixelRect& desktop);

}  // namespace bothy::helper
