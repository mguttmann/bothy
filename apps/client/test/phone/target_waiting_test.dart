// SPDX-License-Identifier: Apache-2.0

// A thread or a channel of a tapped push or a link that this device does not hold while the
// server cannot be asked (docs/decisions.d/AUD-client-phone-01.md; 01 6.7; 15 11): the page
// says that it waits instead of staying empty, asks again once the connection is live, then
// shows the target, or "Not found or no access" when the server answers so.

import 'dart:async';

import 'package:client/app/providers/session.dart';
import 'package:client/app/providers/target.dart';
import 'package:client/app/target_resolver.dart';
import 'package:client/forum/post_list_column.dart';
import 'package:client/phone/phone_pages.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart' hide ConnectionState;
import 'package:flutter_riverpod/misc.dart' show Override;
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/sync_engine.dart';

import '../support/column_host.dart';
import '../support/new_chat_fixtures.dart' show fillForums, server;
import '../support/structure.dart';
import '../support/thread_host.dart';

/// A post of the forum `server` that this device never stored.
final Thread _late = thread(7, title: 'Deploy window');

/// A forum this device never stored.
final Channel _lateForum = channel(9, kind: ChannelKind.forum, name: 'late');

/// The server as the test switches it: unreachable at first.
final class _Server {
  _Server(this.reads);

  final FakeThreadReads reads;

  /// What the target check answers once the server can be asked.
  TargetStatus answer = TargetStatus.visible;
  bool reachable = false;
  final List<String> checks = [];
  final StreamController<ConnectionState> states =
      StreamController<ConnectionState>.broadcast();

  List<Override> get overrides => [
    ...threadOverrides(reads),
    targetStatusProvider.overrideWith((ref, path) async {
      checks.add(path);
      return reachable ? answer : TargetStatus.unknown;
    }),
    connectionStateProvider.overrideWith((ref) => states.stream),
  ];

  /// The connection drops and is live again; the server answers from now on.
  Future<void> live(WidgetTester tester, ColumnHost host) async {
    reachable = true;
    states.add(const ConnectionState(ConnectionPhase.backoff));
    await host.settle(tester);
    states.add(const ConnectionState(ConnectionPhase.live));
    await host.settle(tester);
  }
}

final Finder _waiting = find.byKey(const ValueKey('target.waiting'));
final Finder _notFound = find.byKey(const ValueKey('target.notFound'));

/// The title of the late post in the header of its history.
final Finder _title = find.textContaining('Deploy window');

void main() {
  late ColumnHost host;
  late _Server srv;

  Future<void> pump(WidgetTester tester, Widget page) async {
    host = await ColumnHost.open(tester);
    await host.run(tester, () => fillForums(host.db));
    srv = _Server(FakeThreadReads(host.db));
    addTearDown(srv.states.close);
    await host.pump(
      tester,
      page,
      platform: 'ios',
      density: DsDensityMode.touch,
      overrides: srv.overrides,
    );
    srv.states.add(const ConnectionState(ConnectionPhase.offline));
    await host.settle(tester);
  }

  final threadPath = '/t/${_late.id.value}';
  final channelPath = '/c/${_lateForum.id.value}';

  testWidgets('a thread the device does not hold: the page waits, asks again '
      'when the connection is live and shows the thread', (tester) async {
    await pump(tester, PhoneThreadPage(location: Uri.parse(threadPath)));
    expect(_waiting, findsOneWidget);
    expect(srv.checks, [threadPath]);
    expect(srv.reads.calls.where((c) => c == 'thread'), hasLength(1));

    srv.reads.threads[_late.id] = _late;
    await srv.live(tester, host);

    expect(srv.checks, [threadPath, threadPath]);
    expect(srv.reads.calls.where((c) => c == 'thread'), hasLength(2));
    expect(_waiting, findsNothing);
    expect(_notFound, findsNothing);
    expect(_title, findsWidgets);
    await host.dispose(tester);
  });

  testWidgets('a thread the server does not know after all: "Not found or no '
      'access" once it can be asked', (tester) async {
    await pump(tester, PhoneThreadPage(location: Uri.parse(threadPath)));
    expect(_waiting, findsOneWidget);
    expect(_notFound, findsNothing);

    srv.answer = TargetStatus.notFound;
    await srv.live(tester, host);

    expect(_waiting, findsNothing);
    expect(_notFound, findsOneWidget);
    await host.dispose(tester);
  });

  testWidgets('a thread the sync brings while the server cannot be asked '
      'shows at once', (tester) async {
    await pump(tester, PhoneThreadPage(location: Uri.parse(threadPath)));
    expect(_waiting, findsOneWidget);

    await host.run(tester, () => host.db.threadsDao.upsertIfNewer(_late));
    await host.settle(tester);

    expect(_waiting, findsNothing);
    expect(_title, findsWidgets);
    await host.dispose(tester);
  });

  testWidgets('a stored thread never shows the waiting mark', (tester) async {
    host = await ColumnHost.open(tester);
    await host.run(tester, () async {
      await fillForums(host.db);
      await host.db.threadsDao.upsertIfNewer(_late);
    });
    srv = _Server(FakeThreadReads(host.db))..reachable = true;
    addTearDown(srv.states.close);
    await host.pump(
      tester,
      PhoneThreadPage(location: Uri.parse(threadPath)),
      platform: 'ios',
      density: DsDensityMode.touch,
      overrides: srv.overrides,
    );
    expect(_waiting, findsNothing);
    expect(_title, findsWidgets);
    // A connection that is live again asks nothing again for a visible target.
    await srv.live(tester, host);
    expect(srv.checks, [threadPath]);
    expect(srv.reads.calls.where((c) => c == 'thread'), isEmpty);
    await host.dispose(tester);
  });

  testWidgets('a channel the device does not hold: the page waits, asks again '
      'when the connection is live and shows the channel the sync brought', (
    tester,
  ) async {
    await pump(tester, PhoneChannelPage(channelId: _lateForum.id));
    expect(_waiting, findsOneWidget);
    expect(srv.checks, [channelPath]);

    await host.run(
      tester,
      () => host.db.structureDao.upsertChannelIfNewer(_lateForum),
    );
    await srv.live(tester, host);

    expect(srv.checks, [channelPath, channelPath]);
    expect(_waiting, findsNothing);
    expect(find.byType(PostListColumn), findsOneWidget);
    await host.dispose(tester);
  });

  testWidgets('a channel the server does not know after all: "Not found or no '
      'access" once it can be asked', (tester) async {
    await pump(tester, PhoneChannelPage(channelId: _lateForum.id));
    expect(_waiting, findsOneWidget);

    srv.answer = TargetStatus.notFound;
    await srv.live(tester, host);

    expect(_waiting, findsNothing);
    expect(_notFound, findsOneWidget);
    await host.dispose(tester);
  });

  test('the fixtures: the forum of the late post is stored, the late forum '
      'is not', () {
    expect(_late.channelId, server.id);
    expect(_lateForum.id, isNot(server.id));
  });
}
