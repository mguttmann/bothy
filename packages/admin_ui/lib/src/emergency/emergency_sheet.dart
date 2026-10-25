// SPDX-License-Identifier: Apache-2.0

/// The confirmation sheet of an emergency action on the phone (docs/spec/07-permissions.md
/// 9.5: revoking takes effect at once and says so; 08 6.2, 14.3: step-up with the device key;
/// R-PR-07; brief M5-09 Design note 2).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

/// The label of the confirming button of an action with step-up: "Confirm with Face ID" on
/// iOS, "Confirm with fingerprint or screen lock" elsewhere (the system prompt of Android).
///
/// The label only names what comes next: whether a step-up happens is the server's answer
/// (`401 step_up_required`), which the step-up interceptor of `auth` handles.
String emergencyStepUpLabel(BuildContext context) {
  final l = AppLocalizations.of(context);
  return switch (Theme.of(context).platform) {
    TargetPlatform.iOS || TargetPlatform.macOS => l.emergencyConfirmFaceId,
    _ => l.emergencyConfirmDeviceLock,
  };
}

/// The bottom sheet before one emergency action: [title], the plain-language [body] that says
/// what happens and that it takes effect at once, a reason field and "Cancel" and the
/// confirming button.
///
/// [stepUp] marks an action that 05 gives step-up: the button reads
/// [emergencyStepUpLabel]; otherwise it reads [confirmLabel]. With [reasonRequired] the
/// button stays disabled until a reason is typed (`revokeGrant` requires one). Pops the typed
/// reason (empty when none was typed) for "Confirm", null for "Cancel" or a swipe down.
///
/// Example:
/// ```dart
/// final reason = await EmergencyConfirmSheet.show(
///   context,
///   title: l.adminDisableTitle('Sam'),
///   body: l.emergencyDisableBody('Sam'),
///   stepUp: true,
///   confirmLabel: l.adminPersonDisable,
/// );
/// ```
class EmergencyConfirmSheet extends StatefulWidget {
  /// Creates the sheet.
  const EmergencyConfirmSheet({
    super.key,
    required this.title,
    required this.body,
    required this.stepUp,
    required this.confirmLabel,
    this.reasonRequired = false,
  });

  /// The question, for example "Disable Sam?".
  final String title;

  /// What happens, in plain language, with "at once".
  final String body;

  /// Whether 05 gives the action step-up.
  final bool stepUp;

  /// The label of the button of an action without step-up, for example "Revoke".
  final String confirmLabel;

  /// Whether a reason must be typed before confirming.
  final bool reasonRequired;

  /// Shows the sheet over [context]; returns the reason (empty when none was typed) or null.
  static Future<String?> show(
    BuildContext context, {
    required String title,
    required String body,
    required bool stepUp,
    required String confirmLabel,
    bool reasonRequired = false,
  }) => showModalBottomSheet<String>(
    context: context,
    isScrollControlled: true,
    useSafeArea: true,
    backgroundColor: context.dsColors.surfaceRaised,
    builder: (_) => EmergencyConfirmSheet(
      title: title,
      body: body,
      stepUp: stepUp,
      confirmLabel: confirmLabel,
      reasonRequired: reasonRequired,
    ),
  );

  @override
  State<EmergencyConfirmSheet> createState() => _EmergencyConfirmSheetState();
}

class _EmergencyConfirmSheetState extends State<EmergencyConfirmSheet> {
  final TextEditingController _reason = TextEditingController();

  @override
  void initState() {
    super.initState();
    _reason.addListener(() => setState(() {}));
  }

  @override
  void dispose() {
    _reason.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final reason = _reason.text.trim();
    final canConfirm = !widget.reasonRequired || reason.isNotEmpty;
    return Padding(
      key: const ValueKey('emergency.sheet'),
      // The keyboard of the reason field pushes the sheet up instead of covering the buttons.
      padding: EdgeInsets.only(bottom: MediaQuery.viewInsetsOf(context).bottom),
      child: SingleChildScrollView(
        padding: const EdgeInsets.all(DsSpace.s16),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Semantics(
              header: true,
              child: Text(
                widget.title,
                style: dsTextStyle(DsType.title)
                    .copyWith(color: colors.textPrimary),
              ),
            ),
            const SizedBox(height: DsSpace.s12),
            DsBanner(
              key: const ValueKey('emergency.sheet.body'),
              tone: DsTone.danger,
              text: widget.body,
            ),
            const SizedBox(height: DsSpace.s16),
            TextField(
              key: const ValueKey('emergency.sheet.reason'),
              controller: _reason,
              decoration: InputDecoration(
                labelText: widget.reasonRequired
                    ? l.adminReasonRequired
                    : l.adminReason,
              ),
            ),
            const SizedBox(height: DsSpace.s16),
            DsButton(
              key: const ValueKey('emergency.sheet.confirm'),
              label: widget.stepUp
                  ? emergencyStepUpLabel(context)
                  : widget.confirmLabel,
              style: DsButtonStyle.danger,
              expand: true,
              onPressed: canConfirm
                  ? () => Navigator.of(context).pop(reason)
                  : null,
            ),
            const SizedBox(height: DsSpace.s8),
            DsButton(
              key: const ValueKey('emergency.sheet.cancel'),
              label: l.selfCancel,
              expand: true,
              onPressed: () => Navigator.of(context).pop(),
            ),
          ],
        ),
      ),
    );
  }
}
