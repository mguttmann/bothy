// SPDX-License-Identifier: Apache-2.0

/// The guard of a step whose failure the app passes over, because the step belongs to an
/// addition that the app works without: push and the sharing glue of iOS and macOS
/// (docs/decisions.d/AUD-client-notifications-01.md AUD-CN1/4).
library;

import 'dart:async';

/// Runs [body], the step [what] of [area]. A failure is swallowed and leaves one line in
/// [log]: the area, the step and the type of the error, never a text of the error.
///
/// Example:
/// ```dart
/// await guarded(log, 'push', 'settings', writeSettings);
/// // after a failure: "push: settings failed (PlatformException)"
/// ```
Future<void> guarded(
  void Function(String line) log,
  String area,
  String what,
  FutureOr<void> Function() body,
) async {
  try {
    await body();
  } on Object catch (e) {
    log('$area: $what failed (${e.runtimeType})');
  }
}

/// The value of [body], the step [what] of [area], or null after a failure, which leaves the
/// line of [guarded] in [log].
Future<T?> guardedValue<T>(
  void Function(String line) log,
  String area,
  String what,
  Future<T> Function() body,
) async {
  try {
    return await body();
  } on Object catch (e) {
    log('$area: $what failed (${e.runtimeType})');
    return null;
  }
}
