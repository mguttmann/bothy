// SPDX-License-Identifier: Apache-2.0

import 'package:admin_ui/admin_ui.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

/// The public status page of the console (docs/spec/16-operations.md 12, "Visibility"; brief
/// M7-21): the [StatusScreen] over `getPublicStatus` without login and without forms. The
/// server answers `404` unless `status.public_enabled`, which the page shows as "only after
/// login"; [onBack] returns to the page of the console's state (the login when signed out).
class PublicStatusPage extends StatelessWidget {
  /// Creates the page.
  const PublicStatusPage({
    super.key,
    required this.source,
    required this.onBack,
  });

  /// `getPublicStatus` on the console's interval.
  final StatusSource source;

  /// Leaves the page.
  final VoidCallback onBack;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    return Material(
      color: context.dsColors.surfaceCanvas,
      child: SingleChildScrollView(
        key: const ValueKey('console.publicStatus'),
        padding: const EdgeInsets.all(DsSpace.s24),
        child: Align(
          alignment: Alignment.topCenter,
          child: ConstrainedBox(
            constraints: const BoxConstraints(
              maxWidth: DsLayout.messageMaxWidth,
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Align(
                  alignment: Alignment.centerLeft,
                  child: TextButton(
                    key: const ValueKey('console.publicStatus.back'),
                    onPressed: onBack,
                    child: Text(l.statusBack),
                  ),
                ),
                StatusScreen(
                  source: source,
                  title: l.statusPublicTitle,
                  notFoundText: l.statusPublicOff,
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
