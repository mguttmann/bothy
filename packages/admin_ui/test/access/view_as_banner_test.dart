// SPDX-License-Identifier: Apache-2.0

// The banner and the structure of the stored view in the console (brief M6-04 fixed
// requirements 13 and 14; 07 9.2): name and remaining time, an end button, the spaces and
// channels the person sees as text only, the spaces the owner sees and the person does not,
// and never any content.

import 'package:admin_ui/admin_ui.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../admin/support.dart';

ViewAsItem _space(String id, String name) => ViewAsItem(
  id: id,
  name: name,
  level: AccessLevel.read,
  capabilities: const [],
);

ViewAsItem _channel(String id, String name, String? spaceId) => ViewAsItem(
  id: id,
  name: name,
  level: AccessLevel.trigger,
  capabilities: const [AccessCapability.upload],
  spaceId: spaceId,
);

ViewAsResult _snapshot({
  List<ViewAsItem> spaces = const [],
  List<ViewAsItem> channels = const [],
}) => ViewAsResult(
  user: const AccessPrincipal(
    id: 'sam',
    kind: 'user',
    handle: 'sam',
    displayName: 'Sam',
  ),
  expiresAt: DateTime.utc(2026, 10, 17, 9, 15),
  spaces: spaces,
  channels: channels,
);

