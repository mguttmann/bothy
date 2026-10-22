// SPDX-License-Identifier: Apache-2.0

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:l10n/l10n.dart';

import '../routes.dart';

/// The shell of 01 5.5: the navigation on the left, the header with the signed-in person and
/// "Log out", and the page. Every page is a placeholder until its ticket fills it.
///
/// Focus moves per column (navigation, then header and page); the arrow keys move within the
/// navigation, and the focus ring comes from the theme of `design_system`.
class ConsoleShell extends StatelessWidget {
  /// Creates the shell for [page].
  const ConsoleShell({
    super.key,
    required this.productName,
    required this.displayName,
    required this.page,
    required this.onOpen,
    required this.onLogout,
  });

  /// `branding.product_name`.
  final String productName;

  /// The signed-in person.
  final String displayName;

  /// The page shown.
  final ConsolePage page;

  /// Opens another navigation entry.
  final void Function(ConsolePage page) onOpen;

  /// Ends the session.
  final VoidCallback onLogout;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    return ColoredBox(
      color: colors.surfaceCanvas,
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          SizedBox(
            width: DsLayout.sidebarWidth,
            child: Material(
              color: colors.surfaceList,
              child: _Navigation(
                productName: productName,
                page: page,
                onOpen: onOpen,
              ),
            ),
          ),
          Expanded(
            child: FocusTraversalGroup(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Padding(
                    padding: const EdgeInsets.all(DsSpace.s16),
                    child: Wrap(
                      alignment: WrapAlignment.spaceBetween,
                      crossAxisAlignment: WrapCrossAlignment.center,
                      spacing: DsSpace.s16,
                      children: [
                        Semantics(
                          header: true,
                          child: Text(
                            navigationLabel(l, page),
                            style: dsTextStyle(DsType.title),
                          ),
                        ),
                        Text(l.consoleSignedInAs(displayName)),
                        OutlinedButton(
                          onPressed: onLogout,
                          child: Text(l.consoleLogout),
                        ),
                      ],
                    ),
                  ),
                  Expanded(
                    child: Padding(
                      padding: const EdgeInsets.all(DsSpace.s16),
                      child: Text(l.consolePlaceholderPage),
                    ),
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

class _Navigation extends StatelessWidget {
  const _Navigation({
    required this.productName,
    required this.page,
    required this.onOpen,
  });

  final String productName;
  final ConsolePage page;
  final void Function(ConsolePage page) onOpen;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    return Semantics(
      container: true,
      explicitChildNodes: true,
      label: l.consoleNavigationLabel,
      child: FocusTraversalGroup(
        child: Shortcuts(
          shortcuts: const {
            SingleActivator(LogicalKeyboardKey.arrowDown): NextFocusIntent(),
            SingleActivator(LogicalKeyboardKey.arrowUp): PreviousFocusIntent(),
          },
          child: ListView(
            padding: const EdgeInsets.symmetric(vertical: DsSpace.s16),
            children: [
              Padding(
                padding: const EdgeInsets.symmetric(
                  horizontal: DsSpace.s16,
                  vertical: DsSpace.s8,
                ),
                child: Text(
                  l.consoleTitle(productName),
                  style: dsTextStyle(DsType.heading),
                ),
              ),
              for (final entry in consoleNavigation)
                Semantics(
                  selected: entry == page,
                  child: ListTile(
                    selected: entry == page,
                    title: Text(navigationLabel(l, entry)),
                    onTap: () => onOpen(entry),
                  ),
                ),
            ],
          ),
        ),
      ),
    );
  }
}
