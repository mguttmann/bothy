# CU-speed Fewer waits and rounds in computer use

Rulings of the speed-up of computer use ([27-computer-use.md](../spec/27-computer-use.md) 5.1,
6.3, 6.4, 7, 10.3). The settle value `computer.settle` itself stays unchanged.

| No | Date | Ruling | Reason |
|---|---|---|---|
| D-CU-speed-1 | 2026-11-26 | A `batch` item before the last one goes as `cu_action` with the optional member `quick`: an input action waits 50 ms (a constant of the host, `quickPause`; a shorter `computer.settle` wins) and returns no image, `wait` only waits, `screenshot`, `zoom` and `cursor_position` are unchanged. The pause is a constant, not a configuration key. | A batch used to wait `computer.settle` and take a screenshot after every item and then dropped all but the last image. The short pause gives a field or menu a moment for the next item; a key would add a server setting for a value nobody tunes. |
| D-CU-speed-2 | 2026-11-26 | Noise message 2 gains `features` (`["quick"]`); C1 sends `quick` only to a host that names it. | Host and runner are updated separately; the host decodes `cu_action` strictly, so an unknown member would end the session with `stream_fault`. |
| D-CU-speed-3 | 2026-11-26 | The result of a batch keeps "the last image any item returned"; after a failed item it can have no image, and the agent takes a screenshot. | A failure in a batch is rare; an extra capture after it would cost the time this change saves. |
| D-CU-speed-4 | 2026-11-26 | `open_session` takes the first screenshot of an active session and returns it with the open text; while paused it takes none, and a failed screenshot leaves the session open with the code in the text. | Saves the screenshot call the agent made after every open; the session itself does not depend on the image. |
