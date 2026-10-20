// SPDX-License-Identifier: Apache-2.0

/// The enrollment code, shown once (docs/spec/08-identity.md 3.2: "shown once"; 06 7: a replay
/// answers `409 one_time_secret`; brief M4-26 requirement 12, Design note 4).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/code_text.dart';
import '../data/admin_repository.dart';
import '../self/device_tile.dart' show activityText;

/// The name of the purpose [purpose].
String enrollmentPurposeText(AppLocalizations l, EnrollmentPurpose purpose) =>
    switch (purpose) {
      EnrollmentPurpose.firstLogin => l.adminCodePurposeFirstLogin,
      EnrollmentPurpose.recovery => l.adminCodePurposeRecovery,
      EnrollmentPurpose.identityLink => l.adminCodePurposeIdentityLink,
    };

/// The code of [name] in groups, copyable and read character by character ([CodeText]), with
/// its purpose and expiry and the note that it is shown only once.
///
/// The dialog holds the only copy of the code: the caller passes it in and keeps no
/// reference, so closing the dialog discards it from memory; a repeated request does not show
/// it again (the server answers `409 one_time_secret`).
class EnrollmentCodeView extends StatelessWidget {
  /// Creates the view.
  const EnrollmentCodeView({
    super.key,
    required this.name,
    required this.code,
    required this.timeZone,
    required this.now,
  });

  /// The display name of the person.
  final String name;

  /// The code.
  final EnrollmentCodeInfo code;

  /// The time zone the expiry is shown in.
  final String timeZone;

  /// The current time, for the relative day of the expiry.
  final DateTime now;

  /// Shows the view over [context] until "Close".
  static Future<void> show(
    BuildContext context, {
    required String name,
    required EnrollmentCodeInfo code,
    required String timeZone,
    DateTime Function() clock = DateTime.now,
  }) => showDialog<void>(
    context: context,
    barrierDismissible: false,
    builder: (_) => EnrollmentCodeView(
      name: name,
      code: code,
      timeZone: timeZone,
      now: clock(),
    ),
  );

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    return AlertDialog(
      key: const ValueKey('code.dialog'),
      backgroundColor: colors.surfaceRaised,
      title: Text(
        l.adminCodeTitle(name),
        style: dsTextStyle(DsType.title).copyWith(color: colors.textPrimary),
      ),
      content: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: DsLayout.overlayWidth),
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              DsBanner(tone: DsTone.warning, text: l.adminCodeOnce),
              const SizedBox(height: DsSpace.s16),
              CodeText(code.code, keepGroups: true),
              const SizedBox(height: DsSpace.s8),
              Text(
                enrollmentPurposeText(l, code.purpose),
                textAlign: TextAlign.center,
                style: dsTextStyle(DsType.bodyStrong)
                    .copyWith(color: colors.textPrimary),
              ),
              Text(
                l.adminCodeExpires(
                  activityText(
                    context,
                    code.expiresAt,
                    now: now,
                    timeZone: timeZone,
                  ),
                ),
                key: const ValueKey('code.expires'),
                textAlign: TextAlign.center,
                style: dsTextStyle(DsType.body)
                    .copyWith(color: colors.textSecondary),
              ),
            ],
          ),
        ),
      ),
      actions: [
        FilledButton(
          key: const ValueKey('code.close'),
          onPressed: () => Navigator.of(context).pop(),
          child: Text(l.adminCodeClose),
        ),
      ],
    );
  }
}
