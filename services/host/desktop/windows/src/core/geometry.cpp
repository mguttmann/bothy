// SPDX-License-Identifier: Apache-2.0

#include "bothy/helper/geometry.hpp"

#include <algorithm>
#include <cmath>

namespace bothy::helper {

namespace {

bool finite(double v) { return std::isfinite(v); }

// Snaps values that are a rounding error away from a whole pixel, so 100 * 1.25 never becomes
// 125.00000000000001 and widens the region by a pixel the agent did not ask for.
double snap(double v) {
  double r = std::round(v);
  return std::fabs(v - r) < 1e-6 ? r : v;
}

}  // namespace

std::optional<PixelRect> native_rect(const ScreenInfo& screen, const std::optional<Rect>& region) {
  if (!finite(screen.x) || !finite(screen.y) || !finite(screen.width) || !finite(screen.height) ||
      !finite(screen.scale) || screen.scale <= 0)
    return std::nullopt;
  auto native_w = static_cast<std::int64_t>(std::llround(screen.width * screen.scale));
  auto native_h = static_cast<std::int64_t>(std::llround(screen.height * screen.scale));
  if (native_w <= 0 || native_h <= 0) return std::nullopt;
  if (!region) return PixelRect{0, 0, native_w, native_h};

  const Rect& r = *region;
  if (!finite(r.x) || !finite(r.y) || !finite(r.width) || !finite(r.height) || r.width <= 0 ||
      r.height <= 0)
    return std::nullopt;
  double x0 = std::floor(snap((r.x - screen.x) * screen.scale));
  double y0 = std::floor(snap((r.y - screen.y) * screen.scale));
  double x1 = std::ceil(snap((r.x + r.width - screen.x) * screen.scale));
  double y1 = std::ceil(snap((r.y + r.height - screen.y) * screen.scale));
  // D-CU-02-14: the region lies inside the screen, as on macOS; the host maps it from a
  // screenshot of the same screen, so a region across the edge is a fault of the request.
  if (x0 < 0 || y0 < 0 || x1 > static_cast<double>(native_w) || y1 > static_cast<double>(native_h))
    return std::nullopt;
  return PixelRect{static_cast<std::int64_t>(x0), static_cast<std::int64_t>(y0),
                   static_cast<std::int64_t>(x1 - x0), static_cast<std::int64_t>(y1 - y0)};
}

PixelSize fit_into(PixelSize size, std::int64_t max_edge) {
  std::int64_t longest = std::max(size.width, size.height);
  if (max_edge < 1 || longest <= max_edge) return size;
  double factor = static_cast<double>(max_edge) / static_cast<double>(longest);
  auto scale = [&](std::int64_t v) {
    return std::max<std::int64_t>(1, static_cast<std::int64_t>(std::llround(static_cast<double>(v) * factor)));
  };
  PixelSize out{scale(size.width), scale(size.height)};
  // Rounding must never push the long edge over the limit.
  if (size.width >= size.height) out.width = max_edge;
  else out.height = max_edge;
  return out;
}

AbsolutePoint to_absolute(Point point, const PixelRect& desktop) {
  if (desktop.width <= 0 || desktop.height <= 0) return {};
  auto axis = [](double v, std::int64_t origin, std::int64_t size) -> std::int32_t {
    double rounded = std::isfinite(v) ? std::round(v) : static_cast<double>(origin);
    auto px = static_cast<std::int64_t>(
        std::clamp(rounded, static_cast<double>(origin), static_cast<double>(origin + size - 1)));
    std::int64_t offset = px - origin;
    // The smallest n with floor(n * size / 65536) == offset.
    std::int64_t n = (offset * 65536 + size - 1) / size;
    return static_cast<std::int32_t>(std::min<std::int64_t>(n, 65535));
  };
  return AbsolutePoint{axis(point.x, desktop.x, desktop.width), axis(point.y, desktop.y, desktop.height)};
}

}  // namespace bothy::helper
