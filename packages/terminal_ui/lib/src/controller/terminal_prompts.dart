// SPDX-License-Identifier: Apache-2.0

/// The questions the controller asks the person before it opens or attaches: the key change
/// warning, the warning of 24 8.4, the dialog "Open terminal" and the texts of the step-up.
/// `DialogTerminalPrompts` answers them with the dialogs of this package; tests answer them
/// directly.
library;

import 'package:api_client/api_client.dart' show Host;
import 'package:flutter/foundation.dart';
import 'package:terminal_ui/src/pinning/pin_check.dart';

/// The profile and working directory chosen in the dialog "Open terminal".
@immutable
final class OpenChoice {
  /// Creates a choice.
  const OpenChoice({required this.profile, this.workdirIndex});

  /// The profile name (`HostProfile.name`).
  final String profile;

  /// The index of the working directory for a profile of kind `claude`, else null.
  final int? workdirIndex;
}

/// Whether the step-up is for opening or for attaching.
enum StepUpPurpose {
  /// `terminal.open`.
  open,

  /// `terminal.attach`.
  attach,
}

/// Asks the person; every answer `false` or null opens nothing.
abstract interface class TerminalPrompts {
  /// Shows the hard warning of a changed key (screen D11, docs/spec/24-terminal.md 5.5) and
  /// returns whether the person accepted the new key ("Accept new key"); "Do not connect" is
  /// the main action and returns false.
  Future<bool> confirmKeyChange(Host host, PinChanged change);

  /// Shows the warning of docs/spec/24-terminal.md 8.4 (the host is assigned to another
  /// person) and returns whether the person still wants to connect.
  Future<bool> confirmForeignHost(Host host);

  /// Shows the dialog "Open terminal" with host, profile and login badge; returns the choice,
  /// or null when the person cancelled.
  Future<OpenChoice?> confirmOpen(Host host, OpenChoice initial);

  /// The purpose text of the step-up prompt (already in the person's language).
  String stepUpPurpose(Host host, StepUpPurpose purpose);
}
