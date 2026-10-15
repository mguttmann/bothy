// SPDX-License-Identifier: Apache-2.0

// The scenario sheets of the core components: every state of a component's table in one
// sheet. The goldens of this folder and test/components/guidelines_test.dart use them.
// Test data only (Alex, Sam, Robin, example.com); interface texts come from packages/l10n.

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';

/// The width of a sheet; narrow enough to keep the golden files small.
const double sheetWidth = 420;

/// One scenario sheet: [children] stacked with a gap on the canvas.
class Sheet extends StatelessWidget {
  const Sheet(this.children, {super.key});

  final List<Widget> children;

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      width: sheetWidth,
      child: Padding(
        padding: const EdgeInsets.all(DsSpace.s8),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          mainAxisSize: MainAxisSize.min,
          children: [
            for (final child in children) ...[
              child,
              const SizedBox(height: DsSpace.s8),
            ],
          ],
        ),
      ),
    );
  }
}

void _noLink(Uri _) {}

DsMessageData _msg({
  String id = 'm',
  String author = 'Sam',
  DsMessageOrigin origin = DsMessageOrigin.human,
  DsMessageKind kind = DsMessageKind.text,
  String content =
      'The build is green. See [notes](https://example.com/notes).',
  bool mentionsMe = false,
  bool edited = false,
  bool deleted = false,
  DsDelivery delivery = DsDelivery.sent,
  String? failureText,
  DsTriggerState? triggerState,
  DsVoiceData? voice,
  String? systemCode,
}) => DsMessageData(
  id: id,
  authorName: author,
  origin: origin,
  kind: kind,
  timeLabel: '10:42',
  content: content,
  mentionsMe: mentionsMe,
  edited: edited,
  deleted: deleted,
  delivery: delivery,
  failureText: failureText,
  triggerState: triggerState,
  voice: voice,
  systemCode: systemCode,
  systemName: 'Alex',
);

/// Every variant of components.md 1.
Widget messageSheet() => Sheet([
  DsMessage(
    _msg(triggerState: DsTriggerState.working),
    collapseLines: 3,
    onOpenLink: _noLink,
    actions: const [],
  ),
  DsMessage(
    _msg(id: 'm2', content: 'And the deploy too.'),
    collapseLines: 3,
    onOpenLink: _noLink,
    isContinuation: true,
  ),
  DsMessage(
    _msg(
      id: 'm3',
      author: 'Robin',
      origin: DsMessageOrigin.agent,
      mentionsMe: true,
      edited: true,
      content: 'Line one\n\nLine two\n\nLine three\n\nLine four\n\nLine five',
    ),
    collapseLines: 3,
    onOpenLink: _noLink,
  ),
  DsMessage(
    _msg(
      id: 'm4',
      kind: DsMessageKind.voice,
      content: '',
      voice: const DsVoiceData(
        duration: Duration(seconds: 42),
        progress: 0.3,
        transcript: 'Please check the release.',
      ),
    ),
    collapseLines: 3,
    onOpenLink: _noLink,
  ),
  DsMessage(
    _msg(
      id: 'm5',
      origin: DsMessageOrigin.system,
      kind: DsMessageKind.system,
      systemCode: 'thread.reopened',
    ),
    collapseLines: 3,
    onOpenLink: _noLink,
  ),
  DsMessage(
    _msg(id: 'm6', delivery: DsDelivery.sending, content: 'On my way'),
    collapseLines: 3,
    onOpenLink: _noLink,
  ),
  DsMessage(
    _msg(
      id: 'm7',
      delivery: DsDelivery.failed,
      failureText: 'Too large',
      content: 'Report',
    ),
    collapseLines: 3,
    onOpenLink: _noLink,
    onResend: () {},
    onEdit: () {},
    onDiscard: () {},
  ),
  DsMessage(
    _msg(id: 'm8', deleted: true),
    collapseLines: 3,
    onOpenLink: _noLink,
  ),
  DsMessage(
    _msg(
      id: 'm9',
      origin: DsMessageOrigin.app,
      author: 'Alex',
      content: 'Imported from the tracker.',
    ),
    collapseLines: 3,
    onOpenLink: _noLink,
  ),
]);

