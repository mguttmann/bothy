// SPDX-License-Identifier: Apache-2.0

// The goldens of the thread view (brief M4-20 Tests; components.md 1 and 9; 01 6.1, 6.6, 6.7):
// the history with grouped, collapsed, edited, deleted, pending, delayed, failed and system
// messages under the header with an operator, and the header without an operator with an
// automatic title over the composer states (offline, too long, read only), light and dark,
// densities comfortable and compact, text scaling 1 and 2, German and English. One file per
// scene and theme keeps each file below repo.max_file_kb.

import 'dart:convert';

import 'package:alchemist/alchemist.dart';
import 'package:client/composer/mention_suggestions.dart';
import 'package:client/thread/message_menu.dart';
import 'package:client/thread/message_tile.dart';
import 'package:client/thread/outbox_rows.dart';
import 'package:client/thread/providers.dart';
import 'package:client/thread/thread_header.dart';
import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

import '../../support/structure.dart';
import '../../support/thread_fixtures.dart';

/// The size of the history column in a cell.
const Size size = Size(640, 1000);

final Map<PrincipalId, Principal> principals = {
  for (final p in people) p.id: p,
};

const ThreadPermissions writer = ThreadPermissions(
  canWrite: true,
  canManage: false,
  readOnly: false,
);

OutboxEntryView view(
  String id,
  String text,
  OutboxStateLabel label, {
  String? code,
}) => OutboxEntryView(
  entry: OutboxEntry(
    id: id,
    kind: MessageOps.createMessage,
    payload: jsonEncode({'content_md': text}),
    state: label == OutboxStateLabel.failed
        ? OutboxState.failed
        : OutboxState.pending,
    attempts: 1,
    createdAt: t0,
    stateChangedAt: t0,
    lastErrorCode: code,
  ),
  stateLabel: label,
);

Widget tile(Message m, {bool continuation = false, String time = '09:00'}) =>
    MessageTile(
      data: messageData(m, principals: principals, me: alex, timeLabel: time),
      continuation: continuation,
      collapseLines: 6,
      actions: messageActions(
        m,
        me: alex,
        permissions: writer,
        mainHistory: false,
      ),
      onAction: (_) {},
      onOpenLink: (_) {},
    );

Widget pending(OutboxEntryView v) => PendingRow(
  view: v,
  authorName: 'Alex',
  timeLabel: '09:40',
  collapseLines: 6,
  onResend: () {},
  onEdit: () {},
  onDiscard: () {},
  onOpenLink: (_) {},
);

/// The history scene.
Widget history() => Builder(
  builder: (context) => ColoredBox(
    color: context.dsColors.surfaceCanvas,
    child: SizedBox.fromSize(
      size: size,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          ThreadHeader(
            data: const ThreadHeaderData(
              title: 'Printer jam on floor 2',
              automaticTitle: false,
              operatorName: 'Robin',
              mainHistory: false,
              done: false,
              canEditTitle: true,
              canSetDone: true,
              canMove: true,
            ),
            onEditTitle: () {},
            onDone: () {},
            onMove: () {},
            onPin: () {},
            onColumn: () {},
          ),
          Expanded(
            child: ClipRect(
              child: OverflowBox(
                alignment: Alignment.topCenter,
                maxHeight: double.infinity,
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    tile(msg(1, author: sam, text: 'The printer stops.')),
                    tile(
                      msg(2, author: sam, text: 'After one page.', minutes: 1),
                      continuation: true,
                    ),
                    tile(
                      msg(
                        3,
                        author: robin,
                        origin: MessageOrigin.agent,
                        text: [
                          for (var i = 1; i <= 10; i++) 'Step $i of the check.',
                        ].join('\n\n'),
                        minutes: 5,
                      ),
                      time: '09:05',
                    ),
                    tile(
                      msg(
                        4,
                        author: alex,
                        text: '@robin thanks, that helped',
                        mentions: [robin],
                        edited: true,
                        minutes: 10,
                      ),
                      time: '09:10',
                    ),
                    tile(msg(5, author: sam, deleted: true, minutes: 12)),
                    tile(
                      msg(
                        6,
                        author: sam,
                        kind: MessageKind.system,
                        origin: MessageOrigin.system,
                        systemCode: SystemCode.threadReopened,
                        text: '',
                        minutes: 20,
                      ),
                      time: '09:20',
                    ),
                    pending(view('p1', 'On my way', OutboxStateLabel.sending)),
                    pending(view('p2', 'Still here', OutboxStateLabel.delayed)),
                    pending(
                      view(
                        'p3',
                        'A very long log',
                        OutboxStateLabel.failed,
                        code: 'payload_too_large',
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
          Padding(
            padding: const EdgeInsets.all(DsSpace.s12),
            child: DsComposer(
              targetName: 'Printer jam on floor 2',
              maxLines: 6,
              enterSends: true,
              onSend: (_) {},
              typingNames: const ['Sam'],
              controller: TextEditingController(text: 'Hello @r'),
              suggestions: [
                for (final p in suggestMentions(principals.values, 'r'))
                  DsSuggestion(
                    id: p.id.value,
                    command: '@${p.handle}',
                    description: p.displayName,
                  ),
              ],
            ),
          ),
        ],
      ),
    ),
  ),
);

/// The header without an operator and the composer states.
Widget states() => Builder(
  builder: (context) {
    final l = AppLocalizations.of(context);
    Widget composer({bool offline = false, bool readOnly = false}) => Padding(
      padding: const EdgeInsets.all(DsSpace.s12),
      child: DsComposer(
        targetName: 'ideas',
        maxLines: 6,
        enterSends: true,
        onSend: (_) {},
        offline: offline,
        readOnly: readOnly,
      ),
    );
    return ColoredBox(
      color: context.dsColors.surfaceCanvas,
      child: SizedBox.fromSize(
        size: const Size(640, 900),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            ThreadHeader(
              data: const ThreadHeaderData(
                title: 'Export to CSV',
                automaticTitle: true,
                operatorName: null,
                mainHistory: false,
                done: true,
                canEditTitle: true,
                canSetDone: true,
              ),
              onEditTitle: () {},
              onDone: () {},
              onMove: () {},
              onPin: () {},
              onColumn: () {},
            ),
            composer(offline: true),
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: DsSpace.s12),
              child: DsBanner(
                tone: DsTone.danger,
                text: l.composerTooLong(1000),
              ),
            ),
            composer(),
            composer(readOnly: true),
          ],
        ),
      ),
    );
  },
);

void main() {
  for (final theme in DsThemeMode.values) {
    goldenTest(
      'the thread history in ${theme.name}: grouped, collapsed, edited, deleted, '
      'pending, delayed, failed and system messages; header with operator',
      fileName: 'thread_history_${theme.name}',
      builder: () => dsGoldenMatrix(
        name: 'thread history',
        themes: [theme],
        densities: DsDensityMode.values,
        textScales: const [1.0, 2.0],
        locales: const [Locale('de'), Locale('en')],
        builder: history,
      ),
    );
    goldenTest(
      'the thread states in ${theme.name}: header without operator, composer '
      'offline, too long and read only',
      fileName: 'thread_states_${theme.name}',
      builder: () => dsGoldenMatrix(
        name: 'thread states',
        themes: [theme],
        densities: DsDensityMode.values,
        textScales: const [1.0, 2.0],
        locales: const [Locale('de'), Locale('en')],
        builder: states,
      ),
    );
  }
}
