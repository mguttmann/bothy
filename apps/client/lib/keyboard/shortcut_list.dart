// SPDX-License-Identifier: Apache-2.0

/// The list of shortcuts that Cmd+/ or Ctrl+/ shows (docs/spec/01-product.md 7; brief M4-24
/// fixed requirement 15): every row of `bindings.dart` with its action and its keys for the
/// platform, in the language of the app, and the rows of the table "In the terminal"
/// (`terminal/terminal_shortcuts.dart`, M4-36).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../terminal/terminal_shortcuts.dart';
import 'bindings.dart';
import 'intents.dart';

/// The rows whose action does not exist yet; empty since M4-36 added the hosts route of "Open
/// hosts and terminal" (brief M4-24 Out of scope).
const Set<ShortcutAction> notYetAvailable = {};

/// The label of [row] in the language of [l].
String shortcutRowLabel(AppLocalizations l, ShortcutRow row) =>
    switch (row.actions.first) {
      ShortcutAction.quickSwitcher => l.shortcutQuickSwitcher,
      ShortcutAction.search => l.shortcutSearch,
      ShortcutAction.searchInThread => l.shortcutSearchInThread,
      ShortcutAction.newPost => l.shortcutNewPost,
      ShortcutAction.focusInput => l.shortcutFocusInput,
      ShortcutAction.nextChannel ||
      ShortcutAction.previousChannel => l.shortcutChannel,
      ShortcutAction.nextUnreadChannel ||
      ShortcutAction.previousUnreadChannel => l.shortcutUnreadChannel,
      ShortcutAction.pinOverview => l.shortcutPinOverview,
      ShortcutAction.nextColumn ||
      ShortcutAction.previousColumn => l.shortcutColumn,
      ShortcutAction.markRead => l.shortcutMarkRead,
      ShortcutAction.markUnread => l.shortcutMarkUnread,
      ShortcutAction.toggleDone => l.shortcutDone,
      ShortcutAction.stopRun => l.shortcutStopRun,
      ShortcutAction.toggleLiveBox => l.shortcutLiveBox,
      ShortcutAction.toggleSessionColumn => l.shortcutSessionColumn,
      ShortcutAction.editLast => l.shortcutEditLast,
      ShortcutAction.send || ShortcutAction.newLine => l.shortcutSend,
      ShortcutAction.runningNow => l.shortcutRunningNow,
      ShortcutAction.mentions => l.shortcutMentions,
      ShortcutAction.settings => l.shortcutSettings,
      ShortcutAction.shortcutList => l.shortcutList,
      ShortcutAction.hostsTerminal => l.shortcutHostsTerminal,
    };

/// The label of the terminal [row] in the language of [l].
String terminalShortcutRowLabel(AppLocalizations l, TerminalShortcutRow row) =>
    switch (row.action) {
      TerminalShortcutAction.copy ||
      TerminalShortcutAction.paste => l.shortcutTerminalCopyPaste,
      TerminalShortcutAction.quickSwitcher => l.shortcutQuickSwitcher,
      TerminalShortcutAction.newSession => l.shortcutTerminalNewSession,
      TerminalShortcutAction.nextSession ||
      TerminalShortcutAction.previousSession => l.shortcutTerminalSession,
      TerminalShortcutAction.detach => l.shortcutTerminalDetach,
      TerminalShortcutAction.search => l.shortcutTerminalSearch,
      TerminalShortcutAction.fontLarger ||
      TerminalShortcutAction.fontSmaller => l.shortcutTerminalFontSize,
    };

/// The key names of the language of [l].
final class LocalizedKeyNames implements KeyNames {
  /// Creates the names.
  const LocalizedKeyNames(this.l);

  /// The texts of the app.
  final AppLocalizations l;

  @override
  String get cmd => l.shortcutKeyCmd;
  @override
  String get ctrl => l.shortcutKeyCtrl;
  @override
  String get option => l.shortcutKeyOption;
  @override
  String get alt => l.shortcutKeyAlt;
  @override
  String get shift => l.shortcutKeyShift;
  @override
  String get arrow => l.shortcutKeyArrow;
  @override
  String get up => l.shortcutKeyUp;
  @override
  String get down => l.shortcutKeyDown;
  @override
  String get escape => l.shortcutKeyEscape;
  @override
  String get tab => l.shortcutKeyTab;
  @override
  String get enter => l.shortcutKeyEnter;
  @override
  String get click => l.shortcutKeyClick;
  @override
  String get then => l.shortcutKeyThen;
  @override
  String get or => l.shortcutKeyOr;
  @override
  String qualifier(ShortcutQualifier qualifier) => switch (qualifier) {
    ShortcutQualifier.inEmptyField => l.shortcutInEmptyField,
    ShortcutQualifier.reversible => l.shortcutReversible,
  };
}

