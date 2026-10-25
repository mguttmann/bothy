// SPDX-License-Identifier: Apache-2.0

// F6 and Shift+F6 between the columns (brief M4-24 Tests; Design note 3; 01 7 and 8 "Focus"):
// they cycle through the visible columns in the order sidebar, list, history, session, skip
// collapsed ones, land on the first item of a column in reading order, and the focus ring is
// painted around the focused item (components.md: focus is always visible).

import 'package:client/app/layout/column_layout.dart';
import 'package:client/app/layout/column_widths.dart';
import 'package:client/keyboard/app_shortcuts.dart';
import 'package:client/keyboard/bindings.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/keys.dart';
import '../support/widgets.dart';

const KeyStroke f6 = KeyStroke(LogicalKeyboardKey.f6);
const KeyStroke shiftF6 = KeyStroke(LogicalKeyboardKey.f6, shift: true);

/// A column with two buttons, the first one above the second.
Widget column(String name, {bool onSidebar = false}) => Column(
  children: [
    for (final i in [1, 2])
      DsTappable(
        key: ValueKey('$name.$i'),
        onTap: () {},
        onSidebar: onSidebar,
        label: '$name $i',
        child: Text('$name $i'),
      ),
  ],
);

Widget shell({bool postList = true, bool session = true}) => hostScreen(
  AppShortcuts(
    platform: ShortcutPlatform.windows,
    child: ColumnLayout(
      widths: ColumnWidths.defaults,
      minWidthFourColumns: 1200,
      minWidthThreeColumns: 900,
      onWidthsChanged: (_) {},
      showPostList: postList,
      showSession: session,
      sidebar: column(AppColumn.sidebar, onSidebar: true),
      postList: column(AppColumn.postList),
      history: column(AppColumn.history),
      session: column(AppColumn.session),
    ),
  ),
);

/// The key of the focused [DsTappable].
String? focused(WidgetTester tester) {
  final context = FocusManager.instance.primaryFocus?.context;
  final key = context?.findAncestorWidgetOfExactType<DsTappable>()?.key;
  return key is ValueKey<String> ? key.value : null;
}

void main() {
  testWidgets('F6 and Shift+F6 cycle through the four columns', (tester) async {
    desktopWindow(tester);
    await tester.pumpWidget(shell());
    final order = [
      'sidebar.1',
      'post_list.1',
      'history.1',
      'session.1',
      'sidebar.1',
    ];
    for (final expected in order) {
      expect(await pressStroke(tester, f6), isTrue);
      expect(focused(tester), expected);
    }
    for (final expected in ['session.1', 'history.1', 'post_list.1']) {
      await pressStroke(tester, shiftF6);
      expect(focused(tester), expected);
    }
  });

  testWidgets('collapsed columns are skipped', (tester) async {
    desktopWindow(tester);
    await tester.pumpWidget(shell(postList: false, session: false));
    final seen = <String?>[];
    for (var i = 0; i < 4; i++) {
      await pressStroke(tester, f6);
      seen.add(focused(tester));
    }
    expect(seen, ['sidebar.1', 'history.1', 'sidebar.1', 'history.1']);
    // A window too narrow for the session column collapses it too.
    tester.view.physicalSize = const Size(1000, 1000);
    await tester.pumpWidget(shell());
    await tester.pump();
    final narrow = <String?>[];
    for (var i = 0; i < 3; i++) {
      await pressStroke(tester, f6);
      narrow.add(focused(tester));
    }
    expect(narrow, isNot(contains('session.1')));
  });

  testWidgets('Tab goes on in reading order inside a column', (tester) async {
    desktopWindow(tester);
    await tester.pumpWidget(shell());
    await pressStroke(tester, f6);
    expect(focused(tester), 'sidebar.1');
    await pressStroke(tester, const KeyStroke(LogicalKeyboardKey.tab));
    expect(focused(tester), 'sidebar.2');
  });

  testWidgets('the focus ring is painted around the focused item', (
    tester,
  ) async {
    desktopWindow(tester);
    await tester.pumpWidget(shell());
    await pressStroke(tester, f6);
    await tester.pump();
    BoxBorder? ringOf(String key) {
      final box = tester.widget<DecoratedBox>(
        find
            .descendant(
              of: find.byKey(ValueKey(key)),
              matching: find.byType(DecoratedBox),
            )
            .first,
      );
      return (box.decoration as BoxDecoration).border;
    }

    final sidebarContext = tester.element(
      find.byKey(const ValueKey('sidebar.1')),
    );
    final ring = ringOf('sidebar.1') as Border?;
    expect(ring, isNotNull);
    expect(ring!.top.color, sidebarContext.dsColors.focusRingOnSidebar);
    expect(ring.top.width, DsBorderWidth.focus);
    expect(ringOf('sidebar.2'), isNull);
    await pressStroke(tester, f6);
    await tester.pump();
    final listRing = ringOf('post_list.1') as Border?;
    expect(listRing?.top.color, sidebarContext.dsAccent.focusRing);
  });
}
