// SPDX-License-Identifier: Apache-2.0

// The demo instance of one showcase run (brief M7-28 Design notes 1 and 2): what
// tools/showcase/capture.py read from `make demo` (infra/compose/demo.sh) and hands to the build as
// the define `SHOWCASE` (base64 of the JSON), because the test app runs in its own sandbox and
// cannot read the files of the demo. The run is local and the demo with its code is removed after
// it.

import 'dart:convert';
import 'dart:io';

import '../support/e2e_world.dart';

/// The connection data and the plan of one showcase run.
final class ShowcaseTarget implements E2eTarget {
  ShowcaseTarget._(Map<String, Object?> j)
    : server = Uri.parse(j['server']! as String),
      instanceId = j['instance_id']! as String,
      caPem = utf8.encode(j['ca_pem']! as String),
      code = j['code']! as String,
      platform = j['platform']! as String,
      upstreamHost = j['upstream_host'] as String?,
      themes = [for (final t in j['themes']! as List<Object?>) t! as String],
      captureWidth = j['capture_width']! as int,
      video = j['video'] as bool? ?? false {
    if (server.scheme != 'https' || !server.hasPort) {
      throw FormatException('server must be https with a port: $server');
    }
    if (!const {'android', 'ios', 'macos', 'windows'}.contains(platform)) {
      throw FormatException('unknown platform $platform');
    }
    if (themes.isEmpty || themes.any((t) => t != 'light' && t != 'dark')) {
      throw FormatException('themes must be light and/or dark: $themes');
    }
    if (captureWidth <= 0) {
      throw FormatException('capture_width must be positive: $captureWidth');
    }
  }

  /// Parses the JSON of capture.py; throws a [FormatException] naming a wrong value and a
  /// [TypeError] for a missing one.
  factory ShowcaseTarget.parse(String json) =>
      ShowcaseTarget._((jsonDecode(json) as Map).cast<String, Object?>());

  /// The target of the define `SHOWCASE`; throws a [StateError] naming the runner when it is
  /// missing (P-09: a run without a demo fails, it is never skipped).
  factory ShowcaseTarget.fromEnvironment() {
    const encoded = String.fromEnvironment('SHOWCASE');
    if (encoded.isEmpty) {
      throw StateError(
        'SHOWCASE is not defined; run the showcase with make showcase PLATFORM=<platform> '
        '(tools/showcase/capture.py), never on its own',
      );
    }
    return ShowcaseTarget.parse(utf8.decode(base64.decode(encoded)));
  }

  /// The address of the demo the app types, `https://localhost:<port>`.
  @override
  final Uri server;

  @override
  final String instanceId;

  /// The PEM of the demo CA of this run.
  final List<int> caPem;

  /// The enrollment code of Alex that `make demo` printed (valid once).
  final String code;

  /// The platform of the build: `android`, `ios`, `macos` or `windows`.
  final String platform;

  /// The address under which the device reaches the loopback of the build host, or null when
  /// it is the same machine (the Android emulator's alias of its host).
  final String? upstreamHost;

  /// The themes to capture, in order.
  final List<String> themes;

  /// The width in pixels of every image.
  final int captureWidth;

  /// Whether the run plays the flow of the video after the images (Android only: the runner
  /// records the screen with `adb shell screenrecord`).
  final bool video;

  /// Whether the build is a phone build.
  bool get phone => platform == 'android' || platform == 'ios';

  /// The address the tunnel connects to, or null for 127.0.0.1.
  InternetAddress? get upstreamAddress =>
      upstreamHost == null ? null : InternetAddress(upstreamHost!);
}
