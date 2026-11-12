// SPDX-License-Identifier: Apache-2.0

/// Stop of a run from a view (docs/spec/01-product.md 6.2; docs/decisions.d/AUD-client-misc-01.md,
/// ruling AUD-CM01/3): the live box, the rows and cards of "Running now", the run graph and
/// `/stop` all stop a run here.
library;

import 'package:design_system/design_system.dart' show dsReasonText;
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';

import 'providers.dart';

/// Stops [run] from the view of [context]: one `cancelRun` through [RunStops], which keeps the
/// run as "Stop on its way" for every view. Returns once the entry is stored.
///
/// A Stop the server refuses for good says so on the messenger of [context] with the reason
/// (the translated reason code, else the text of the problem code), also when the view that
/// sent it is gone by then.
Future<void> stopRun(BuildContext context, WidgetRef ref, RunSummary run) {
  final messenger = ScaffoldMessenger.maybeOf(context);
  final l = AppLocalizations.of(context);
  return ref
      .read(runStopsProvider.notifier)
      .stop(
        run,
        onRefused: (refusal) {
          final reason = refusal.reason;
          messenger
            ?..hideCurrentSnackBar()
            ..showSnackBar(
              SnackBar(
                key: ValueKey('running.stopRefused.${run.id.value}'),
                content: Text(
                  l.runningStopRefused(
                    (reason == null ? null : dsReasonText(l, reason)) ??
                        problemText(l, refusal.code, reason: reason),
                  ),
                ),
                persist: false,
              ),
            );
        },
      );
}
