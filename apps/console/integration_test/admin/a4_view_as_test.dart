// SPDX-License-Identifier: Apache-2.0

// A4 in the browser (brief M6-04 Design note 3 and 4): start "View as person" for Sam from
// "Who has access?", see the banner and the structure (no content, ADR 0028), keep the banner
// on another page, and end the view with the button.

import 'package:console/console.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:patrol/patrol.dart';

import 'support.dart';

void main() {
  patrolTest('A4 view as Sam with the banner and the structure', ($) async {
    final seed = E2eSeed.read();
    final support = seed.space('support');
    await grantsAt($, 'space.${support.id}');
    await tapKey($, 'grants.access');
    await pick($, 'access.viewAs', 'access.viewAs.${seed.sam.id}');

    await waitFor($, byKey('viewAs.structure'));
    await waitFor($, byKey('viewAs.banner'));
    expect(find.textContaining('Viewing as Sam'), findsOneWidget);
    expect(byKey('viewAs.remaining'), findsOneWidget);
    expect(byKey('viewAs.space.${support.id}'), findsOneWidget);
    expect(find.textContaining('Engineering'), findsWidgets);
    expect(byKey('viewAs.structureOnly'), findsOneWidget);
    final console = controllerOf($);
    expect(console.path, '/people/view-as');

    console.open(ConsolePage.agents);
    await $.tester.pump(const Duration(milliseconds: 500));
    expect(byKey('viewAs.banner'), findsOneWidget);

    await tapKey($, 'viewAs.end');
    await waitGone($, byKey('viewAs.banner'));
  });
}
