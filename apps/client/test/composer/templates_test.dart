// SPDX-License-Identifier: Apache-2.0

// Templates in the composer (brief M4-20 Tests; 01 3.3): personal and channel templates are
// offered and inserted; the current text is saved as a personal template through the outbox.

import 'package:client/sync/message_ops_app.dart';
import 'package:client/thread/thread_reads.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/sync_engine.dart';

import '../support/column_host.dart';
import '../support/thread_fixtures.dart';
import '../support/thread_host.dart';

void main() {
  testWidgets('personal and channel templates are inserted', (tester) async {
    final host = await ColumnHost.open(tester);
    await host.run(tester, () => seedThread(host.db, messages: [msg(1)]));
    final reads = FakeThreadReads(host.db)
      ..personal.add(
        const TemplateItem(
          id: 't1',
          name: 'Greeting',
          contentMd: 'Hello, how can I help?',
          personal: true,
        ),
      )
      ..channel.add(
        const TemplateItem(
          id: 't2',
          name: 'Escalation',
          contentMd: 'Please escalate this.',
          personal: false,
        ),
      );
    await pumpThread(tester, host, reads);

    await tester.tap(find.byKey(const ValueKey('composer.templates')));
    await tester.pump(const Duration(milliseconds: 500));
    await host.settle(tester);
    expect(find.text(dsIsolate('Greeting')), findsOneWidget);
    expect(find.text(dsIsolate('Escalation')), findsOneWidget);
    await tester.tap(find.byKey(const ValueKey('template.t2')));
    await tester.pump(const Duration(milliseconds: 500));
    await host.settle(tester);
    expect(
      tester.widget<TextField>(composerField).controller!.text,
      'Please escalate this.',
    );

    await tester.tap(find.byKey(const ValueKey('composer.templates')));
    await tester.pump(const Duration(milliseconds: 500));
    await host.settle(tester);
    await tester.tap(find.byKey(const ValueKey('template.t1')));
    await tester.pump(const Duration(milliseconds: 500));
    await host.settle(tester);
    expect(
      tester.widget<TextField>(composerField).controller!.text,
      'Please escalate this.Hello, how can I help?',
    );
    await host.dispose(tester);
  });

  testWidgets('the current text is saved as a personal template', (
    tester,
  ) async {
    final host = await ColumnHost.open(tester);
    await host.run(tester, () => seedThread(host.db, messages: [msg(1)]));
    await pumpThread(tester, host, FakeThreadReads(host.db));
    await tester.enterText(composerField, 'See the wiki page.');
    await host.settle(tester);
    await tester.tap(find.byKey(const ValueKey('composer.templates')));
    await tester.pump(const Duration(milliseconds: 500));
    await host.settle(tester);
    await tester.tap(find.byKey(const ValueKey('template.save')));
    await tester.pump(const Duration(milliseconds: 500));
    await host.settle(tester);
    await tester.enterText(find.byKey(const ValueKey('template.name')), 'Wiki');
    await tester.pump();
    await tester.tap(find.byKey(const ValueKey('template.saveConfirm')));
    await tester.pump(const Duration(milliseconds: 500));
    await host.settle(tester);
    final saved = (await host.run(
      tester,
      () => host.db.outboxDao.allEntries(),
    )).singleWhere((e) => e.kind == AppMessageOps.createPersonalTemplate);
    expect(decodePayload(saved.payload), {
      'name': 'Wiki',
      'content_md': 'See the wiki page.',
    });
    await host.dispose(tester);
  });
}
