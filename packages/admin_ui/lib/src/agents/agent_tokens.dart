// SPDX-License-Identifier: Apache-2.0

/// The API tokens of an agent on its page (08 12; ADR 0061; brief M4-28 Design note 4, fixed
/// requirement 16): the list with prefix, validity and last use, "Create token" with the secret
/// shown once, and "Revoke" per active token.
library;

import 'package:api_client/api_client.dart' show ApiProblem;
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:l10n/l10n.dart';

import '../common/admin_action.dart';
import '../common/confirm_change.dart';
import '../common/problem_banner.dart';
import '../data/admin_repository.dart';
import '../data/agents_repository.dart';
import '../grants/grant_labels.dart' show lastDayLabel;
import '../groups/member_editor.dart' show parseDay;
import '../self/device_tile.dart' show activityText;
import '../self/self_feedback.dart';

/// The token section of the agent page: list (`listTokens`, newest first), create with the
/// secret shown once, revoke.
///
/// The secret of a new token exists only in the answer of `createToken` and in the dialog that
/// shows it: the section keeps no reference, so after "Close" it is gone from the state and
/// from the tree, and a reload shows only the prefix (08 12: shown once). `token.create` has
/// step-up and `token.revoke` has none (05); the step-up interceptor follows the server's
/// answer, the section never asks itself. A `403` hides the refused control (15 1 principle 1).
///
/// Example: `AgentTokens(repository: repository, agent: agent, timeZone: 'Europe/Berlin')`.
class AgentTokens extends StatefulWidget {
  /// Creates the section for [agent].
  const AgentTokens({
    super.key,
    required this.repository,
    required this.agent,
    this.timeZone = 'UTC',
    this.clock = DateTime.now,
  });

  /// The administration's calls.
  final AdminRepository repository;

  /// The agent whose tokens are shown.
  final AgentRecord agent;

  /// The time zone of the signed-in person.
  final String timeZone;

  /// The current time.
  final DateTime Function() clock;

  @override
  State<AgentTokens> createState() => _AgentTokensState();
}

