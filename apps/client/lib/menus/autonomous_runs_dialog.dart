// SPDX-License-Identifier: Apache-2.0

/// The channel switch "Let agents continue on their own" (`allow_autonomous_runs`, 01 6.21 item
/// 5, 07 2.14 item 1; brief AR-08): an entry of the channel menu for a person with `manage`
/// that opens a dialog with the switch and the sentence that owners always may and that every
/// other person's wake-ups need the switch. A change goes into the outbox
/// (`setChannelAutonomousRuns` with the stored channel version as `If-Match`). The client has
/// no other channel settings screen, so the menu is where the switch lives (ruling D-AR-08-4).
/// Hidden in a direct message (the server refuses `dm_channel`) and on a server before stage
/// AR, whose channel carries no switch. The sync snapshot and the channel events carry no
/// `allow_autonomous_runs` either, so a channel whose stored value is still unknown asks
/// `getChannel` when its menu opens ([autonomousRunsOfServer]); the entry shows once the
/// answer carries the switch.
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';
import 'package:local_store/local_store.dart' show AccessView;
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../app/providers/session.dart';
import '../sidebar/providers.dart';
import '../sync/outbox_outcome.dart';
import '../sync/wake_ops.dart';
import '../thread/providers.dart';

part 'autonomous_runs_dialog.g.dart';

/// Whether the channel menu of [channel] offers the switch to a person with [access]; [known]
/// is the switch as the server reported it, null while unknown (a server before stage AR).
bool offersAutonomousRunsSwitch(
  Channel channel,
  AccessView? access, {
  bool? known,
}) =>
    (known ?? channel.allowAutonomousRuns) != null &&
    channel.kind != ChannelKind.dm &&
    channel.archivedAt == null &&
    (access?.capabilities.contains(Capability.manage) ?? false);

/// `allow_autonomous_runs` of [channelId] as `getChannel` reports it, stored with the channel's
/// version; null when the answer carries no switch (a server before stage AR) or the read
/// failed. The sync snapshot and the channel events of a server of stage AR leave the member
/// out, so this read is what fills it (AR-08).
@riverpod
Future<bool?> autonomousRunsOfServer(Ref ref, String channelId) async {
  final engine = ref.watch(syncEngineProvider);
  final db = ref.watch(localDatabaseProvider);
  if (engine == null || db == null) return null;
  try {
    final channel = (await engine.api.getChannelsApi().getChannel(
      channelId: channelId,
    )).data?.channel;
    final allowed = channel?.allowAutonomousRuns;
    if (channel == null || allowed == null) return null;
    await db.structureDao.putChannelAutonomousRuns(
      ChannelId.parse(channelId),
      ResourceVersion(channel.version),
      allowed: allowed,
    );
    return allowed;
  } on Object {
    return null;
  }
}

/// The entry "Let agents continue on their own..." of the menu of [channel], or none.
List<Widget> autonomousRunsMenuItems(
  BuildContext context,
  WidgetRef ref,
  Channel channel,
) {
  final access = ref.watch(channelAccessProvider(channel.id.value)).value;
  // Not asked while the stored channel knows the switch or the person may not change it.
  final known =
      channel.allowAutonomousRuns ??
      (offersAutonomousRunsSwitch(channel, access, known: false)
          ? ref.watch(autonomousRunsOfServerProvider(channel.id.value)).value
          : null);
  if (!offersAutonomousRunsSwitch(channel, access, known: known)) {
    return const [];
  }
  return [
    DsMenuItem(
      key: const ValueKey('menu.autonomousRuns'),
      onPressed: () => unawaited(
        showDialog<void>(
          context: context,
          builder: (_) => AutonomousRunsDialog(channelId: channel.id),
        ),
      ),
      child: Text(AppLocalizations.of(context).autonomousRunsMenu),
    ),
  ];
}

/// The dialog with the switch of [channelId]; it follows the stored channel, so the switch
/// shows the server's value once the answer or `channel.updated` arrived.
class AutonomousRunsDialog extends ConsumerStatefulWidget {
  /// Creates the dialog.
  const AutonomousRunsDialog({super.key, required this.channelId});

  /// The channel.
  final ChannelId channelId;

  @override
  ConsumerState<AutonomousRunsDialog> createState() =>
      _AutonomousRunsDialogState();
}

class _AutonomousRunsDialogState extends ConsumerState<AutonomousRunsDialog> {
  /// The value the person chose while its write waits, or null.
  bool? _pending;
  String? _refusal;

  Future<void> _set(Channel channel, bool allowed) async {
    final engine = ref.read(syncEngineProvider);
    if (engine == null || _pending != null) return;
    final l = AppLocalizations.of(context);
    setState(() {
      _pending = allowed;
      _refusal = null;
    });
    final entry = await enqueueAutonomousRuns(
      engine,
      channel.id,
      channel.version,
      allowed: allowed,
    );
    // The wait ends with the entry, also when the dialog is closed by then (AUD-CA01/4).
    final refusal = await outboxOutcome(engine.outbox, entry.id);
    if (!mounted) return;
    final reason = refusal?.reason;
    setState(() {
      // Sent and stored: the stored channel carries the value now. Refused: its reason.
      _pending = null;
      _refusal = refusal == null
          ? null
          : l.autonomousRunsRefused(
              (reason == null ? null : dsReasonText(l, reason)) ??
                  problemText(l, refusal.code, reason: reason),
            );
    });
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final channel = ref
        .watch(storedChannelOfRouteProvider(widget.channelId.value))
        .value;
    final value = _pending ?? channel?.allowAutonomousRuns ?? false;
    final refusal = _refusal;
    return AlertDialog(
      title: Text(l.autonomousRunsTitle),
      content: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          SwitchListTile(
            key: const ValueKey('autonomousRuns.switch'),
            contentPadding: EdgeInsets.zero,
            title: Text(l.autonomousRunsTitle),
            value: value,
            onChanged: channel == null || _pending != null
                ? null
                : (v) => unawaited(_set(channel, v)),
          ),
          Text(
            l.autonomousRunsExplanation,
            style: dsTextStyle(DsType.bodySm)
                .copyWith(color: colors.textSecondary),
          ),
          if (refusal != null) ...[
            const SizedBox(height: DsSpace.s8),
            Text(refusal, style: dsTextStyle(DsType.bodySm)),
          ],
        ],
      ),
      actions: [
        DsButton(
          key: const ValueKey('autonomousRuns.close'),
          label: l.autonomousRunsClose,
          onPressed: () => Navigator.of(context).pop(),
        ),
      ],
    );
  }
}
