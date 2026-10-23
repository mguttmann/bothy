// SPDX-License-Identifier: Apache-2.0

/// The API tokens of an agent or an app (08 12; ADR 0061; briefs M4-28 Design note 4 and M6-08
/// Ruling 6): the list with prefix, validity and last use, "Create token" with the secret shown
/// once, and "Revoke" per active token. The agent page and the app page differ only in texts,
/// keys and whether the DPoP thumbprint is required ([PrincipalTokensStyle]).
library;

import 'package:api_client/api_client.dart' show ApiProblem;
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../data/admin_repository.dart';
import '../data/agents_repository.dart';
import '../grants/grant_labels.dart' show lastDayLabel;
import '../groups/member_editor.dart' show parseDay;
import '../secrets/once_only_secret.dart';
import '../self/device_tile.dart' show activityText;
import '../self/self_feedback.dart';
import 'admin_action.dart';
import 'confirm_change.dart';
import 'problem_banner.dart';

/// The texts and keys of one kind of token section.
///
/// Keys: the section `<sectionKey>`, the create button `<createKey>`, each row `<rowKey>.<id>`
/// and its revoke button `<rowKey>.<id>.revoke`, the form fields `<formKey>.name`,
/// `<formKey>.lastDay`, `<formKey>.dpop`, and the secret dialog with the prefix [secretKey]
/// ([OnceOnlySecret]).
final class PrincipalTokensStyle {
  /// Creates the style.
  const PrincipalTokensStyle({
    required this.sectionKey,
    required this.createKey,
    required this.rowKey,
    required this.formKey,
    required this.secretKey,
    required this.dpopRequired,
    required this.createLabel,
    required this.formTitle,
    required this.formBody,
    required this.dpopLabel,
    required this.purpose,
    required this.secretTitle,
    required this.secretNotice,
    required this.revokeBody,
  });

  /// The key of the section.
  final String sectionKey;

  /// The key of the create button.
  final String createKey;

  /// The prefix of the row keys.
  final String rowKey;

  /// The prefix of the form field keys.
  final String formKey;

  /// The key prefix of the secret dialog.
  final String secretKey;

  /// Whether the form requires the DPoP thumbprint (`auth.dpop_required_for` contains `agent`
  /// by default, never `app`).
  final bool dpopRequired;

  /// The label of the create button and of the form's confirmation.
  final String Function(AppLocalizations l) createLabel;

  /// The title of the form for the principal `name`.
  final String Function(AppLocalizations l, String name) formTitle;

  /// The body of the form for the principal `name`.
  final String Function(AppLocalizations l, String name) formBody;

  /// The label of the DPoP field.
  final String Function(AppLocalizations l) dpopLabel;

  /// The step-up purpose of `createToken` for the principal `name`.
  final String Function(AppLocalizations l, String name) purpose;

  /// The title of the secret dialog for the principal `name` and the token `tokenName`.
  final String Function(AppLocalizations l, String name, String tokenName)
  secretTitle;

  /// The warning of the secret dialog.
  final String Function(AppLocalizations l) secretNotice;

  /// The body of the revoke confirmation.
  final String Function(AppLocalizations l) revokeBody;
}

/// The token section of the principal [principalId] named [principalName]: list
/// (`listTokens`, newest first), create with the secret shown once, revoke.
///
/// The secret of a new token exists only in the answer of `createToken` and in the dialog that
/// shows it: the section keeps no reference, so after "Close" it is gone from the state and
/// from the tree, and a reload shows only the prefix (08 12: shown once). `token.create` has
/// step-up and `token.revoke` has none (05); the step-up interceptor follows the server's
/// answer, the section never asks itself. A `403` hides the refused control (15 1 principle 1).
///
/// Example: `PrincipalTokens(repository: r, principalId: a.id, principalName: a.displayName,
/// style: agentTokensStyle)`.
class PrincipalTokens extends StatefulWidget {
  /// Creates the section.
  const PrincipalTokens({
    super.key,
    required this.repository,
    required this.principalId,
    required this.principalName,
    required this.style,
    this.canCreate = true,
    this.timeZone = 'UTC',
    this.clock = DateTime.now,
  });

  /// The administration's calls.
  final AdminRepository repository;

  /// The agent or app whose tokens are shown.
  final String principalId;

  /// Its display name.
  final String principalName;

  /// Texts and keys.
  final PrincipalTokensStyle style;

  /// Whether "Create token" is offered (an uninstalled app gets none).
  final bool canCreate;

  /// The time zone of the signed-in person.
  final String timeZone;

