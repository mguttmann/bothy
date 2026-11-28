### Changed

- The computer helper shows nothing while no agent controls the computer: no menu bar item on
  macOS, no tray icon on Windows, no window. Its menu bar item or tray icon appears only during a
  session, with the session text and "Stop the agent", next to the border and the stop button.
- The switch "Allow computer control" moved into a settings window of the helper. It opens when you
  open "Computer Helper" again (macOS: Finder or Spotlight; Windows: the new Start menu entry), and
  by itself after installing until the switch was switched on once. `bothy-host doctor` names this
  place while the switch is off.

### Added

- While an agent controls the computer, an orange ring with a soft glow follows the pointer and a
  short pulse marks every click the agent makes. Like the border, both let clicks through and are
  left out of the agent's screenshots.
