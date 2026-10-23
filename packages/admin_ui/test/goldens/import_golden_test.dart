// SPDX-License-Identifier: Apache-2.0

// Goldens of the import of M8-08 (brief M8-08 Tests): the list, the source page, the channel
// table in every phase, the create dialog, the operator dialog and every confirmation; light
// and dark, text scaling 1 and 2, German and English. One file per view keeps each file below
// repo.max_file_kb.

import 'package:admin_ui/admin_ui.dart';
import 'package:alchemist/alchemist.dart';
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

FakeAdminRepository _repository() =>
    FakeAdminRepository(prompt: StepUpDialogPrompt(() => null))
      ..stepUpFresh = true;

/// A running source with tickets led here (operator ok), questions mirrored and the existing
/// channel not imported, server and tool counters and one failed item.
(FakeAdminRepository, String) _running() {
  final r = _repository();
  final id = r.seedImport(
    'Old community server',
    state: 'running',
    stats: const {
      'server': {
        'message': {'posted': 120, 'skipped': 4},
        'user': {'posted': 9},
        'source_changed': 1,
      },
      'tool': {
        'threads': {'open': 3, 'done': 17},
        'reactions_not_taken': 52,
      },
    },
  );
  r.postChannels(id, ['301', '302']);
  final channels =
      (r.importsById[id]!['config']! as Map)['channels']! as List<Object?>;
  (channels.first! as Map)['lead'] = 'bothy';
  r
    ..operatorElsewhere(id, '301', 'texter')
    ..importItemsById[id]!.add(
      const ImportItemEntry(
        id: 'item-f',
        originKey: 'forum_tag:302:9',
        itemKind: 'forum_tag',
        state: 'failed',
        error: 'name_collision',
      ),
    );
  return (r, id);
}

ImportChannelRow _row(
  String discordId,
  ImportLead lead, {
  String? name,
  bool imported = true,
  String? operator,
  bool planned = true,
}) {
  final texter = fakeAgentRecord('texter', name: 'Texter');
  return ImportChannelRow(
    mapped: MappedChannel(
      discordId: discordId,
      lead: lead,
      createName: name,
      plannedAgentHandle: planned ? 'texter' : null,
    ),
    item: imported
        ? ImportItemEntry(
            id: 'item-$discordId',
            originKey: 'channel:$discordId',
            itemKind: 'channel',
            state: 'posted',
            targetId: 'ch-$discordId',
          )
        : null,
    channel: imported
        ? ImportChannelState(
            id: 'ch-$discordId',
            name: name ?? 'existing',
            readOnly: lead == ImportLead.discord,
            operatorAgentId: operator,
            version: 2,
          )
        : null,
    plannedAgent: planned ? texter : null,
  );
}

final List<ImportChannelRow> _rows = [
  _row('301', ImportLead.bothy, name: 'tickets', operator: 'texter'),
  _row('302', ImportLead.bothy, name: 'questions'),
  _row('303', ImportLead.bothy, name: 'releases', operator: 'support'),
  _row('304', ImportLead.discord, name: 'announcements', planned: false),
  _row('305', ImportLead.discord, name: 'archive', imported: false),
];

final Map<String, Widget Function()> _views = {
  'list': () {
    final r = _repository();
    r
      ..seedImport(
        'Pilot',
        state: 'failed',
        stats: const {
          'server': {'cancelled': true},
        },
      )
      ..seedImport(
        'Old community server',
        state: 'running',
        stats: const {
          'server': {
            'channel': {'posted': 2},
            'message': {'posted': 120},
          },
        },
      );
    return _frame(
      ImportListPage(
        repository: r,
        onOpen: (_) {},
        pickMapping: () async => null,
      ),
    );
  },
  'source': () {
    final (r, id) = _running();
    return _frame(ImportSourcePage(repository: r, sourceId: id, onBack: () {}));
  },
  'source_draft': () {
    final r = _repository();
    final id = r.seedImport('Old community server');
    return _frame(ImportSourcePage(repository: r, sourceId: id, onBack: () {}));
  },
  'channels': () => _frame(
    ImportChannelTable(
      rows: _rows,
      actionable: true,
      onLeadBothy: (_) {},
      onLeadDiscord: (_) {},
      onSetOperator: (_) {},
    ),
  ),
  'create': () => ImportCreateDialog(
    repository: _repository(),
    pickMapping: () async => null,
  ),
  'operator': () => ImportOperatorDialog(
    repository: _repository(),
    row: _rows[1],
    agents: [
      fakeAgentRecord('texter', name: 'Texter'),
      fakeAgentRecord('support', name: 'Support'),
    ],
  ),
  'operator_remove': () => ImportOperatorDialog(
    repository: _repository(),
    row: _rows[3],
    agents: const [],
    remove: true,
  ),
  for (final a in ImportStateAction.values)
    'confirm_${a.name}': () =>
        ImportConfirmDialog.state(action: a, name: 'Old community server'),
  'confirm_lead_bothy': () => ImportConfirmDialog.leadBothy(row: _rows[2]),
  'confirm_lead_discord': () => ImportConfirmDialog.leadDiscord(row: _rows[0]),
  'confirm_create': () =>
      const ImportConfirmDialog.create(name: 'Old community server'),
};

void main() {
  for (final MapEntry(key: name, value: view) in _views.entries) {
    goldenTest(
      '$name in light and dark, text scale 1 and 2, de and en',
      fileName: 'import_$name',
      pumpBeforeTest: dsPumpIcons,
      builder: () => dsGoldenMatrix(
        name: name,
        builder: view,
        textScales: const [1.0, 2.0],
        locales: const [Locale('de'), Locale('en')],
      ),
    );
  }
}
