// SPDX-License-Identifier: Apache-2.0

import 'package:console/console.dart';
import 'package:console/main.dart' as app;
import 'package:flutter/rendering.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  // ADR 0028 "Confirmation": semantics are on after main(), without a screen reader asking.
  testWidgets('main() turns semantics on', (tester) async {
    expect(SemanticsBinding.instance.semanticsEnabled, isFalse);
    app.main();
    await tester.pump();
    expect(SemanticsBinding.instance.semanticsEnabled, isTrue);
    await tester.pumpAndSettle();
    releaseConsoleSemantics();
  }, semanticsEnabled: false);
}
