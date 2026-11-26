// SPDX-License-Identifier: Apache-2.0

/// The dialog that sets the own password with its one-time code (docs/spec/08-identity.md 2.5:
/// setup in two steps; design note docs/design/sign-in-methods.md item 3).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:l10n/l10n.dart';

import '../data/self_repository.dart' show SelfActionFailed;
import '../data/sign_in_repository.dart';
import 'self_feedback.dart';
import 'totp_qr.dart';

/// What the password dialog does.
enum PasswordDialogMode {
  /// A first password and a new one-time code: both steps.
  setUp,

  /// A new password only; the confirmed one-time code stays.
  changePassword,

  /// A new one-time code only (the code was never confirmed, or a new phone): the second step.
  newCode,
}

/// Sets the password and the one-time code in two steps over [repository].
///
/// Step one sends the typed password (twice, equal) with `totp_enroll` ([PasswordDialogMode.setUp])
/// or alone ([PasswordDialogMode.changePassword]); [PasswordDialogMode.newCode] sends only
/// `totp_enroll`. Step two shows the otpauth URI of the answer as a QR code and its key as text,
/// and sends the code the person types. The secret lives only in this dialog's state. Every
/// failure stays in the dialog with its text (a cancelled step-up says nothing); the dialog pops
/// the success notice, or null when the person cancels.
class PasswordDialog extends StatefulWidget {
  /// Creates the dialog.
  const PasswordDialog({
    super.key,
    required this.repository,
    required this.mode,
  });

  /// Sets the password and the code.
  final SignInRepository repository;

  /// What the dialog does.
  final PasswordDialogMode mode;

  /// Shows the dialog over [context]; returns the success notice or null.
  static Future<SelfNotice?> show(
    BuildContext context, {
    required SignInRepository repository,
    required PasswordDialogMode mode,
  }) => showDialog<SelfNotice>(
    context: context,
    barrierDismissible: false,
    builder: (_) => PasswordDialog(repository: repository, mode: mode),
  );

  @override
  State<PasswordDialog> createState() => _PasswordDialogState();
}

class _PasswordDialogState extends State<PasswordDialog> {
  final _password = TextEditingController();
  final _repeat = TextEditingController();
  final _code = TextEditingController();
  String? _uri;
  String? _error;
  bool _busy = false;

  @override
  void initState() {
    super.initState();
    for (final c in [_password, _repeat, _code]) {
      c.addListener(() => setState(() {}));
    }
  }

  @override
  void dispose() {
    _password.dispose();
    _repeat.dispose();
    _code.dispose();
    super.dispose();
  }

  bool get _codeStep =>
      _uri != null || widget.mode == PasswordDialogMode.newCode;

