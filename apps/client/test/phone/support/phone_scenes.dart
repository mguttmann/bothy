// SPDX-License-Identifier: Apache-2.0

// The phone screens M1 to M4 and M7 of 01 5.3 as scenes for the goldens, the text scaling
// test and the tap target test (brief M5-02 Tests): the views of M4-19 to M4-21 in the phone's
// frame (back button, banner, tab bar), with the data of the desktop goldens. White label:
// Alex, Sam, Robin and `example.com`.

import 'dart:convert';

import 'package:client/agents/components_mapping.dart';
import 'package:client/agents/run_mapping.dart';
import 'package:client/forum/forum_model.dart';
import 'package:client/forum/post_list_column.dart';
import 'package:client/phone/phone_navigation.dart';
import 'package:client/phone/phone_shell.dart';
import 'package:client/sidebar/sidebar_column.dart';
import 'package:client/sidebar/sidebar_model.dart';
import 'package:client/sidebar/sidebar_order.dart';
import 'package:client/thread/message_menu.dart';
import 'package:client/thread/message_tile.dart';
import 'package:client/thread/outbox_rows.dart';
import 'package:client/thread/providers.dart';
import 'package:client/thread/thread_header.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

import '../../support/agent_fakes.dart';
import '../../support/card_host.dart' show cardId, cardMessage;
import '../../support/structure.dart';
import '../../support/thread_fixtures.dart';
import 'phone_app.dart' show phoneSize;

const Duration _interval = Duration(seconds: 10);

final Map<PrincipalId, Principal> _principals = {
  for (final p in people) p.id: p,
};

const ThreadPermissions _writer = ThreadPermissions(
  canWrite: true,
  canManage: false,
  readOnly: false,
);

/// The phone frame: the banner, the back button to [back], the [body] and the tab bar on a
/// tab's first page ([tab]).
Widget phoneFrame({
  required Widget body,
  Widget? banner,
  String? back,
  PhoneTab? tab,
  int running = 0,
}) => Builder(
  builder: (context) => SizedBox.fromSize(
    size: phoneSize,
    child: Scaffold(
      backgroundColor: context.dsColors.surfaceCanvas,
      body: SafeArea(
        bottom: tab == null,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            ?banner,
            if (back != null) PhoneBackButton(label: back, onBack: () {}),
            Expanded(child: body),
          ],
        ),
      ),
      bottomNavigationBar: tab == null
          ? null
          : PhoneTabBarView(active: tab, running: running, onSelect: (_) {}),
    ),
  ),
);

SidebarChannel _entry(
  Channel c, {
  int unread = 0,
  int mentions = 0,
  bool run = false,
}) => SidebarChannel(
  channel: c,
  counts: ThreadCounts(unread: unread, mentions: mentions),
  runActive: run,
  muted: false,
);

/// M1 Spaces.
Widget spacesScene() => phoneFrame(
  tab: PhoneTab.spaces,
  running: 2,
  body: Builder(
    builder: (context) => ColoredBox(
      color: context.dsColors.surfaceSidebar,
      child: SidebarView(
        productName: 'Team Chat',
        nodes: [
          SidebarSpaceNode(
            entry: SidebarSpace(space: space(1, name: 'Support')),
            channels: [
              _entry(channel(1, name: 'general')),
              _entry(
                channel(2, name: 'requests', kind: ChannelKind.forum),
                unread: 3,
                run: true,
              ),
              _entry(channel(3, name: 'alerts')),
            ],
          ),
          SidebarSpaceNode(
            entry: SidebarSpace(space: space(2, name: 'Engineering')),
            channels: [
              _entry(
                channel(4, spaceN: 2, name: 'server', kind: ChannelKind.forum),
                mentions: 1,
                unread: 1,
              ),
            ],
          ),
        ],
        runningCount: 2,
        onRunningNow: () {},
        onJumpTo: () {},
        onToggle: (_, _) {},
        onMove: (_, _) {},
        onOpenChannel: (_) {},
        onSettings: () {},
        onLogout: () {},
        showFooter: false,
      ),
    ),
  ),
);