/// Shows the list of shortcuts for [platform] as a dialog over [context].
///
/// Example: `showShortcutList(context, ShortcutPlatform.current)`.
Future<void> showShortcutList(
  BuildContext context,
  ShortcutPlatform platform,
) => showDialog<void>(
  context: context,
  builder: (context) => Dialog(
    key: const ValueKey('shortcutList.dialog'),
    child: ConstrainedBox(
      constraints: const BoxConstraints(maxWidth: DsLayout.columnMin * 2),
      child: ShortcutListView(
        platform: platform,
        onClose: () => Navigator.of(context).pop(),
      ),
    ),
  ),
);

/// The list itself: a title, one line per row of the table with the action and its keys, and
/// "Close". It reads no provider, so goldens build it directly.
class ShortcutListView extends StatelessWidget {
  /// Creates the list for [platform].
  const ShortcutListView({super.key, required this.platform, this.onClose});

  /// The keyboard family whose keys are shown.
  final ShortcutPlatform platform;

  /// Closes the list, or null when there is nothing to close.
  final VoidCallback? onClose;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final c = context.dsColors;
    final names = LocalizedKeyNames(l);
    final keyStyle = dsTextStyle(DsType.code).copyWith(color: c.textPrimary);
    final noteStyle = dsTextStyle(DsType.caption)
        .copyWith(color: c.textSecondary);
    final terminalNames = TerminalKeyNames(
      names,
      plus: l.shortcutKeyPlus,
      minus: l.shortcutKeyMinus,
      left: l.shortcutKeyLeft,
      right: l.shortcutKeyRight,
    );
    Widget line(Key key, String label, String keys, {bool disabled = false}) =>
        Padding(
          key: key,
          padding: const EdgeInsets.symmetric(vertical: DsSpace.s4),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(
                child: Text(
                  label,
                  style: dsTextStyle(DsType.body)
                      .copyWith(color: c.textPrimary),
                ),
              ),
              const SizedBox(width: DsSpace.s12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(keys, style: keyStyle),
                    if (disabled)
                      Text(l.shortcutListDisabled, style: noteStyle),
                  ],
                ),
              ),
            ],
          ),
        );
    return Semantics(
      key: const ValueKey('shortcutList'),
      container: true,
      label: l.shortcutListTitle,
      child: Padding(
        padding: const EdgeInsets.all(DsSpace.s16),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(
              l.shortcutListTitle,
              style: dsTextStyle(DsType.title).copyWith(color: c.textPrimary),
            ),
            const SizedBox(height: DsSpace.s12),
            Flexible(
              child: SingleChildScrollView(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    for (final row in shortcutRows)
                      line(
                        ValueKey('shortcutList.row.${row.actions.first.name}'),
                        shortcutRowLabel(l, row),
                        describeRow(row, platform, names),
                        disabled: row.actions.any(notYetAvailable.contains),
                      ),
                    Padding(
                      padding: const EdgeInsets.only(
                        top: DsSpace.s12,
                        bottom: DsSpace.s4,
                      ),
                      child: Semantics(
                        header: true,
                        child: Text(
                          l.shortcutListTerminal,
                          style: dsTextStyle(DsType.heading)
                              .copyWith(color: c.textPrimary),
                        ),
                      ),
                    ),
                    for (final row in terminalShortcutRows)
                      line(
                        ValueKey('shortcutList.terminal.${row.action.name}'),
                        terminalShortcutRowLabel(l, row),
                        describeTerminalRow(row, platform, terminalNames),
                      ),
                  ],
                ),
              ),
            ),
            if (onClose != null) ...[
              const SizedBox(height: DsSpace.s12),
              Align(
                alignment: AlignmentDirectional.centerEnd,
                child: DsButton(
                  key: const ValueKey('shortcutList.close'),
                  label: l.shortcutListClose,
                  style: DsButtonStyle.secondary,
                  onPressed: onClose,
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }
}
