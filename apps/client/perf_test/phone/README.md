# Performance level of the phone apps

The measurement of the phone targets `client.perf.*` of
[15 19](../../../../docs/spec/15-client-architecture.md#19-performance-targets) (brief M5-11,
Design note 5). It runs as a profile build on an attached physical iPhone or Android phone:

```sh
apps/client/tool/mobile_perf.sh                              # the first attached physical phone
MOBILE_PERF_DEVICE=<flutter device id> apps/client/tool/mobile_perf.sh
```

Without a physical phone (or without `flutter` and `dart` on PATH, with the reason on stderr) the
script prints exactly one line,
`SKIP perf-mobile: no physical reference device attached (targets not proven)`, and ends with 0;
then only the unit tests of the evaluation (`test/perf/`) speak for the level. The iOS simulator
builds only in debug mode and an emulator is no phone of 15 19, so neither counts as attached.

## Files

| File | Purpose |
|---|---|
| `mobile_perf_test.dart` | the measurement: cold start, thread open, local echo, patch to frame, jank ratio, catch-up rate, memory with five open threads |
| `mobile_perf_app.dart` | the app under measurement with the ports of a phone build (platform device keys and secure storage, encrypted database on its own isolate, the scheduler's frames) |
| `perf_fake.dart` | the fake server of M4-17 with the world of the measurement on its own isolate |
| `mobile_device.dart` | the model identifier, system and process age of the phone, read in the app from the platform |
| `mobile_perf_summary.dart` | the evaluation: targets from `AppConfig`, the fixed list of reference devices, the result |
| `mobile_perf_cli.dart` | the helper of `tool/mobile_perf.sh` (attached phones, report and exit code of a summary) |

The driver is `test_driver/mobile_perf_driver.dart`; it writes `build/perf/mobile-summary.json`
(or the file `MOBILE_PERF_SUMMARY` names), also after a failed run.

## What a result means

| Exit code | Result |
|---|---|
| 0 | a reference device met every target (evidence for 15 19), or the SKIP line |
| 1 | a target was missed on a reference device, the run failed, or no summary was written |
| 2 | `MOBILE_PERF_DEVICE` names a device that is not among the attached physical phones (without any attached phone the script skips), or the device list failed |
| 3 | the phone is no reference device: the values are reported, the targets are not proven |

Only the models in `mobileReferenceDevices` of `mobile_perf_summary.dart` count as reference
devices; each entry names its reason. The app reads the model from the platform itself
(`utsname.machine` on iOS, `ro.product.model` on Android), so no setting on the host can turn a
run into evidence. The summary names the device, its system, whether it is a reference device,
and per target the value, the limit, the samples and the verdict.

## Why the fake server inside the app

The desktop level measures against the Compose stack. A profile build of a phone cannot reach
that stack: only debug builds of the phone apps trust the stack's test CA. The fake server of
M4-17 inside the app needs nothing but the phone, and its patches and pages isolate the client
from network and server load. It runs on its own isolate, so its contract checks run on another
core; the isolates share one heap, so its garbage collection can still pause the app, and its
memory stays in the resident size. Both can only make the values worse, never better.

## Measurement points

The same points as the desktop level (docs/decisions.d/M4-34.md, D-M4-34-5), with these
differences:

- The cold start adds to each of its samples the time from the start of the process to the entry
  of `main` (process, engine and Dart VM), read once from the platform (`kinfo_proc` on iOS,
  `/proc/self/stat` and the runner's time since the boot on Android); without it the target is
  not measured and fails. The run starts the app without waiting for the driver
  (`--no-start-paused`), so that time holds no wait of the tool. What a process does only once
  after `main` (the binding, fonts, loading the database library) is not repeated by the
  restarts, as on the desktop level.
- The catch-up records when the frames that open and close it arrived (`catchup_delivery_ms`
  next to `catchup_ms` in the details), so a slow rate shows whether the delivery or the client
  took the time.
- The jank ratio counts the frames of continuous scroll gestures, without idle pauses between
  them.
- Memory is `ProcessInfo.currentRss` with five threads in the gateway focus: the resident set the
  kernel reports for the process (`resident_size` of the Mach task on iOS, the resident pages of
  `/proc/self/statm` on Android), not the footprint the system limits an app by.

A run with `flutter test perf_test/phone/mobile_perf_test.dart` on the Flutter tester checks the
flow of the measurement on a development host (software keys and memory secrets, no profile
build); its values are never evidence.

The phone must stay unlocked during a run (the database key is readable only while the device is
unlocked), and it needs a screen lock (a passcode on iOS, a PIN or pattern on Android): the app's
step-up key cannot be created without one, and the app then cannot open its instance.
