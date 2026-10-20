// SPDX-License-Identifier: Apache-2.0

// The "New post" dialog (brief M4-19 Design note 3): title and first message are required,
// and every tag of the forum is offered; whether the person may set a restricted tag is the
// server's decision (15 1 principle 1).

import 'package:client/forum/new_post_dialog.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/structure.dart';
import '../support/widgets.dart';

void main() {
  testWidgets('every tag is offered, a restricted one too', (tester) async {
    final restricted = ForumTag(
      id: tagId(3),
      channelId: channelId(1),
      name: 'release',
      sortKey: 'a3',
      restricted: true,
    );
    NewPost? result;
    desktopWindow(tester);
    await tester.pumpWidget(
      hostScreen(
        Builder(
          builder: (context) => TextButton(
            onPressed: () async => result = await showNewPostDialog(context, [
              tag(1, 'bug'),
              restricted,
            ]),
            child: const Text('open'),
          ),
        ),
      ),
    );
    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();
    expect(
      find.byKey(ValueKey('newPost.tag.${tagId(1).value}')),
      findsOneWidget,
    );
    expect(
      find.byKey(ValueKey('newPost.tag.${tagId(3).value}')),
      findsOneWidget,
    );
    await tester.enterText(
      find.byKey(const ValueKey('newPost.title')),
      'Ship it',
    );
    await tester.enterText(
      find.byKey(const ValueKey('newPost.message')),
      'Version 2 is ready.',
    );
    await tester.tap(find.byKey(ValueKey('newPost.tag.${tagId(3).value}')));
    await tester.pump();
    await tester.tap(find.byKey(const ValueKey('newPost.create')));
    await tester.pumpAndSettle();
    expect(result?.title, 'Ship it');
    expect(result?.tagIds, [tagId(3)]);
  });
}
