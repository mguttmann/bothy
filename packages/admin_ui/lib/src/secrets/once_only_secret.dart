// SPDX-License-Identifier: Apache-2.0

/// The one dialog that shows a secret the server returns once: app and agent tokens, the
/// address of an incoming webhook with its token, the HMAC secret of an outgoing subscription
/// (11 4.1, 7, 8.1; 08 12; brief M6-08 Design note 4).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:l10n/l10n.dart';

/// Shows [secret] with a copy action and the warning that it will not be shown again.
///
/// The secret lives only in the state of this dialog: the caller hands it over and keeps no
/// reference, the state drops it when the person closes the dialog, and nothing here passes it
/// to a logger, a crash report, the local database or browser storage (fixed requirement 17). It
/// is excluded from the semantics tree until the person moves the focus onto it, so a screen
/// reader does not announce it when the dialog opens. The dialog cannot be dismissed by a tap
/// outside, so the secret is not lost before it was copied.
///
/// Keys (used by the widget and `patrol` tests): `<keyPrefix>.secretDialog`, `<keyPrefix>.secret`,
/// `<keyPrefix>.copy`, `<keyPrefix>.secretClose`.
///
/// Example:
/// ```dart
/// await OnceOnlySecret.show(
///   context,
///   title: l.adminTokenSecretTitle(token.name),
///   label: l.adminTokenSecretLabel,
///   notice: l.adminTokenSecretOnce,
///   secret: created.secret,
///   keyPrefix: 'token',
/// );
/// ```
class OnceOnlySecret extends StatefulWidget {
  /// Creates the dialog of [secret].
  const OnceOnlySecret({
    super.key,
    required this.title,
    required this.label,
    required this.notice,
    required this.secret,
    this.details = const [],
    this.keyPrefix = 'secret',
  });

  /// The title, for example "Token Deploy".
  final String title;

  /// The label above the secret, for example "Token secret".
  final String label;

  /// The warning that the secret is shown once and where it belongs.
  final String notice;

  /// The secret.
  final String secret;

  /// Further lines below the secret that are no secret, for example the key ID of the webhook
  /// signatures.
  final List<Widget> details;

  /// The prefix of the keys of the dialog's widgets.
  final String keyPrefix;

  /// Shows the dialog over [context] until "Close".
  static Future<void> show(
    BuildContext context, {
    required String title,
    required String label,
    required String notice,
    required String secret,
    List<Widget> details = const [],
    String keyPrefix = 'secret',
  }) => showDialog<void>(
    context: context,
    barrierDismissible: false,
    builder: (_) => OnceOnlySecret(
      title: title,
      label: label,
      notice: notice,
      secret: secret,
      details: details,
      keyPrefix: keyPrefix,
    ),
  );

  @override
  State<OnceOnlySecret> createState() => _OnceOnlySecretState();
}

class _OnceOnlySecretState extends State<OnceOnlySecret> {
  // The only place the secret is kept; null after "Close".
  String? _secret;
  bool _copied = false;
  bool _focused = false;

  @override
  void initState() {
    super.initState();
    _secret = widget.secret;
  }

  Future<void> _copy() async {
    final secret = _secret;
    if (secret == null) return;
    await Clipboard.setData(ClipboardData(text: secret));
    if (mounted) setState(() => _copied = true);
  }

  void _close() {
    setState(() => _secret = null);
    Navigator.of(context).pop();
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final p = widget.keyPrefix;
    final secret = _secret;
    return AlertDialog(
      key: ValueKey('$p.secretDialog'),
      backgroundColor: colors.surfaceRaised,
      title: Text(
        widget.title,
        style: dsTextStyle(DsType.title).copyWith(color: colors.textPrimary),
      ),
      content: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: DsLayout.overlayWidth),
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              DsBanner(tone: DsTone.warning, text: widget.notice),
              const SizedBox(height: DsSpace.s16),
              Text(
                widget.label,
                style: dsTextStyle(
                  DsType.bodyStrong,
                ).copyWith(color: colors.textPrimary),
              ),
              const SizedBox(height: DsSpace.s4),
              if (secret != null)
                Focus(
                  canRequestFocus: false,
                  onFocusChange: (f) => setState(() => _focused = f),
                  child: Semantics(
                    label: widget.label,
                    child: ExcludeSemantics(
                      excluding: !_focused,
                      // SelectableText, not SelectionArea: in the web console a SelectionArea
                      // makes the engine insert a style element without the nonce, which the
                      // CSP blocks (D-M6-04-6).
                      child: SelectableText(
                        secret,
                        key: ValueKey('$p.secret'),
                        style: dsTextStyle(
                          DsType.code,
                        ).copyWith(color: colors.textPrimary),
                      ),
                    ),
                  ),
                ),
              Align(
                alignment: AlignmentDirectional.centerStart,
                child: TextButton(
                  key: ValueKey('$p.copy'),
                  onPressed: secret == null ? null : _copy,
                  child: Text(_copied ? l.onboardingCopied : l.onboardingCopy),
                ),
              ),
              ...widget.details,
            ],
          ),
        ),
      ),
      actions: [
        FilledButton(
          key: ValueKey('$p.secretClose'),
          onPressed: _close,
          child: Text(l.adminCodeClose),
        ),
      ],
    );
  }
}
