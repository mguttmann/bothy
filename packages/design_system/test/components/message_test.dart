// SPDX-License-Identifier: Apache-2.0

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

const _text = DsMessageData(
  id: 'm1',
  authorName: 'Alex',
  origin: DsMessageOrigin.human,
  kind: DsMessageKind.text,
  timeLabel: '10:42',
  content: 'The build is green.',
);

DsMessageData _with({
  String authorName = 'Alex',
  DsMessageOrigin origin = DsMessageOrigin.human,
  DsMessageKind kind = DsMessageKind.text,
  String content = 'The build is green.',
  bool mentionsMe = false,
  bool edited = false,
  bool deleted = false,
  DsDelivery delivery = DsDelivery.sent,
  String? failureText,
  DsTriggerState? triggerState,
  DsVoiceData? voice,
  String? systemCode,
  String? systemName,
  List<String> attachments = const [],
  List<DsReaction> reactions = const [],
  String? replyToName,
}) => DsMessageData(
  id: 'm1',
  authorName: authorName,
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
  systemName: systemName,
  attachmentNames: attachments,
  reactions: reactions,
  replyToName: replyToName,
  replyExcerpt: replyToName == null ? null : 'Earlier text',
);

Widget _message(
  DsMessageData data, {
  bool isContinuation = false,
  int collapseLines = 24,
  List<DsMessageAction> actions = const [],
  Widget? liveBox,
  Widget? components,
  VoidCallback? onResend,
  VoidCallback? onEdit,
  VoidCallback? onDiscard,
}) => DsMessage(
  data,
  collapseLines: collapseLines,
  isContinuation: isContinuation,
  onOpenLink: (_) {},
  actions: actions,
  liveBox: liveBox,
  components: components,
  onResend: onResend,
  onEdit: onEdit,
  onDiscard: onDiscard,
);

