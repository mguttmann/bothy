// SPDX-License-Identifier: Apache-2.0

// The goldens of the agent interface (brief M4-21 Tests; 01 D3, D4, D5, D1; components.md 2, 5,
// 8, 9, 10): the live box in the four states of D3 with the collapsed line and the interrupted
// line, the card states of components.md 8, the slash suggestions with the session column's
// agent part; light and dark, densities comfortable and compact, text scaling 1 and 2, German
// and English. One file per scene and theme keeps each file below repo.max_file_kb.

import 'package:alchemist/alchemist.dart';
import 'package:client/agents/commands/builtin_commands.dart';
import 'package:client/agents/commands/command_suggestions.dart';
import 'package:client/agents/components_mapping.dart';
import 'package:client/agents/run_mapping.dart';
import 'package:client/session_column/agent_section.dart';
import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';
import 'package:sync_engine/sync_engine.dart';

import '../../support/agent_fakes.dart';
import '../../support/card_host.dart' show cardId, cardMessage;
import '../../support/structure.dart';
import '../../support/thread_fixtures.dart';

final Map<PrincipalId, Principal> principals = {
  for (final p in people) p.id: p,
};

DsRunData fromDoc(Map<String, Object?> json) => runFromDocument(
  RunDocument.fromJson(json),
  agentName: 'Robin',
  now: t0.add(const Duration(minutes: 4, seconds: 12)),
);

Widget scene(double width, List<Widget> children) => Builder(
  builder: (context) => ColoredBox(
    color: context.dsColors.surfaceCanvas,
    child: SizedBox(
      width: width,
      child: Padding(
        padding: const EdgeInsets.all(DsSpace.s12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          mainAxisSize: MainAxisSize.min,
          children: [
            for (final c in children) ...[
              c,
              const SizedBox(height: DsSpace.s12),
            ],
          ],
        ),
      ),
    ),
  ),
);

/// D3: running, collapsed after the end, waiting for an answer, failed; and the line of an
/// interrupted run.
Widget liveBoxes() => Builder(
  builder: (context) {
    final l = AppLocalizations.of(context);
    const interval = Duration(seconds: 10);
    return scene(720, [
      DsLiveBox(
        fromDoc(
          runDoc(
            1,
            statusLine: 'Checking if resume restores the focus',
            lastText: 'On resume, the focus is taken over from the session.',
            contextPct: 62,
            steps: {
              's1': step('s1', 1, status: 'completed', name: 'read'),
              's2': step('s2', 2, kind: 'subagent', name: 'tests'),
              's3': step('s3', 3, parent: 's2', name: 'bash'),
            },
          ),
        ),
        visibleSteps: 6,
        announceMinInterval: interval,
        onStop: () {},
        onCollapse: () {},
      ),
      DsLiveBox.collapsed(
        runFromSummary(
          run(
            1,
            RunState.completed,
          ).copyWith(durationMs: 28 * 60 * 1000, stepCount: 3),
          agentName: 'Robin',
          now: t0,
          steps: (tools: 3, subagents: 2),
        ),
        announceMinInterval: interval,
        onExpand: () {},
      ),
      DsLiveBox(
        fromDoc(
          runDoc(
            1,
            state: 'waiting_input',
            statusLine: 'Which database should the test use?',
            mode: 'personal',
          ),
        ),
        visibleSteps: 6,
        announceMinInterval: interval,
        onStop: () {},
      ),
      DsLiveBox(
        fromDoc(runDoc(1, state: 'failed', reason: 'billing_mode_mismatch')),
        visibleSteps: 6,
        announceMinInterval: interval,
      ),
      Row(
        children: [
          Expanded(child: Text(l.liveBoxReasonInterrupted)),
          DsButton(label: l.liveBoxSendAgain, onPressed: () {}),
        ],
      ),
    ]);
  },
);

/// components.md 8: active, not permitted, sent and waiting, used, expired, disabled, error.
Widget cards() => Builder(
  builder: (context) {
    final l = AppLocalizations.of(context);
    final active = [
      button(1, cardId, customId: 'once', label: 'Allow once'),
      button(
        2,
        cardId,
        customId: 'session',
        label: 'For this session',
        style: 'secondary',
      ),
      button(3, cardId, customId: 'always', label: 'Always', style: 'success'),
      button(4, cardId, customId: 'deny', label: 'Deny', style: 'danger'),
    ];
    final message = cardMessage('go test wants to use the network.');
    final header = componentHeader(
      message,
      [
        button(
          1,
          cardId,
          customId: 'once',
          label: 'Allow once',
          capability: Capability.approveActions,
          restrictedTo: ComponentRestriction.runStarter,
        ),
      ],
      principals: principals,
      runLabel: '09:30',
      runStarterName: 'Alex',
    );
    Widget card(
      List<MessageComponent> components, {
      bool permitted = true,
      String? reason,
      String? pending,
      String? error,
    }) => DsMessageComponents(
      componentData(
        message,
        components,
        principals: principals,
        now: t0,
        timeLabel: (_) => '10:24',
        pendingCustomId: pending,
        errorText: error,
      ),
      permitted: permitted,
      deniedReasonText: reason,
      header: header,
      agentText: message.contentMd,
      onOpenLink: (_) {},
      onActivate: (_, _) {},
    );
    return scene(640, [
      card(active),
      card(
        active,
        permitted: false,
        reason: l.componentsApproverRunStarter('Alex'),
      ),
      card(active, pending: 'once'),
      card([
        button(
          1,
          cardId,
          customId: 'once',
          label: 'Allow once',
          state: ComponentState.used,
          usedBy: alex,
          usedAt: t0,
        ),
      ]),
      card([
        button(
          1,
          cardId,
          customId: 'once',
          label: 'Allow once',
          state: ComponentState.expired,
        ),
      ]),
      card([
        button(
          1,
          cardId,
          customId: 'once',
          label: 'Allow once',
          state: ComponentState.disabled,
        ),
      ]),
      card(active, error: l.problemComponentUsed),
    ]);
  },
);