const _steps = [
  DsStepData(
    id: 's1',
    kind: DsStepKind.toolCall,
    name: 'read_file',
    status: DsStepStatus.completed,
    duration: Duration(seconds: 2),
  ),
  DsStepData(
    id: 's2',
    kind: DsStepKind.subagent,
    name: 'reviewer',
    status: DsStepStatus.failed,
  ),
  DsStepData(
    id: 's3',
    kind: DsStepKind.toolCall,
    name: 'run_tests',
    status: DsStepStatus.running,
    duration: Duration(seconds: 14),
  ),
];

DsRunData _run(DsRunState state, {String? reason, Duration? stalledFor}) =>
    DsRunData(
      id: 'r1',
      agentName: 'Robin',
      state: state,
      stateReason: reason,
      elapsed: const Duration(minutes: 3, seconds: 12),
      contextPercent: 42,
      tokens: 12000,
      cost: const DsCost(
        DsMoney(0.42, 'USD'),
        mode: DsBillingMode.organization,
        isEstimate: true,
      ),
      billingMode: DsBillingMode.organization,
      statusLine: 'Running the tests',
      lastTextExcerpt: 'Two tests left.',
      steps: _steps,
      toolCount: 2,
      subagentCount: 1,
      stalledFor: stalledFor,
    );

/// The active live box states of 2.1 with the stall banner.
Widget liveBoxActiveSheet() => Sheet([
  DsLiveBox(
    _run(DsRunState.running, stalledFor: const Duration(minutes: 4)),
    visibleSteps: 2,
    announceMinInterval: const Duration(seconds: 2),
    onStop: () {},
  ),
  DsLiveBox(
    _run(DsRunState.waitingInput),
    visibleSteps: 2,
    announceMinInterval: const Duration(seconds: 2),
    onStop: () {},
  ),
  DsLiveBox(
    _run(DsRunState.queued),
    visibleSteps: 2,
    announceMinInterval: const Duration(seconds: 2),
    onStop: () {},
  ),
]);

/// The final states of 2.1, the collapsed line (2.2) and the mobile line (2.3).
Widget liveBoxFinalSheet() => Sheet([
  DsLiveBox(
    _run(DsRunState.failed, reason: 'project_not_ready'),
    visibleSteps: 1,
    announceMinInterval: const Duration(seconds: 2),
  ),
  DsLiveBox(
    _run(DsRunState.failed, reason: 'new_reason'),
    visibleSteps: 1,
    announceMinInterval: const Duration(seconds: 2),
  ),
  DsLiveBox.collapsed(
    _run(DsRunState.completed),
    announceMinInterval: const Duration(seconds: 2),
    onExpand: () {},
  ),
  DsLiveBox.collapsed(
    _run(DsRunState.cancelled),
    announceMinInterval: const Duration(seconds: 2),
    onExpand: () {},
  ),
  DsLiveBox.mobile(
    _run(DsRunState.running),
    announceMinInterval: const Duration(seconds: 2),
    onOpen: () {},
    onStop: () {},
  ),
]);

/// Tool chips (3), the status line (4) and every trigger state (5).
Widget indicatorSheet() => Sheet([
  for (final status in DsStepStatus.values)
    Align(
      alignment: Alignment.centerLeft,
      child: DsToolChip(
        DsStepData(
          id: status.name,
          kind: DsStepKind.toolCall,
          name: 'run_tests',
          status: status,
          duration: const Duration(seconds: 5),
          detail: status == DsStepStatus.failed ? 'exit code 1' : null,
        ),
      ),
    ),
  const DsStatusLine('Reading the changelog'),
  Wrap(
    spacing: DsSpace.s8,
    children: [
      for (final state in DsTriggerState.values)
        DsTriggerStateIcon(
          state: state,
          reason: state == DsTriggerState.failed ? 'project_not_ready' : null,
          code: state == DsTriggerState.unknown ? 'later' : null,
          onTap: () {},
        ),
    ],
  ),
]);