class _AgentTokensState extends State<AgentTokens>
    with AdminActionState<AgentTokens> {
  List<AgentToken>? _tokens;
  ApiProblem? _problem;

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void didUpdateWidget(AgentTokens old) {
    super.didUpdateWidget(old);
    if (old.agent.id != widget.agent.id) _load();
  }

  Future<void> _load() async {
    try {
      final tokens = await widget.repository.tokens(widget.agent.id);
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
    final agent = widget.agent.displayName;
    final request = await showDialog<_TokenRequest>(
      context: context,
      builder: (_) =>
          _CreateTokenDialog(agentName: agent, timeZone: widget.timeZone),
    );
    if (request == null || !mounted) return;
    AgentTokenCreated? created;
    await runChange('create', () async {
      created = await widget.repository.createToken(
        widget.agent.id,
        name: request.name,
        expiresAt: request.expiresAt,
        dpopJkt: request.dpopJkt,
        purpose: l.stepUpPurposeCreateToken(agent),
      );
      return null;
    });
    final shown = created;
    // Dropped before the dialog opens, so only the dialog holds the secret (Design note 4).
    created = null;
    if (shown != null && mounted) {
      await TokenSecretView.show(
        context,
        name: shown.token.name,
        secret: shown.secret,
      );
    }
    if (mounted) await _load();
  }

  Future<void> _revoke(AgentToken token) async {
    final l = AppLocalizations.of(context);
    final ok = await ConfirmChange.show(
      context,
      title: l.adminTokenRevokeTitle(token.name),
      body: l.adminTokenRevokeBody,
      confirmLabel: l.adminTokenRevoke,
      destructive: true,
    );
    if (ok == null || !mounted) return;
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
    return Column(
      key: const ValueKey('agentTokens'),
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
            if (tokens != null && !denied.contains('create'))
              OutlinedButton(
                key: const ValueKey('tokens.create'),
                onPressed: busy ? null : _create,
                child: Text(l.adminTokenCreate),
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
    return Padding(
      key: ValueKey('token.${token.id}'),
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
              key: ValueKey('token.${token.id}.revoke'),
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

/// The form "Create token": name, last day (`YYYY-MM-DD`) and an optional DPoP thumbprint;
/// pops a [_TokenRequest] or null.
class _CreateTokenDialog extends StatefulWidget {
  const _CreateTokenDialog({required this.agentName, required this.timeZone});

  final String agentName;

  final String timeZone;

  @override
  State<_CreateTokenDialog> createState() => _CreateTokenDialogState();
}

class _CreateTokenDialogState extends State<_CreateTokenDialog> {
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

  /// The end of the typed last day: the start of the day after it in the person's time zone,
  /// so the token holds through the typed day (as a group membership does, 07 2.1).
  DateTime? _expiresAt(BuildContext context) {
    final day = parseDay(_lastDay.text);
    if (day == null) return null;
    final next = DateTime.utc(day.year, day.month, day.day + 1);
    final format = L10nFormat(Localizations.localeOf(context));
    try {
      return format.dayStart(
        next.year,
        next.month,
        next.day,
        timeZone: widget.timeZone,
      );
    } on ArgumentError {
      return next;
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final name = _name.text.trim();
    final dayText = _lastDay.text.trim();
    final expiresAt = _expiresAt(context);
    final dpop = _dpop.text.trim();
    return ConfirmChange(
      title: l.adminTokenCreateTitle(widget.agentName),
      body: l.adminTokenCreateBody(widget.agentName),
      confirmLabel: l.adminTokenCreate,
      // 08 12: an agent token is bound to the adapter's DPoP key (`auth.dpop_required_for`
      // contains `agent` by default), so the thumbprint is required like the name and the day.
      canConfirm: name.isNotEmpty && expiresAt != null && dpop.isNotEmpty,
      onConfirm: (_) =>
          Navigator.of(context).pop(_TokenRequest(name, expiresAt!, dpop)),
      fields: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          TextField(
            key: const ValueKey('token.name'),
            controller: _name,
            decoration: InputDecoration(labelText: l.adminTokenName),
          ),
          TextField(
            key: const ValueKey('token.lastDay'),
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
            key: const ValueKey('token.dpop'),
            controller: _dpop,
            decoration: InputDecoration(labelText: l.adminTokenDpop),
          ),
        ],
      ),
    );
  }
}

/// The secret of a new token, shown once: in the mono font, selectable and copyable, with the
/// note that it is shown only once (08 12; pattern of the enrollment code, M4-26 Design note
/// 4).
///
/// The dialog holds the only copy of [secret]: the caller passes it in and keeps no reference,
/// so closing the dialog discards it; the list shows only the prefix afterwards.
///
/// Example: `TokenSecretView.show(context, name: 'Deploy', secret: created.secret)`.
class TokenSecretView extends StatefulWidget {
  /// Creates the view of [secret] for the token [name].
  const TokenSecretView({super.key, required this.name, required this.secret});

  /// The name of the token.
  final String name;

  /// The secret.
  final String secret;

  /// Shows the view over [context] until "Close"; it cannot be dismissed by a tap outside, so
  /// the secret is not lost before it was copied.
  static Future<void> show(
    BuildContext context, {
    required String name,
    required String secret,
  }) => showDialog<void>(
    context: context,
    barrierDismissible: false,
    builder: (_) => TokenSecretView(name: name, secret: secret),
  );

  @override
  State<TokenSecretView> createState() => _TokenSecretViewState();
}

class _TokenSecretViewState extends State<TokenSecretView> {
  bool _copied = false;

  Future<void> _copy() async {
    await Clipboard.setData(ClipboardData(text: widget.secret));
    if (mounted) setState(() => _copied = true);
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    return AlertDialog(
      key: const ValueKey('token.secretDialog'),
      backgroundColor: colors.surfaceRaised,
      title: Text(
        l.adminTokenSecretTitle(widget.name),
        style: dsTextStyle(DsType.title).copyWith(color: colors.textPrimary),
      ),
      content: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: DsLayout.overlayWidth),
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              DsBanner(tone: DsTone.warning, text: l.adminTokenSecretOnce),
              const SizedBox(height: DsSpace.s16),
              Text(
                l.adminTokenSecretLabel,
                style: dsTextStyle(DsType.bodyStrong)
                    .copyWith(color: colors.textPrimary),
              ),
              const SizedBox(height: DsSpace.s4),
              Semantics(
                label: l.adminTokenSecretLabel,
                // SelectableText, not SelectionArea: in the web console a SelectionArea makes the
                // engine insert a style element without the nonce, which the CSP blocks (brief
                // M6-04, D-M6-04-6).
                child: SelectableText(
                  widget.secret,
                  key: const ValueKey('token.secret'),
                  style: dsTextStyle(DsType.code)
                      .copyWith(color: colors.textPrimary),
                ),
              ),
              Align(
                alignment: AlignmentDirectional.centerStart,
                child: TextButton(
                  key: const ValueKey('token.copy'),
                  onPressed: _copy,
                  child: Text(_copied ? l.onboardingCopied : l.onboardingCopy),
                ),
              ),
            ],
          ),
        ),
      ),
      actions: [
        FilledButton(
          key: const ValueKey('token.secretClose'),
          onPressed: () => Navigator.of(context).pop(),
          child: Text(l.adminCodeClose),
        ),
      ],
    );
  }
}
