// SPDX-License-Identifier: Apache-2.0

// Hosts the history column of thread 1 over an in-memory database with fake REST reads (no
// network), Alex as the signed-in person.

import 'package:client/sidebar/providers.dart';
import 'package:client/thread/history_column.dart';
import 'package:client/thread/providers.dart';
import 'package:client/thread/thread_reads.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/misc.dart' show Override;
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';

import 'column_host.dart';
import 'structure.dart';
import 'thread_fixtures.dart';

/// REST reads of the thread view answered by the test.
class FakeThreadReads implements ThreadReads {
  /// Creates the reads on [db]; older pages and ranges come from [archive] (stored when asked).
  FakeThreadReads(this.db, {this.archive = const []});

  /// The database the loaded messages go to.
  final LocalDatabase db;

  /// The messages the "server" has beyond the stored ones.
  final List<Message> archive;

  /// The earlier versions per message.
  final Map<MessageId, List<Revision>> revisionsOf = {};

  /// The personal templates.
  final List<TemplateItem> personal = [];

  /// The templates of the channel.
  final List<TemplateItem> channel = [];

  /// The calls, for assertions.
  final List<String> calls = [];

  @override
  Future<bool> loadOlder(ThreadId thread, ThreadSeq before) async {
    calls.add('older ${before.value}');
    final page = [
      for (final m in archive)
        if (m.threadId == thread && m.threadSeq < before) m,
    ];
    final take = page.length > 20 ? page.sublist(page.length - 20) : page;
    if (take.isEmpty) return false;
    for (final m in take) {
      await db.messagesDao.upsertIfNewer(m);
    }
    await db.messagesDao.addLoadedRange(
      thread,
      take.first.threadSeq,
      ThreadSeq(before.value - 1),
    );
    return take.first.threadSeq.value > 1;
  }

  @override
  Future<void> loadRange(ThreadId thread, ThreadSeq from, ThreadSeq to) async {
    calls.add('range ${from.value}-${to.value}');
    for (final m in archive) {
      if (m.threadId == thread && m.threadSeq >= from && m.threadSeq <= to) {
        await db.messagesDao.upsertIfNewer(m);
      }
    }
    await db.messagesDao.addLoadedRange(thread, from, to);
  }

  @override
  Future<ThreadSeq?> positionOf(MessageId id) async {
    calls.add('locate');
    for (final m in archive) {
      if (m.id == id) return m.threadSeq;
    }
    return (await db.messagesDao.byId(id))?.threadSeq;
  }

  @override
  Future<List<Revision>> revisions(MessageId id) async => revisionsOf[id] ?? [];

  @override
  Future<TemplateLists> templates(ChannelId channelId) async =>
      TemplateLists(personal: personal, channel: channel);

  /// The threads the "server" has; [loadThread] stores them.
  final Map<ThreadId, Thread> threads = {};

  @override
  Future<void> loadThread(ThreadId thread) async {
    calls.add('thread');
    final t = threads[thread];
    if (t == null) throw StateError('not found');
    await db.threadsDao.upsertIfNewer(t);
  }

  /// The threads the "server" lists per channel; [loadChannelThreads] stores them.
  final Map<ChannelId, List<Thread>> channelThreads = {};

  @override
  Future<void> loadChannelThreads(ChannelId channel) async {
    calls.add('threads');
    for (final t in channelThreads[channel] ?? const <Thread>[]) {
      await db.threadsDao.upsertIfNewer(t);
    }
  }

  /// Pinned messages the "server" lists beyond the stored ones.
  final List<Message> pins = [];

  @override
  Future<void> loadPins(ThreadId thread) async {
    calls.add('pins');
    for (final m in pins) {
      if (m.threadId == thread) await db.messagesDao.upsertIfNewer(m);
    }
  }
}

/// The overrides of the thread tests: Alex signed in, the [reads].
List<Override> threadOverrides(FakeThreadReads reads) => [
  ownPrincipalProvider.overrideWith(
    (ref) async => people.firstWhere((p) => p.id == alex),
  ),
  threadReadsProvider.overrideWithValue(reads),
];

/// Pumps the history column of thread [n] (with [jumpTo]) on [host].
Future<void> pumpThread(
  WidgetTester tester,
  ColumnHost host,
  FakeThreadReads reads, {
  int n = 1,
  MessageId? jumpTo,
  Locale locale = const Locale('en'),
}) => host.pump(
  tester,
  HistoryColumn(threadId: threadId(n), jumpTo: jumpTo),
  location: '/t/${threadId(n).value}',
  locale: locale,
  overrides: threadOverrides(reads),
);

/// The text field of the composer.
final Finder composerField = find.descendant(
  of: find.byKey(const ValueKey('composer')),
  matching: find.byType(TextField),
);
