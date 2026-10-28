// SPDX-License-Identifier: Apache-2.0

// The new chat view in the desktop shell over the local database (brief M4-41 Tests; 01 6.8
// item 8, 7; 09 12; D-M4-41-1, D-M4-41-4): it opens from "+", the forum's context menu and
// Cmd+N or Ctrl+N with the focus in the input; the title is optional with an operator agent
// and required without one; Esc leaves; an empty draft is deleted on leaving and a draft with
// text stays; sending puts one `createThread` with `clear_draft` into the outbox, shows
// "Sending" and opens the post once the server created it; a refusal shows "Try again".

import 'dart:convert';

import 'package:client/app/layout/desktop_shell.dart' show newChatForum;
import 'package:client/keyboard/bindings.dart';
import 'package:client/keyboard/intents.dart';
import 'package:client/sync/thread_ops.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';

import '../support/column_host.dart';
import '../support/keys.dart';
import '../support/new_chat_fixtures.dart';
import '../support/structure.dart';
import '../support/widgets.dart';

/// The binding of "New post or new chat in the current forum" on the platform of the test
/// (01 7: Cmd+N on macOS, Ctrl+N on Windows).
KeyStroke newChatStroke() => shortcutRows
    .expand((r) => r.bindings)
    .firstWhere((b) => b.action == ShortcutAction.newPost)
    .on(ShortcutPlatform.current)
    .single
    .single;

/// macOS (Cmd+N) and Windows (Ctrl+N).
final TargetPlatformVariant desktops = TargetPlatformVariant({
  TargetPlatform.macOS,
  TargetPlatform.windows,
});

/// Opens the host with the forums stored and the shell at [location].
Future<ColumnHost> pumpShell(
  WidgetTester tester, {
  required String location,
  CreatedPosts? created,
  Future<void> Function(LocalDatabase db)? more,
}) async {
  final host = await ColumnHost.open(tester);
  await tester.runAsync(() async {
    await fillForums(host.db);
    await more?.call(host.db);
  });
  await host.pump(
    tester,
    shell,
    location: location,
    overrides: shellOverrides(created: created),
  );
  return host;
}

/// Puts the keyboard focus on the sidebar row of [c].
Future<void> focusRow(WidgetTester tester, ColumnHost host, Channel c) async {
  tester.widget<DsChannelRow>(channelRow(c)).focusNode!.requestFocus();
  await host.settle(tester);
}

Future<List<OutboxEntry>> outbox(WidgetTester tester, ColumnHost host) =>
    host.run(tester, host.db.outboxDao.allEntries);

/// Types [text] into the composer of the view.
Future<void> type(WidgetTester tester, ColumnHost host, String text) async {
  await tester.enterText(composerField, text);
  await host.settle(tester);
}

/// Enter in the composer (Enter sends by default, 01 7).
Future<void> pressEnter(WidgetTester tester, ColumnHost host) async {
  await tester.showKeyboard(composerField);
  await tester.sendKeyEvent(LogicalKeyboardKey.enter);
  await host.settle(tester);
}

/// A new post of [server] this device sent earlier (outbox entry 0x801): title "Restart",
/// sent with `clear_draft` as the view sends it.
({String id, String kind, String payload}) sentPost() {
  final request = ThreadOps.postRequest(
    forum: server.id,
    title: 'Restart',
    contentMd: 'Restart the gateway',
    clearDraft: true,
  );
  return (
    id: uuid(0x801),
    kind: request.kind,
    payload: jsonEncode(request.payload),
  );
}

/// Stores [post] in the outbox of [db], pending.
Future<void> storeSentPost(
  LocalDatabase db,
  ({String id, String kind, String payload}) post,
) => db.outboxDao
    .enqueue(id: post.id, kind: post.kind, payload: post.payload)
    .then((_) {});

String heading(WidgetTester tester) =>
    tester.widget<Text>(find.byKey(const ValueKey('newChat.heading'))).data!;

