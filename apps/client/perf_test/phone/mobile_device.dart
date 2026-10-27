// SPDX-License-Identifier: Apache-2.0

// What the phone under measurement reports about itself, read in the app (brief M5-11 Design
// note 5): the model identifier and system for the summary and the reference list, and the
// age of the app's process for the cold start. The values come from the platform inside the
// measured process, not from a label the host passes in, so no switch on the host can turn a
// run into evidence.
//
// - iOS: the model identifier is `utsname.machine` of `uname(3)` (for example `iPhone17,3`), the
//   system `Platform.operatingSystemVersion`, the start of the process `p_starttime` of
//   `sysctl(3)` with `KERN_PROC_PID`, all through dart:ffi on the C library of the process;
//   macOS shares these interfaces, so the unit test checks them on a Mac host.
// - Android: `ro.product.manufacturer`, `ro.product.model`, `ro.build.version.release` and
//   `ro.build.version.sdk` of `getprop`, the start of the process from `/proc/self/stat` and the
//   time since the boot from the runner (`SystemClock.elapsedRealtime()`; the system refuses an
//   app `/proc/uptime`).
//
// Anything that cannot be read gives the model [unknownModel] or no age: a run on such a device
// is never evidence (mobile_perf_summary.dart) and the cold start is not measured.

import 'dart:convert';
import 'dart:ffi';
import 'dart:io';

import 'package:client/platform/android_platform.dart' show AndroidPlatformApi;

import 'mobile_perf_summary.dart';

/// The model or maker of a device whose platform did not tell it.
const String unknownModel = 'unknown';

/// The phone this process runs on; on another host the system of the host with
/// [unknownModel] as the maker (a run there is never evidence).
Future<MobileDevice> readMobileDevice() async {
  if (Platform.isAndroid) return androidDevice(await _getprop());
  if (Platform.isIOS) {
    return MobileDevice(
      platform: 'ios',
      manufacturer: 'Apple',
      model: darwinMachine() ?? unknownModel,
      os: 'iOS ${Platform.operatingSystemVersion}',
    );
  }
  return MobileDevice(
    platform: Platform.operatingSystem,
    manufacturer: unknownModel,
    model: (Platform.isMacOS ? darwinMachine() : null) ?? unknownModel,
    os: '${Platform.operatingSystem} ${Platform.operatingSystemVersion}',
  );
}

/// The Android device described by the system properties [props] (the output of
/// [parseGetprop]); a missing property gives [unknownModel].
MobileDevice androidDevice(Map<String, String> props) {
  String prop(String name) {
    final v = props[name]?.trim() ?? '';
    return v.isEmpty ? unknownModel : v;
  }

  return MobileDevice(
    platform: 'android',
    manufacturer: prop('ro.product.manufacturer'),
    model: prop('ro.product.model'),
    os:
        'Android ${prop('ro.build.version.release')} '
        '(API ${prop('ro.build.version.sdk')})',
  );
}

/// The properties in [output] of `getprop` without arguments: one `[name]: [value]` per line;
/// other lines are skipped.
Map<String, String> parseGetprop(String output) {
  final line = RegExp(r'^\[([^\]]+)\]: \[(.*)\]$');
  return {
    for (final l in const LineSplitter().convert(output))
      if (line.firstMatch(l.trim()) case final m?) m.group(1)!: m.group(2)!,
  };
}

Future<Map<String, String>> _getprop() async {
  try {
    final r = await Process.run('/system/bin/getprop', const []);
    return r.exitCode == 0 ? parseGetprop(r.stdout as String) : const {};
  } on ProcessException {
    return const {};
  }
}

/// The moment `main` was entered and what the platform tells about the start of the process,
/// for the cold start: the time from the start of the process to the entry of `main` (process,
/// engine and Dart VM), which the restarts of the app inside the process cannot repeat.
///
/// Example: `final entry = MainEntry.record();` first in `main`, later
/// `final share = await entry.processToMain();`.
final class MainEntry {
  MainEntry._(this._startTicks, this._darwinAge, this._sinceMain);

  /// The entry of `main` of an Android process that started [startTicks] clock ticks after the
  /// boot (null when `/proc/self/stat` could not be read).
  MainEntry.android(int? startTicks)
    : this._(startTicks, null, Stopwatch()..start());

  /// Records the entry of `main`; call it first in `main`. On iOS and macOS the age is read at
  /// once from the kernel; on Android the start of the process comes from `/proc/self/stat` now
  /// and the time since the boot later ([processToMain]), because the system lets an app read
  /// neither `/proc/uptime` nor its clock without the runner.
  factory MainEntry.record() {
    if (Platform.isAndroid) {
      int? ticks;
      try {
        ticks = processStartTicks(File('/proc/self/stat').readAsStringSync());
      } on FileSystemException {
        ticks = null;
      }
      return MainEntry.android(ticks);
    }
    final start = darwinProcessStart();
    return MainEntry._(
      null,
      start == null ? null : DateTime.now().difference(start),
      Stopwatch()..start(),
    );
  }

  final int? _startTicks;
  final Duration? _darwinAge;
  final Stopwatch _sinceMain;

