// SPDX-License-Identifier: Apache-2.0

/// The questions of the terminal in the console: the dialogs of `terminal_ui` with the console's
/// pinning statement (docs/spec/24-terminal.md 5.1; 15 23.3; brief M6-18 Design note 4).
library;

import 'package:api_client/api_client.dart' show Host;
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';
import 'package:terminal_ui/terminal_ui.dart';

/// [TerminalPrompts] of the console: the dialogs of `terminal_ui`, and below the dialog "Open
/// terminal" and the key change warning the sentence that in the browser the pin check only
/// notices a key change the server reports and does not protect against the server itself
/// ([ConsolePinningNotice]).
///
/// The sentence is the console's own: the server delivers the console's code, so a malicious
/// server could send a version without the check (24 5.1); the view claims nothing else
/// (15 23.3). The warning of 24 8.4 and the purpose texts are those of `terminal_ui` unchanged.
///
/// Example:
/// ```dart
/// final prompts = ConsoleTerminalPrompts(() => navigatorKey.currentContext, selfUserId: me);
/// ```
final class ConsoleTerminalPrompts implements TerminalPrompts {
  /// Creates the prompts over [context]; [selfUserId] finds the person's working folders.
  ConsoleTerminalPrompts(this.context, {required this.selfUserId})
    : _dialogs = DialogTerminalPrompts(context, selfUserId: selfUserId);

  /// The context of the navigator that shows the dialogs, or null.
  final BuildContext? Function() context;

  /// The person's own user ID.
  final String selfUserId;

  final DialogTerminalPrompts _dialogs;

  @override
  Future<bool> confirmKeyChange(Host host, PinChanged change) async =>
      await _show<bool>(
        KeyChangedDialog(
          stored: change.pinned.fingerprint,
          current: change.currentFingerprint,
        ),
      ) ??
      false;

  @override
  Future<bool> confirmForeignHost(Host host) =>
      _dialogs.confirmForeignHost(host);

  @override
  Future<OpenChoice?> confirmOpen(Host host, OpenChoice initial) => _show(
    OpenTerminalDialog(host: host, initial: initial, selfUserId: selfUserId),
  );

  @override
  String stepUpPurpose(Host host, StepUpPurpose purpose) =>
      _dialogs.stepUpPurpose(host, purpose);

  Future<T?> _show<T>(Widget dialog) async {
    final where = context();
    if (where == null || !where.mounted) return null;
    return showDialog<T>(
      context: where,
      builder: (_) => ConsolePinningNotice(dialog: dialog),
    );
  }
}

/// Shows [dialog] with the pinning statement of the console below it.
///
/// The dialog keeps the space above the statement and centres itself there, so the statement
/// never covers its buttons, whatever its height. A tap beside both still reaches the barrier,
/// so the dialog closes as without the statement.
class ConsolePinningNotice extends StatelessWidget {
  /// Creates the notice below [dialog].
  const ConsolePinningNotice({super.key, required this.dialog});

  /// The dialog of `terminal_ui` ("Open terminal" or "Key changed").
  final Widget dialog;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    return Column(
      children: [
        Expanded(child: dialog),
        Padding(
          padding: const EdgeInsets.fromLTRB(
            DsSpace.s24,
            0,
            DsSpace.s24,
            DsSpace.s24,
          ),
          child: Center(
            child: ConstrainedBox(
              constraints: const BoxConstraints(
                maxWidth: DsLayout.columnMin * 3,
              ),
              child: Material(
                color: colors.surfaceRaised,
                borderRadius: BorderRadius.circular(DsRadius.lg),
                child: Padding(
                  padding: const EdgeInsets.all(DsSpace.s12),
                  child: Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      DsIcon(
                        DsIcons.securityLock,
                        size: DsIconSizeRole.sm,
                        color: colors.textSecondary,
                        decorative: true,
                      ),
                      const SizedBox(width: DsSpace.s8),
                      Flexible(
                        child: Text(
                          l.consoleTerminalPinningNotice,
                          key: const ValueKey('console.terminal.pinningNotice'),
                          style: dsTextStyle(DsType.caption)
                              .copyWith(color: colors.textPrimary),
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            ),
          ),
        ),
      ],
    );
  }
}