void main() {
  group('opening', () {
    testWidgets(
      '"+" on a forum row opens the view with the focus in the input',
      (tester) async {
        final host = await pumpShell(tester, location: forumPath(server));
        // The selected forum row shows "+" (components.md 7).
        await tester.tap(find.byTooltip(en.sidebarNewChatIn('server')));
        await host.settle(tester);
        expect(host.visited.last, newChatPath(server));
        expect(find.byKey(const ValueKey('newChat.view')), findsOneWidget);
        expect(heading(tester), en.sidebarNewChatIn(dsIsolate('server')));
        expect(composerFocused(tester), isTrue);
        await host.dispose(tester);
      },
    );

    testWidgets(
      '"+" on a forum without a project binding reads "New post in"',
      (tester) async {
        final host = await pumpShell(tester, location: forumPath(ideas));
        await tester.tap(find.byTooltip(en.sidebarNewPostIn('ideas')));
        await host.settle(tester);
        expect(host.visited.last, newChatPath(ideas));
        expect(heading(tester), en.sidebarNewPostIn(dsIsolate('ideas')));
        expect(composerFocused(tester), isTrue);
        await host.dispose(tester);
      },
    );

    testWidgets('the context menu of a forum row opens the view', (
      tester,
    ) async {
      final host = await pumpShell(tester, location: '/');
      await tester.tap(channelRow(server), buttons: kSecondaryMouseButton);
      await host.settle(tester);
      expect(find.byKey(const ValueKey('menu.newChat')), findsOneWidget);
      expect(
        find.text(dsIsolate(en.sidebarNewChatIn('server'))),
        findsOneWidget,
      );
      await tester.tap(find.byKey(const ValueKey('menu.newChat')));
      await host.settle(tester);
      expect(host.visited.last, newChatPath(server));
      expect(find.byKey(const ValueKey('newChat.view')), findsOneWidget);
      expect(composerFocused(tester), isTrue);
      await host.dispose(tester);
    });

    testWidgets('the context menu of a text channel has no new chat entry', (
      tester,
    ) async {
      final host = await pumpShell(tester, location: '/');
      await tester.tap(channelRow(general), buttons: kSecondaryMouseButton);
      await host.settle(tester);
      expect(find.byKey(const ValueKey('menu.newChat')), findsNothing);
      await tester.sendKeyEvent(LogicalKeyboardKey.escape);
      await host.settle(tester);
      await host.dispose(tester);
    });

    testWidgets(
      'Cmd+N or Ctrl+N opens the view of the focused forum row before the selected forum',
      (tester) async {
        final host = await pumpShell(tester, location: forumPath(ideas));
        await focusRow(tester, host, server);
        expect(await pressStroke(tester, newChatStroke()), isTrue);
        await host.settle(tester);
        expect(host.visited.last, newChatPath(server));
        expect(composerFocused(tester), isTrue);
        await host.dispose(tester);
      },
      variant: desktops,
    );

    testWidgets(
      'Cmd+N or Ctrl+N opens the view of the selected forum without a focused row',
      (tester) async {
        final host = await pumpShell(tester, location: forumPath(ideas));
        // The focus in the content column, outside the sidebar rows.
        final newPost = find.byKey(const ValueKey('postList.newPost'));
        Focus.of(
          tester.element(
            find.descendant(of: newPost, matching: find.byType(Semantics)).last,
          ),
        ).requestFocus();
        await host.settle(tester);
        expect(await pressStroke(tester, newChatStroke()), isTrue);
        await host.settle(tester);
        expect(host.visited.last, newChatPath(ideas));
        expect(composerFocused(tester), isTrue);
        await host.dispose(tester);
      },
      variant: desktops,
    );

    testWidgets('Cmd+N or Ctrl+N on a focused text channel row opens nothing', (
      tester,
    ) async {
      final host = await pumpShell(tester, location: forumPath(server));
      await focusRow(tester, host, general);
      final before = List.of(host.visited);
      await pressStroke(tester, newChatStroke());
      await host.settle(tester);
      expect(host.visited, before);
      expect(find.byKey(const ValueKey('newChat.view')), findsNothing);
      await host.dispose(tester);
    }, variant: desktops);

    testWidgets(
      'newChatForum: the focused row wins, a text channel row gives null',
      (tester) async {
        final host = await pumpShell(tester, location: forumPath(server));
        BuildContext rowContext(Channel c) => tester.element(channelRow(c));
        expect(
          newChatForum(rowContext(ideas), selected: server.id),
          ideas.id,
          reason: 'the focused forum row',
        );
        expect(
          newChatForum(rowContext(general), selected: server.id),
          isNull,
          reason: 'a text channel row',
        );
        expect(
          newChatForum(null, selected: server.id),
          server.id,
          reason: 'no focused row: the selected forum',
        );
        expect(newChatForum(null, selected: null), isNull);
        await host.dispose(tester);
      },
    );
  });

  group('title rule', () {
    testWidgets(
      'with an operator agent the title is optional and an empty title is sent',
      (tester) async {
        final host = await pumpShell(tester, location: newChatPath(server));
        expect(find.text(en.newChatTitleOptional), findsOneWidget);
        expect(find.text(en.newChatTitleAgentHint), findsOneWidget);
        expect(
          tester
              .widget<Text>(find.byKey(const ValueKey('newChat.operator')))
              .data,
          en.threadOperator(dsIsolate('Robin')),
        );
        await type(tester, host, 'Fix the login page');
        expect(
          find.byKey(const ValueKey('newChat.titleMissing')),
          findsNothing,
        );
        await pressEnter(tester, host);
        final sent = creates(await outbox(tester, host));
        expect(sent, hasLength(1));
        expect(payloadOf(sent.single)['title'], '');
        await host.dispose(tester);
      },
    );

    testWidgets(
      'without an operator agent the title is required before sending',
      (tester) async {
        final host = await pumpShell(tester, location: newChatPath(ideas));
        expect(find.text(en.newPostTitle), findsOneWidget);
        expect(find.text(en.newChatTitleOptional), findsNothing);
        expect(find.text(en.newChatTitleAgentHint), findsNothing);
        expect(
          tester
              .widget<Text>(find.byKey(const ValueKey('newChat.operator')))
              .data,
          en.threadNoOperator,
        );
        expect(
          find.byKey(const ValueKey('newChat.titleMissing')),
          findsNothing,
          reason: 'no hint before any text',
        );
        await type(tester, host, 'Dark mode for the docs');
        expect(
          find.byKey(const ValueKey('newChat.titleMissing')),
          findsOneWidget,
        );
        expect(find.text(en.newChatTitleMissing), findsOneWidget);
        expect(
          tester
              .widget<DsComposer>(
                find.byKey(const ValueKey('newChat.composer')),
              )
              .sendEnabled,
          isFalse,
        );
        await pressEnter(tester, host);
        expect(creates(await outbox(tester, host)), isEmpty);
        expect(host.visited.last, newChatPath(ideas));

        await tester.enterText(titleField, 'Dark mode');
        await host.settle(tester);
        expect(
          find.byKey(const ValueKey('newChat.titleMissing')),
          findsNothing,
        );
        await pressEnter(tester, host);
        final sent = creates(await outbox(tester, host));
        expect(sent, hasLength(1));
        expect(payloadOf(sent.single)['title'], 'Dark mode');
        expect(payloadOf(sent.single)['content_md'], 'Dark mode for the docs');
        await host.dispose(tester);
      },
    );
  });

  group('leaving', () {
    testWidgets('Esc leaves the view to the forum overview', (tester) async {
      final host = await pumpShell(tester, location: forumPath(server));
      await tester.tap(find.byTooltip(en.sidebarNewChatIn('server')));
      await host.settle(tester);
      expect(composerFocused(tester), isTrue);
      await tester.sendKeyEvent(LogicalKeyboardKey.escape);
      await host.settle(tester);
      expect(host.visited.last, forumPath(server));
      expect(find.byKey(const ValueKey('newChat.view')), findsNothing);
      await host.dispose(tester);
    });

    testWidgets('a draft typed and cleared is deleted on leaving', (
      tester,
    ) async {
      final host = await pumpShell(tester, location: newChatPath(server));
      await type(tester, host, 'Half a thought');
      expect(
        payloadOf(draftWrites(await outbox(tester, host)).single)['content_md'],
        'Half a thought',
      );
      await type(tester, host, '');
      expect(draftRow(server), findsOneWidget, reason: 'the view still writes');
      await tester.sendKeyEvent(LogicalKeyboardKey.escape);
      await host.settle(tester);
      expect(host.visited.last, forumPath(server));
      final writes = draftWrites(await outbox(tester, host));
      expect(writes, isNotEmpty);
      expect(payloadOf(writes.last)['content_md'], '');
      expect(payloadOf(writes.last)['channel_id'], server.id.value);
      expect(draftRow(server), findsNothing);
      await host.dispose(tester);
    });

    testWidgets('a stored draft emptied in the view is deleted on leaving', (
      tester,
    ) async {
      final host = await pumpShell(
        tester,
        location: newChatPath(server),
        more: (db) => db.personalDao.upsertDraftIfNewer(
          storedDraft(server.id, 'Old idea'),
        ),
      );
      expect(composerText(tester), 'Old idea');
      expect(draftRow(server), findsOneWidget);
      await type(tester, host, '');
      await tester.sendKeyEvent(LogicalKeyboardKey.escape);
      await host.settle(tester);
      final writes = draftWrites(await outbox(tester, host));
      expect(writes, isNotEmpty);
      expect(payloadOf(writes.last)['content_md'], '');
      expect(
        await host.run(tester, host.db.personalDao.allDrafts),
        isEmpty,
        reason: 'the stored draft row is removed',
      );
      expect(draftRow(server), findsNothing);
      await host.dispose(tester);
    });

    testWidgets('a draft with text stays when leaving', (tester) async {
      final host = await pumpShell(tester, location: newChatPath(server));
      await type(tester, host, 'Keep me');
      await tester.sendKeyEvent(LogicalKeyboardKey.escape);
      await host.settle(tester);
      expect(host.visited.last, forumPath(server));
      final writes = draftWrites(await outbox(tester, host));
      expect(writes, hasLength(1));
      expect(payloadOf(writes.single)['content_md'], 'Keep me');
      expect(draftRow(server), findsOneWidget);
      await host.dispose(tester);
    });
  });

  group('sending', () {
    testWidgets(
      'puts one createThread with clear_draft and the tags in the forum order into the outbox',
      (tester) async {
        final host = await pumpShell(
          tester,
          location: newChatPath(server),
          more: (db) => db.personalDao.upsertDraftIfNewer(
            storedDraft(server.id, 'Draft from before'),
          ),
        );
        // Chosen against the forum's order: ops, then bug.
        await tester.tap(find.byKey(ValueKey('newChat.tag.${tagId(3).value}')));
        await tester.tap(find.byKey(ValueKey('newChat.tag.${tagId(1).value}')));
        await host.settle(tester);
        await type(tester, host, 'Restart the gateway');
        await pressEnter(tester, host);
        final all = await outbox(tester, host);
        final sent = creates(all);
        expect(sent, hasLength(1));
        final body = payloadOf(sent.single);
        expect(body['channel_id'], server.id.value);
        expect(body['title'], '');
        expect(body['content_md'], 'Restart the gateway');
        expect(body['tag_ids'], [tagId(1).value, tagId(3).value]);
        expect(body['clear_draft'], isTrue);
        expect(
          ofKind(all, ThreadOps.deleteChannelDraft),
          isEmpty,
          reason: 'D-M4-41-1: the server deletes the draft with the message',
        );
        expect(
          await host.run(tester, host.db.personalDao.allDrafts),
          isEmpty,
          reason: 'the stored copy of the draft goes with the send',
        );
        // The draft row becomes the row of the sent post (01 6.8 item 8).
        expect(draftRow(server), findsNothing);
        expect(sentRow(server, sent.single.id), findsOneWidget);
        expect(
          host.visited,
          contains(newChatPath(server, entry: sent.single.id)),
          reason: 'the router goes to the view of the sent post',
        );
        await host.dispose(tester);
      },
    );

    testWidgets(
      'after sending the view of the sent post stays until the server created it',
      (tester) async {
        final host = await pumpShell(tester, location: newChatPath(server));
        await type(tester, host, 'Restart the gateway');
        await pressEnter(tester, host);
        final entry = creates(await outbox(tester, host)).single.id;
        // The entry waits in the outbox (the engine is idle), so the view of the sent post
        // must stay with "Sending" (01 6.6, brief M4-41 requirement 14).
        expect(host.visited.last, newChatPath(server, entry: entry));
        expect(find.byKey(const ValueKey('newChat.sending')), findsOneWidget);
        await host.dispose(tester);
      },
    );

    testWidgets(
      'the sent view shows "Sending" and opens the post when the server created it',
      (tester) async {
        final created = CreatedPosts();
        final entry = sentPost();
        final host = await pumpShell(
          tester,
          location: newChatPath(server, entry: entry.id),
          created: created,
          more: (db) => storeSentPost(db, entry),
        );
        expect(host.visited.last, newChatPath(server, entry: entry.id));
        expect(find.byKey(const ValueKey('newChat.view')), findsOneWidget);
        expect(find.byKey(const ValueKey('newChat.sending')), findsOneWidget);
        expect(find.text(en.messageSending), findsWidgets);
        expect(find.byKey(const ValueKey('newChat.composer')), findsNothing);

        created.record(entry.id, threadId(9));
        await host.settle(tester);
        expect(host.visited.last, '/t/${threadId(9).value}');
        await host.dispose(tester);
      },
    );

    testWidgets('a refused post shows the notice with Try again', (
      tester,
    ) async {
      final entry = sentPost();
      final host = await pumpShell(
        tester,
        location: newChatPath(server, entry: entry.id),
        more: (db) async {
          await storeSentPost(db, entry);
          await db.outboxDao.recordFailure(
            entry.id,
            code: 'forbidden',
            reason: 'restricted tag',
          );
        },
      );
      expect(host.visited.last, newChatPath(server, entry: entry.id));
      expect(find.byKey(const ValueKey('newChat.view')), findsOneWidget);
      expect(find.byKey(const ValueKey('newChat.sending')), findsNothing);
      expect(
        find.text(en.postListCreateFailed(dsIsolate('Restart'))),
        findsOneWidget,
      );
      expect(
        find.text(en.postListFailureReason('restricted tag')),
        findsOneWidget,
      );
      final retry = find.byKey(ValueKey('postList.failed.retry.${entry.id}'));
      expect(retry, findsOneWidget);
      expect(
        find.descendant(of: retry, matching: find.text(en.postListTryAgain)),
        findsOneWidget,
      );
      await tester.tap(retry);
      await host.settle(tester);
      final again = creates(await outbox(tester, host));
      expect(again, hasLength(1));
      expect(again.single.state, isNot(OutboxState.failed));
      await host.dispose(tester);
    });
  });
}
