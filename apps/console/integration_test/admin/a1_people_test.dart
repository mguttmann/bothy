// SPDX-License-Identifier: Apache-2.0

// A1 in the browser (brief M6-04 Design note 4): create a person, whose enrollment code is
// shown once with a copy action, and approve the waiting device of Sam that the runner
// registered through the API, with the typed verification code (08 5.2).

import 'package:console/console.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:patrol/patrol.dart';

import 'support.dart';

void main() {
  patrolTest('A1 create a person and approve a waiting device', ($) async {
    final seed = E2eSeed.read();
    final console = await openConsole($, '/people');

    await tapKey($, 'people.add');
    await typeKey($, 'createPerson.name', 'Robin');
    await typeKey($, 'createPerson.handle', 'robin');
    await tapKey($, 'confirm.submit');
    await waitFor($, byKey('code.dialog'));
    expect(byKey('code.copy'), findsOneWidget);
    await tapKey($, 'code.copy');
    await tapKey($, 'code.close');
    await waitGone($, byKey('code.dialog'));
    // The code is shown once: closing discards it.
    expect(find.textContaining('enrollment code is shown once'), findsNothing);
    await waitFor($, find.text('Robin'));

    console.open(ConsolePage.devices);
    final device = seed.pendingDevice.id;
    await tapKey($, 'waiting.$device.approve');
    await typeKey($, 'approveDevice.code', seed.pendingCode);
    await tapKey($, 'approveDevice.submit');
    await waitFor($, find.textContaining('is now active.'));
    await waitGone($, byKey('waiting.$device'));
    expectStepUp();
  });
}
