// SPDX-License-Identifier: Apache-2.0

/// Faults of the app in the screens of the administration (ruling AUD-AM1/3).
///
/// A screen catches what a load or an action throws so that it can say something instead of
/// staying empty. Two kinds of error are expected there: an answer of the server or its absence
/// ([SelfActionFailed]) and an error of the login layer ([AuthException]: a cancelled step-up,
/// a session that is gone). Everything else is a fault of the app itself, for example a wrong
/// cast or a value the app could not read. Such a fault is no server error and must not vanish
/// in a banner: [reportAppFault] hands it to [adminFaultReporter], and the screen says that the
/// app failed.
library;

import 'package:auth/auth.dart' show AuthException;
import 'package:flutter/foundation.dart';

import '../data/self_repository.dart' show SelfActionFailed;

/// Receives a fault of the app that a screen caught: the [error] and the [stack] of the place
/// that threw it, or of the place that read it when the error carries none.
typedef AdminFaultReporter = void Function(Object error, StackTrace stack);

/// The receiver of the faults of the app. A host may set its own, for example one that writes
/// to its log; the default is [reportFaultToFlutter].
AdminFaultReporter adminFaultReporter = reportFaultToFlutter;

/// Hands [error] to [FlutterError.reportError], so it reaches the error handler of the app
/// (`FlutterError.onError`): the console of a debug build, and the crash reports of a host that
/// collects them.
void reportFaultToFlutter(Object error, StackTrace stack) =>
    FlutterError.reportError(
      FlutterErrorDetails(
        exception: error,
        stack: stack,
        library: 'admin_ui',
        context: ErrorDescription('while a screen of the administration ran'),
      ),
    );

/// Whether [error] is a fault of the app: neither an answer of the server or its absence nor
/// an error of the login layer.
bool isAppFault(Object error) =>
    error is! SelfActionFailed && error is! AuthException;

/// The errors that were reported. A screen that keeps a failed load reads its error again at
/// every build, and one fault is one report.
final Expando<bool> _reported = Expando<bool>('admin_ui.reported');

/// Reports [error] through [adminFaultReporter] when it is a fault of the app ([isAppFault]),
/// once per error, and returns whether it is one.
///
/// [stack] is the stack of the `catch`; without it the stack is the one an [Error] carries, else
/// the stack of this call.
bool reportAppFault(Object error, [StackTrace? stack]) {
  if (!isAppFault(error)) return false;
  // An expando takes no number, text, boolean or record; such a value is never kept by a
  // screen in a way that reads it twice.
  final tracked =
      error is! num && error is! String && error is! bool && error is! Record;
  if (tracked) {
    if (_reported[error] ?? false) return true;
    _reported[error] = true;
  }
  adminFaultReporter(
    error,
    stack ?? (error is Error ? error.stackTrace : null) ?? StackTrace.current,
  );
  return true;
}