  /// The current time.
  final DateTime Function() clock;

  @override
  State<PrincipalTokens> createState() => _PrincipalTokensState();
}

class _PrincipalTokensState extends State<PrincipalTokens>
    with AdminActionState<PrincipalTokens> {
  List<AgentToken>? _tokens;
  ApiProblem? _problem;

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void didUpdateWidget(PrincipalTokens old) {
    super.didUpdateWidget(old);
    if (old.principalId != widget.principalId) _load();
  }

  Future<void> _load() async {
    try {
      final tokens = await widget.repository.tokens(widget.principalId);
      // Newest first; the server's order is not part of the contract.
      tokens.sort((a, b) => b.createdAt.compareTo(a.createdAt));
      if (mounted) {
        setState(() {
          _tokens = tokens;
          _problem = null;
        });
      }
    } on Object catch (e) {
      if (mounted) setState(() => _problem = loadProblem(e));
    }
  }

  Future<void> _create() async {
    final l = AppLocalizations.of(context);
    final s = widget.style;
    final name = widget.principalName;
    final request = await showDialog<_TokenRequest>(
      context: context,
      builder: (_) =>
          _TokenForm(principalName: name, style: s, timeZone: widget.timeZone),
    );
    if (request == null || !mounted) return;
    AgentTokenCreated? created;
    await runChange('create', () async {
      created = await widget.repository.createToken(
        widget.principalId,
        name: request.name,
        expiresAt: request.expiresAt,
        dpopJkt: request.dpopJkt,
        purpose: s.purpose(l, name),
      );
      return null;
    });
    final shown = created;
    // Dropped before the dialog opens, so only the dialog holds the secret.
    created = null;
    if (shown != null && mounted) {
      await OnceOnlySecret.show(
        context,
        title: s.secretTitle(l, name, shown.token.name),
        label: l.adminTokenSecretLabel,
        notice: s.secretNotice(l),
        secret: shown.secret,
        keyPrefix: s.secretKey,
      );
    }
    if (mounted) await _load();
  }

  Future<void> _revoke(AgentToken token) async {
    final l = AppLocalizations.of(context);
    final ok = await ConfirmChange.show(
      context,
      title: l.adminTokenRevokeTitle(token.name),
      body: widget.style.revokeBody(l),
      confirmLabel: l.adminTokenRevoke,
      destructive: true,
    );
    if (ok == null || !mounted) return;
    // `token.revoke` has no step-up (05).
    await runChange('${token.id}.revoke', () async {
      await widget.repository.revokeToken(token.id);
      return SelfNotice(l.adminTokenRevokedNotice(token.name));
    });
    if (mounted) await _load();
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final tokens = _tokens;
    final s = widget.style;
    return Column(
      key: ValueKey(s.sectionKey),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Row(
          children: [
            Expanded(
              child: Semantics(
                header: true,
                child: Text(
                  l.adminTokensTitle,
                  style: dsTextStyle(DsType.heading)
                      .copyWith(color: colors.textPrimary),
                ),
              ),
            ),
            if (tokens != null &&
                widget.canCreate &&
                !denied.contains('create'))
              OutlinedButton(
                key: ValueKey(s.createKey),
                onPressed: busy ? null : _create,
                child: Text(s.createLabel(l)),
              ),
          ],
        ),
        const SizedBox(height: DsSpace.s8),
        noticeBanner(),
        if (_problem case final problem?)
          ProblemBanner(problem: problem, onRetry: _load)
        else if (tokens != null) ...[
          if (tokens.isEmpty)
            Text(
              l.adminTokensEmpty,
              style: dsTextStyle(DsType.body)
                  .copyWith(color: colors.textSecondary),
            ),
          for (final token in tokens) ...[
            _row(context, l, colors, token),
            Divider(height: 1, color: colors.borderSubtle),
          ],
        ],
      ],
    );
  }

  Widget _row(
    BuildContext context,
    AppLocalizations l,
    DsColors colors,
    AgentToken token,
  ) {
    final details = [
      if (token.revokedAt != null) l.adminTokenRevokedLabel,
      if (token.expiresAt case final end?)
        l.adminTokenValidUntil(
          // The form asks for the last day, so the list shows the same day.
          lastDayLabel(
            context,
            end,
            now: widget.clock(),
            timeZone: widget.timeZone,
          ),
        ),
      if (token.lastUsedAt case final used?)
        l.adminTokenLastUsed(
          activityText(
            context,
            used,
            now: widget.clock(),
            timeZone: widget.timeZone,
          ),
        ),
      if (token.dpopBound) l.adminTokenBound,
    ];
    final secondary = dsTextStyle(DsType.bodySm)
        .copyWith(color: colors.textSecondary);
    final row = widget.style.rowKey;
    return Padding(
      key: ValueKey('$row.${token.id}'),
      padding: const EdgeInsets.symmetric(vertical: DsSpace.s8),
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  token.name,
                  style: dsTextStyle(DsType.bodyStrong)
                      .copyWith(color: colors.textPrimary),
                ),
                // The prefix only (ADR 0061): it names the token, it is no secret.
                Text(
                  token.prefix,
                  style: dsTextStyle(DsType.code)
                      .copyWith(color: colors.textSecondary),
                ),
                Wrap(
                  spacing: DsSpace.s12,
                  children: [
                    for (final d in details) Text(d, style: secondary),
                  ],
                ),
              ],
            ),
          ),
          if (token.revokedAt == null && !denied.contains('${token.id}.revoke'))
            OutlinedButton(
              key: ValueKey('$row.${token.id}.revoke'),
              onPressed: busy ? null : () => _revoke(token),
              child: Text(l.adminTokenRevoke),
            ),
        ],
      ),
    );
  }
}

