// SPDX-License-Identifier: Apache-2.0

/// The offline banner of both shells after a close that waits for the person (15 7.2): `4015`,
/// `4017` and `4018` reconnect only on a user action, so the banner names the reason and offers
/// "Reconnect" (`GatewayConnection.connectNow`).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/widgets.dart';
import 'package:l10n/l10n.dart';
import 'package:sync_engine/sync_engine.dart';

/// The reason text of [reason] when the connection waits for the person to reconnect, or null
/// when it reconnects by itself (the network, the backoff, the foreground of a phone).
String? offlineReasonText(AppLocalizations l, ConnectionReason? reason) =>
    switch (reason?.kind) {
      ConnectionReasonKind.deviceReplaced => l.connectionDeviceReplaced,
      ConnectionReasonKind.forbidden => l.connectionForbidden,
      ConnectionReasonKind.closedByServer when reason?.code == 4015 =>
        l.connectionEndedByServer,
      _ => null,
    };

/// The banner while the engine has no connection: [text] ("Offline"), or with a reason of
/// [offlineReasonText] that reason and "Reconnect", which calls [onReconnect].
class OfflineBanner extends StatelessWidget {
  /// Creates the banner.
  const OfflineBanner({
    super.key,
    required this.text,
    required this.reason,
    required this.onReconnect,
  });

  /// The text without a reason that waits for the person.
  final String text;

  /// The reason of the connection state, or null before the engine reports one.
  final ConnectionReason? reason;

  /// Connects again; null hides the action.
  final VoidCallback? onReconnect;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final why = offlineReasonText(l, reason);
    return DsBanner(
      key: const ValueKey('shell.offline'),
      tone: DsTone.info,
      text: why ?? text,
      actionLabel: why == null || onReconnect == null
          ? null
          : l.connectionReconnect,
      onAction: why == null ? null : onReconnect,
      liveRegion: true,
    );
  }
}
