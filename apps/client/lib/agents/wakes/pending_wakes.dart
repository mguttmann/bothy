// SPDX-License-Identifier: Apache-2.0

/// The pending wake-ups of a thread above the composer (01 6.21 item 1, R-PR-37; brief AR-08):
/// per wake-up its name, its condition, when it fires at the earliest and its deadline, and
/// "Cancel" for the wake-up's person and a person with `manage` on the channel (07 2.14 item
/// 5). A usage-limit wake-up reads "Usage limit of the account reached; continues at <time>"
/// (09 18.19.10). The list follows the local table, so `run.wake_changed` changes it at once;
/// it loads the thread's wake-ups when it opens. Nothing shows without a pending wake-up.
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';

import '../../app/providers/ports.dart';
import '../../app/providers/session.dart';
import '../../settings/settings_providers.dart';
import '../../sync/outbox_outcome.dart';
import '../../sync/wake_ops.dart';
import '../../thread/providers.dart';
import '../providers.dart';
import 'wake_view.dart';

/// The time of [at] for a wake-up line: the clock time today, else the day and the time.
String wakeTime(
  L10nFormat format,
  DateTime at, {
  required DateTime now,
  required String timeZone,
}) {
  final day = format.relativeDay(at, now: now, timeZone: timeZone);
  final time = format.time(at, timeZone: timeZone);
  final today = format.relativeDay(now, now: now, timeZone: timeZone);
  return day == today ? time : '$day $time';
}

/// The pending wake-ups of [threadId] in [channelId].
class PendingWakes extends ConsumerStatefulWidget {
  /// Creates the list.
  const PendingWakes({
    super.key,
    required this.threadId,
    required this.channelId,
  });

  /// The thread.
  final ThreadId threadId;

  /// Its channel, for the capability `manage`.
  final ChannelId channelId;

  @override
  ConsumerState<PendingWakes> createState() => _PendingWakesState();
}

class _PendingWakesState extends ConsumerState<PendingWakes> {
  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void didUpdateWidget(PendingWakes old) {
    super.didUpdateWidget(old);
    if (old.threadId != widget.threadId) _load();
  }

  /// Loads the thread's wake-ups once per opened thread; a failed read keeps what the table
  /// holds (offline, or a server before stage AR).
  void _load() {
    final loader = ref.read(wakeLoaderProvider);
    unawaited(loader.load(widget.threadId).catchError((Object _) {}));
  }

  @override
  Widget build(BuildContext context) {
    final wakes = pendingWakes(
      ref.watch(threadWakesProvider(widget.threadId.value)).value ?? const [],
    );
    if (wakes.isEmpty) return const SizedBox.shrink();
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final me = ref.watch(myPrincipalIdProvider);
    final access = ref
        .watch(channelAccessProvider(widget.channelId.value))
        .value;
    final zone = ref.watch(personTimeZoneProvider);
    final now = ref.watch(appPortsProvider).clock.now();
    final format = dsFormat(context);
    String when(DateTime at) => wakeTime(format, at, now: now, timeZone: zone);
    return Container(
      key: const ValueKey('wakes.pending'),
      decoration: BoxDecoration(
        border: Border(top: BorderSide(color: colors.borderDefault)),
      ),
      padding: const EdgeInsets.symmetric(
        horizontal: DsSpace.s12,
        vertical: DsSpace.s4,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(
            l.wakesPendingTitle,
            style: dsTextStyle(DsType.caption)
                .copyWith(color: colors.textSecondary),
          ),
          for (final wake in wakes)
            _WakeRow(
              key: ValueKey('wakes.pending.${wake.id.value}'),
              wake: wake,
              when: when,
              canCancel: mayCancelWake(wake, me, access),
            ),
        ],
      ),
    );
  }
}

/// One pending wake-up.
class _WakeRow extends ConsumerStatefulWidget {
  const _WakeRow({
    super.key,
    required this.wake,
    required this.when,
    required this.canCancel,
  });

  final RunWake wake;
  final String Function(DateTime at) when;
  final bool canCancel;

  @override
  ConsumerState<_WakeRow> createState() => _WakeRowState();
}

class _WakeRowState extends ConsumerState<_WakeRow> {
  bool _busy = false;
  String? _refusal;

  /// One activation puts one cancel into the outbox; the button stays disabled while it waits
  /// and after the server accepted it (the row then leaves the list). A refusal shows its
  /// reason and offers the button again.
  Future<void> _cancel() async {
    if (_busy) return;
    final engine = ref.read(syncEngineProvider);
    if (engine == null) return;
    final l = AppLocalizations.of(context);
    setState(() {
      _busy = true;
      _refusal = null;
    });
    final entry = await enqueueWakeCancel(engine, widget.wake.id);
    // The wait ends with the entry, also when this row is gone by then (AUD-CA01/4).
    final refusal = await outboxOutcome(engine.outbox, entry.id);
    if (refusal == null || !mounted) return;
    final reason = refusal.reason;
    setState(() {
      _busy = false;
      _refusal = l.wakesCancelRefused(
        (reason == null ? null : dsReasonText(l, reason)) ??
            problemText(l, refusal.code, reason: reason),
      );
    });
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final wake = widget.wake;
    final name = wakeName(l, wake);
    final condition = wakeCondition(l, wake, widget.when);
    final earliest = widget.when(wake.notBefore);
    final deadline = widget.when(wake.deadline);
    final title = wake.reason == RunWakeReason.usageLimit
        ? l.wakesUsageLimitContinues(earliest)
        : name;
    final details = wake.reason == RunWakeReason.usageLimit
        ? l.wakesDeadline(deadline)
        : '$condition, ${l.wakesEarliest(earliest)}, '
              '${l.wakesDeadline(deadline)}';
    final refusal = _refusal;
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: DsSpace.s4),
      child: Row(
        children: [
          Expanded(
            child: Semantics(
              container: true,
              label: l.wakesPendingItemLabel(
                title,
                condition,
                earliest,
                deadline,
              ),
              excludeSemantics: true,
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text(title, style: dsTextStyle(DsType.bodySm)),
                  Text(
                    details,
                    style: dsTextStyle(DsType.caption)
                        .copyWith(color: colors.textSecondary),
                  ),
                  if (refusal != null)
                    Text(
                      refusal,
                      style: dsTextStyle(DsType.caption)
                          .copyWith(color: colors.textSecondary),
                    ),
                ],
              ),
            ),
          ),
          if (widget.canCancel)
            Semantics(
              label: l.wakesCancelLabel(name),
              excludeSemantics: true,
              button: true,
              child: DsButton(
                key: ValueKey('wakes.cancel.${wake.id.value}'),
                label: l.wakesCancel,
                loading: _busy,
                onPressed: _busy ? null : () => unawaited(_cancel()),
              ),
            ),
        ],
      ),
    );
  }
}
