// SPDX-License-Identifier: Apache-2.0

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:l10n/l10n.dart';

import '../routes.dart';

/// Below this width of the shell the navigation moves into a drawer behind a menu button in the
/// header, so a narrow window or a browser zoom of 400 percent keeps the page usable without
/// scrolling sideways (WCAG 1.4.10: 320 CSS pixels).
const double consoleNarrowWidth = 640;

/// The shell of 01 5.5: the navigation on the left, the header with the signed-in person and
/// "Log out", and the page [body]. [banner] stands above the header on every page (the stored
/// "View as person", 07 9.2); [notice] is the message of a failed request of the signed-in
/// console (a logout without an answer, a step-up that did not complete), and [busy] disables
/// "Log out" while a request runs.
///
/// The document title names the page and the console (WCAG 2.4.2). Focus moves per column
/// (navigation, then header and page); the arrow keys move within the navigation, and the focus
/// ring comes from the theme of `design_system`. Narrower than [consoleNarrowWidth], the
/// navigation is a drawer opened from the header.
class ConsoleShell extends StatelessWidget {
  /// Creates the shell for [page].
  const ConsoleShell({
    super.key,
    required this.productName,
    required this.displayName,
    required this.page,
    required this.onOpen,
    required this.onLogout,
    required this.body,
    this.banner,
    this.notice,
    this.busy = false,
    this.focusSelected = false,
    this.navigation = consoleNavigation,
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

  /// The content of the page.
  final Widget body;

  /// The banner above the header of every page, or null.
  final Widget? banner;

  /// The message of the last failed request, already in the person's language, or null.
  final String? notice;

  /// Whether a request of the shell runs ("Log out" is disabled meanwhile).
  final bool busy;

  /// Whether the entry of [page] takes the focus when the shell appears: the person chose the
  /// page in the navigation, and the focus stays on that entry (WCAG 2.4.3).
  final bool focusSelected;

  /// The entries of the navigation, [consoleNavigation] or [consoleNavigationFor].
  final List<ConsolePage> navigation;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    return Title(
      title: l.consoleDocumentTitle(
        navigationLabel(l, page),
        l.consoleTitle(productName),
      ),
      color: colors.accentBg,
      child: LayoutBuilder(
        builder: (context, box) {
          final narrow = box.maxWidth < consoleNarrowWidth;
          Widget navigation({VoidCallback? closeDrawer}) => _Navigation(
            productName: productName,
            page: page,
            entries: this.navigation,
            focusSelected: focusSelected,
            onOpen: (entry) {
              // The drawer closes on a choice, so the page shows right away.
              closeDrawer?.call();
              onOpen(entry);
            },
          );
          final content = ColoredBox(
            color: colors.surfaceCanvas,
            child: FocusTraversalGroup(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  ?banner,
                  Padding(
                    padding: const EdgeInsets.all(DsSpace.s16),
                    child: Wrap(
                      alignment: WrapAlignment.spaceBetween,
                      crossAxisAlignment: WrapCrossAlignment.center,
                      spacing: DsSpace.s16,
                      children: [
                        Row(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            if (narrow)
                              const DrawerButton(
                                key: ValueKey('console.shell.menu'),
                              ),
                            Flexible(
                              child: Semantics(
                                header: true,
                                child: Text(
                                  navigationLabel(l, page),
                                  style: dsTextStyle(DsType.title),
                                ),
                              ),
                            ),
                          ],
                        ),
                        Text(l.consoleSignedInAs(displayName)),
                        OutlinedButton(
                          onPressed: busy ? null : onLogout,
                          child: Text(l.consoleLogout),
                        ),
                      ],
                    ),
                  ),
                  if (notice case final text?)
                    Padding(
                      padding: const EdgeInsets.fromLTRB(
                        DsSpace.s16,
                        0,
                        DsSpace.s16,
                        DsSpace.s16,
                      ),
                      child: DsBanner(
                        key: const ValueKey('console.shell.notice'),
                        tone: DsTone.danger,
                        text: text,
                        liveRegion: true,
                      ),
                    ),
                  Expanded(child: body),
                ],
              ),
            ),
          );
          if (narrow) {
            return Scaffold(
              backgroundColor: colors.surfaceCanvas,
              drawer: Drawer(
                backgroundColor: colors.surfaceList,
                child: Builder(
                  builder: (inner) =>
                      navigation(closeDrawer: Scaffold.of(inner).closeDrawer),
                ),
              ),
              body: content,
            );
          }
          return ColoredBox(
            color: colors.surfaceCanvas,
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                SizedBox(
                  width: DsLayout.sidebarWidth,
                  child: Material(
                    color: colors.surfaceList,
                    child: navigation(),
                  ),
                ),
                Expanded(child: content),
              ],
            ),
          );
        },
      ),
    );
  }
}

class _Navigation extends StatelessWidget {
  const _Navigation({
    required this.productName,
    required this.page,
    required this.entries,
    required this.onOpen,
    required this.focusSelected,
  });

  final String productName;
  final ConsolePage page;
  final List<ConsolePage> entries;
  final bool focusSelected;
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
              for (final entry in entries)
                Semantics(
                  selected: entry == page,
                  child: ListTile(
                    autofocus: focusSelected && entry == page,
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
