// SPDX-License-Identifier: Apache-2.0

/// Typing the code of a waiting device (docs/spec/08-identity.md 5.2; 01 5.6 E4: "Type this
/// code on one of your active devices: Settings, Devices and sessions, Confirm").
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:l10n/l10n.dart';

/// Asks for the verification code that the waiting device [deviceName] shows.
///
/// The field accepts digits only (`DeviceApprove.verification_code` is `^[0-9]+$`). Pops the
/// code for "Confirm" and null for "Cancel"; the caller sends it (`approveDevice`), and a wrong
/// code revokes the waiting device on the server.
class ConfirmDeviceDialog extends StatefulWidget {
  /// Creates the dialog.
  const ConfirmDeviceDialog({super.key, required this.deviceName});

  /// The name of the waiting device.
  final String deviceName;

  /// Shows the dialog over [context]; returns the typed code or null.
  static Future<String?> show(BuildContext context, String deviceName) =>
      showDialog<String>(
        context: context,
        builder: (_) => ConfirmDeviceDialog(deviceName: deviceName),
      );

  @override
  State<ConfirmDeviceDialog> createState() => _ConfirmDeviceDialogState();
}

class _ConfirmDeviceDialogState extends State<ConfirmDeviceDialog> {
  final TextEditingController _code = TextEditingController();

  @override
  void initState() {
    super.initState();
    _code.addListener(() => setState(() {}));
  }

  @override
  void dispose() {
    _code.dispose();
    super.dispose();
  }

  void _submit() {
    if (_code.text.isEmpty) return;
    Navigator.of(context).pop(_code.text);
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    return CodeDialog(
      dialogKey: const ValueKey('confirmDevice.dialog'),
      title: l.devicesConfirmTitle(widget.deviceName),
      hint: l.devicesConfirmHint,
      field: TextField(
        key: const ValueKey('confirmDevice.code'),
        controller: _code,
        autofocus: true,
        keyboardType: TextInputType.number,
        inputFormatters: [FilteringTextInputFormatter.digitsOnly],
        style: dsTextStyle(DsType.code),
        decoration: InputDecoration(labelText: l.devicesConfirmCode),
        onSubmitted: (_) => _submit(),
      ),
      submitKey: const ValueKey('confirmDevice.submit'),
      submitLabel: l.devicesConfirmSubmit,
      onSubmit: _code.text.isEmpty ? null : _submit,
    );
  }
}

/// The frame of the two code dialogs: title, hint, the field and "Cancel" and the submit
/// button.
class CodeDialog extends StatelessWidget {
  /// Creates the frame.
  const CodeDialog({
    super.key,
    required this.dialogKey,
    required this.title,
    required this.hint,
    required this.field,
    required this.submitKey,
    required this.submitLabel,
    required this.onSubmit,
  });

  /// The key of the dialog, for tests.
  final Key dialogKey;

  /// The title.
  final String title;

  /// Where the code comes from.
  final String hint;

  /// The code field.
  final Widget field;

  /// The key of the submit button.
  final Key submitKey;

  /// The label of the submit button.
  final String submitLabel;

  /// The submit action; null while the field is empty.
  final VoidCallback? onSubmit;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    return AlertDialog(
      key: dialogKey,
      backgroundColor: colors.surfaceRaised,
      title: Text(
        title,
        style: dsTextStyle(DsType.title).copyWith(color: colors.textPrimary),
      ),
      content: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: DsLayout.overlayWidth),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(
              hint,
              style: dsTextStyle(DsType.body)
                  .copyWith(color: colors.textSecondary),
            ),
            const SizedBox(height: DsSpace.s16),
            field,
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: Text(l.selfCancel),
        ),
        FilledButton(
          key: submitKey,
          onPressed: onSubmit,
          child: Text(submitLabel),
        ),
      ],
    );
  }
}
