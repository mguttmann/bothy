// SPDX-License-Identifier: Apache-2.0

/// "Add device": the pairing code of the new device (docs/spec/08-identity.md 5.3; 01 5.6 E5).
library;

import 'package:auth/auth.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import 'confirm_device_dialog.dart';

/// Asks for the pairing code that the new device shows.
///
/// The code is normalized as M4-09 does for typed codes (`normalizeCode`: case, spaces and
/// hyphens do not count). Pops the normalized code for "Add" and null for "Cancel".
class AddDeviceDialog extends StatefulWidget {
  /// Creates the dialog.
  const AddDeviceDialog({super.key});

  /// Shows the dialog over [context]; returns the normalized code or null.
  static Future<String?> show(BuildContext context) => showDialog<String>(
    context: context,
    builder: (_) => const AddDeviceDialog(),
  );

  @override
  State<AddDeviceDialog> createState() => _AddDeviceDialogState();
}

class _AddDeviceDialogState extends State<AddDeviceDialog> {
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

  String get _normalized => normalizeCode(_code.text);

  void _submit() {
    if (_normalized.isEmpty) return;
    Navigator.of(context).pop(_normalized);
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    return CodeDialog(
      dialogKey: const ValueKey('addDevice.dialog'),
      title: l.devicesAddTitle,
      hint: l.devicesAddHint,
      field: TextField(
        key: const ValueKey('addDevice.code'),
        controller: _code,
        autofocus: true,
        textCapitalization: TextCapitalization.characters,
        style: dsTextStyle(DsType.code),
        decoration: InputDecoration(labelText: l.devicesAddCode),
        onSubmitted: (_) => _submit(),
      ),
      submitKey: const ValueKey('addDevice.submit'),
      submitLabel: l.devicesAddSubmit,
      onSubmit: _normalized.isEmpty ? null : _submit,
    );
  }
}