  /// Runs [call]; a failure shows its text in the dialog (a field error of the server reads as
  /// the rule it broke).
  Future<void> _run(Future<void> Function() call) async {
    final l = AppLocalizations.of(context);
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await call();
    } on Object catch (e) {
      final text = _fieldText(l, e) ?? selfFailure(l, e)?.text;
      if (mounted) setState(() => _error = text);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  static String? _fieldText(AppLocalizations l, Object e) {
    if (e is! SelfActionFailed) return null;
    for (final f in e.problem?.errors ?? const []) {
      switch ((f.pointer, f.code)) {
        case ('/password', 'too_short'):
          return l.signInPasswordTooShort;
        case ('/password', 'common_password'):
          return l.signInPasswordCommon;
        case ('/totp_code', 'totp_invalid'):
          return l.signInTotpWrongCode;
      }
    }
    return null;
  }

  Future<void> _sendPassword() async {
    final l = AppLocalizations.of(context);
    if (_password.text != _repeat.text) {
      setState(() => _error = l.signInPasswordMismatch);
      return;
    }
    final change = widget.mode == PasswordDialogMode.changePassword;
    await _run(() async {
      final uri = await widget.repository.setPassword(
        _password.text,
        enrollTotp: !change,
        purpose: l.stepUpPurposeSetPassword,
      );
      if (!mounted) return;
      if (change) {
        Navigator.of(context).pop(SelfNotice(l.signInPasswordChanged));
        return;
      }
      setState(() => _uri = uri);
    });
  }

  Future<void> _newSecret() async {
    final l = AppLocalizations.of(context);
    await _run(() async {
      final uri = await widget.repository.setPassword(
        null,
        enrollTotp: true,
        purpose: l.stepUpPurposeSetPassword,
      );
      if (mounted) setState(() => _uri = uri);
    });
  }

  Future<void> _confirm() async {
    final l = AppLocalizations.of(context);
    await _run(() async {
      await widget.repository.confirmTotp(
        _code.text,
        purpose: l.stepUpPurposeSetPassword,
      );
      if (mounted) Navigator.of(context).pop(SelfNotice(l.signInPasswordDone));
    });
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final body = dsTextStyle(DsType.body).copyWith(color: colors.textSecondary);
    final uri = _uri;
    final List<Widget> fields;
    final String submitLabel;
    final VoidCallback? submit;
    if (!_codeStep) {
      fields = [
        TextField(
          key: const ValueKey('signIn.password.new'),
          controller: _password,
          autofocus: true,
          obscureText: true,
          enabled: !_busy,
          autofillHints: const [AutofillHints.newPassword],
          decoration: InputDecoration(labelText: l.signInPasswordNew),
        ),
        const SizedBox(height: DsSpace.s8),
        TextField(
          key: const ValueKey('signIn.password.repeat'),
          controller: _repeat,
          obscureText: true,
          enabled: !_busy,
          autofillHints: const [AutofillHints.newPassword],
          decoration: InputDecoration(labelText: l.signInPasswordRepeat),
          onSubmitted: (_) => _sendPassword(),
        ),
      ];
      submitLabel = widget.mode == PasswordDialogMode.changePassword
          ? l.signInSave
          : l.signInNext;
      submit = _busy || _password.text.isEmpty || _repeat.text.isEmpty
          ? null
          : _sendPassword;
    } else if (uri == null) {
      // newCode before the server sent the secret: one button asks for it.
      fields = [Text(l.signInTotpHint, style: body)];
      submitLabel = l.signInNext;
      submit = _busy ? null : _newSecret;
    } else {
      final key = totpSecretOf(uri);
      fields = [
        Text(l.signInTotpHint, style: body),
        const SizedBox(height: DsSpace.s16),
        Center(
          child: TotpQrCode(
            key: const ValueKey('signIn.totp.qr'),
            data: uri,
            semanticLabel: l.signInTotpQrLabel,
          ),
        ),
        if (key != null) ...[
          const SizedBox(height: DsSpace.s8),
          SelectableText(
            l.signInTotpKey(key),
            key: const ValueKey('signIn.totp.key'),
            textAlign: TextAlign.center,
            style: dsTextStyle(DsType.code).copyWith(color: colors.textPrimary),
          ),
        ],
        const SizedBox(height: DsSpace.s16),
        TextField(
          key: const ValueKey('signIn.totp.code'),
          controller: _code,
          autofocus: true,
          enabled: !_busy,
          keyboardType: TextInputType.number,
          autofillHints: const [AutofillHints.oneTimeCode],
          inputFormatters: [
            FilteringTextInputFormatter.digitsOnly,
            LengthLimitingTextInputFormatter(6),
          ],
          style: dsTextStyle(DsType.code),
          decoration: InputDecoration(labelText: l.signInTotpCode),
          onSubmitted: (_) => _code.text.length == 6 ? _confirm() : null,
        ),
      ];
      submitLabel = l.signInTotpConfirm;
      submit = _busy || _code.text.length != 6 ? null : _confirm;
    }
    return AlertDialog(
      key: const ValueKey('signIn.password.dialog'),
      backgroundColor: colors.surfaceRaised,
      title: Text(
        _codeStep ? l.signInTotpTitle : l.signInPasswordDialogTitle,
        style: dsTextStyle(DsType.title).copyWith(color: colors.textPrimary),
      ),
      content: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: DsLayout.overlayWidth),
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              if (_error case final error?) ...[
                DsBanner(
                  key: const ValueKey('signIn.password.error'),
                  tone: DsTone.danger,
                  text: error,
                  liveRegion: true,
                ),
                const SizedBox(height: DsSpace.s8),
              ],
              ...fields,
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          key: const ValueKey('signIn.password.cancel'),
          onPressed: _busy ? null : () => Navigator.of(context).pop(),
          child: Text(l.selfCancel),
        ),
        FilledButton(
          key: const ValueKey('signIn.password.submit'),
          onPressed: submit,
          child: Text(submitLabel),
        ),
      ],
    );
  }
}
