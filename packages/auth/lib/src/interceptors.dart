// SPDX-License-Identifier: Apache-2.0

/// The Dio interceptors that plug login into the client factory of M4-02 (its slot between
/// the headers and the problem interceptor).
///
/// Order in the slot, which is also the order Dio runs their `onError`: [RefreshInterceptor],
/// [DpopInterceptor], [StepUpInterceptor]. The refresh runs before the proof is built, so the
/// proof's `ath` always hashes the token that is sent; each `onError` handles only its own
/// problem code and passes every other error on.
library;

import 'package:auth/src/auth_events.dart';
import 'package:auth/src/dpop.dart';
import 'package:auth/src/errors.dart';
import 'package:auth/src/refresh.dart';
import 'package:auth/src/session_core.dart';
import 'package:auth/src/step_up.dart';
import 'package:dio/dio.dart';

const _nonceRetried = 'auth.nonce_retried';
const _refreshRetried = 'auth.refresh_retried';
const _stepUpRetried = 'auth.step_up_retried';
const _tokenVersion = 'auth.token_version';
const _generation = 'auth.generation';

/// Repeats the request of [err] through every interceptor of [core]'s client and hands the
/// result to [handler]; every auth repeat goes through here.
///
/// A late answer to a request of an earlier session is never resent with the token of the
/// session that replaced it. A `FormData` body is sent as a fresh clone, because Dio finalizes
/// a `FormData` once; a stream body cannot be read twice, so its first error goes back to the
/// caller instead of a broken repeat.
Future<void> _repeat(
  SessionCore core,
  DioException err,
  ErrorInterceptorHandler handler,
) async {
  final options = err.requestOptions;
  final sentIn = options.extra[_generation];
  if (sentIn != null && sentIn != core.generation) return handler.next(err);
  final data = options.data;
  if (data is FormData) {
    options.data = data.clone();
  } else if (data is Stream) {
    return handler.next(err);
  }
  try {
    handler.resolve(await core.api.dio.fetch<Object?>(options));
  } on DioException catch (e) {
    handler.next(e);
  }
}

/// Refreshes before a request whose token has expired and once after `401 token_expired` or
/// `401 unauthenticated`; ends the session on `401 session_revoked` (brief M4-09 Design notes
/// 4 and requirement 17).
///
/// Only operations that need a person's token are touched; the login operations
/// (`security: []`) pass unchanged.
class RefreshInterceptor extends Interceptor {
  /// Internal: created by the session over its [SessionCore].
  RefreshInterceptor(this._core, this._refresh);

  final SessionCore _core;
  final RefreshCoordinator _refresh;

  @override
  Future<void> onRequest(
    RequestOptions options,
    RequestInterceptorHandler handler,
  ) async {
    if (!needsToken(options)) return handler.next(options);
    final state = _core.tokens.state;
    if (state == null || state.isExpiredAt(_core.clock.now())) {
      try {
        await _refresh.ensureFresh();
      } on AuthException catch (e) {
        return handler.reject(authError(options, e));
      }
    }
    handler.next(options);
  }

  @override
  Future<void> onError(
    DioException err,
    ErrorInterceptorHandler handler,
  ) async {
    final options = err.requestOptions;
    final problem = problemOf(err);
    if (!needsToken(options) || problem == null || problem.status != 401) {
      return handler.next(err);
    }
    if (problem.code == 'session_revoked') {
      // R-SY-14: a revoked session or device ends here; the app deletes local data. A late
      // answer to a request of an earlier session (sent before a logout and a new login) must
      // not end the session that replaced it.
      await _core.signOutIfCurrent(
        options.extra[_generation] as int?,
        SignOutReason.sessionRevoked,
      );
      return handler.next(err);
    }
    if ((problem.code != 'token_expired' &&
            problem.code != 'unauthenticated') ||
        options.extra[_refreshRetried] == true) {
      return handler.next(err);
    }
    options.extra[_refreshRetried] = true;
    // Refresh only when the rejected token is still the current one; a request that was sent
    // with an older token just repeats with the token another caller already fetched.
    if (options.extra[_tokenVersion] == _core.tokens.version) {
      try {
        await _refresh.refreshNow();
      } on AuthException catch (e) {
        return handler.next(authError(options, e));
      }
    }
    await _repeat(_core, err, handler);
  }
}

