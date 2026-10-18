// SPDX-License-Identifier: Apache-2.0

/// An owner approves the waiting device of a person with the code that device shows
/// (docs/spec/08-identity.md 5.2: "The approving person types it in"; the code never appears
/// for owners, so the person reads it out).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:l10n/l10n.dart';

import '../self/confirm_device_dialog.dart' show CodeDialog;

/// Asks for the verification code of the device [deviceName] of [personName].
///
/// Digits only (`DeviceApprove.verification_code` is `^[0-9]+$`). Pops the code for "Approve"
/// and null for "Cancel"; a wrong code revokes the waiting device on the server.
class ApproveDeviceDialog extends StatefulWidget {
  /// Creates the dialog.
  const ApproveDeviceDialog({
    super.key,
    required this.personName,
    required this.deviceName,
  });

  /// The display name of the person.
  final String personName;

  /// The name of the waiting device.
  final String deviceName;

  /// Shows the dialog over [context]; returns the typed code or null.
  static Future<String?> show(
    BuildContext context, {
    required String personName,
    required String deviceName,
  }) => showDialog<String>(
    context: context,
    builder: (_) =>
        ApproveDeviceDialog(personName: personName, deviceName: deviceName),
  );

  @override
  State<ApproveDeviceDialog> createState() => _ApproveDeviceDialogState();
}

class _ApproveDeviceDialogState extends State<ApproveDeviceDialog> {
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
      dialogKey: const ValueKey('approveDevice.dialog'),
      title: l.adminApproveTitle(widget.deviceName, widget.personName),
      hint: l.adminApproveHint(widget.personName),
      field: TextField(
        key: const ValueKey('approveDevice.code'),
        controller: _code,
        autofocus: true,
        keyboardType: TextInputType.number,
        inputFormatters: [FilteringTextInputFormatter.digitsOnly],
        style: dsTextStyle(DsType.code),
        decoration: InputDecoration(labelText: l.devicesConfirmCode),
        onSubmitted: (_) => _submit(),
      ),
      submitKey: const ValueKey('approveDevice.submit'),
      submitLabel: l.adminApprove,
      onSubmit: _code.text.isEmpty ? null : _submit,
    );
  }
}
