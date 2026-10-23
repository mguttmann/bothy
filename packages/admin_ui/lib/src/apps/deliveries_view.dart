// SPDX-License-Identifier: Apache-2.0

/// The deliveries of one outgoing webhook subscription (11 7; D-M2-26-6, D-M2-26-14, D-M2-26-20;
/// brief M6-08 Design note 3, Ruling 4).
library;

import 'package:api_client/api_client.dart' show ApiProblem;
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/admin_action.dart';
import '../common/problem_banner.dart';
import '../data/admin_repository.dart';
import '../data/apps_repository.dart';
import '../data/self_repository.dart' show SelfActionFailed;
import '../self/device_tile.dart' show activityText;
import '../self/self_feedback.dart';
import 'subscription_labels.dart';

/// The deliveries of [subscription], newest first as the server lists them, one page at a time
/// with "Load more" and a reload.
///
/// Each delivery shows its status, attempts, next attempt, last status code, last error as text,
/// the event type and sequence (or that the event left the log) and when it was created and
/// delivered. The server never stores a response body (11 7), so none is shown. "Redeliver"
/// makes a delivery due again (`webhook_out.redeliver`, no step-up in 05); the server refuses
/// an event that left the log with `409 conflict`, reason `event_expired`, and the view says so.
///
/// Keys: `deliveries.view`, `deliveries.reload`, `deliveries.more`, row `delivery.<id>`, status
/// `delivery.status.<id>`, `delivery.redeliver.<id>`.
///
/// Example: `DeliveriesView(repository: repository, subscription: sub, timeZone: 'Europe/Berlin')`.
class DeliveriesView extends StatefulWidget {
  /// Creates the view of the deliveries of [subscription].
  const DeliveriesView({
    super.key,
    required this.repository,
    required this.subscription,
    this.timeZone = 'UTC',
    this.clock = DateTime.now,
  });

  /// The administration's calls.
  final AdminRepository repository;

  /// The subscription whose deliveries are shown.
  final AppSubscription subscription;

  /// The time zone of the signed-in person.
  final String timeZone;

  /// The current time.
  final DateTime Function() clock;

  @override
  State<DeliveriesView> createState() => _DeliveriesViewState();
}

class _DeliveriesViewState extends State<DeliveriesView>
    with AdminActionState<DeliveriesView> {
  List<AppDelivery>? _deliveries;
  String? _next;
  ApiProblem? _problem;
  bool _loading = false;

  @override
  void initState() {
    super.initState();
    _load();
  }

  /// Loads the first page, or with [more] the page after the loaded ones.
  Future<void> _load({bool more = false}) async {
    setState(() => _loading = true);
    try {
      final page = await widget.repository.deliveries(
        widget.subscription.id,
        cursor: more ? _next : null,
      );
      if (!mounted) return;
      setState(() {
        _deliveries = [if (more) ...?_deliveries, ...page.items];
        _next = page.nextCursor;
        _problem = null;
      });
    } on Object catch (e) {
      if (mounted) setState(() => _problem = loadProblem(e));
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  Future<void> _redeliver(AppDelivery delivery) async {
    final l = AppLocalizations.of(context);
    await runChange('${delivery.id}.redeliver', () async {
      try {
        final updated = await widget.repository.redeliver(
          delivery.id,
          purpose: l.adminHookPurposeRedeliver,
        );
        if (mounted) {
          setState(() {
            _deliveries = [
              for (final d in _deliveries ?? const <AppDelivery>[])
                d.id == updated.id ? updated : d,
            ];
          });
        }
        return SelfNotice(l.adminHookRedelivered);
      } on SelfActionFailed catch (e) {
        final p = e.problem;
        if (p?.code != 'conflict' || p?.reason != 'event_expired') rethrow;
        // The server's reason, in words: the event is gone, so no retry can help.
        return SelfNotice(l.adminHookRedeliverExpired, tone: DsTone.danger);
      }
    });
  }

  String _when(BuildContext context, DateTime at) =>
      activityText(context, at, now: widget.clock(), timeZone: widget.timeZone);

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final deliveries = _deliveries;
    return Column(
      key: const ValueKey('deliveries.view'),
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Row(
          children: [
            Expanded(
              child: Semantics(
                header: true,
                child: Text(
                  l.adminHookDeliveriesTitle(widget.subscription.url),
                  style: dsTextStyle(DsType.heading)
                      .copyWith(color: colors.textPrimary),
                ),
              ),
            ),
            TextButton(
              key: const ValueKey('deliveries.reload'),
              onPressed: _loading ? null : _load,
              child: Text(l.adminHookReload),
            ),
          ],
        ),
        const SizedBox(height: DsSpace.s8),
        noticeBanner(),
        if (_problem case final problem?)
          ProblemBanner(problem: problem, onRetry: _load)
        else if (deliveries != null) ...[
          if (deliveries.isEmpty)
            Text(
              l.adminHookDeliveriesEmpty,
              style: dsTextStyle(DsType.body)
                  .copyWith(color: colors.textSecondary),
            ),
          for (final d in deliveries) ...[
            _row(context, l, colors, d),
            Divider(height: 1, color: colors.borderSubtle),
          ],
          if (_next != null)
            Align(
              alignment: AlignmentDirectional.centerStart,
              child: TextButton(
                key: const ValueKey('deliveries.more'),
                onPressed: _loading ? null : () => _load(more: true),
                child: Text(l.adminHookMore),
              ),
            ),
        ],
      ],
    );
  }

  Widget _row(
    BuildContext context,
    AppLocalizations l,
    DsColors colors,
    AppDelivery d,
  ) {
    final seq = '${d.eventSeq}';
    final details = [
      l.adminHookAttempts(d.attempt),
      if (d.nextAttemptAt case final next?)
        l.adminHookNextAttempt(_when(context, next)),
      if (d.responseCode case final code?) l.adminHookLastStatus('$code'),
      if (d.lastError case final error?)
        l.adminHookLastError(deliveryErrorText(l, error)),
      l.adminHookCreatedAt(_when(context, d.createdAt)),
      if (d.deliveredAt case final at?)
        l.adminHookDeliveredAt(_when(context, at)),
    ];
    final secondary = dsTextStyle(DsType.bodySm)
        .copyWith(color: colors.textSecondary);
    return Padding(
      key: ValueKey('delivery.${d.id}'),
      padding: const EdgeInsets.symmetric(vertical: DsSpace.s8),
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  deliveryStatusText(l, d.status),
                  key: ValueKey('delivery.status.${d.id}'),
                  style: dsTextStyle(DsType.bodyStrong)
                      .copyWith(color: colors.textPrimary),
                ),
                Text(
                  switch (d.eventType) {
                    final type? => l.adminHookEventSeq(type, seq),
                    null => l.adminHookEventGone(seq),
                  },
                  style: dsTextStyle(DsType.body)
                      .copyWith(color: colors.textSecondary),
                ),
                Wrap(
                  spacing: DsSpace.s12,
                  children: [
                    for (final t in details) Text(t, style: secondary),
                  ],
                ),
              ],
            ),
          ),
          if (!denied.contains('${d.id}.redeliver'))
            OutlinedButton(
              key: ValueKey('delivery.redeliver.${d.id}'),
              onPressed: busy ? null : () => _redeliver(d),
              child: Text(l.adminHookRedeliver),
            ),
        ],
      ),
    );
  }
}