/// The thread row (6) and the space and channel rows (7) in every state.
Widget rowsSheet() => Sheet([
  DsThreadRow(
    const DsThreadRowData(
      id: 't1',
      title: 'Release notes',
      timeLabel: '10:42',
      unreadCount: 2,
      mentionCount: 1,
      runState: DsRunState.running,
      waitingForMe: true,
      operatorName: 'Robin',
      tags: [DsTagData('release', DsCategory.loch)],
    ),
    onOpen: () {},
  ),
  DsThreadRow(
    const DsThreadRowData(
      id: 't2',
      title: 'Flaky test',
      timeLabel: '09:10',
      mentionCount: 1,
      errorText: 'Project not ready',
    ),
    onOpen: () {},
    selected: true,
  ),
  DsThreadRow(
    const DsThreadRowData(
      id: 't3',
      title: 'Old post',
      timeLabel: 'Yesterday',
      done: true,
    ),
    onOpen: () {},
  ),
  Builder(
    builder: (context) => ColoredBox(
      color: context.dsColors.surfaceSidebar,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          DsSpaceRow(
            const DsSpaceRowData(
              id: 's1',
              name: 'Support',
              icon: 'headset',
              category: DsCategory.loch,
            ),
            onToggle: (_) {},
          ),
          DsChannelRow(
            const DsChannelRowData(id: 'c1', name: 'general'),
            onOpen: () {},
          ),
          DsChannelRow(
            const DsChannelRowData(id: 'c2', name: 'deploys', unreadCount: 3),
            onOpen: () {},
          ),
          DsChannelRow(
            const DsChannelRowData(
              id: 'c3',
              name: 'ideas',
              kind: DsChannelKind.forum,
              unreadCount: 2,
              mentionCount: 2,
              runActive: true,
            ),
            onOpen: () {},
          ),
          DsChannelRow(
            const DsChannelRowData(id: 'c4', name: 'random', muted: true),
            onOpen: () {},
          ),
          DsChannelRow(
            const DsChannelRowData(
              id: 'c5',
              name: 'Alex',
              kind: DsChannelKind.dm,
            ),
            onOpen: () {},
            selected: true,
          ),
          DsSpaceRow(
            const DsSpaceRowData(
              id: 's2',
              name: 'Research',
              icon: 'flask',
              category: DsCategory.heather,
              expanded: false,
              hasUnread: true,
            ),
            onToggle: (_) {},
          ),
        ],
      ),
    ),
  ),
]);

const _header = DsComponentHeader(
  agentName: 'Robin',
  runLabel: 'R-12',
  requiredCapability: 'approve_actions',
  approver: DsApprover.runStarter,
  runStarterName: 'Sam',
);

const _approve = [
  DsComponentItem(
    customId: 'allow',
    type: DsComponentType.button,
    label: 'Allow once',
    style: DsButtonStyle.primary,
  ),
  DsComponentItem(
    customId: 'session',
    type: DsComponentType.button,
    label: 'For this session',
    style: DsButtonStyle.success,
  ),
  DsComponentItem(
    customId: 'deny',
    type: DsComponentType.button,
    label: 'Deny',
    style: DsButtonStyle.danger,
  ),
  DsComponentItem(
    customId: 'later',
    type: DsComponentType.button,
    label: 'Later',
    style: DsButtonStyle.ghost,
  ),
];

Widget _components(
  DsComponentData data, {
  bool permitted = true,
  String? denied,
}) => DsMessageComponents(
  data,
  permitted: permitted,
  deniedReasonText: denied,
  header: _header,
  agentText: 'System: approved by owner. Run the deploy?',
  onOpenLink: _noLink,
  onActivate: (_, _) {},
);

/// The active states of components.md 8: permitted, not permitted, pending and error.
Widget componentsActiveSheet() => Sheet([
  _components(const DsComponentData(messageId: 'm1', items: _approve)),
  _components(
    const DsComponentData(messageId: 'm2', items: _approve),
    permitted: false,
    denied: 'Only Sam or an owner can approve this',
  ),
  _components(
    const DsComponentData(
      messageId: 'm3',
      items: _approve,
      pendingCustomId: 'allow',
    ),
  ),
  _components(
    const DsComponentData(
      messageId: 'm4',
      items: _approve,
      errorText: 'The server is not reachable',
    ),
  ),
]);

