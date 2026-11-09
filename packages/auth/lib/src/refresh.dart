// SPDX-License-Identifier: Apache-2.0

/// Refresh as a single flight per refresh family (08 7.4, 15 8, R-CL-07; brief M4-09 Design
/// note 4).
library;

import 'dart:async';

import 'package:api_client/api_client.dart';
import 'package:auth/src/auth_events.dart';
import 'package:auth/src/errors.dart';
import 'package:auth/src/session_core.dart';
import 'package:dio/dio.dart';

/// The value the generated login operations get for their required `DPoP` header parameter;
/// the `DpopInterceptor` replaces it with a fresh proof on every attempt, so a nonce retry
/// signs anew (06 3.1 items 2 and 3).
const String proofFromInterceptor = 'pending';

/// Problem codes of a refresh that mean the refresh token is no longer usable.
///
/// `invalid_dpop_proof` is not one of them: the server checks the proof before it looks at the
/// refresh token and revokes nothing for a bad proof (08 7.3), so the token is still good. A
/// proof that arrives too old (signed before the device slept) is signed anew once by the
/// `DpopInterceptor`; when that fails too, the refresh is [RefreshUnavailable] and the
/// session stays.
const Set<String> _rejectedCodes = {'unauthenticated', 'token_expired'};

/// Connect, send and receive timeout of one refresh request.
///
/// Together they end a refresh well inside `auth.dpop_proof_max_age` (default 60 s, 08 7.3),
/// so a request whose proof was signed before the device slept fails as a network error
/// instead of reaching the server minutes later with an expired proof.
const Duration refreshRequestTimeout = Duration(seconds: 15);

/// Runs at most one refresh of the session's family at a time.
///
/// Two refreshes with one refresh token would look like reuse to the server and revoke the
/// family and the session (08 7.4); so every caller that needs a refresh while one runs
/// awaits the same future. Order of one refresh: `refreshToken` with a proof of the DPoP key
/// (the family's key, RFC 9449 5), write the new refresh token to the `SecretStore` and wait
/// for the write, then publish the new access token (docs/DECISIONS.md D-M4-09-3).
///
/// Example:
/// ```dart
/// final token = await session.refresh.ensureFresh(); // refreshes only when expired
/// await session.refresh.refreshNow(); // gateway `reauth_required` (M4-13)
/// ```
final class RefreshCoordinator {
  /// Internal: created by the session over its [SessionCore].
  RefreshCoordinator(this._core);

  final SessionCore _core;
  Future<String>? _inflight;

  /// Returns a valid access token: the current one while its `exp` lies ahead by the injected
  /// clock, else the one of a refresh (joining a running refresh).
  ///
  /// Throws [NotSignedIn] without a session, [SessionEnded] when the server ended it, and
  /// [RefreshUnavailable] on a network or transient error (the old state stays).
  Future<String> ensureFresh() {
    final state = _core.tokens.state;
    if (state != null && !state.isExpiredAt(_core.clock.now())) {
      return Future.value(state.accessToken);
    }
    return refreshNow();
  }

  /// Refreshes now, or joins the refresh that is running, and returns the new access token.
  ///
  /// On `401 session_revoked` the session ends with [SignOutReason.sessionRevoked]; on
  /// `401 unauthenticated` or `401 token_expired` with [SignOutReason.refreshRejected]; both
  /// clear the store, emit `SignedOut` and throw [SessionEnded]. A network error, a timeout
  /// ([refreshRequestTimeout]), a `401 invalid_dpop_proof` that remains after one repeat with a
  /// fresh proof, or any other answer keeps the state and the stored refresh token, emits
  /// `RefreshFailed` and throws [RefreshUnavailable].
  Future<String> refreshNow() {
    final running = _inflight;
    if (running != null) return running;
    final next = _run();
    _inflight = next;
    // Clear the slot when this refresh ends, whichever way; a later caller starts a new one.
    unawaited(
      next.then<void>((_) {}, onError: (Object _) {}).whenComplete(() {
        if (identical(_inflight, next)) _inflight = null;
      }),
    );
    return next;
  }

  Future<String> _run() async {
    final generation = _core.generation;
    final refreshToken = await _core.secrets.read(_core.refreshTokenKey);
    if (refreshToken == null) throw const NotSignedIn();
    _core.log('refresh started');
    final Response<TokenResponse> response;
    try {
      response = await _core.api.getAuthApi().refreshToken(
        dPoP: proofFromInterceptor,
        tokenRefreshRequest: TokenRefreshRequest(
          (b) => b..refreshToken = refreshToken,
        ),
        extra: {
          internalRequestExtra: true,
          requestTimeoutExtra: refreshRequestTimeout,
        },
      );
    } on DioException catch (e) {
      throw await _failure(e, generation);
    }
    final state = await _core.adopt(
      response.data!,
      generationAtStart: generation,
    );
    if (state == null) throw const SessionEnded(SignOutReason.loggedOut);
    return state.accessToken;
  }

  Future<AuthException> _failure(DioException e, int generation) async {
    final own = e.error;
    if (own is AuthException) return own;
    if (generation != _core.generation) {
      // The session this refresh belonged to ended while it ran; its late failure must not
      // touch the store or the events of the session that replaced it.
      return const SessionEnded(SignOutReason.loggedOut);
    }
    final problem = problemOf(e);
    if (problem != null && problem.status == 401) {
      final reason = problem.code == 'session_revoked'
          ? SignOutReason.sessionRevoked
          : _rejectedCodes.contains(problem.code)
          ? SignOutReason.refreshRejected
          : null;
      if (reason != null) {
        _core.log('refresh rejected: ${problem.code}');
        await _core.signOut(reason);
        return SessionEnded(reason);
      }
    }
    final status = problem?.status;
    _core.log(
      'refresh failed: ${status == null ? 'network error' : 'status $status'}',
    );
    _core.emit(RefreshFailed(status: status));
    return RefreshUnavailable(status: status);
  }
}