void main() {
  testWidgets('text message: avatar, header, content; label order', (
    tester,
  ) async {
    final l = await en();
    await pumpDs(tester, _message(_text));
    expect(find.byType(DsAvatar), findsOneWidget);
    expect(find.text('10:42'), findsOneWidget);
    expect(find.text('The build is green.'), findsOneWidget);
    expect(
      find.bySemanticsLabel(
        RegExp(
          '^${RegExp.escape(l.messageSemanticsLabel('Alex', '10:42', 'The build is green.'))}',
        ),
      ),
      findsOneWidget,
    );
  });

  testWidgets('a mention starts the label with Mentions you and tints the row', (
    tester,
  ) async {
    final l = await en();
    await pumpDs(tester, _message(_with(mentionsMe: true)));
    expect(
      find.bySemanticsLabel(
        RegExp(
          '^${RegExp.escape(l.messageMentionsYou)}, ${RegExp.escape(l.messageSemanticsLabel('Alex', '10:42', 'The build is green.'))}',
        ),
      ),
      findsOneWidget,
    );
    final state = tester.state<DsMessageState>(find.byType(DsMessage));
    expect(state.background, DsColors.light.attentionSubtle);
  });

  testWidgets('a continuation message has no avatar and no header', (
    tester,
  ) async {
    await pumpDs(tester, _message(_text, isContinuation: true));
    expect(find.byType(DsAvatar), findsNothing);
    expect(find.text('Alex'), findsNothing);
    expect(find.text('The build is green.'), findsOneWidget);
  });

  testWidgets('origin badges for agent, app, import and system', (
    tester,
  ) async {
    final l = await en();
    final badges = {
      DsMessageOrigin.agent: l.messageOriginAgent,
      DsMessageOrigin.app: l.messageOriginApp,
      DsMessageOrigin.import: l.messageOriginImport,
      DsMessageOrigin.system: l.messageOriginSystem,
    };
    for (final MapEntry(key: origin, value: badge) in badges.entries) {
      await pumpDs(tester, _message(_with(origin: origin)));
      expect(find.text(badge), findsOneWidget, reason: origin.name);
    }
    await pumpDs(tester, _message(_with()));
    for (final badge in badges.values) {
      expect(find.text(badge), findsNothing);
    }
  });

  testWidgets('foreign names are isolated in the text, not in the label', (
    tester,
  ) async {
    final l = await en();
    const evil = 'Sam\u202Egnp.exe';
    await pumpDs(tester, _message(_with(authorName: evil)));
    expect(find.text('\u2068$evil\u2069'), findsOneWidget);
    expect(
      find.bySemanticsLabel(
        RegExp(
          '^${RegExp.escape(l.messageSemanticsLabel(evil, '10:42', 'The build is green.'))}',
        ),
      ),
      findsOneWidget,
    );
    final semantics = tester.getSemantics(find.byType(DsMessage)).label;
    expect(semantics, isNot(contains('\u2068')));
  });

  testWidgets(
    'long content collapses after the parameter lines with Show more',
    (tester) async {
      final l = await en();
      final long = List.generate(30, (i) => 'line $i').join('\n\n');
      await pumpDs(tester, _message(_with(content: long), collapseLines: 5));
      await tester.pump();
      expect(find.text(l.messageShowMore), findsOneWidget);
      final collapsed = tester.getSize(find.byType(DsCollapsible)).height;
      await tester.tap(find.text(l.messageShowMore));
      await tester.pump();
      expect(find.text(l.messageShowMore), findsNothing);
      expect(
        tester.getSize(find.byType(DsMarkdown)).height,
        greaterThan(collapsed),
      );

      await pumpDs(tester, _message(_with(content: 'short'), collapseLines: 5));
      await tester.pump();
      expect(find.text(l.messageShowMore), findsNothing);
    },
  );

  testWidgets('the hover bar is reachable by Tab from the focused message', (
    tester,
  ) async {
    final l = await en();
    final used = <String>[];
    await pumpDs(
      tester,
      _message(
        _text,
        actions: [
          DsMessageAction(
            glyph: DsIcons.actionReply,
            label: l.messageActionReply,
            onPressed: () => used.add('reply'),
          ),
          DsMessageAction(
            glyph: DsIcons.actionReact,
            label: l.messageActionReact,
            onPressed: () => used.add('react'),
          ),
        ],
      ),
    );
    expect(find.bySemanticsLabel(l.messageActionReply), findsNothing);
    await tester.sendKeyEvent(LogicalKeyboardKey.tab);
    await tester.pump();
    final state = tester.state<DsMessageState>(find.byType(DsMessage));
    expect(state.focusedMessage, isTrue);
    expect(find.bySemanticsLabel(l.messageActionReply), findsOneWidget);
    await tester.sendKeyEvent(LogicalKeyboardKey.tab);
    await tester.pump();
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await tester.pump();
    expect(used, ['reply']);
  });

  testWidgets('edited, deleted, sending and failed', (tester) async {
    final l = await en();
    await pumpDs(tester, _message(_with(edited: true)));
    expect(allText(tester), contains(l.messageEdited));

    await pumpDs(
      tester,
      _message(
        _with(deleted: true),
        actions: [
          DsMessageAction(
            glyph: DsIcons.actionReply,
            label: 'r',
            onPressed: () {},
          ),
        ],
      ),
    );
    expect(find.text(l.messageDeleted), findsOneWidget);
    expect(find.text('The build is green.'), findsNothing);
    await tester.sendKeyEvent(LogicalKeyboardKey.tab);
    await tester.pump();
    expect(find.bySemanticsLabel('r'), findsNothing);

    await pumpDs(tester, _message(_with(delivery: DsDelivery.sending)));
    expect(find.text(l.messageSending), findsOneWidget);
    expect(
      tester.widgetList<DsIcon>(find.byType(DsIcon)).map((i) => i.glyph),
      contains(DsIcons.triggerQueued),
    );

    final calls = <String>[];
    await pumpDs(
      tester,
      _message(
        _with(
          delivery: DsDelivery.failed,
          failureText: 'The content is too large',
        ),
        onResend: () => calls.add('resend'),
        onEdit: () => calls.add('edit'),
        onDiscard: () => calls.add('discard'),
      ),
    );
    expect(find.text('The content is too large'), findsOneWidget);
    await tester.tap(find.text(l.messageResend));
    await tester.tap(find.text(l.messageEdit));
    await tester.tap(find.text(l.messageDiscard));
    expect(calls, ['resend', 'edit', 'discard']);
  });

  testWidgets('system, voice, run anchor, card, reply, attachments, reactions', (
    tester,
  ) async {
    final l = await en();
    await pumpDs(
      tester,
      _message(
        _with(
          origin: DsMessageOrigin.system,
          kind: DsMessageKind.system,
          content: '',
          systemCode: 'thread.reopened',
          systemName: 'Sam',
        ),
      ),
    );
    expect(
      find.text(l.messageSystemThreadReopened('\u2068Sam\u2069')),
      findsOneWidget,
    );
    expect(find.byType(DsAvatar), findsNothing);

    await pumpDs(
      tester,
      _message(
        _with(
          origin: DsMessageOrigin.system,
          kind: DsMessageKind.system,
          content: '',
          systemCode: 'future.code',
        ),
      ),
    );
    expect(find.text(l.messageSystemUnknown('future.code')), findsOneWidget);

    var played = 0;
    await pumpDs(
      tester,
      DsMessage(
        _with(
          kind: DsMessageKind.voice,
          content: '',
          voice: const DsVoiceData(
            duration: Duration(seconds: 42),
            progress: 0.5,
            transcript: 'Spoken words',
          ),
        ),
        collapseLines: 24,
        onOpenLink: (_) {},
        onPlayVoice: () => played++,
      ),
    );
    expect(find.text('Spoken words'), findsOneWidget);
    expect(find.byType(LinearProgressIndicator), findsOneWidget);
    await tester.tap(find.bySemanticsLabel(l.messageVoicePlay));
    expect(played, 1);

    await pumpDs(
      tester,
      _message(
        _with(
          kind: DsMessageKind.run,
          origin: DsMessageOrigin.agent,
          content: '',
        ),
        liveBox: const Text('live box here'),
      ),
    );
    expect(find.text('live box here'), findsOneWidget);

    await pumpDs(
      tester,
      _message(
        _with(kind: DsMessageKind.card, origin: DsMessageOrigin.agent),
        components: const Text('components here'),
      ),
    );
    expect(find.text('components here'), findsOneWidget);
    // The card body renders the agent text below its header (SEC-OUT-4), so the
    // message does not draw the content a second time above the header.
    expect(find.text('The build is green.'), findsNothing);

    await pumpDs(
      tester,
      _message(
        _with(
          replyToName: 'Sam',
          attachments: ['report.pdf'],
          reactions: [const DsReaction('+1', 2, mine: true)],
        ),
      ),
    );
    expect(find.bySemanticsLabel(l.messageReplyTo('Sam')), findsOneWidget);
    expect(
      find.bySemanticsLabel(l.messageAttachment('report.pdf')),
      findsOneWidget,
    );
    expect(find.bySemanticsLabel(l.messageReaction(2, '+1')), findsOneWidget);
  });

  testWidgets('a trigger state shows after the time stamp', (tester) async {
    await pumpDs(tester, _message(_with(triggerState: DsTriggerState.working)));
    expect(find.byType(DsTriggerStateIcon), findsOneWidget);
  });
}