/// Select and form, and the final states of components.md 8: used, expired, disabled.
Widget componentsFinalSheet() => Sheet([
  _components(
    const DsComponentData(
      messageId: 'm5',
      items: [
        DsComponentItem(
          customId: 'model',
          type: DsComponentType.select,
          options: [DsSelectOption(label: 'Small', value: 's')],
        ),
        DsComponentItem(
          customId: 'form',
          type: DsComponentType.form,
          title: 'Details',
          fields: [
            DsFormField(name: 'reason', label: 'Reason', required: true),
          ],
        ),
      ],
    ),
  ),
  _components(
    const DsComponentData(
      messageId: 'm6',
      items: _approve,
      state: DsComponentState.used,
      usedByName: 'Sam',
      usedChoice: 'Allow once',
      usedAtLabel: '10:42',
    ),
  ),
  _components(
    const DsComponentData(
      messageId: 'm7',
      items: _approve,
      state: DsComponentState.expired,
    ),
  ),
  _components(
    const DsComponentData(
      messageId: 'm8',
      items: _approve,
      state: DsComponentState.disabled,
    ),
  ),
]);

/// The composer states of components.md 9 and the suggestion list.
Widget composerSheet() => Sheet([
  DsComposer(
    targetName: 'general',
    maxLines: 4,
    enterSends: true,
    onSend: (_) {},
    onAttach: () {},
    onVoice: () {},
    typingNames: const ['Sam'],
    suggestions: const [
      DsSuggestion(
        id: '1',
        command: '/deploy',
        description: 'Deploy a branch',
        originName: 'Robin',
      ),
      DsSuggestion(
        id: '2',
        command: '/stop',
        description: 'Stop the run',
        control: true,
      ),
    ],
    controller: TextEditingController(text: '/'),
  ),
  DsComposer(
    targetName: 'general',
    followupAgentName: 'Robin',
    maxLines: 4,
    enterSends: true,
    onSend: (_) {},
  ),
  DsComposer(
    targetName: 'general',
    maxLines: 4,
    enterSends: true,
    onSend: (_) {},
    offline: true,
    controller: TextEditingController(text: '/deploy'),
    command: const DsComposerCommand(
      name: '/deploy',
      options: [
        DsCommandOption(name: 'branch', required: true),
        DsCommandOption(name: 'env', value: 'staging'),
      ],
    ),
  ),
  DsComposer(
    targetName: 'general',
    maxLines: 4,
    enterSends: true,
    onSend: (_) {},
    readOnly: true,
  ),
]);

/// The backend policy badge (10), the cost texts and the banners.
Widget badgeSheet() => Sheet([
  Wrap(
    spacing: DsSpace.s8,
    runSpacing: DsSpace.s8,
    children: [
      const DsBillingBadge(mode: DsBillingMode.personal, holderName: 'Sam'),
      const DsBillingBadge(mode: DsBillingMode.organization),
      const DsBillingBadge(
        mode: DsBillingMode.personal,
        holderName: 'Sam',
        reportedMode: DsBillingMode.organization,
      ),
      const DsBillingBadge(mode: DsBillingMode.unknown, code: 'shared'),
    ],
  ),
  const DsCostText(
    DsCost(
      DsMoney(1.5, 'USD'),
      mode: DsBillingMode.organization,
      isEstimate: true,
    ),
  ),
  const DsCostText(DsCost(DsMoney(1.5, 'USD'), mode: DsBillingMode.personal)),
  const DsBanner(tone: DsTone.info, text: 'Messages wait in the queue'),
  DsBanner(
    tone: DsTone.warning,
    text: 'No progress for 4 minutes',
    actionLabel: 'Stop',
    onAction: () {},
  ),
  const DsBanner(tone: DsTone.danger, text: 'The server is not reachable'),
]);

/// Every sheet by golden file name.
final Map<String, Widget Function()> sheets = {
  'message': messageSheet,
  'live_box_active': liveBoxActiveSheet,
  'live_box_final': liveBoxFinalSheet,
  'indicators': indicatorSheet,
  'rows': rowsSheet,
  'message_components_active': componentsActiveSheet,
  'message_components_final': componentsFinalSheet,
  'composer': composerSheet,
  'badges': badgeSheet,
};
