// SPDX-License-Identifier: Apache-2.0

/// The API tokens of an agent on its page (08 12; ADR 0061; brief M4-28 Design note 4, fixed
/// requirement 16): the list with prefix, validity and last use, "Create token" with the secret
/// shown once, and "Revoke" per active token.
library;

import 'package:flutter/material.dart';

import '../common/principal_tokens.dart';
import '../data/admin_repository.dart';
import '../data/agents_repository.dart';

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
