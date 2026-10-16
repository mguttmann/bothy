// SPDX-License-Identifier: Apache-2.0

import 'package:client/main.dart' as app;
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('the app starts with an empty MaterialApp without text', (
    tester,
  ) async {
    app.main();
    await tester.pump();
    expect(find.byType(MaterialApp), findsOneWidget);
    expect(find.byType(Text), findsNothing);
  });
}
