// SPDX-License-Identifier: Apache-2.0

// The forums, people and helpers of the new chat tests (brief M4-41 Tests): a project forum
// with an operator agent and tags, a forum without an operator, a text channel, and the desktop
// shell over the column host.

import 'dart:convert';

import 'package:client/app/layout/desktop_shell.dart';
import 'package:client/app/providers/target.dart';
import 'package:client/app/target_resolver.dart';
import 'package:client/forum/providers.dart';
import 'package:client/sidebar/providers.dart';
import 'package:client/sync/thread_ops.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/misc.dart' show Override;
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:local_store/local_store.dart';

import 'structure.dart';

/// The project forum `server` (channel 1): operator agent Robin and the tags bug, idea, ops.
final Channel server = channel(
  1,
  kind: ChannelKind.forum,
  name: 'server',
  operator: robin,
).copyWith(projectBound: true);

/// The forum `ideas` (channel 2) without an operator agent.
final Channel ideas = channel(2, kind: ChannelKind.forum, name: 'ideas');

/// The text channel `general` (channel 3).
final Channel general = channel(3, name: 'general');

/// The tags of [server] in the forum's order.
final List<ForumTag> serverTags = [
  tag(1, 'bug'),
  tag(2, 'idea'),
  tag(3, 'ops'),
];

/// Robin, the operator agent.
final Principal robinAgent = Principal(
  id: robin,
  kind: PrincipalKind.agent,
  handle: 'robin',
  displayName: 'Robin',
);

/// Stores the space, the three channels, the tags and Robin in [db].
Future<void> fillForums(LocalDatabase db) async {
  await db.structureDao.upsertSpaceIfNewer(space(1));
  for (final c in [server, ideas, general]) {
    await db.structureDao.upsertChannelIfNewer(c);
  }
  await db.structureDao.putForumTags(server.id, serverTags);
  await db.principalsDao.put(robinAgent);
}

/// The ID of draft [n].
DraftId draftId(int n) => DraftId.parse(uuid(0x900 + n));

/// The person's stored draft of a new post in [forum] with [text] (03 `drafts` with
/// `thread_id` NULL), as another device or an earlier run left it.
Draft storedDraft(ChannelId forum, String text, {int n = 1}) => Draft(
  id: draftId(n),
  channelId: forum,
  contentMd: text,
  fileIds: const [],
  version: const ResourceVersion(3),
  updatedAt: t0,
);

/// The overrides of the shell tests; [created] records the threads of sent posts.
List<Override> shellOverrides({
  CreatedPosts? created,
  List<Override> more = const [],
}) => [
  targetStatusProvider.overrideWith((ref, path) async => TargetStatus.visible),
  ownPrincipalProvider.overrideWith(
    (ref) async => Principal(
      id: alex,
      kind: PrincipalKind.user,
      handle: 'alex',
      displayName: 'Alex',
    ),
  ),
  if (created != null) createdPostsProvider.overrideWithValue(created),
  ...more,
];

/// The shell of the router's location.
final Widget shell = Builder(
  builder: (context) => DesktopShell(location: GoRouterState.of(context).uri),
);

/// The path of the forum overview of [forum].
String forumPath(Channel forum) => '/c/${forum.id.value}';

/// The path of the new chat view of [forum], with the sent [entry].
String newChatPath(Channel forum, {String? entry}) =>
    '/c/${forum.id.value}/new${entry == null ? '' : '?entry=$entry'}';

/// The sidebar row of [c].
Finder channelRow(Channel c) =>
    find.byKey(ValueKey('sidebar.channel.${c.id.value}'));

/// The draft row of [forum].
Finder draftRow(Channel forum) =>
    find.byKey(ValueKey('sidebar.newChat.${forum.id.value}.draft'));

/// The row of the sent post of outbox entry [entry] in [forum].
Finder sentRow(Channel forum, String entry) =>
    find.byKey(ValueKey('sidebar.newChat.${forum.id.value}.$entry'));

/// Every new chat row (draft or sent) of [forum].
Finder newChatRows(Channel forum) => find.byWidgetPredicate(
  (w) =>
      w is DsSidebarDraftRow &&
      w.key is ValueKey<String> &&
      (w.key! as ValueKey<String>).value.startsWith(
        'sidebar.newChat.${forum.id.value}.',
      ),
);

/// The text field of the composer of the new chat view.
Finder get composerField => find.descendant(
  of: find.byKey(const ValueKey('newChat.composer')),
  matching: find.byType(EditableText),
);

/// The text field of the title of the new chat view.
Finder get titleField => find.descendant(
  of: find.byKey(const ValueKey('newChat.title')),
  matching: find.byType(EditableText),
);

/// The text in the composer of the new chat view.
String composerText(WidgetTester tester) =>
    tester.widget<EditableText>(composerField).controller.text;

/// Whether the composer's text field has the keyboard focus.
bool composerFocused(WidgetTester tester) =>
    tester.widget<EditableText>(composerField).focusNode.hasFocus;

/// The decoded payload of [e].
Map<String, Object?> payloadOf(OutboxEntry e) =>
    (jsonDecode(e.payload) as Map).cast<String, Object?>();

/// The outbox entries of [kind].
List<OutboxEntry> ofKind(List<OutboxEntry> all, String kind) => [
  for (final e in all)
    if (e.kind == kind) e,
];

/// The `createThread` entries of [all].
List<OutboxEntry> creates(List<OutboxEntry> all) =>
    ofKind(all, ThreadOps.createThread);

/// The `putChannelDraft` entries of [all].
List<OutboxEntry> draftWrites(List<OutboxEntry> all) =>
    ofKind(all, ThreadOps.putChannelDraft);

/// The top of [f] on screen.
double topOf(WidgetTester tester, Finder f) => tester.getTopLeft(f).dy;
