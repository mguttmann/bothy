// SPDX-License-Identifier: Apache-2.0

/// The texts of the login errors (docs/spec/01-produkt.md 5.6: the cause and the next step).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import 'onboarding_controller.dart';

/// The text of [notice].
String noticeText(AppLocalizations l, LoginNotice notice) => switch (notice) {
  LoginNotice.failed => l.onboardingLoginFailed,
  LoginNotice.unreachable => l.onboardingUnreachable,
  LoginNotice.browserCancelled => l.onboardingBrowserCancelled,
  LoginNotice.browserExpired => l.onboardingBrowserExpired,
};

/// The banner of [notice], a polite live region.
class NoticeBanner extends StatelessWidget {
  /// Creates the banner.
  const NoticeBanner(this.notice, {super.key});

  /// The error.
  final LoginNotice notice;

  @override
  Widget build(BuildContext context) => DsBanner(
    key: ValueKey('notice.${notice.name}'),
    tone: DsTone.danger,
    text: noticeText(AppLocalizations.of(context), notice),
    liveRegion: true,
  );
}
