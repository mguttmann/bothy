### Fixed

- Unread chats, channels and posts now stand in bold in the lists and in the sidebar, and bold
  text and mentions in messages are drawn bold. The text styles of the design system fixed the
  weight of the font, so a weight that a view set afterwards changed nothing on the screen; in
  the list of chats bold is the only visible mark of an unread entry.
- Text fields, dropdown fields, switches that are off and segmented buttons have an edge that
  is visible in the light and in the dark theme (at least 3:1 against the surface, WCAG 1.4.11):
  the underline or border of a field and the ring and thumb of a switch. The theme drew these
  edges in the color of decorative borders.

### Changed

- The apps no longer bundle the Roboto font files of the golden test library: the test
  libraries are dev dependencies of the design system, so they are no dependencies of an app.
- The specification (15, section 20) says where the platform goldens and the measured text
  contrast of the screens run: in the tests of a member on a Mac, in no CI job.
