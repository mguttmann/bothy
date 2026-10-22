// SPDX-License-Identifier: Apache-2.0

// A3 in the browser (brief M6-04 Design note 4): "Who has access?" for the space Support shows
// the owner Alex and Sam with their sources.

import 'package:flutter_test/flutter_test.dart';
import 'package:patrol/patrol.dart';

import 'support.dart';

void main() {
  patrolTest('A3 Who has access for Support', ($) async {
    final seed = E2eSeed.read();
    await grantsAt($, 'space.${seed.space('support').id}');
    await tapKey($, 'grants.access');
    await waitFor($, byKey('access.entry.${seed.sam.id}'));
    expect(byKey('access.entry.${seed.owner.id}'), findsOneWidget);
    expect(byKey('access.viewAs'), findsOneWidget);
  });
}
