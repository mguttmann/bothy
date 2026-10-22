// SPDX-License-Identifier: Apache-2.0

/// The texts of "Permission hints" and "Expiring soon": the kind as icon and label and the
/// sentences of each hint or entry, from the templates of `l10n` filled with the names of the
/// page (docs/spec/07-permissions.md 6, 9.4, 9.5; brief M6-07 Design note 4).
///
/// Only wording: which hints exist and what they name is the server's (15 1 principle 1).
library;

import 'package:design_system/design_system.dart';
import 'package:l10n/l10n.dart';

import '../data/access_repository.dart';
import '../grants/grant_labels.dart';

/// The label of [kind], for example "Read horizon (H1)".
String hintKindLabel(AppLocalizations l, AccessHintKind kind) => switch (kind) {
  AccessHintKind.h1ReadHorizon => l.hintsKindH1,
  AccessHintKind.grantToDisabled => l.hintsKindGrantToDisabled,
  AccessHintKind.triggerBlocked => l.hintsKindTriggerBlocked,
  AccessHintKind.agentPersonal => l.hintsKindAgentPersonal,
  AccessHintKind.unknown => l.hintsKindUnknown,
};

/// The icon of [kind]: an eye for what an agent lets a person see, a prohibition sign for a
/// disabled subject, a lock for a blocked trigger, the personal badge for an agent on a
/// subscription.
DsGlyph hintKindGlyph(AccessHintKind kind) => switch (kind) {
  AccessHintKind.h1ReadHorizon => DsIcons.adminViewAs,
  AccessHintKind.grantToDisabled => DsIcons.runCancelled,
  AccessHintKind.triggerBlocked => DsIcons.securityLock,
  AccessHintKind.agentPersonal => DsIcons.billingPersonal,
  AccessHintKind.unknown => DsIcons.stateInfo,
};

/// The label of [kind], for example "Group membership".
String expiringKindLabel(AppLocalizations l, ExpiringKind kind) =>
    switch (kind) {
      ExpiringKind.grant => l.expiringKindGrant,
      ExpiringKind.membership => l.expiringKindMembership,
      ExpiringKind.token => l.expiringKindToken,
      ExpiringKind.unknown => l.expiringKindUnknown,
    };

/// The icon of [kind].
DsGlyph expiringKindGlyph(ExpiringKind kind) => switch (kind) {
  ExpiringKind.grant => DsIcons.securityKey,
  ExpiringKind.membership => DsIcons.group,
  ExpiringKind.token => DsIcons.stepBackgroundJob,
  ExpiringKind.unknown => DsIcons.stateInfo,
};

/// The name of the subject of [grant]: the group's name or the principal's display name.
String grantSubjectName(AdminGrant grant, AccessPageLabels labels) =>
    grant.subjectKind == AdminSubjectKind.group
    ? labels.group(grant.subjectId)
    : labels.principal(grant.subjectId);

/// The level of [grant] with its capabilities ("Trigger agents, Upload files"), or the
/// capabilities alone for a grant on a host, which has no level (07 2.12).
String grantLevelText(AppLocalizations l, AdminGrant grant) {
  final level = grant.level;
  if (level == null) {
    return grant.capabilities.map((c) => capabilityLabel(l, c)).join(', ');
  }
  return levelWithCapabilities(l, level, grant.capabilities);
}

/// The target of [grant] with the names of [labels].
AdminTarget grantTarget(AdminGrant grant, AccessPageLabels labels) =>
    labels.target(grant.targetKind, grant.targetId);

/// The sentences of [hint] in the language of [l], one per line: the hint itself and, for a
/// blocked trigger, one line per guard rule, for an agent on a subscription its trigger set.
List<String> hintLines(
  AppLocalizations l,
  AccessHint hint,
  AccessPageLabels labels,
) {
  final grant = hint.grant;
  switch (hint.kind) {
    case AccessHintKind.h1ReadHorizon:
      return [
        l.hintsH1Text(
          labels.principal(hint.personId ?? ''),
          labels.channelPath(hint.channelId ?? ''),
          labels.principal(hint.agentId ?? ''),
        ),
      ];
    case AccessHintKind.grantToDisabled when grant != null:
      return [
        l.hintsGrantToDisabledText(
          grantSubjectName(grant, labels),
          targetLabel(l, grantTarget(grant, labels)),
          grantLevelText(l, grant),
        ),
      ];
    case AccessHintKind.triggerBlocked when grant != null:
      return [
        l.hintsTriggerBlockedText(
          grantSubjectName(grant, labels),
          targetLabel(l, grantTarget(grant, labels)),
        ),
        for (final rule in hint.rules)
          ?switch (rule) {
            'sr2' => l.hintsRuleSr2,
            'sr3' => l.hintsRuleSr3,
            _ => null,
          },
      ];
    case AccessHintKind.agentPersonal:
      final agent = labels.principal(hint.agentId ?? '');
      final holder = hint.credentialHolderId;
      return [
        if (holder == null)
          l.hintsAgentPersonalNoHolder(agent)
        else
          l.hintsAgentPersonalText(agent, labels.principal(holder)),
        if (hint.triggerSet.isEmpty)
          l.hintsAgentPersonalNobody
        else
          l.hintsAgentPersonalSet(
            hint.triggerSet.map(labels.principal).join(', '),
          ),
      ];
    case _:
      // A kind this build does not know, or a grant kind without its grant.
      return [l.hintsUnknownText];
  }
}

/// The text of [entry] in the language of [l]: who and what expires.
String expiringText(
  AppLocalizations l,
  ExpiringEntry entry,
  AccessPageLabels labels,
) {
  final grant = entry.grant;
  return switch (entry.kind) {
    ExpiringKind.grant when grant != null => l.expiringGrantText(
      grantSubjectName(grant, labels),
      targetLabel(l, grantTarget(grant, labels)),
      grantLevelText(l, grant),
    ),
    ExpiringKind.membership => l.expiringMembershipText(
      labels.principal(entry.userId ?? ''),
      labels.group(entry.groupId ?? ''),
    ),
    ExpiringKind.token when isAppToken(entry, labels) => l.expiringAppTokenText(
      labels.principal(entry.principalId ?? ''),
    ),
    ExpiringKind.token => l.expiringAgentTokenText(
      labels.principal(entry.principalId ?? ''),
    ),
    _ => l.expiringUnknownText,
  };
}

/// Whether the token of [entry] belongs to an app (the labels name the holder's kind): the
/// administration has a token screen for agents only (M4-28), so an app token has no action and
/// a note on how to renew it.
bool isAppToken(ExpiringEntry entry, AccessPageLabels labels) =>
    labels.principals[entry.principalId]?.kind == 'app';