/// What the create form asks for.
final class _TokenRequest {
  const _TokenRequest(this.name, this.expiresAt, this.dpopJkt);

  final String name;

  final DateTime expiresAt;

  final String? dpopJkt;
}

/// The end of the typed last day [text] (`YYYY-MM-DD`): the start of the day after it in
/// [timeZone], so the token holds through the typed day (as a group membership does, 07 2.1);
/// null for a text that is no day.
///
/// Example: `tokenExpiry(context, '2026-12-31', 'Europe/Berlin')` is 2026-12-31 23:00 UTC.
DateTime? tokenExpiry(BuildContext context, String text, String timeZone) {
  final day = parseDay(text);
  if (day == null) return null;
  final next = DateTime.utc(day.year, day.month, day.day + 1);
  final format = L10nFormat(Localizations.localeOf(context));
  try {
    return format.dayStart(next.year, next.month, next.day, timeZone: timeZone);
  } on ArgumentError {
    return next;
  }
}

/// The form "Create token": name, last day (`YYYY-MM-DD`) and the DPoP thumbprint, required
/// or optional by the style; pops a [_TokenRequest] or null.
class _TokenForm extends StatefulWidget {
  const _TokenForm({
    required this.principalName,
    required this.style,
    required this.timeZone,
  });

  final String principalName;

  final PrincipalTokensStyle style;

  final String timeZone;

  @override
  State<_TokenForm> createState() => _TokenFormState();
}

class _TokenFormState extends State<_TokenForm> {
  final TextEditingController _name = TextEditingController();
  final TextEditingController _lastDay = TextEditingController();
  final TextEditingController _dpop = TextEditingController();

  @override
  void initState() {
    super.initState();
    for (final c in [_name, _lastDay, _dpop]) {
      c.addListener(() => setState(() {}));
    }
  }

  @override
  void dispose() {
    _name.dispose();
    _lastDay.dispose();
    _dpop.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final s = widget.style;
    final name = _name.text.trim();
    final dayText = _lastDay.text.trim();
    final expiresAt = tokenExpiry(context, _lastDay.text, widget.timeZone);
    final dpop = _dpop.text.trim();
    return ConfirmChange(
      title: s.formTitle(l, widget.principalName),
      body: s.formBody(l, widget.principalName),
      confirmLabel: s.createLabel(l),
      canConfirm:
          name.isNotEmpty &&
          expiresAt != null &&
          (!s.dpopRequired || dpop.isNotEmpty),
      onConfirm: (_) =>
          Navigator.of(context)
              .pop(_TokenRequest(name, expiresAt!, dpop.isEmpty ? null : dpop)),
      fields: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          TextField(
            key: ValueKey('${s.formKey}.name'),
            controller: _name,
            decoration: InputDecoration(labelText: l.adminTokenName),
          ),
          TextField(
            key: ValueKey('${s.formKey}.lastDay'),
            controller: _lastDay,
            keyboardType: TextInputType.datetime,
            decoration: InputDecoration(
              labelText: l.adminTokenLastDay,
              errorText: dayText.isEmpty || expiresAt != null
                  ? null
                  : l.adminTokenLastDayInvalid,
            ),
          ),
          TextField(
            key: ValueKey('${s.formKey}.dpop'),
            controller: _dpop,
            decoration: InputDecoration(labelText: s.dpopLabel(l)),
          ),
        ],
      ),
    );
  }
}
