// SPDX-License-Identifier: Apache-2.0

/// [TerminalPrompts] answered with the dialogs of this package.
library;

import 'package:api_client/api_client.dart' show Host;
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart' show AppLocalizations;
import 'package:terminal_ui/src/controller/terminal_prompts.dart';
import 'package:terminal_ui/src/pinning/pin_check.dart';
import 'package:terminal_ui/src/widgets/foreign_host_warning.dart';
import 'package:terminal_ui/src/widgets/key_changed_dialog.dart';
import 'package:terminal_ui/src/widgets/open_terminal_dialog.dart';

/// Shows [KeyChangedDialog], [ForeignHostWarningDialog] and [OpenTerminalDialog] over the
/// navigator of [context]; without a mounted context every question is answered with "no".
///
/// ```dart
/// final prompts = DialogTerminalPrompts(() => navigatorKey.currentContext, selfUserId: me);
/// ```
final class DialogTerminalPrompts implements TerminalPrompts {
  /// Creates the prompts over [context].
  DialogTerminalPrompts(this.context, {required this.selfUserId});

  /// The context of the navigator that shows the dialogs, or null.
  final BuildContext? Function() context;

  /// The person's own user ID (working folders of the dialog "Open terminal").
  final String selfUserId;

  @override
  Future<bool> confirmKeyChange(Host host, PinChanged change) async =>
      await _show<bool>(
        (_) => KeyChangedDialog(
          stored: change.pinned.fingerprint,
          current: change.currentFingerprint,
        ),
      ) ??
      false;

  @override
  Future<bool> confirmForeignHost(Host host) async =>
      await _show<bool>((_) => ForeignHostWarningDialog(hostName: host.name)) ??
      false;

  @override
  Future<OpenChoice?> confirmOpen(Host host, OpenChoice initial) => _show(
    (_) => OpenTerminalDialog(
      host: host,
      initial: initial,
      selfUserId: selfUserId,
    ),
  );

  @override
  String stepUpPurpose(Host host, StepUpPurpose purpose) {
    final where = context();
    if (where == null) return host.name;
    final AppLocalizations l = dsL10n(where);
    return switch (purpose) {
      StepUpPurpose.open => l.terminalStepUpOpen(host.name),
      StepUpPurpose.attach => l.terminalStepUpAttach(host.name),
    };
  }

  Future<T?> _show<T>(WidgetBuilder builder) async {
    final where = context();
    if (where == null || !where.mounted) return null;
    return showDialog<T>(context: where, builder: builder);
  }
}
