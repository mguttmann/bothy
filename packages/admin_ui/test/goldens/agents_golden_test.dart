// SPDX-License-Identifier: Apache-2.0

// Goldens of screen A5 (brief M4-28 Tests): the agent list, the agent page with the server's
// warnings, the switch dialogs of one agent and of several agents, and the guided flow (the
// preview with the I1 conflict and its offer, and the result); light and dark, two densities,
// text scaling 1 and 2, German and English. One file per view keeps each file below
// repo.max_file_kb.

import 'package:admin_ui/admin_ui.dart';
import 'package:alchemist/alchemist.dart';
import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';

import '../admin/support.dart';

Widget _frame(Widget child) => Material(
  type: MaterialType.transparency,
  child: SizedBox(
    width: 760,
    child: Padding(padding: const EdgeInsets.all(16), child: child),
  ),
);

/// Support runs on organization while its adapter still reports personal with an unknown
/// login type: the server reports two warnings (10 8).
FakeAdminRepository _repository() {
  final r = FakeAdminRepository(prompt: StepUpDialogPrompt(() => null))
    ..stepUpFresh = true;
  r.agentRecordsById['support'] = fakeAgentRecord(
    'support',
    name: 'Support',
    reported: AgentBillingMode.personal,
    keyType: 'unknown',
  );
  r.agentRecordsById['desk'] = fakeAgentRecord(
    'desk',
    name: 'Desk',
    mode: AgentBillingMode.personal,
    holderId: 'sam',
    reported: null,
  );
  r.tokensById['support'] = [
    AgentToken(
      id: 'token-a',
      name: 'Adapter production',
      prefix: 'bothy_agt_EXAMPL',
      createdAt: DateTime.utc(2026, 10, 1),
      expiresAt: DateTime.utc(2027, 10, 1),
      lastUsedAt: DateTime.utc(2026, 10, 17, 8),
      dpopBound: true,
    ),
    AgentToken(
      id: 'token-b',
      name: 'Old adapter',
      prefix: 'bothy_agt_OLDKEY',
      createdAt: DateTime.utc(2026, 9, 1),
      expiresAt: DateTime.utc(2026, 12, 1),
      revokedAt: DateTime.utc(2026, 10, 2),
    ),
  ];
  r.agentList.add(
    const AdminAgent(id: 'support', handle: 'support', displayName: 'Support'),
  );
  return r;
}

const InvariantFinding _conflict = InvariantFinding(
  code: 'i1_violation',
  agentId: 'support',
  holderId: 'alex',
  principalIds: ['alex', 'sam'],
  organizationReady: false,
);

AccessNames _names(FakeAdminRepository r) =>
    AccessNames(people: r.peopleById.values.toList(), agents: r.agentList);

/// The scenes of the goldens, shared with the accessibility tests (test/a11y).
final Map<String, Widget Function()> views = {
  'list': () => _frame(AgentsList(repository: _repository(), onOpen: (_) {})),
  'page': () => _frame(
    AgentPage(repository: _repository(), agentId: 'support', clock: testClock),
  ),
  'switch': () {
    final r = _repository();
    return SwitchModeDialog(
      agent: r.agentRecordsById['desk']!,
      people: r.peopleById.values.toList(),
    );
  },
  'switch_all': () {
    final r = _repository();
    return SwitchAllDialog(
      repository: r,
      agents: r.agentRecordsById.values.toList(),
    );
  },
  'guided_preview': () {
    final r = _repository();
    return _frame(
      PreviewPanel(
        preview: const GrantPreviewResult(
          allowed: false,
          effects: [],
          violations: [_conflict],
          hints: [],
          text: 'Sam gets the level "trigger" on Support.',
          textLocale: 'en',
        ),
        names: _names(r),
        person: 'Sam',
        onGuided: (_) {},
      ),
    );
  },
  'i1': () {
    final r = _repository();
    return _frame(
      I1Explanation(
        conflicts: const [
          InvariantFinding(
            code: 'i1_violation',
            agentId: 'support',
            holderId: 'alex',
            principalIds: ['alex', 'sam', 'robin', 'hidden'],
          ),
        ],
        names: _names(r),
      ),
    );
  },
  'guided_result': () =>
      GuidedResult(repository: _repository(), agentIds: const ['support']),
};

void main() {
  for (final MapEntry(key: name, value: view) in views.entries) {
    goldenTest(
      '$name in light and dark, two densities, text scale 1 and 2, de and en',
      fileName: 'agents_$name',
      pumpBeforeTest: dsPumpIcons,
      builder: () => dsGoldenMatrix(
        name: name,
        builder: view,
        densities: const [DsDensityMode.comfortable, DsDensityMode.compact],
        textScales: const [1.0, 2.0],
        locales: const [Locale('de'), Locale('en')],
      ),
    );
  }
}
