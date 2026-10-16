// SPDX-License-Identifier: Apache-2.0

/// The session flow of the gateway: `hello`, `identify` or `resume`, `ready`, `resumed` and
/// `reauth` (04 6.3; brief M4-13 Design note 2).
library;

import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart';
import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';

import 'frames.dart';

/// The gateway protocol version of this client, the value of `info.x-protocol-versions` in
/// `docs/api/asyncapi.yaml` (04 6.1).
const int gatewayProtocolVersion = 1;

/// Builds the session frames of one person's device from the auth session and the stored sync
/// state.
///
/// The gateway proof travels in the frame, never in upgrade headers (04 6.3 item 3, R-SY-01):
/// `htm` `GET`, `htu` the HTTPS form of the gateway URL, the nonce of `hello` or
/// `reauth_required` and `ath` of the token, built by `DpopProofFactory.forGateway` of `auth`.
/// The `sync_epoch` of `hello`, `ready` and `resumed` is kept in memory; the stored epoch is
/// written together with the cursor by the catch-up of M4-14 (04 7.5 item 2).
///
/// Example:
/// ```dart
/// final session = GatewaySession(
///   auth: authSession,
///   syncState: db.syncStateDao,
///   gatewayUrl: Uri.parse('wss://chat.example.com/api/v1/gateway'),
///   client: clientInfo,
///   clock: const SystemClock(),
/// );
/// final frame = await session.answerHello(hello); // IdentifyFrame or ResumeFrame
/// ```
final class GatewaySession {
  /// Creates the session flow.
  GatewaySession({
    required this._auth,
    required this._syncState,
    required this.gatewayUrl,
    required this.client,
    required this._clock,
  });

  final AuthSession _auth;
  final SyncStateDao _syncState;
  final Clock _clock;

  /// The gateway address (`wss://<host>/api/v1/gateway`).
  final Uri gatewayUrl;

  /// Platform and version of the app for `identify`.
  final GatewayClientInfo client;

  /// When set, the next answer is `identify` even within the resume window (`4006`); cleared
  /// by `ready`.
  bool forceIdentify = false;

  /// The `sync_epoch` of the current `hello`, `ready` or `resumed`.
  String? syncEpoch;

  /// Whether `hello` offers this client's protocol version; otherwise the app needs an update
  /// and nothing is sent (Design note 2).
  bool supports(GatewayHello hello) =>
      hello.protocolVersions.contains(gatewayProtocolVersion);

  /// The answer to [hello]: `resume` when a gateway session is stored and the last disconnect
  /// lies less than `resume_window_ms` back (04 6.3 item 7), otherwise `identify` with cursor
  /// and epoch when stored (absent on the first connection, item 3).
  ///
  /// Stores the nonce of `hello` in the `NonceStore` of `auth`. Throws the `AuthException` of
  /// `RefreshCoordinator.ensureFresh` when no valid token can be had, and [NotSignedIn] when
  /// the session knows no device.
  Future<GatewayFrame> answerHello(GatewayHello hello) async {
    _auth.nonces.update(hello.dpopNonce);
    syncEpoch = hello.syncEpoch;
    final token = await _auth.refresh.ensureFresh();
    final state = await _syncState.read();
    final proof = await _auth.proofs.forGateway(
      gatewayUrl: gatewayUrl,
      accessToken: token,
      nonce: hello.dpopNonce,
    );
    final window = Duration(milliseconds: hello.resumeWindowMs);
    final sessionId = state.gatewaySessionId;
    final cursor = state.cursor;
    final disconnectedAt = state.disconnectedAt;
    if (!forceIdentify &&
        sessionId != null &&
        cursor != null &&
        disconnectedAt != null &&
        _clock.now().difference(disconnectedAt) < window) {
      return ResumeFrame(
        GatewayResume(
          (b) => b
            ..gatewaySessionId = sessionId
            ..accessToken = token
            ..dpop = proof
            ..cursor = cursor.value
            ..syncEpoch = state.syncEpoch,
        ),
      );
    }
    final device = _auth.session?.deviceId;
    if (device == null) throw const NotSignedIn();
    return IdentifyFrame(
      GatewayIdentify(
        (b) => b
          ..protocolVersion = gatewayProtocolVersion
          ..accessToken = token
          ..dpop = proof
          ..deviceId = device.value
          ..cursor = cursor?.value
          ..syncEpoch = cursor == null ? null : state.syncEpoch
          ..client = client.toBuilder(),
      ),
    );
  }

  /// Stores `gateway_session_id`, `permission_version` and `head_seq` of [ready] in
  /// `sync_state` (04 6.3 item 4) and keeps its epoch in memory.
  Future<void> onReady(GatewayReady ready) async {
    await _syncState.saveSession(
      gatewaySessionId: ready.gatewaySessionId,
      permissionVersion: PermissionVersion(ready.permissionVersion),
      headSeq: GlobalSeq(ready.headSeq),
    );
    syncEpoch = ready.syncEpoch;
    forceIdentify = false;
  }

  /// Keeps the epoch of [resumed] in memory.
  void onResumed(GatewayResumed resumed) => syncEpoch = resumed.syncEpoch;

  /// The stored cursor, the `last_seq` of heartbeats until the catch-up reports progress.
  Future<GlobalSeq?> storedCursor() async => (await _syncState.read()).cursor;

  /// Renews the token through `RefreshCoordinator.refreshNow` and returns `reauth` with a
  /// fresh gateway proof over the nonce of [required] (04 6.3 item 6).
  ///
  /// Throws the `AuthException` of the refresh: [SessionEnded] when the server ended the
  /// session, [RefreshUnavailable] on a transient failure.
  Future<ReauthFrame> answerReauth(GatewayReauthRequired required) async {
    _auth.nonces.update(required.dpopNonce);
    final token = await _auth.refresh.refreshNow();
    final proof = await _auth.proofs.forGateway(
      gatewayUrl: gatewayUrl,
      accessToken: token,
      nonce: required.dpopNonce,
    );
    return ReauthFrame(
      GatewayReauth(
        (b) => b
          ..accessToken = token
          ..dpop = proof,
      ),
    );
  }

  /// Stores the time of a disconnect, the start of the resume window (Design note 2).
  Future<void> markDisconnected() => _syncState.markDisconnected(_clock.now());
}
