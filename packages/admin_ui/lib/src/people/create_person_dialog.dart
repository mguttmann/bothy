// SPDX-License-Identifier: Apache-2.0

/// Creating a person: handle, display name and the local account (docs/spec/01-product.md 5.2
/// A1: no self-registration, no invitation links; 08 3.2).
library;

import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/confirm_change.dart';

/// What [CreatePersonDialog] asks for.
final class CreatePersonRequest {
  /// Creates the request.
  const CreatePersonRequest({
    required this.handle,
    required this.displayName,
    required this.localAccount,
  });

  /// The handle (`UserCreate.handle`, lower case, `^[a-z0-9][a-z0-9._-]*$`).
  final String handle;

  /// The display name.
  final String displayName;

  /// Whether the person gets a local identity and a `first_login` enrollment code.
  final bool localAccount;
}

/// The handle pattern of `UserCreate` (docs/api/openapi.yaml).
final RegExp handlePattern = RegExp(r'^[a-z0-9][a-z0-9._-]*$');

/// The confirmation of "Add person" with its fields ([ConfirmChange]): it says what happens
/// (an account; with a local account the code follows once). Pops a [CreatePersonRequest] for
/// "Create", null for "Cancel".
class CreatePersonDialog extends StatefulWidget {
  /// Creates the dialog.
  const CreatePersonDialog({super.key});

  /// Shows the dialog over [context].
  static Future<CreatePersonRequest?> show(BuildContext context) =>
      showDialog<CreatePersonRequest>(
        context: context,
        builder: (_) => const CreatePersonDialog(),
      );

  @override
  State<CreatePersonDialog> createState() => _CreatePersonDialogState();
}

class _CreatePersonDialogState extends State<CreatePersonDialog> {
  final TextEditingController _handle = TextEditingController();
  final TextEditingController _name = TextEditingController();
  bool _local = true;

  @override
  void initState() {
    super.initState();
    _handle.addListener(() => setState(() {}));
    _name.addListener(() => setState(() {}));
  }

  @override
  void dispose() {
    _handle.dispose();
    _name.dispose();
    super.dispose();
  }

  bool get _valid =>
      handlePattern.hasMatch(_handle.text.trim()) &&
      _name.text.trim().isNotEmpty;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final handle = _handle.text.trim();
    return ConfirmChange(
      title: l.adminCreatePersonTitle,
      body: l.adminCreatePersonBody,
      confirmLabel: l.adminCreatePersonSubmit,
      canConfirm: _valid,
      onConfirm: (_) => Navigator.of(context).pop(
        CreatePersonRequest(
          handle: handle,
          displayName: _name.text.trim(),
          localAccount: _local,
        ),
      ),
      fields: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          TextField(
            key: const ValueKey('createPerson.name'),
            controller: _name,
            autofocus: true,
            decoration: InputDecoration(labelText: l.adminCreatePersonName),
          ),
          TextField(
            key: const ValueKey('createPerson.handle'),
            controller: _handle,
            autocorrect: false,
            decoration: InputDecoration(
              labelText: l.adminCreatePersonHandle,
              errorText: handle.isEmpty || handlePattern.hasMatch(handle)
                  ? null
                  : l.adminCreatePersonHandleInvalid,
            ),
          ),
          CheckboxListTile(
            key: const ValueKey('createPerson.local'),
            contentPadding: EdgeInsets.zero,
            value: _local,
            title: Text(l.adminCreatePersonLocal),
            onChanged: (v) => setState(() => _local = v ?? true),
          ),
        ],
      ),
    );
  }
}
