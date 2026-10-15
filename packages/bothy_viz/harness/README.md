# bothy_viz_harness

Harness app of [`bothy_viz`](../README.md) (docs/spec/25-visualisierung.md 7.1, row `harness/`):
it renders every basic form (and, from M6-12, every view) from the fixtures of
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
`no_cost_permission`, `offline`, `load`; a schema without a file of that state shows `normal`).
The time-lapse slider moves the fake clock of the fixture live source by one frame interval per
step, so the replayed frames and then the stale marker (`viz.stale_after`) appear.

## Tests

`test/harness_test.dart` pumps the app without a server and switches every toggle once; run it
with `make test COMPONENTS=apps` or `flutter test` in this folder.
