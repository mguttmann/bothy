### Added

- A unit test of the design system checks every text and icon color pair of
  `design/design-rules.json` for WCAG AA (4.5:1 for text, 3:1 for large text and icons) on the
  colors the app renders, for the light and the dark theme and every accent.

### Fixed

- The Spaces page of the phone is readable in the light theme on iOS and Android: the sidebar
  view paints its own deep green brand surface, so the space, channel and forum names, the
  product name, "Running now", "All posts" and the icons no longer stand in light green on a
  near-white page. In the dark theme the page now has the same brand surface as the desktop
  sidebar instead of the content surface.
