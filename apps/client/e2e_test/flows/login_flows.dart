// SPDX-License-Identifier: Apache-2.0

// Login flows against the real server (brief M4-34 product requirement 13): the enrollment login
// with device keys and DPoP, and a waiting device that the first device of Alex confirms.

import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/e2e_app.dart';
import '../support/e2e_context.dart';

/// Registers the login flows.
void loginFlows(E2eContext c) {
  testWidgets('enrollment login with device keys and DPoP', (tester) async {
    final app = await E2eApp.start(tester, c.world);
    try {
      await app.connect();
      await app.enroll(await c.alexCode());
      await app.waitFor(find.byKey(const ValueKey('column.sidebar')));
      await app.waitLive();
      await app.waitFor(
        find.byKey(ValueKey('sidebar.channel.${c.world.generalId}')),
      );
      await app.waitFor(
        find.byKey(ValueKey('sidebar.channel.${c.world.forumId}')),
      );
    } finally {
      await app.stop();
    }
  });
}