void main() {
  final now = DateTime.utc(2026, 10, 17, 8);

  Future<void> pumpBanner(
    WidgetTester tester,
    DateTime endsAt, {
    VoidCallback? onEnd,
  }) => pumpInApp(
    tester,
    ViewAsBanner(name: 'Sam', endsAt: endsAt, now: now, onEnd: onEnd),
  );

  String remaining(WidgetTester tester) =>
      tester.widget<Text>(find.byKey(const ValueKey('viewAs.remaining'))).data!;

  testWidgets('the banner names the person and the time left', (tester) async {
    await pumpBanner(
      tester,
      now.add(const Duration(minutes: 14, seconds: 5)),
      onEnd: () {},
    );

    expect(
      tester.widget<Text>(find.byKey(const ValueKey('viewAs.banner'))).data,
      'Viewing as Sam (read-only). Nothing is sent. Logged.',
    );
    expect(remaining(tester), '14:05 left');
    final live = tester.widget<Semantics>(
      find
          .ancestor(
            of: find.byKey(const ValueKey('viewAs.banner')),
            matching: find.byType(Semantics),
          )
          .first,
    );
    expect(live.properties.liveRegion, isTrue);
  });

  testWidgets('minutes go past 59', (tester) async {
    await pumpBanner(tester, now.add(const Duration(minutes: 75, seconds: 3)));

    expect(remaining(tester), '75:03 left');
  });

  testWidgets('an end in the past shows zero', (tester) async {
    await pumpBanner(tester, now.subtract(const Duration(seconds: 30)));

    expect(remaining(tester), '00:00 left');
  });

  testWidgets('the end button calls the host', (tester) async {
    var ends = 0;
    await pumpBanner(
      tester,
      now.add(const Duration(minutes: 1)),
      onEnd: () => ends++,
    );

    await tester.tap(find.byKey(const ValueKey('viewAs.end')));
    expect(ends, 1);
    expect(find.text('End view'), findsOneWidget);
  });

  testWidgets('without a callback the end button is disabled', (tester) async {
    await pumpBanner(tester, now.add(const Duration(minutes: 1)));

    final end = tester.widget<TextButton>(
      find.byKey(const ValueKey('viewAs.end')),
    );
    expect(end.onPressed, isNull);
  });

  testWidgets('the banner spans the full width', (tester) async {
    await pumpBanner(tester, now.add(const Duration(minutes: 1)));

    // The test app is 1200 wide with a padding of 24.
    expect(tester.getSize(find.byType(ViewAsBanner)).width, 1200 - 2 * 24);
  });

  Future<void> pumpStructure(
    WidgetTester tester,
    ViewAsResult? snapshot, {
    List<String> notVisible = const [],
  }) => pumpInApp(
    tester,
    ViewAsStructure(name: 'Sam', snapshot: snapshot, notVisible: notVisible),
  );

  testWidgets('the structure lists each space with its channels below', (
    tester,
  ) async {
    await pumpStructure(
      tester,
      _snapshot(
        spaces: [_space('marketing', 'Marketing'), _space('sales', 'Sales')],
        channels: [
          _channel('offers', 'Offers', 'sales'),
          _channel('campaigns', 'Kampagnen', 'marketing'),
          _channel('stray', 'Stray', 'hidden'),
          _channel('loose', 'Loose', null),
        ],
      ),
    );

    expect(find.text('Sidebar as Sam sees it'), findsOneWidget);
    double top(String key) => tester.getTopLeft(find.byKey(ValueKey(key))).dy;
    // The row spans the width; its first text shows the indent.
    double left(String key) => tester
        .getTopLeft(
          find
              .descendant(
                of: find.byKey(ValueKey(key)),
                matching: find.byType(Text),
              )
              .first,
        )
        .dx;
    final order = [
      'viewAs.space.marketing',
      'viewAs.channel.campaigns',
      'viewAs.space.sales',
      'viewAs.channel.offers',
      'viewAs.channel.stray',
      'viewAs.channel.loose',
    ];
    for (var i = 1; i < order.length; i++) {
      expect(top(order[i]), greaterThan(top(order[i - 1])), reason: order[i]);
    }
    expect(
      left('viewAs.channel.campaigns'),
      greaterThan(left('viewAs.space.marketing')),
    );
    final space = find.byKey(const ValueKey('viewAs.space.marketing'));
    expect(
      find.descendant(of: space, matching: find.text('Marketing')),
      findsOneWidget,
    );
    expect(
      find.descendant(of: space, matching: find.text('Read')),
      findsOneWidget,
    );
    expect(
      find.descendant(
        of: find.byKey(const ValueKey('viewAs.channel.campaigns')),
        matching: find.text('Kampagnen'),
      ),
      findsOneWidget,
    );
    // A channel whose space is not listed carries the path it can.
    expect(
      find.descendant(
        of: find.byKey(const ValueKey('viewAs.channel.stray')),
        matching: find.text('Stray'),
      ),
      findsOneWidget,
    );
    expect(find.byKey(const ValueKey('viewAs.structureOnly')), findsOneWidget);
    expect(
      find.text(
        'The console shows structure and access only, never the content of '
        'a channel.',
      ),
      findsOneWidget,
    );
    expect(find.byKey(const ValueKey('viewAs.notVisible')), findsNothing);
    // Text only: nothing in the structure can be tapped.
    expect(find.byType(InkWell), findsNothing);
    expect(find.byType(GestureDetector), findsNothing);
  });

  testWidgets('the spaces the person does not see are named', (tester) async {
    await pumpStructure(
      tester,
      _snapshot(spaces: [_space('marketing', 'Marketing')]),
      notVisible: const ['Board', 'Sales'],
    );

    expect(
      tester.widget<Text>(find.byKey(const ValueKey('viewAs.notVisible'))).data,
      'Not visible for Sam: Board, Sales',
    );
  });

  testWidgets('an empty snapshot says nothing is visible', (tester) async {
    await pumpStructure(tester, _snapshot());

    expect(find.text('Nothing visible.'), findsOneWidget);
    expect(find.byKey(const ValueKey('viewAs.structureOnly')), findsOneWidget);
  });

  testWidgets('without a snapshot only the later note shows', (tester) async {
    await pumpStructure(tester, null, notVisible: const ['Sales']);

    expect(find.byKey(const ValueKey('viewAs.later')), findsOneWidget);
    expect(
      find.text(
        'The structure appears when a view starts. End this view and start '
        'it again to see it.',
      ),
      findsOneWidget,
    );
    expect(find.text('Sidebar as Sam sees it'), findsNothing);
    expect(find.byKey(const ValueKey('viewAs.notVisible')), findsNothing);
    expect(find.byKey(const ValueKey('viewAs.structureOnly')), findsNothing);
  });

  test('notVisibleSpaces names the owner spaces missing from the view', () {
    const targets = [
      AdminTarget(AdminTargetKind.instance),
      AdminTarget(AdminTargetKind.space, id: 'sales', name: 'Sales'),
      AdminTarget(AdminTargetKind.space, id: 'marketing', name: 'Marketing'),
      AdminTarget(
        AdminTargetKind.channel,
        id: 'offers',
        name: 'Offers',
        spaceName: 'Sales',
      ),
      AdminTarget(AdminTargetKind.space, id: 'board', name: 'board'),
      AdminTarget(AdminTargetKind.host, id: 'h1', name: 'Build'),
    ];
    final snapshot = _snapshot(
      spaces: [_space('marketing', 'Marketing')],
      channels: [_channel('offers', 'Offers', 'sales')],
    );

    expect(notVisibleSpaces(targets, snapshot), ['board', 'Sales']);
    expect(
      notVisibleSpaces(
        targets,
        _snapshot(
          spaces: [
            _space('marketing', 'Marketing'),
            _space('sales', 'sales'),
            _space('board', 'Board'),
          ],
        ),
      ),
      isEmpty,
    );
  });
}
