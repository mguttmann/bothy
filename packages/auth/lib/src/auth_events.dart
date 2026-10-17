// SPDX-License-Identifier: Apache-2.0

/// The events of a session the app reacts to (brief M4-09 Scope): signed in, device pending,
/// signed out with a reason, refresh failed.
library;

import 'package:auth/src/redact.dart';
import 'package:auth/src/tokens.dart';
import 'package:domain/domain.dart';

/// Why a session ended.
enum SignOutReason {
  /// The person logged out (`logout`).
  loggedOut,

  /// The server revoked the session, the device or the person (`401 session_revoked`, gateway
  /// `4012`); the app deletes its local data (R-SY-14).
  sessionRevoked,

  /// The pending device was rejected or expired (08 5.2); the app deletes its local data.
  deviceRevoked,

  /// The server no longer accepts the refresh token (expired, unknown, wrong key); a new
  /// login is needed.
  refreshRejected,
}

/// A device that waits for approval and the code it shows (08 5.2).
final class PendingDevice {
  /// Creates the description.
  const PendingDevice({required this.deviceId, this.verificationCode});

  /// The waiting device.
  final DeviceId deviceId;

  /// The digits the approving person types in; shown only on this device.
  final String? verificationCode;

  @override
  String toString() =>
      'PendingDevice(${deviceId.value}, code: '
      '${redacted('verification code', verificationCode)})';
}

/// An event of the session's [AuthSession.events] stream.
sealed class AuthEvent {
  const AuthEvent();
}

/// A login or an approval finished; the session is usable.
final class SignedIn extends AuthEvent {
  /// Creates the event.
  const SignedIn(this.session);

  /// The new session.
  final SessionInfo session;

  @override
  String toString() => 'SignedIn($session)';
}

/// The login succeeded but the device waits for approval (`403 device_pending` until then).
final class DevicePending extends AuthEvent {
  /// Creates the event.
  const DevicePending(this.device);

  /// The waiting device.
  final PendingDevice device;

  @override
  String toString() => 'DevicePending($device)';
}

/// The session ended; on [SignOutReason.sessionRevoked] and [SignOutReason.deviceRevoked]
/// the app deletes the local data of this server (R-SY-14).
final class SignedOut extends AuthEvent {
  /// Creates the event.
  const SignedOut(this.reason);

  /// Why it ended.
  final SignOutReason reason;

  /// Whether the app must delete the local data of this server.
  bool get deleteLocalData =>
      reason == SignOutReason.sessionRevoked ||
      reason == SignOutReason.deviceRevoked;

  @override
  String toString() => 'SignedOut(${reason.name})';
}

/// A refresh failed for a transient reason; the old state stays (brief M4-09 Design note 4).
final class RefreshFailed extends AuthEvent {
  /// Creates the event; [status] as in `RefreshUnavailable`.
  const RefreshFailed({this.status});

  /// HTTP status of the failed refresh, or `null` for a network error.
  final int? status;

  @override
  String toString() =>
      'RefreshFailed(${status == null ? 'network error' : 'status $status'})';
}