  /// The time from the start of the process to the entry of `main`, or null where the platform
  /// did not tell it. On Android it asks [sinceBootMillis] for the time since the boot, by
  /// default the runner's `SystemClock.elapsedRealtime()` (`AndroidPlatformApi`), so the
  /// binding must be initialised; that clock counts deep sleep, like the start in
  /// `/proc/self/stat`. Throws the `PlatformException` of the runner when it cannot answer.
  Future<Duration?> processToMain({
    Future<int> Function()? sinceBootMillis,
  }) async {
    final darwin = _darwinAge;
    if (darwin != null) return darwin;
    final ticks = _startTicks;
    if (ticks == null) return null;
    final boot =
        await (sinceBootMillis ?? AndroidPlatformApi().sinceBootMillis)();
    return ageAtMain(
      startTicks: ticks,
      sinceBoot: Duration(milliseconds: boot),
      sinceMain: _sinceMain.elapsed,
    );
  }
}

/// The start of a process after the boot in clock ticks: field 22 of its `/proc/<pid>/stat`
/// [stat], or null when the line cannot be read. The name in field 2 may hold spaces and
/// parentheses, so the fields are counted after its last `)`.
int? processStartTicks(String stat) {
  final close = stat.lastIndexOf(')');
  if (close < 0) return null;
  final fields = stat.substring(close + 1).trim().split(RegExp(r'\s+'));
  // fields[0] is field 3 (the state), so field 22 is at index 19.
  if (fields.length < 20) return null;
  return int.tryParse(fields[19]);
}

/// The age of a process at the entry of `main`: [sinceBoot] (asked [sinceMain] after the
/// entry) less [sinceMain] and less the start of the process, [startTicks] clock ticks of
/// `USER_HZ` after the boot, which Linux fixes at 100 per second for user space on arm, arm64
/// and x86. Null when the start lies after the entry (clocks that cannot be).
Duration? ageAtMain({
  required int startTicks,
  required Duration sinceBoot,
  required Duration sinceMain,
}) {
  final age = sinceBoot - sinceMain - Duration(milliseconds: startTicks * 10);
  return age.isNegative ? null : age;
}

// The C library of the process (libSystem on iOS and macOS).
final DynamicLibrary _libc = DynamicLibrary.process();

final Pointer<Uint8> Function(int) _malloc = _libc
    .lookupFunction<
      Pointer<Uint8> Function(Size),
      Pointer<Uint8> Function(int)
    >('malloc');

final void Function(Pointer<Uint8>) _free = _libc
    .lookupFunction<
      Void Function(Pointer<Uint8>),
      void Function(Pointer<Uint8>)
    >('free');

/// `_SYS_NAMELEN` of Darwin's `<sys/utsname.h>`: each of the five fields of `struct utsname`
/// (sysname, nodename, release, version, machine) is a character array of this length.
const int _darwinNameLength = 256;

/// The hardware model identifier of a Darwin device, `utsname.machine` of `uname(3)`: on an
/// iPhone for example `iPhone17,3`, on a Mac the architecture (`arm64`). Null elsewhere or when
/// the call fails.
String? darwinMachine() {
  if (!Platform.isIOS && !Platform.isMacOS) return null;
  final uname = _libc
      .lookupFunction<
        Int32 Function(Pointer<Uint8>),
        int Function(Pointer<Uint8>)
      >('uname');
  final buffer = _malloc(5 * _darwinNameLength);
  try {
    if (uname(buffer) != 0) return null;
    final bytes = <int>[];
    for (var i = 4 * _darwinNameLength; i < 5 * _darwinNameLength; i++) {
      final b = buffer[i];
      if (b == 0) break;
      bytes.add(b);
    }
    return bytes.isEmpty ? null : latin1.decode(bytes);
  } finally {
    _free(buffer);
  }
}

/// The start of this process on a Darwin system, `kp_proc.p_starttime` of the `kinfo_proc` that
/// `sysctl(3)` returns for `CTL_KERN`, `KERN_PROC`, `KERN_PROC_PID` and the own process ID; null
/// elsewhere or when the call fails.
///
/// `p_starttime` is the first member of `struct extern_proc`, which is the first member of
/// `struct kinfo_proc`, so it sits at offset 0: a `struct timeval` of a 64-bit `tv_sec` and a
/// 32-bit `tv_usec` (64-bit Darwin, the only Darwin Flutter builds for).
DateTime? darwinProcessStart() {
  if (!Platform.isIOS && !Platform.isMacOS) return null;
  final sysctl = _libc
      .lookupFunction<
        Int32 Function(
          Pointer<Int32>,
          Uint32,
          Pointer<Uint8>,
          Pointer<Uint64>,
          Pointer<Void>,
          Size,
        ),
        int Function(
          Pointer<Int32>,
          int,
          Pointer<Uint8>,
          Pointer<Uint64>,
          Pointer<Void>,
          int,
        )
      >('sysctl');
  const ctlKern = 1;
  const kernProc = 14;
  const kernProcPid = 1;
  const infoBytes = 1024;
  final mib = _malloc(4 * sizeOf<Int32>()).cast<Int32>();
  final info = _malloc(infoBytes);
  final length = _malloc(sizeOf<Uint64>()).cast<Uint64>();
  try {
    mib[0] = ctlKern;
    mib[1] = kernProc;
    mib[2] = kernProcPid;
    mib[3] = pid;
    length.value = infoBytes;
    if (sysctl(mib, 4, info, length, nullptr, 0) != 0) return null;
    if (length.value < 16) return null;
    final seconds = info.cast<Int64>().value;
    final micros = Pointer<Int32>.fromAddress(info.address + 8).value;
    if (seconds <= 0) return null;
    return DateTime.fromMicrosecondsSinceEpoch(seconds * 1000000 + micros);
  } finally {
    _free(mib.cast<Uint8>());
    _free(info);
    _free(length.cast<Uint8>());
  }
}
