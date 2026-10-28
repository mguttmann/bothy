// SPDX-License-Identifier: Apache-2.0

// Finders and steps of the new chat view of a forum (brief M4-41) for the integration tests:
// the title field, the composer's text field and its send button.

import 'package:design_system/design_system.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:l10n/l10n.dart';

/// The English texts the finders match.
final AppLocalizations newChatEn = lookupAppLocalizations(const Locale('en'));

/// The new chat view.
final Finder newChatView = find.byKey(const ValueKey('newChat.view'));

/// The title field of the view.
final Finder newChatTitle = find.byKey(const ValueKey('newChat.title'));

/// The text field of the view's composer.
final Finder newChatField = find.descendant(
  of: find.byKey(const ValueKey('newChat.composer')),
  matching: find.byType(EditableText),
);

/// The send button of the view's composer.
final Finder newChatSend = find.descendant(
  of: find.byKey(const ValueKey('newChat.composer')),
  matching: find.byWidgetPredicate(
    (w) => w is DsTappable && w.label == newChatEn.composerSend,
  ),
);

/// Whether the send button of the view's composer reacts to a tap.
///
/// Returns false while the view or its button is not shown.
bool newChatSendEnabled() {
  final found = newChatSend.evaluate();
  if (found.isEmpty) return false;
  return (found.single.widget as DsTappable).onTap != null;
}

/// The sidebar row of [forum]'s draft of a new post.
Finder draftRow(String forum) =>
    find.byKey(ValueKey('sidebar.newChat.$forum.draft'));

/// The sidebar rows of [forum]'s posts sent from this device that the server has not created
/// yet (key `sidebar.newChat.<forum>.<outbox entry>`).
Finder sentRows(String forum) => find.byWidgetPredicate((w) {
  final key = w.key;
  return key is ValueKey<String> &&
      key.value.startsWith('sidebar.newChat.$forum.') &&
      key.value != 'sidebar.newChat.$forum.draft';
});

/// The sidebar row of the post [thread].
Finder threadRow(String thread) =>
    find.byKey(ValueKey('sidebar.thread.$thread'));

/// The location of the app's router, for example `/t/<id>`.
String currentRoute(WidgetTester tester) =>
    GoRouter.of(tester.element(find.byType(Navigator).first))
        .routerDelegate
        .currentConfiguration
        .uri
        .toString();
