// SPDX-License-Identifier: Apache-2.0

import 'package:l10n/l10n.dart';

import '../session/console_controller.dart';

/// The text of [notice] in [l]: each names the cause and the next step (01 5.6).
String noticeText(AppLocalizations l, ConsoleNotice notice) => switch (notice) {
  ConsoleNotice.loginFailed => l.consoleLoginFailed,
  ConsoleNotice.passkeyFailed => l.consoleLoginPasskeyFailed,
  ConsoleNotice.oidcFailed => l.consoleLoginOidcFailed,
  ConsoleNotice.oidcCancelled => l.consoleLoginOidcCancelled,
  ConsoleNotice.oidcExpired => l.consoleLoginOidcExpired,
  ConsoleNotice.unavailable => l.consoleLoginUnavailable,
  ConsoleNotice.stepUpFailed => l.consoleStepUpFailed,
};
