// SPDX-License-Identifier: Apache-2.0

/// The token state of one session: the access token in memory, what the app may know about
/// the session, and the shared holder the interceptors read (08 7, 15 8).
library;

import 'dart:convert';

import 'package:auth/src/redact.dart';
import 'package:domain/domain.dart';

/// What the app may show or log about the signed-in session; contains no secret.
final class SessionInfo {
  /// Creates the information.
  const SessionInfo({
    required this.sid,
    required this.deviceId,
    required this.accessExpiresAt,
  });

  /// The session ID (`sid` of the access token, `Session.id`); signed into every step-up JWS.
  final String sid;

  /// The device of the session.
  final DeviceId deviceId;

  /// When the current access token expires (its `exp`, else `expires_in` after receipt).
  final DateTime accessExpiresAt;

  @override
  String toString() =>
      'SessionInfo(sid: $sid, device: ${deviceId.value}, '
      'access expires: ${accessExpiresAt.toIso8601String()})';
}

/// A token pair as the server issued it (`TokenResponse`) and the session it belongs to.
///
/// [toString] prints the kind and length of the tokens only (Design note 7).
final class TokenState {
  /// Creates the state.
  const TokenState({
    required this.accessToken,
    required this.refreshToken,
    required this.session,
  });

  /// The DPoP-bound access token (`Authorization: DPoP <token>`); kept in memory only.
  final String accessToken;

  /// The refresh token of the family; persisted in the `SecretStore` before use.
  final String refreshToken;

  /// The session the tokens belong to.
  final SessionInfo session;

  /// Whether the access token has expired at [now].
  bool isExpiredAt(DateTime now) => !now.isBefore(session.accessExpiresAt);

  @override
  String toString() =>
      'TokenState(access: ${redacted('access token', accessToken)}, '
      'refresh: ${redacted('refresh token', refreshToken)}, session: $session)';
}

/// Reads the `exp` claim (seconds) of a JWS access token without checking it; the client only
/// uses it to decide when to refresh, the server checks the token (08 7.2).
///
/// Returns `null` when [token] is not a JWS with a numeric `exp`.
DateTime? accessTokenExpiry(String token) {
  final parts = token.split('.');
  if (parts.length != 3) return null;
  try {
    final json = jsonDecode(
      utf8.decode(base64Url.decode(base64Url.normalize(parts[1]))),
    );
    final exp = json is Map ? json['exp'] : null;
    if (exp is! num) return null;
    return DateTime.fromMillisecondsSinceEpoch(
      (exp * 1000).toInt(),
      isUtc: true,
    );
  } on FormatException {
    return null;
  }
}

/// The mutable token state of one `AuthSession`, shared by the coordinator and the
/// interceptors; nothing outside the session writes it.
final class TokenHolder {
  TokenState? _state;
  int _version = 0;

  /// Counts every change of [state]; a request records it, so a `401` for an older token does
  /// not start a second refresh.
  int get version => _version;

  /// The current state, or `null` while signed out or before the first refresh after a
  /// restart.
  TokenState? get state => _state;

  /// Publishes [state] as the current one; `null` clears it.
  set state(TokenState? state) {
    _state = state;
    _version++;
  }

  @override
  String toString() => 'TokenHolder(${_state == null ? 'empty' : 'set'})';
}
