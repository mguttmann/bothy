// SPDX-License-Identifier: Apache-2.0

import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
// The import is the subject of the test: the library compiles and loads in a widget test.
// ignore: unused_import
import 'package:terminal_ui/terminal_ui.dart';

void main() {
  testWidgets('the library loads in a widget test', (tester) async {
    await tester.pumpWidget(const SizedBox());
    expect(find.byType(SizedBox), findsOneWidget);
  });
}