/// M2 Post list.
Widget postListScene() {
  const day = 24 * 60;
  final forum = channel(2, name: 'requests', kind: ChannelKind.forum);
  final model = buildPostList(
    threads: [
      thread(
        1,
        channelN: 2,
        title: 'Gateway: resume after network change',
        activeMinutes: -day,
        operator: robin,
      ),
      thread(
        2,
        channelN: 2,
        title: 'Check migration 0012',
        activeMinutes: -2 * day,
      ),
      thread(
        3,
        channelN: 2,
        title: 'Review: grant wizard',
        activeMinutes: -3 * day,
      ),
      thread(
        4,
        channelN: 2,
        title: 'Costs per space',
        activeMinutes: -4 * day,
        state: ThreadState.closed,
      ),
    ],
    channel: forum,
    filter: const PostListFilter(),
    counts: {threadId(3): const ThreadCounts(unread: 1, mentions: 1)},
    runs: [
      run(1, RunState.running, threadN: 1, channelN: 2),
      run(2, RunState.waitingInput, threadN: 2, channelN: 2),
    ],
    me: alex,
  );
  return phoneFrame(
    back: 'Spaces',
    body: PostListView(
      forum: forum,
      filter: const PostListFilter(),
      model: model,
      tags: const [],
      names: {robin: 'Robin'},
      now: t0,
      doneCollapsed: true,
      morePages: false,
      onNewPost: () {},
      onView: (_) {},
      onTags: (_) {},
      onSort: (_) {},
      onToggleDone: (_) {},
      onOpenPost: (_) {},
    ),
  );
}

Widget _header({bool done = false}) => ThreadHeader(
  compact: true,
  data: ThreadHeaderData(
    title: 'Gateway: resume after network change',
    automaticTitle: false,
    operatorName: 'Robin',
    mainHistory: false,
    done: done,
    canEditTitle: true,
    canSetDone: true,
  ),
  onEditTitle: () {},
  onDone: () {},
  onMove: () {},
  onPin: () {},
  onColumn: () {},
);

Widget _tile(Message m, {String time = '10:22'}) => MessageTile(
  data: messageData(m, principals: _principals, me: alex, timeLabel: time),
  continuation: false,
  collapseLines: 6,
  actions: messageActions(
    m,
    me: alex,
    permissions: _writer,
    mainHistory: false,
  ),
  onAction: (_) {},
  onOpenLink: (_) {},
);

Widget _composer({bool offline = false}) => Padding(
  padding: const EdgeInsets.all(DsSpace.s8),
  child: DsComposer(
    targetName: 'Robin',
    maxLines: 6,
    enterSends: false,
    offline: offline,
    onSend: (_) {},
  ),
);

/// The approval card of M3: buttons stacked at full width.
Widget _approval() {
  final message = cardMessage('go test wants to use the network.');
  final components = [
    button(1, cardId, customId: 'once', label: 'Allow once'),
    button(
      2,
      cardId,
      customId: 'session',
      label: 'For this session',
      style: 'secondary',
    ),
    button(3, cardId, customId: 'deny', label: 'Deny', style: 'danger'),
  ];
  return DsMessageComponents(
    componentData(
      message,
      components,
      principals: _principals,
      now: t0,
      timeLabel: (_) => '10:16',
    ),
    key: const ValueKey('scene.approval'),
    permitted: true,
    deniedReasonText: null,
    header: componentHeader(
      message,
      components,
      principals: _principals,
      runLabel: '10:12',
    ),
    agentText: message.contentMd,
    onOpenLink: (_) {},
    onActivate: (_, _) {},
    stacked: true,
    sheets: true,
  );
}

DsRunData _runOf(Map<String, Object?> doc) => runFromDocument(
  RunDocument.fromJson(doc),
  agentName: 'Robin',
  now: t0.add(const Duration(minutes: 4, seconds: 12)),
);