/// Adds `Authorization: DPoP <token>` and a fresh `DPoP` proof, adopts every `DPoP-Nonce`, and
/// repeats once on `401 use_dpop_nonce` (06 3.1, RFC 9449 8; brief M4-09 Design note 3).
///
/// Operations that need a person's token get both headers, the proof with `ath`. Login
/// operations get a proof without `ath` where their header parameter `DPoP` asks for one
/// (`beginPairing`, `completePairing`, `completeOidcLogin` and the other completions) and no
/// `Authorization`. A second `use_dpop_nonce` goes to the caller.
class DpopInterceptor extends Interceptor {
  /// Internal: created by the session over its [SessionCore] with its [proofs].
  DpopInterceptor(this._core, this._proofs);

  final SessionCore _core;
  final DpopProofFactory _proofs;

  @override
  Future<void> onRequest(
    RequestOptions options,
    RequestInterceptorHandler handler,
  ) async {
    try {
      if (needsToken(options)) {
        final state = _core.tokens.state;
        if (state == null) {
          return handler.reject(authError(options, const NotSignedIn()));
        }
        options.extra[_tokenVersion] = _core.tokens.version;
        options.extra[_generation] = _core.generation;
        options.headers['Authorization'] = 'DPoP ${state.accessToken}';
        options.headers['DPoP'] = await _proofs.forRequest(
          method: options.method,
          url: options.uri,
          accessToken: state.accessToken,
        );
      } else if (carriesProofHeader(options)) {
        // The header map of Dio ignores case, so this replaces the placeholder.
        options.headers['DPoP'] = await _proofs.forRequest(
          method: options.method,
          url: options.uri,
        );
      }
    } on Object catch (e) {
      // A signer failure (lost key, StateError of a bad signature) ends this request only.
      return handler.reject(authError(options, e));
    }
    handler.next(options);
  }

  @override
  void onResponse(
    Response<dynamic> response,
    ResponseInterceptorHandler handler,
  ) {
    _core.nonces.update(response.headers.value('dpop-nonce'));
    handler.next(response);
  }

  @override
  Future<void> onError(
    DioException err,
    ErrorInterceptorHandler handler,
  ) async {
    _core.nonces.update(err.response?.headers.value('dpop-nonce'));
    final options = err.requestOptions;
    final problem = problemOf(err);
    final proofRequest = needsToken(options) || carriesProofHeader(options);
    if (!proofRequest ||
        problem == null ||
        problem.status != 401 ||
        problem.code != 'use_dpop_nonce' ||
        options.extra[_nonceRetried] == true) {
      return handler.next(err);
    }
    options.extra[_nonceRetried] = true;
    _core.log('nonce retry');
    await _repeat(_core, err, handler);
  }
}

/// Key of `RequestOptions.extra` with the text the step-up prompt shows for this request,
/// already in the person's language (the UI passes it from `packages/l10n`).
///
/// Example:
/// ```dart
/// await api.getDevicesApi().approveDevice(
///   deviceId: id,
///   deviceApprove: body,
///   extra: {idempotencyKeyExtra: key, stepUpPurposeExtra: l10n.stepUpApproveDevice},
/// );
/// ```
const String stepUpPurposeExtra = 'auth.step_up_purpose';

/// On `401 step_up_required` (any `reason`, `terminal_step_up_not_fresh` included) runs the
/// step-up once and repeats the original request with the same `Idempotency-Key` and body, a
/// new proof and the current token (08 6.2, 6.3, 14.3; brief M4-09 Design note 6).
///
/// A cancelled prompt ends the request with [StepUpCancelled]; a second `step_up_required`
/// after a successful confirmation goes to the caller, never into a loop.
class StepUpInterceptor extends Interceptor {
  /// Internal: created by the session over its [SessionCore] with its [stepUp] service.
  StepUpInterceptor(this._core, this._stepUp);

  final SessionCore _core;
  final StepUpService _stepUp;

  @override
  Future<void> onError(
    DioException err,
    ErrorInterceptorHandler handler,
  ) async {
    final options = err.requestOptions;
    final problem = problemOf(err);
    if (!needsToken(options) ||
        options.extra[internalRequestExtra] == true ||
        problem == null ||
        problem.status != 401 ||
        problem.code != 'step_up_required' ||
        options.extra[_stepUpRetried] == true) {
      return handler.next(err);
    }
    options.extra[_stepUpRetried] = true;
    try {
      await _stepUp.confirm(
        purposeText: options.extra[stepUpPurposeExtra] as String? ?? '',
      );
    } on DioException catch (e) {
      return handler.next(e);
    } on Object catch (e) {
      return handler.next(authError(options, e));
    }
    // The headers interceptor writes the same Idempotency-Key again from the request's extra;
    // the DPoP interceptor signs a new proof for the current token.
    await _repeat(_core, err, handler);
  }
}
