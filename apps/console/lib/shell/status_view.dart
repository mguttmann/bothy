// SPDX-License-Identifier: Apache-2.0

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../login/card.dart';

/// A full-page notice without any action of the console: the start, a version mismatch or a
/// server that cannot be reached. Only [onReload] is offered, so nothing else can run.
class StatusView extends StatelessWidget {
  /// Creates the view; without [message] it shows the progress of the start with [working]
  /// (the default text when null).
  const StatusView({super.key, this.message, this.onReload, this.working});

  /// The text below the progress, or null for the default.
  final String? working;

  /// The notice, or null for the progress.
  final String? message;

  /// Reloads the page, or null.
  final VoidCallback? onReload;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final text = message;
    return ConsoleCard(
      children: [
        if (text == null) ...[
          const Center(child: CircularProgressIndicator()),
          cardGap,
          Text(working ?? l.consoleWorking, textAlign: TextAlign.center),
        ] else
          DsBanner(tone: DsTone.warning, text: text, liveRegion: true),
        if (onReload != null) ...[
          cardGap,
          FilledButton(onPressed: onReload, child: Text(l.consoleReload)),
        ],
      ],
    );
  }
}
