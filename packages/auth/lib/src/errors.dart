// SPDX-License-Identifier: Apache-2.0

/// The errors of `auth`; none carries a secret (brief M4-09 Design note 7).
library;

import 'package:auth/src/auth_events.dart';

/// Base of every error this package throws or puts into a `DioException`.
sealed class AuthException implements Exception {
  const AuthException();
}

/// There is no session: never signed in, or signed out before.
final class NotSignedIn extends AuthException {
  /// Creates the error.
  const NotSignedIn();

  @override
  String toString() => 'NotSignedIn: no session; sign in first';
}

/// The session ended while a call waited for it (revoked, refresh rejected, logged out).
final class SessionEnded extends AuthException {
  /// Creates the error with the [reason] the `SignedOut` event also carries.
  const SessionEnded(this.reason);

  /// Why the session ended.
  final SignOutReason reason;

  @override
  String toString() => 'SessionEnded: ${reason.name}';
}

/// A refresh could not reach the server or got a transient error; the old state stays and the
/// caller retries later with backoff (brief M4-09 Design note 4).
final class RefreshUnavailable extends AuthException {
  /// Creates the error; [status] is the HTTP status of a transient answer, `null` for a
  /// network error.
  const RefreshUnavailable({this.status});

  /// HTTP status of the failed refresh, or `null` without a response.
  final int? status;

  @override
  String toString() =>
      'RefreshUnavailable: ${status == null ? 'network error' : 'status $status'}';
}

/// The person declined the step-up prompt or the operating system's confirmation.
final class StepUpCancelled extends AuthException {
  /// Creates the error.
  const StepUpCancelled();

  @override
  String toString() => 'StepUpCancelled: the confirmation was cancelled';
}

/// No step-up method is left for this session: the device has no step-up key (Windows by
/// design, 08 6.1, D-M4-11-2) and the server refuses the step-up with password and TOTP
/// (`403 step_up_not_possible`, for example a person without a local account).
final class StepUpUnavailable extends AuthException {
  /// Creates the error.
  const StepUpUnavailable();

  @override
  String toString() =>
      'StepUpUnavailable: no step-up key and no password step-up for this session';
}
