# CU-speed-2 Adaptive wait and fewer rounds in computer use

Rulings of levers 1 and 3 of the design note
[cu-speed-2026-11-28.md](../design/cu-speed-2026-11-28.md) ([27-computer-use.md](../spec/27-computer-use.md)
6.4, 7, 10.3, 18). Lever 2 (accessibility text) is not part of this ticket.

| No | Date | Ruling | Reason |
|---|---|---|---|
| D-CU-speed-2-1 | 2026-11-28 | The screen comparison of the adaptive wait runs in the host service (C3), on the screenshots it takes through the existing `capture` of the helper (27 9.4); the helpers, their protocol and Noise message 2 stay unchanged. | The host already holds every screenshot and a clock; a change probe in the helpers would need a new helper op on two platforms and a version check, for a gain of at most the setup of one capture per look. With the screenshot as the look, the last look is also the image of the result, so a quiet end saves the extra capture after the wait. |
| D-CU-speed-2-2 | 2026-11-28 | `wait` and the settle after an input action without `quick` end when the screen stayed unchanged for 300 ms after a change, or when nothing changed for 1.5 s (`wait`) or 400 ms (settle) since the start; the first look comes 100 ms after the start, then one every 100 ms; the limit is `duration_ms` or `computer.settle`. | A page or a dialog usually paints within the first second, so the change rule ends most waits shortly after the screen arrived; the idle rule ends a wait in which nothing visible happens without running to a long limit, and is shorter for the settle, where the action has just been injected. The limit keeps a video, a game or an endless animation exactly as long as a fixed wait. |
| D-CU-speed-2-3 | 2026-11-28 | Two looks differ when more than 2 per mille of the cells of 8 by 8 image pixels change their mean brightness by more than 16 of 255; a look of another size always differs. | Small regions (a blinking caret, a spinner, a ticking clock, a progress line) must not keep a wait open; a new page, a dialog, a menu or a video change far more cells. The brightness of the JPEG screenshot is decoded in Go without a new dependency; an identical screen gives identical bytes. |
| D-CU-speed-2-4 | 2026-11-28 | No look starts that would end after the limit, judged by the duration of the session's previous screenshot; a failed look (a lock, a refusal, no JPEG) stops the looking and the wait runs to its limit and takes its image as before. | A slow capture (a still desktop of several monitors on Windows waits for the duplication's timeout) must not make a settle longer than the fixed one; the failure paths keep the earlier behaviour. |
| D-CU-speed-2-5 | 2026-11-28 | The values of D-CU-speed-2-2 and 2-3 are constants of the host, not configuration keys; `computer.settle` keeps its name and default and becomes the longest settle. | A new key would change `computer.open`, which the host decodes strictly, so an older host would refuse every session of a newer server; nobody tunes these values per installation (as D-CU-speed-1). |
| D-CU-speed-2-6 | 2026-11-28 | The tool descriptions of `bothy-computer-mcp` and the runner's section `## Computer use` steer toward one `batch` per decision (a whole form with Tab between fields), keys in every program (Tab, Return, Escape, the program's shortcuts and menus), and a `wait` with the time the step usually needs as the last item (1500 ms for a page, the example of before). | A model round costs 2.5 to 3.7 s (design note 2.4). Red team finding: a hint toward long limits (5000 ms) would make a screen that never settles (a video, an animation, a host before this change) wait longer than before; with the limits of before such a screen waits as long as before and a quiet one returns early. |
| D-CU-speed-2-7 | 2026-11-28 | A `wait` with `quick` (before the last batch item) keeps its full `duration_ms`; only the last `wait` and the settle are adaptive. The hints tell the model to put such a wait where the screen stays still before it is ready (a program that starts) and the adaptive wait last. | Review finding: a launcher closes (a change) and the window paints seconds later; an adaptive wait ends in that gap, and the quick items after it would type into whatever window is in front, since they get no image. A fixed wait before an adaptive one expresses "at least this long, then until quiet". |

## Open minors

Collected from the review of 2026-11-28, for the next run in this area:

- The guard of D-CU-speed-2-4 judges by the previous capture and leaves out the decode; on a
  Windows desktop whose capture waits for the duplication's timeout, the first look of a session
  can end after the limit, and a capped end adds its own capture.
- A long last `wait` on a video takes up to ten captures per second; the load on the real helpers
  (a new duplication per capture on Windows) is not measured.
- `TestQuietInSession` counts the looks of the settle but does not prove that no extra capture
  follows a quiet end.
- The settle ends 400 ms after the start when nothing changed; a dialog that appears between
  400 and 500 ms after a click is missed, which the fixed 500 ms caught (red team r1).
- `jpeg.Decode` reads the helper's bytes without checking the dimensions first; the helper is the
  authenticated local component (red team r1).
