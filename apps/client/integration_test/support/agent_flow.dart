// SPDX-License-Identifier: Apache-2.0

// The shared start of the agent interface tests of M4-21: the fake with Robin as the operator
// of `general`, Alex signed in on the thread "Release notes", and small finders.

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/testing.dart';

import 'test_app.dart';

/// The opened forum post of an agent test.
final class AgentThread {
  AgentThread._(this.app, this.forum, this.thread);

  /// The app.
  final TestApp app;

  /// The forum `deploys` (Alex `trigger`, Sam `write`, Robin `write` and its operator).
  final String forum;

  /// The post "Release checklist" in it.
  final String thread;
}

/// Starts the app with the forum `deploys` and its post "Release checklist", lets [setup]
/// change the world before the sign-in, signs Alex in and opens the post (01 D1).
Future<AgentThread> openAgentThread(
  WidgetTester tester, {
  void Function(FakeWorld world, WorldSeed seed, String forum, String thread)?
  setup,
}) async {
  final app = await TestApp.start(tester);
  final world = app.fake.server.world;
  final seed = app.fake.seed;
  final forum = world.addForum(seed.space, 'deploys', {
    seed.alex.id: 'trigger',
    seed.sam.id: 'write',
    seed.robin.id: 'write',
  });
  final thread = world.addThread(
    forum,
    'Release checklist',
    createdBy: seed.alex.id,
  );
  world
    ..resolveMentions = true
    ..setChannelOperator(forum, seed.robin.id);
  setup?.call(world, seed, forum, thread);
  await app.connect();
  await app.waitFor(find.byKey(const ValueKey('e2.loginName')));
  await app.loginWithPassword();
  await app.waitFor(find.byKey(const ValueKey('column.sidebar')));
  await app.waitLive();
  await app.waitFor(find.byKey(ValueKey('sidebar.channel.$forum')));
  await tester.tap(find.byKey(ValueKey('sidebar.channel.$forum')));
  await app.waitFor(find.byKey(ValueKey('postList.post.$thread')));
  await tester.tap(find.byKey(ValueKey('postList.post.$thread')));
  await app.waitFor(find.byKey(ValueKey('history.$thread')));
  return AgentThread._(app, forum, thread);
}

/// Pumps until [condition] holds, with the bound of [TestApp.waitFor].
Future<void> waitUntil(
  WidgetTester tester,
  String what,
  bool Function() condition,
) async {
  final end = DateTime.now().add(const Duration(seconds: 20));
  while (!condition()) {
    if (DateTime.now().isAfter(end)) fail('timed out: $what');
    await tester.pump(const Duration(milliseconds: 50));
  }
}

/// The composer's text field.
final Finder composerText = find.descendant(
  of: find.byKey(const ValueKey('composer')),
  matching: find.byType(TextField),
);

/// Types [text] into the composer and sends it with Enter.
Future<void> sendText(WidgetTester tester, String text) async {
  await tester.enterText(composerText, text);
  await tester.pump();
  await tester.sendKeyEvent(LogicalKeyboardKey.enter);
  await tester.pump();
}

/// The trigger state icon of the message [messageId], or nothing.
Finder triggerOf(String messageId, DsTriggerState state) => find.descendant(
  of: find.byKey(ValueKey('message.$messageId')),
  matching: find.byWidgetPredicate(
    (w) => w is DsTriggerStateIcon && w.state == state,
  ),
);

/// The newest message of [authorId] in [threadId] on the server.
String newestOf(FakeWorld world, String threadId, String authorId) =>
    world
            .messagesOf(threadId)
            .lastWhere((m) => (m['author']! as Map)['id'] == authorId)['id']!
        as String;
