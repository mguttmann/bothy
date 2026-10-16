// SPDX-License-Identifier: Apache-2.0

/// The interface through which catch-up and the dispatcher (M4-14) plug into the connection
/// (brief M4-13 Design note 5).
library;

import 'package:api_client/api_client.dart';
import 'package:domain/domain.dart';

import 'connection_state.dart';
import 'frames.dart';

/// Receives the session events of the gateway connection; M4-14 implements it.
///
/// The connection calls the methods one after another, in frame order, and waits for each
/// returned future before it handles the next frame, so a listener applies frames in order.
abstract interface class SessionListener {
  /// `ready` arrived; the connection stored `gateway_session_id`, `permission_version` and
  /// `head_seq` and set [ConnectionPhase.catchingUp]. The listener applies the snapshot and
  /// stores cursor and `sync_epoch` together (04 7.5 item 2).
  Future<void> onReady(GatewayReady ready, SessionControl control);

  /// `resumed` arrived; the catch-up from the cursor follows (04 6.3 item 7).
  Future<void> onResumed(GatewayResumed resumed, SessionControl control);

  /// Any other server frame after `ready` or `resumed` (`dispatch`, `cursor`,
  /// `catchup.complete`, `resync_required`, `error`).
  Future<void> onFrame(GatewayFrame frame, SessionControl control);

  /// The connection that delivered `ready` or `resumed` is gone.
  void onDisconnected();
}

/// What a [SessionListener] may do with the connection.
abstract interface class SessionControl {
  /// Sets [ConnectionPhase.catchingUp], [ConnectionPhase.resyncing] or
  /// [ConnectionPhase.live] (15 7.2); other phases belong to the connection.
  ///
  /// Throws an [ArgumentError] for another phase.
  void setPhase(ConnectionPhase phase);

  /// Sends [frame] on the current connection; returns `false` when it is gone.
  bool send(GatewayFrame frame);

  /// The `seq` of the last processed event, sent as `last_seq` with the next heartbeat
  /// (04 6.3 item 5); until set, the stored cursor.
  set lastProcessedSeq(GlobalSeq seq);

  /// The `sync_epoch` of the current `hello` and `ready` or `resumed`, kept in memory only
  /// (04 7.5).
  String? get syncEpoch;
}
