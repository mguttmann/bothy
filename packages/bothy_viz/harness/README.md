# bothy_viz_harness

Harness app of [`bothy_viz`](../README.md) (docs/spec/25-visualization.md 7.1, row `harness/`):
it renders every basic form and every view from the fixtures of
[`../fixtures/`](../fixtures/README.md), without a server and without network.

Kind: Flutter app, a member of the Dart workspace (`resolution: workspace`, `publish_to: none`),
with the same allowed members as `bothy_viz` plus `bothy_viz` itself
(`tools/arch-check/rules.yaml`, member `bothy_viz_harness`).

## Run it

The platform folders are not checked in (docs/DECISIONS.md D-M6-11-6). Create them once in this
folder and run on macOS, Windows or the web:

```sh
flutter create --platforms=macos,windows,web .
flutter run -d macos   # or -d windows, -d chrome
```

## Switches

The toolbar sets theme (light, dark), density (`comfortable`, `compact`, `touch`), text size
(1, 1.5, 2), language (de, en) and the fixture state (`normal`, `empty`, `truncated`,
`no_cost_permission`, `offline`, `load`, `rate_limits`, `day_boundary`; a schema without a file
of that state shows `normal`). The time-lapse slider moves the fake clock of the fixture live
sources by one frame interval per step, so the replayed frames, then the stale marker
(`viz.stale_after`) and the timeline playback appear. The list shows the marks and then the six
views of the registry; TV mode ends on a key press or a click and returns to the first entry.

## Tests

`test/harness_test.dart` pumps the app without a server, switches every toggle once and opens
every view; run it with `make test COMPONENTS=apps` or `flutter test` in this folder.

`integration_test/topology_perf_test.dart` renders the live topology with the `load` frame and
replays frames on the fixture clock; it fails when the p95 of `FrameTiming.buildDuration` is
above `viz.perf.frame_build_p95`. It needs macOS and runs in profile mode through
`tool/perf_macos.sh` (listed in `platform-tests.json`), so `make apps-platform` and `make perf`
run it on a Mac and print a SKIP line elsewhere. The script makes the ignored `macos/` folder
with `flutter create` on its first run and removes the other files that command adds.