/// The history of a thread under the compact header, with [top] pinned over it (the live box)
/// and the input at the bottom.
Widget _thread(
  List<Widget> history, {
  Widget? top,
  Widget? banner,
  bool offline = false,
}) => phoneFrame(
  banner: banner,
  back: 'requests',
  body: Column(
    crossAxisAlignment: CrossAxisAlignment.stretch,
    children: [
      _header(),
      if (top != null)
        Padding(padding: const EdgeInsets.all(DsSpace.s8), child: top),
      Expanded(
        child: ListView(
          padding: const EdgeInsets.symmetric(horizontal: DsSpace.s8),
          children: history,
        ),
      ),
      _composer(offline: offline),
    ],
  ),
);

/// M3 with a running run: the compact live box with Stop at the top right, the approval.
Widget runningScene() => _thread(
  top: DsLiveBox.mobile(
    _runOf(
      runDoc(1, statusLine: 'go test ./internal/sync/...', contextPct: 62),
    ),
    key: const ValueKey('scene.livebox'),
    announceMinInterval: _interval,
    onOpen: () {},
    onStop: () {},
  ),
  [
    _approval(),
    _tile(msg(4, author: alex, text: 'Please include expired threads.')),
  ],
);

/// M3 with a run waiting for an answer.
Widget waitingScene() => _thread(
  top: DsLiveBox.mobile(
    _runOf(
      runDoc(
        1,
        state: 'waiting_input',
        statusLine: 'Which database should the test use?',
      ),
    ),
    key: const ValueKey('scene.livebox'),
    announceMinInterval: _interval,
    onOpen: () {},
    onStop: () {},
  ),
  [_tile(msg(4, author: alex, text: 'Please include expired threads.'))],
);

/// M3 after the run: the collapsed line in the history.
Widget collapsedScene() => _thread([
  _tile(msg(4, author: alex, text: 'Please include expired threads.')),
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
    key: const ValueKey('scene.livebox'),
    announceMinInterval: _interval,
    onExpand: () {},
  ),
]);

/// M4 Large components: buttons at full width, a select that opens the system picker and a
/// form that opens as a sheet.
Widget componentsScene() => _thread([
  _approval(),
  DsMessageComponents(
    const DsComponentData(
      messageId: 'm2',
      items: [
        DsComponentItem(
          customId: 'model',
          type: DsComponentType.select,
          options: [
            DsSelectOption(label: 'Small', value: 's'),
            DsSelectOption(label: 'Large', value: 'l'),
          ],
        ),
        DsComponentItem(
          customId: 'details',
          type: DsComponentType.form,
          title: 'Release details',
          fields: [DsFormField(name: 'notes', label: 'Notes', required: true)],
        ),
      ],
    ),
    key: const ValueKey('scene.components'),
    permitted: true,
    deniedReasonText: null,
    header: const DsComponentHeader(agentName: 'Robin', runLabel: '10:12'),
    agentText: 'Which model, and what goes into the notes?',
    onOpenLink: (_) {},
    onActivate: (_, _) {},
    stacked: true,
    sheets: true,
  ),
]);

OutboxEntryView _waiting(
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

Widget _pending(OutboxEntryView v) => PendingRow(
  view: v,
  authorName: 'Alex',
  timeLabel: '10:24',
  collapseLines: 6,
  onResend: () {},
  onEdit: () {},
  onDiscard: () {},
  onOpenLink: (_) {},
);

/// M7 Offline: the banner, a message waiting with "sending" and a failed one with Retry,
/// Edit and Discard.
Widget offlineScene() => _thread(
  banner: const PhoneBanner(phase: ConnectionPhase.offline, waiting: 2),
  offline: true,
  [
    _tile(msg(4, author: alex, text: 'Please include expired threads.')),
    _pending(
      _waiting('p1', 'And summarize afterwards.', OutboxStateLabel.sending),
    ),
    _pending(
      _waiting(
        'p2',
        'The full log of the night.',
        OutboxStateLabel.failed,
        code: 'payload_too_large',
      ),
    ),
  ],
);

/// Every phone scene by name.
final Map<String, Widget Function()> phoneScenes = {
  'm1_spaces': spacesScene,
  'm2_post_list': postListScene,
  'm3_running': runningScene,
  'm3_waiting': waitingScene,
  'm3_collapsed': collapsedScene,
  'm4_components': componentsScene,
  'm7_offline': offlineScene,
};
