// SPDX-License-Identifier: Apache-2.0

/// The API tokens of an agent on its page (08 12; ADR 0061; brief M4-28 Design note 4, fixed
/// requirement 16): the list with prefix, validity and last use, "Create token" with the secret
/// shown once, and "Revoke" per active token.
library;

import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/principal_tokens.dart';
import '../data/admin_repository.dart';
import '../data/agents_repository.dart';
import '../secrets/once_only_secret.dart';

/// The token section of the agent page: [PrincipalTokens] with the texts of an agent and the
/// keys `agentTokens`, `tokens.create`, `token.<id>`, `token.name` and the secret `token.*`;
/// the DPoP thumbprint is required (08 12: `auth.dpop_required_for` contains `agent`).
///
/// Example: `AgentTokens(repository: repository, agent: agent, timeZone: 'Europe/Berlin')`.
class AgentTokens extends StatelessWidget {
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

  /// The texts and keys of an agent's tokens.
  static final PrincipalTokensStyle style = PrincipalTokensStyle(
    sectionKey: 'agentTokens',
    createKey: 'tokens.create',
    rowKey: 'token',
    formKey: 'token',
    secretKey: 'token',
    dpopRequired: true,
    createLabel: (l) => l.adminTokenCreate,
    formTitle: (l, name) => l.adminTokenCreateTitle(name),
    formBody: (l, name) => l.adminTokenCreateBody(name),
    dpopLabel: (l) => l.adminTokenDpop,
    purpose: (l, name) => l.stepUpPurposeCreateToken(name),
    secretTitle: (l, _, token) => l.adminTokenSecretTitle(token),
    secretNotice: (l) => l.adminTokenSecretOnce,
    revokeBody: (l) => l.adminTokenRevokeBody,
  );

  @override
  Widget build(BuildContext context) => PrincipalTokens(
    repository: repository,
    principalId: agent.id,
    principalName: agent.displayName,
    style: style,
    timeZone: timeZone,
    clock: clock,
  );
}

/// The dialog that shows the secret of a new token once: the shared [OnceOnlySecret] with the
/// texts of a token and the keys `token.*` (08 12; brief M6-08 Design note 4).
///
/// Example: `TokenSecretView.show(context, name: 'Deploy', secret: created.secret)`.
class TokenSecretView extends StatelessWidget {
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
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    return OnceOnlySecret(
      title: l.adminTokenSecretTitle(name),
      label: l.adminTokenSecretLabel,
      notice: l.adminTokenSecretOnce,
      secret: secret,
      keyPrefix: 'token',
    );
  }
}
