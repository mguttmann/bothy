// SPDX-License-Identifier: Apache-2.0

// The Flutter target sheet of Android (brief M5-06 Design notes 1 and 6; 01 5.3 M6, 5.4):
// header Cancel, "Share to", Send; the preview of the first item; the comment; the recent
// targets newest first and the search; the note on write permission; Send stays disabled until
// a target is chosen, a new post has a title and the files fit `file_max_upload_bytes` and
// `message_max_attachments`, with the reason shown.

import 'package:client/share/share_sheet.dart';
import 'package:client/share/share_target.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/widgets.dart';
import 'share_samples.dart';

const InstanceLimits _limits = InstanceLimits(
  messageMaxContentBytes: 65536,
  messageMaxAttachments: 2,
  fileMaxUploadBytes: 2000000,
  threadTitleMaxChars: 200,
  voiceMaxDurationMs: 600000,
);

void main() {
  late List<ShareChoice> sent;
  late int cancelled;
  late List<String> queries;

  ShareSheetController controller({
    List<ShareFile>? files,
    String text = '',
    InstanceLimits? limits = _limits,
  }) => ShareSheetController(
    files: files ?? [sharedFile('screenshot.png', 1258291)],
    text: text,
    limits: limits,
    recent: () async => sheetTargets,
    search: (q) async {
      queries.add(q);
      return q == 'gen' ? [sheetTargets.last] : const [];
    },
  );

  Future<ShareSheetController> pump(
    WidgetTester tester, [
    ShareSheetController? c,
  ]) async {
    final sheet = c ?? controller();
    await tester.pumpWidget(
      hostScreen(
        ShareSheet(
          controller: sheet,
          onCancel: () => cancelled++,
          onSend: (choice) async => sent.add(choice),
        ),
      ),
    );
    await tester.pumpAndSettle();
    return sheet;
  }

  DsButton send(WidgetTester tester) =>
      tester.widget<DsButton>(find.byKey(const ValueKey('share.send')));

  setUp(() {
    sent = [];
    cancelled = 0;
    queries = [];
  });

  testWidgets('header, preview, comment, recent targets and the note', (
    tester,
  ) async {
    await pump(
      tester,
      controller(
        files: [
          sharedFile('screenshot.png', 1258291),
          sharedFile('second.png', 10),
        ],
        text: 'https://example.com/issue',
      ),
    );
    expect(find.text('Share to'), findsOneWidget);
    expect(find.byKey(const ValueKey('share.cancel')), findsOneWidget);
    expect(find.text(dsIsolate('screenshot.png')), findsOneWidget);
    expect(find.text('1.2 MB'), findsOneWidget);
    expect(find.text('and 1 more file'), findsOneWidget);
    expect(
      tester
          .widget<TextField>(find.byKey(const ValueKey('share.comment')))
          .controller!
          .text,
      'https://example.com/issue',
    );
    expect(find.text(dsIsolate('server > Gateway: resume')), findsOneWidget);
    expect(find.text('${dsIsolate('requests')} (new post)'), findsOneWidget);
    expect(find.text(dsIsolate('general')), findsOneWidget);
    for (var i = 0; i < 3; i++) {
      expect(find.byKey(ValueKey('share.target.$i')), findsOneWidget);
    }
    expect(find.text('Only targets with write permission.'), findsOneWidget);
    expect(send(tester).onPressed, isNull, reason: 'no target chosen');
  });

  testWidgets(
    'a chosen target enables Send; Send hands over target and comment',
    (tester) async {
      await pump(tester);
      await tester.tap(find.byKey(const ValueKey('share.target.0')));
      await tester.pumpAndSettle();
      expect(send(tester).onPressed, isNotNull);
      await tester.enterText(
        find.byKey(const ValueKey('share.comment')),
        'See error above',
      );
      await tester.tap(find.byKey(const ValueKey('share.send')));
      await tester.pumpAndSettle();
      expect(sent.single.target, sheetTargets.first);
      expect(sent.single.comment, 'See error above');
      expect(sent.single.title, isNull);
    },
  );

  testWidgets('a new post needs its title', (tester) async {
    await pump(tester);
    await tester.tap(find.byKey(const ValueKey('share.target.1')));
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('share.title')), findsOneWidget);
    expect(send(tester).onPressed, isNull);
    await tester.enterText(
      find.byKey(const ValueKey('share.title')),
      'Printer error',
    );
    await tester.pump();
    await tester.tap(find.byKey(const ValueKey('share.send')));
    await tester.pumpAndSettle();
    expect(sent.single.target.kind, ShareTargetKind.newPost);
    expect(sent.single.title, 'Printer error');
  });

  testWidgets('a file over file_max_upload_bytes keeps Send disabled with the '
      'reason', (tester) async {
    await pump(tester, controller(files: [sharedFile('video.mov', 3000000)]));
    await tester.tap(find.byKey(const ValueKey('share.target.0')));
    await tester.pumpAndSettle();
    expect(
      find.text('${dsIsolate('video.mov')} is larger than the limit of 1.9 MB'),
      findsOneWidget,
    );
    expect(send(tester).onPressed, isNull);
  });

  testWidgets('more files than message_max_attachments keep Send disabled with '
      'the reason', (tester) async {
    await pump(
      tester,
      controller(
        files: [
          sharedFile('a.png', 1),
          sharedFile('b.png', 1),
          sharedFile('c.png', 1),
        ],
      ),
    );
    await tester.tap(find.byKey(const ValueKey('share.target.0')));
    await tester.pumpAndSettle();
    expect(find.text('At most 2 files per message'), findsOneWidget);
    expect(send(tester).onPressed, isNull);
  });

  testWidgets('the search replaces the recent targets; nothing found says so', (
    tester,
  ) async {
    await pump(tester);
    await tester.enterText(find.byKey(const ValueKey('share.search')), 'gen');
    await tester.pumpAndSettle();
    expect(queries, ['gen']);
    expect(find.text(dsIsolate('server > Gateway: resume')), findsNothing);
    expect(find.text(dsIsolate('general')), findsOneWidget);
    await tester.tap(find.byKey(const ValueKey('share.target.0')));
    await tester.pumpAndSettle();
    expect(send(tester).onPressed, isNotNull);

    await tester.enterText(find.byKey(const ValueKey('share.search')), 'zzz');
    await tester.pumpAndSettle();
    expect(find.text('No target found'), findsOneWidget);
    await tester.enterText(find.byKey(const ValueKey('share.search')), '');
    await tester.pumpAndSettle();
    expect(find.text(dsIsolate('server > Gateway: resume')), findsOneWidget);
  });

  testWidgets('Cancel closes without sending', (tester) async {
    await pump(tester);
    await tester.tap(find.byKey(const ValueKey('share.cancel')));
    await tester.pumpAndSettle();
    expect(cancelled, 1);
    expect(sent, isEmpty);
  });

  testWidgets(
    'a text share without files and without a comment cannot be sent',
    (tester) async {
      await pump(tester, controller(files: const []));
      await tester.tap(find.byKey(const ValueKey('share.target.2')));
      await tester.pumpAndSettle();
      expect(send(tester).onPressed, isNull);
      await tester.enterText(find.byKey(const ValueKey('share.comment')), 'Hi');
      await tester.pump();
      expect(send(tester).onPressed, isNotNull);
    },
  );
}
