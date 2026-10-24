// SPDX-License-Identifier: Apache-2.0

/// The top of the sidebar: the mark and `branding.product_name` of the instance, the
/// "Jump to" field that opens the quick switcher D6 and the entry "Running now" D10 with the
/// count of active runs (components.md 7; 01 12; brief M4-23).
library;

import 'dart:typed_data';

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

/// The head of the sidebar.
///
/// [productName] is `branding.product_name` of the instance, never a literal (fixed
/// requirement 16); [logo] the bytes of `getBrandingLogo`, or null for the product's mark
/// (a tile in the accent color, 01 12).
class SidebarHeader extends StatelessWidget {
  /// Creates the head.
  const SidebarHeader({
    super.key,
    required this.productName,
    this.logo,
    this.onJumpTo,
    this.onRunningNow,
    this.runningCount = 0,
  });

  /// `branding.product_name`.
  final String productName;

  /// The logo bytes, or null.
  final Uint8List? logo;

  /// Opens the quick switcher; null disables the field.
  final VoidCallback? onJumpTo;

  /// Opens "Running now"; null disables the entry.
  final VoidCallback? onRunningNow;

  /// The number of active runs in the local `runs` table; 0 shows no counter.
  final int runningCount;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final c = context.dsColors;
    final bytes = logo;
    final edge = DsSize.avatarSm;
    return Padding(
      padding: const EdgeInsets.fromLTRB(
        DsSpace.s12,
        DsSpace.s12,
        DsSpace.s12,
        DsSpace.s8,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Row(
            children: [
              SizedBox.square(
                dimension: edge,
                child: bytes != null
                    ? Image.memory(
                        bytes,
                        key: const ValueKey('sidebar.logo'),
                        fit: BoxFit.contain,
                        excludeFromSemantics: true,
                      )
                    : DecoratedBox(
                        key: const ValueKey('sidebar.mark'),
                        decoration: BoxDecoration(
                          color: context.dsAccent.accentBg,
                          borderRadius: BorderRadius.circular(DsRadius.sm),
                        ),
                      ),
              ),
              const SizedBox(width: DsSpace.s8),
              Expanded(
                child: Semantics(
                  header: true,
                  child: Text(
                    productName,
                    key: const ValueKey('sidebar.productName'),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: dsTextStyle(DsType.heading)
                        .copyWith(color: c.textOnSidebar),
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: DsSpace.s12),
          DsTappable(
            key: const ValueKey('sidebar.jumpTo'),
            onTap: onJumpTo,
            label: l.sidebarJumpTo,
            onSidebar: true,
            excludeChildSemantics: true,
            child: Container(
              constraints: BoxConstraints(
                minHeight: context.dsDensity.targetMin,
              ),
              padding: const EdgeInsets.symmetric(horizontal: DsSpace.s8),
              decoration: BoxDecoration(
                border: Border.all(
                  color: c.borderOnSidebar,
                  width: DsBorderWidth.hairline,
                ),
                borderRadius: BorderRadius.circular(DsRadius.md),
              ),
              child: Row(
                children: [
                  DsIcon(
                    DsIcons.actionQuickSwitch,
                    size: DsIconSizeRole.sm,
                    color: c.textOnSidebarSecondary,
                    decorative: true,
                  ),
                  const SizedBox(width: DsSpace.s8),
                  Expanded(
                    child: Text(
                      l.sidebarJumpTo,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: dsTextStyle(DsType.bodySm)
                          .copyWith(color: c.textOnSidebarSecondary),
                    ),
                  ),
                ],
              ),
            ),
          ),
          // A static glyph: the spinning `run.running` would never let an idle frame settle;
          // the counter says whether runs are active (components.md 2.1).
          const SizedBox(height: DsSpace.s4),
          DsTappable(
            key: const ValueKey('sidebar.runningNow'),
            onTap: onRunningNow,
            label: runningCount > 0
                ? l.sidebarRunningNowCount(runningCount)
                : l.sidebarRunningNow,
            onSidebar: true,
            excludeChildSemantics: true,
            child: Container(
              constraints: BoxConstraints(
                minHeight: context.dsDensity.rowChannel,
              ),
              padding: const EdgeInsets.symmetric(horizontal: DsSpace.s8),
              child: Row(
                children: [
                  DsIcon(
                    DsIcons.stepWorkflow,
                    size: DsIconSizeRole.sm,
                    color: c.categorySpruceOnSidebar,
                    decorative: true,
                  ),
                  const SizedBox(width: DsSpace.s8),
                  Expanded(
                    child: Text(
                      l.sidebarRunningNow,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: dsTextStyle(DsType.bodySm)
                          .copyWith(color: c.textOnSidebar),
                    ),
                  ),
                  if (runningCount > 0)
                    Text(
                      dsFormat(context).count(runningCount),
                      key: const ValueKey('sidebar.runningCount'),
                      style: dsTextStyle(DsType.label)
                          .copyWith(color: c.textOnSidebarSecondary),
                    ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}