/// D5 and D1: the slash suggestions with the operator's commands first, the follow-up form,
/// and the agent part of the session column.
Widget commandsAndSession() => Builder(
  builder: (context) {
    final l = AppLocalizations.of(context);
    final nova = PrincipalId.parse(uuid(0xa4));
    final all = {
      ...principals,
      nova: Principal(
        id: nova,
        kind: PrincipalKind.agent,
        handle: 'nova',
        displayName: 'Nova',
      ),
    };
    Command command(int n, String name, PrincipalId owner) => Command(
      id: CommandId.parse(uuid(0x900 + n)),
      ownerPrincipalId: owner,
      name: name,
      description: 'About $name',
      scopeKind: TargetKind.instance,
      options: const [],
      requiredLevel: CommandLevel.trigger,
      control: false,
      version: const ResourceVersion(1),
    );
    final items = commandSuggestions(
      'st',
      builtins: BuiltinCommand.values,
      commands: [command(1, 'stats', robin), command(2, 'stats', nova)],
      operatorId: robin,
      principals: all,
    );
    return scene(640, [
      DsComposer(
        targetName: 'Deploy',
        followupAgentName: 'Robin',
        maxLines: 6,
        enterSends: true,
        onSend: (_) {},
        controller: TextEditingController(text: '/st'),
        suggestions: [for (final i in items) suggestionOf(l, i, all)],
      ),
      DsComposer(
        targetName: 'Deploy',
        maxLines: 6,
        enterSends: true,
        onSend: (_) {},
        controller: TextEditingController(text: '/model model-l'),
        command: const DsComposerCommand(
          name: '/model',
          options: [
            DsCommandOption(name: 'name', required: true, value: 'model-l'),
          ],
        ),
        suggestions: const [
          DsSuggestion(id: 'choice.model-large', command: 'model-large'),
        ],
      ),
      Text(l.agentWorking(dsIsolate('Robin'))),
      AgentSectionView(
        data: (
          name: 'Robin',
          presence: AgentRuntimeState.online,
          billingMode: BillingMode.organization,
          reportedBillingMode: BillingMode.personal,
          model: 'model-large',
          session: 'sess-7f3a',
          project: 'website, main',
          contextPercent: 62,
          costs: [
            (
              mode: BillingMode.organization,
              money: const Money(
                amountMicros: 1840000,
                currency: 'USD',
                isEstimate: true,
              ),
            ),
          ],
          runs: (count: 5, more: false),
          earlier: const ['sess-1a2b', 'sess-9c8d'],
        ),
      ),
    ]);
  },
);

void main() {
  for (final theme in DsThemeMode.values) {
    goldenTest(
      'the live box in ${theme.name}: running, collapsed, waiting for an '
      'answer, failed and interrupted',
      fileName: 'agents_livebox_${theme.name}',
      builder: () => dsGoldenMatrix(
        name: 'live box',
        themes: [theme],
        densities: DsDensityMode.values,
        textScales: const [1.0, 2.0],
        locales: const [Locale('de'), Locale('en')],
        builder: liveBoxes,
      ),
    );
    goldenTest(
      'the card states in ${theme.name}: active, not permitted, waiting, '
      'used, expired, disabled, error',
      fileName: 'agents_cards_${theme.name}',
      builder: () => dsGoldenMatrix(
        name: 'cards',
        themes: [theme],
        densities: DsDensityMode.values,
        textScales: const [1.0, 2.0],
        locales: const [Locale('de'), Locale('en')],
        builder: cards,
      ),
    );
    goldenTest(
      'the slash suggestions and the session agent part in ${theme.name}',
      fileName: 'agents_commands_session_${theme.name}',
      builder: () => dsGoldenMatrix(
        name: 'commands and session',
        themes: [theme],
        densities: DsDensityMode.values,
        textScales: const [1.0, 2.0],
        locales: const [Locale('de'), Locale('en')],
        builder: commandsAndSession,
      ),
    );
  }
}
