// SPDX-License-Identifier: Apache-2.0

/// The banner of "View as person" while the console holds the stored view
/// (docs/spec/01-product.md 5.2 A4; 07 9.2; brief M6-04 fixed requirement 13).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

/// The bar across the top of every page while the owner views as [name]: "Viewing as `<name>`
/// (read-only). Nothing is sent. Logged.", the time left until [endsAt] as `mm:ss` and "End
/// view".
///
/// The banner does not tick: the host passes [now] and rebuilds it every second, so one clock
/// of the host drives the banner and the end of the view together. The time left is clamped at
/// zero and its minutes may pass 59 (`75:03`), because a view is not limited to an hour by the
/// client. [onEnd] ends the view in the host (`endViewAs`); null disables the button, for
/// example while the end is on its way. The name is a live region, so a screen reader announces
/// the view when it starts.
///
/// Example:
/// ```dart
/// ViewAsBanner(name: 'Sam', endsAt: view.endsAt, now: clock(), onEnd: view.end);
/// ```
class ViewAsBanner extends StatelessWidget {
  /// Creates the banner.
  const ViewAsBanner({
    super.key,
    required this.name,
    required this.endsAt,
    required this.now,
    required this.onEnd,
  });

  /// The display name of the person the owner views as.
  final String name;

  /// When the server ends the view.
  final DateTime endsAt;

  /// The current time of the host.
  final DateTime now;

  /// Ends the view, or null to disable "End view".
  final VoidCallback? onEnd;

  /// The time left as `mm:ss`, never below `00:00`.
  String _remaining() {
    final left = endsAt.difference(now);
    final seconds = left.isNegative ? 0 : left.inSeconds;
    String two(int n) => n.toString().padLeft(2, '0');
    return '${two(seconds ~/ 60)}:${two(seconds % 60)}';
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    return Container(
      width: double.infinity,
      decoration: BoxDecoration(
        color: colors.accentSubtle,
        border: Border(bottom: BorderSide(color: colors.accentBg, width: 2)),
      ),
      padding: const EdgeInsets.symmetric(
        horizontal: DsSpace.s16,
        vertical: DsSpace.s8,
      ),
      child: Wrap(
        spacing: DsSpace.s16,
        runSpacing: DsSpace.s4,
        crossAxisAlignment: WrapCrossAlignment.center,
        children: [
          Semantics(
            liveRegion: true,
            child: Text(
              l.adminViewAsBanner(name),
              key: const ValueKey('viewAs.banner'),
              style: dsTextStyle(DsType.bodyStrong)
                  .copyWith(color: colors.textPrimary),
            ),
          ),
          Text(
            l.adminViewAsRemaining(_remaining()),
            key: const ValueKey('viewAs.remaining'),
            style: dsTextStyle(DsType.body)
                .copyWith(color: colors.textSecondary),
          ),
          TextButton(
            key: const ValueKey('viewAs.end'),
            onPressed: onEnd,
            child: Text(l.adminViewAsEnd),
          ),
        ],
      ),
    );
  }
}
