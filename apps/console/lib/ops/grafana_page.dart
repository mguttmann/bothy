// SPDX-License-Identifier: Apache-2.0

/// The page "Operations (Grafana)" of the console (docs/spec/25-visualization.md 9.2 item 6,
/// R-VZ-12, brief M6-17 Design note 3).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../platform/browser.dart';
import 'grafana_frame.dart';

/// The dashboard of `grafana_embed_url` in an iframe that fills the content area, with the note
/// that the page follows the permissions of Grafana, not those of the instance (level b of 25
/// 9.2 item 6), and a link that opens the dashboard in a new tab ([ExternalLinks], `noopener`).
///
/// The console shows the page only when `getInstance` names `grafana_embed_url`, which the
/// server sends to owners only; without [url] the page states that no dashboard is set up and
/// embeds nothing.
class ConsoleGrafanaPage extends StatelessWidget {
  /// Creates the page for [url].
  const ConsoleGrafanaPage({super.key, required this.url, this.links});

  /// The dashboard, or null when `getInstance` names none.
  final Uri? url;

  /// Opens a link in a new tab, or null (no link).
  final ExternalLinks? links;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final dashboard = url;
    if (dashboard == null) {
      return Padding(
        padding: const EdgeInsets.all(DsSpace.s16),
        child: Text(l.consoleGrafanaUnavailable),
      );
    }
    final open = links;
    return Column(
      key: const ValueKey('console.grafana'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: DsSpace.s16),
          child: Wrap(
            alignment: WrapAlignment.spaceBetween,
            crossAxisAlignment: WrapCrossAlignment.center,
            spacing: DsSpace.s16,
            children: [
              Text(l.consoleGrafanaNote),
              if (open != null)
                TextButton.icon(
                  key: const ValueKey('console.grafana.open'),
                  onPressed: () => open.open(dashboard),
                  icon: const Icon(Icons.open_in_new),
                  label: Text(l.consoleGrafanaOpen),
                ),
            ],
          ),
        ),
        const SizedBox(height: DsSpace.s8),
        Expanded(
          child: ConsoleGrafanaFrame(
            url: dashboard,
            label: l.consoleGrafanaFrameLabel,
          ),
        ),
      ],
    );
  }
}
